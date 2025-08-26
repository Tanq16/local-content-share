package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed templates/* static/*
var content embed.FS

// SSE client management
var (
	clients   = make(map[chan string]bool)
	clientMux sync.Mutex
)

type Entry struct {
	ID       string
	Content  string
	Type     string
	Filename string
}

type ExpirationTracker struct {
	Expirations map[string]time.Time `json:"expirations"`
	mu          sync.Mutex           // mutex for thread safety
}

var expirationTracker *ExpirationTracker
var expirationOptions = []string{"Never", "1 hour", "4 hours", "1 day", "Custom"}

func initExpirationTracker() *ExpirationTracker {
	tracker := &ExpirationTracker{
		Expirations: make(map[string]time.Time),
	}
	// Load existing expirations from file
	expirationFile := filepath.Join("data", "expirations.json")
	if _, err := os.Stat(expirationFile); err == nil {
		data, err := os.ReadFile(expirationFile)
		if err == nil {
			var storedTracker ExpirationTracker
			if err := json.Unmarshal(data, &storedTracker); err == nil {
				tracker.Expirations = storedTracker.Expirations
			}
		}
	}
	return tracker
}

func parseCustomDuration(customExpiry string) time.Duration {
	customExpiry = strings.TrimSpace(customExpiry)
	// Regex to match the format like 1h, 30m, 2d, etc.
	re := regexp.MustCompile(`^(\d+)([hmMdwy])$`)
	matches := re.FindStringSubmatch(customExpiry)
	if len(matches) < 2 { // bad value
		return 5 * time.Minute
	}
	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 5 * time.Minute
	}
	unit := strings.ToLower(matches[2])
	switch unit {
	case "m": // minutes
		if value < 5 {
			return 5 * time.Minute
		}
		return time.Duration(value) * time.Minute
	case "h": // hours
		return time.Duration(value) * time.Hour
	case "d": // days
		return time.Duration(value) * 24 * time.Hour
	case "w": // weeks
		return time.Duration(value) * 7 * 24 * time.Hour
	case "M": // months
		return time.Duration(value) * 30 * 24 * time.Hour
	case "y": // years
		return time.Duration(value) * 365 * 24 * time.Hour
	default:
		return 5 * time.Minute
	}
}

func (t *ExpirationTracker) SetExpiration(fileID, expiryOption string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if expiryOption == "Never" {
		delete(t.Expirations, fileID)
	} else {
		var duration time.Duration
		switch expiryOption {
		case "1 hour":
			duration = 1 * time.Hour
		case "4 hours":
			duration = 4 * time.Hour
		case "1 day":
			duration = 24 * time.Hour
		case "Custom":
			// Should not happen anymore.
			return
		default:
			if len(expiryOption) > 0 {
				duration = parseCustomDuration(expiryOption)
			} else {
				delete(t.Expirations, fileID)
				return
			}
		}
		t.Expirations[fileID] = time.Now().Add(duration)
	}
	t.saveToFile()
}

func (t *ExpirationTracker) saveToFile() {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		log.Printf("Error marshaling expirations: %v", err)
		return
	}
	expirationFile := filepath.Join("data", "expirations.json")
	if err := os.WriteFile(expirationFile, data, 0644); err != nil {
		log.Printf("Error saving expirations: %v", err)
	}
}

func (t *ExpirationTracker) CleanupExpired() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	var expiredFiles []string
	// Find expired files
	for fileID, expiryTime := range t.Expirations {
		if now.After(expiryTime) {
			expiredFiles = append(expiredFiles, fileID)
		}
	}
	// Delete expired files
	for _, fileID := range expiredFiles {
		err := os.Remove(filepath.Join("data", fileID))
		if err != nil && !os.IsNotExist(err) {
			log.Printf("Error removing expired file %s: %v", fileID, err)
		} else {
			log.Printf("Removed expired file: %s", fileID)
		}
		delete(t.Expirations, fileID)
	}
	if len(expiredFiles) > 0 {
		t.saveToFile()
		notifyContentChange()
	}
	return expiredFiles
}

var listenAddress = flag.String("listen", ":8080", "host:port in which the server will listen")

// Placeholder content for notepad files
const mdPlaceholder = `# Welcome to Markdown Notepad

Start typing your markdown here...

## Features

- **Bold** and *italic* text
- [Links](https://example.com)
- Lists (ordered and unordered)
- Code blocks
- And more!

` + "```" + `
function example() {
  console.log("Hello, Markdown!");
}
` + "```"

func generateUniqueFilename(baseDir, baseName string) string {
	baseName = strings.TrimSpace(baseName)
	// Sanitize: allow only letters (+unicode), numbers, space, dot, hyphen, underscore, () and []
	reg := regexp.MustCompile(`[^\p{L}\p{N}\p{M}\s\.\-_()\[\]]`)
	sanitizedName := reg.ReplaceAllString(baseName, "-")
	log.Printf("Sanitized name %s TO %s\n", baseName, sanitizedName)
	// First try without random prefix
	if _, err := os.Stat(filepath.Join(baseDir, sanitizedName)); os.IsNotExist(err) {
		return sanitizedName
	}
	// If file exists, add random prefix until we find a unique name
	for {
		randChars := fmt.Sprintf("%04d", rand.Intn(10000))
		newName := fmt.Sprintf("%s-%s", randChars, sanitizedName)
		if _, err := os.Stat(filepath.Join(baseDir, newName)); os.IsNotExist(err) {
			return newName
		}
	}
}

func handleContentUpdates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	messageChan := make(chan string)
	clientMux.Lock()
	clients[messageChan] = true
	clientMux.Unlock()

	defer func() {
		clientMux.Lock()
		delete(clients, messageChan)
		clientMux.Unlock()
		close(messageChan)
	}()
	// Send an initial message
	fmt.Fprintf(w, "data: %s\n\n", "connected")
	w.(http.Flusher).Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		}
	}
}

func notifyContentChange() {
	clientMux.Lock()
	defer clientMux.Unlock()
	for client := range clients {
		select {
		case client <- "content_updated":
		default:
		}
	}
}

func main() {
	flag.Parse()

	if err := os.MkdirAll(filepath.Join("data", "files"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("data", "text"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("data", "notepad"), 0755); err != nil {
		log.Fatal(err)
	}
	log.Println("Data directory created/reused without errors.")
	createFileIfNotExists("notepad/md.file", mdPlaceholder)
	createFileIfNotExists("links.file", "")

	// Initialize the expiration tracker
	expirationTracker = initExpirationTracker()
	customExpiry := os.Getenv("DEFAULT_EXPIRY")
	if customExpiry != "" {
		if customExpiry == "1d" {
			expirationOptions = []string{"1 day", "Never", "1 hour", "4 hours", "Custom"}
		} else if customExpiry == "4h" {
			expirationOptions = []string{"4 hours", "Never", "1 hour", "1 day", "Custom"}
		} else if customExpiry == "1h" {
			expirationOptions = []string{"1 hour", "Never", "4 hours", "1 day", "Custom"}
		} else {
			expirationOptions = append([]string{customExpiry}, expirationOptions...)
		}
	}

	// Goroutine to periodically expire files
	go func() {
		ticker := time.NewTicker(3 * time.Minute) // 3 minutes is sparse enough, load is extremely minimal as the operation is fast (in memory tracker)
		defer ticker.Stop()
		for range ticker.C {
			expirationTracker.CleanupExpired()
		}
	}()

	tmpl := template.Must(template.ParseFS(content, "templates/*.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Clean up expired files on page load
		expirationTracker.CleanupExpired()
		entries := []Entry{}
		// Read text snippets
		textFiles, _ := os.ReadDir(filepath.Join("data", "text"))
		for _, file := range textFiles {
			if file.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join("data", "text", file.Name()))
			if err != nil {
				continue
			}
			entries = append(entries, Entry{
				ID:       filepath.Join("text", file.Name()),
				Type:     "text",
				Content:  string(data),
				Filename: file.Name(),
			})
		}
		// Read files
		files, _ := os.ReadDir(filepath.Join("data", "files"))
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			entries = append(entries, Entry{
				ID:       filepath.Join("files", file.Name()),
				Type:     "file",
				Filename: file.Name(),
			})
		}
		// Read links
		data, err := os.ReadFile(filepath.Join("data", "links.file"))
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				entries = append(entries, Entry{
					ID:       "link/" + url.QueryEscape(line),
					Type:     "link",
					Content:  line,
					Filename: line,
				})
			}
		}
		tmpl.ExecuteTemplate(w, "index.html", entries)
	})

	http.HandleFunc("/md", func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "md.html", nil)
	})

	// Retrieve custom expiration options
	http.HandleFunc("/getExpiryOptions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expirationOptions)
	})

	// Serve static files from embedded filesystem
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub-filesystem: %v", err)
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFS.Open("style.css")
		if err != nil {
			http.Error(w, "Style not found", http.StatusNotFound)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "text/css")
		io.Copy(w, file)
	})

	http.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFS.Open("manifest.json")
		if err != nil {
			http.Error(w, "Manifest not found", http.StatusNotFound)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, file)
	})

	http.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFS.Open("sw.js")
		if err != nil {
			http.Error(w, "Service worker not found", http.StatusNotFound)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "application/javascript")
		io.Copy(w, file)
	})

	http.HandleFunc("/md.js", func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFS.Open("md.js")
		if err != nil {
			http.Error(w, "JavaScript not found", http.StatusNotFound)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "application/javascript")
		io.Copy(w, file)
	})

	// Handle favicon and icons
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFS.Open("favicon.ico")
		if err != nil {
			http.Error(w, "Favicon not found", http.StatusNotFound)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "image/x-icon")
		io.Copy(w, file)
	})

	http.HandleFunc("/icon-192.png", func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFS.Open("icon-192.png")
		if err != nil {
			http.Error(w, "Icon not found", http.StatusNotFound)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "image/png")
		io.Copy(w, file)
	})

	http.HandleFunc("/icon-512.png", func(w http.ResponseWriter, r *http.Request) {
		file, err := staticFS.Open("icon-512.png")
		if err != nil {
			http.Error(w, "Icon not found", http.StatusNotFound)
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", "image/png")
		io.Copy(w, file)
	})

	// API endpoint to load notepad content
	http.HandleFunc("/notepad/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			filename := strings.TrimPrefix(r.URL.Path, "/notepad/")
			if filename != "md.file" { // && filename != "rtext.file" {
				http.Error(w, "Invalid notepad file", http.StatusBadRequest)
				return
			}
			content, err := os.ReadFile(filepath.Join("data", "notepad", filename))
			if err != nil {
				http.Error(w, "Error reading notepad file", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write(content)
			return
		} else if r.Method == "POST" {
			filename := strings.TrimPrefix(r.URL.Path, "/notepad/")
			if filename != "md.file" { // && filename != "rtext.file" {
				http.Error(w, "Invalid notepad file", http.StatusBadRequest)
				return
			}
			content, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Error reading request body", http.StatusInternalServerError)
				return
			}
			err = os.WriteFile(filepath.Join("data", "notepad", filename), content, 0644)
			if err != nil {
				http.Error(w, "Error saving notepad file", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Saved"))
			log.Printf("Saved notepad content to %s\n", filename)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(100 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entryType := r.FormValue("type")
		expiryOption := r.FormValue("expiry")
		content := r.FormValue("content")
		name := r.FormValue("name")
		if entryType == "link" {
			// Handle link submission
			if content == "" {
				http.Error(w, "URL content cannot be empty", http.StatusBadRequest)
				return
			}
			u, err := url.ParseRequestURI(content)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				http.Error(w, "Invalid URL format. Must start with http:// or https://", http.StatusBadRequest)
				return
			}
			linksFilePath := filepath.Join("data", "links.file")
			f, err := os.OpenFile(linksFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer f.Close()
			if _, err := f.WriteString(content + "\n"); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			log.Printf("Saved link %s\n", content)
		} else {
			// Handle file and text submission
			files := r.MultipartForm.File["file-upload"]
			if len(files) > 0 {
				// File submission
				for _, fileHeader := range files {
					err := func() error {
						file, err := fileHeader.Open()
						if err != nil {
							return err
						}
						defer file.Close()
						fileName := name
						if fileName == "" {
							fileName = fileHeader.Filename
						}
						uniqueFileName := generateUniqueFilename("data/files", fileName)
						f, err := os.Create(filepath.Join("data/files", uniqueFileName))
						if err != nil {
							return err
						}
						defer f.Close()
						if _, err := io.Copy(f, file); err != nil {
							return err
						}
						if expiryOption != "Never" {
							fileID := filepath.Join("files", uniqueFileName)
							expirationTracker.SetExpiration(fileID, expiryOption)
						}
						log.Printf("Saved file %s with expiry %s\n", uniqueFileName, expiryOption)
						return nil
					}()
					if err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
			} else if content != "" {
				// Text snippet submission
				filename := name
				if filename == "" {
					filename = time.Now().Format("Jan-02 15-04-05")
				}
				uniqueFileName := generateUniqueFilename("data/text", filename)
				err := os.WriteFile(filepath.Join("data/text", uniqueFileName), []byte(content), 0644)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if expiryOption != "Never" {
					fileID := filepath.Join("text", uniqueFileName)
					expirationTracker.SetExpiration(fileID, expiryOption)
				}
				log.Printf("Saved text snippet %s with expiry %s\n", uniqueFileName, expiryOption)
			}
		}
		notifyContentChange()
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/rename/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		oldPath := strings.TrimPrefix(r.URL.Path, "/rename/")
		newName := r.FormValue("newname")
		if newName == "" {
			http.Error(w, "New name cannot be empty", http.StatusBadRequest)
			return
		}
		baseDir := filepath.Dir(filepath.Join("data", oldPath))
		newName = generateUniqueFilename(baseDir, newName)

		// Get the new full path
		newPath := filepath.Join(baseDir, newName)
		oldFullPath := filepath.Join("data", oldPath)
		// Check if there's an expiration for this file
		expirationTracker.mu.Lock()
		expiryTime, hasExpiry := expirationTracker.Expirations[oldPath]
		if hasExpiry {
			// Remove old entry and add new one
			delete(expirationTracker.Expirations, oldPath)
			relNewPath := strings.TrimPrefix(newPath, "data/")
			relNewPath = strings.ReplaceAll(relNewPath, "\\", "/") // Ensure cross-platform path separators
			expirationTracker.Expirations[relNewPath] = expiryTime
			expirationTracker.saveToFile()
		}
		expirationTracker.mu.Unlock()
		// Rename the file
		err := os.Rename(oldFullPath, newPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		notifyContentChange()
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Printf("Renamed %s to %s\n", oldPath, newName)
	})

	http.HandleFunc("/raw/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/raw/")
		if !strings.HasPrefix(id, "text/") {
			http.Error(w, "Only text files can be accessed", http.StatusBadRequest)
			return
		}
		content, err := os.ReadFile(filepath.Join("data", id))
		if err != nil {
			http.Error(w, "File not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(content)
	})

	http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/download/")
		filePath := filepath.Join("data", filename)
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		file, err := os.Open(filePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Brute force method to determine content type
		ext := strings.ToLower(filepath.Ext(filename))
		var contentType string
		switch ext {
		case ".pdf":
			contentType = "application/pdf"
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".svg":
			contentType = "image/svg+xml"
		default:
			buffer := make([]byte, 512)
			_, err = file.Read(buffer)
			if err != nil && err != io.EOF {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			contentType = http.DetectContentType(buffer)
			_, err = file.Seek(0, 0)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		baseFilename := filepath.Base(filename)
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", baseFilename))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, err = io.Copy(w, file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("Served %s for download\n", filename)
	})

	http.HandleFunc("/view/", func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/view/")
		http.ServeFile(w, r, filepath.Join("data", filename))
		log.Printf("Served %s for viewing\n", filename)
	})

	http.HandleFunc("/delete/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/delete/")
		// Handle link deletion
		if strings.HasPrefix(id, "link/") {
			linkToDelete, err := url.QueryUnescape(strings.TrimPrefix(id, "link/"))
			if err != nil {
				http.Error(w, "Invalid link format for deletion", http.StatusBadRequest)
				return
			}
			linksFilePath := filepath.Join("data", "links.file")
			data, err := os.ReadFile(linksFilePath)
			if err != nil {
				http.Error(w, "Failed to read links file for deletion", http.StatusInternalServerError)
				return
			}
			lines := strings.Split(string(data), "\n")
			var newLines []string
			var found bool
			for _, line := range lines {
				if strings.TrimSpace(line) == strings.TrimSpace(linkToDelete) && !found {
					found = true // Remove only the first occurrence
					continue
				}
				if strings.TrimSpace(line) != "" {
					newLines = append(newLines, line)
				}
			}
			output := strings.Join(newLines, "\n")
			// Add newline for correctness
			if output != "" {
				output += "\n"
			}
			err = os.WriteFile(linksFilePath, []byte(output), 0644)
			if err != nil {
				http.Error(w, "Failed to write links file after deletion", http.StatusInternalServerError)
				return
			}
			notifyContentChange()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "ok"}`))
			log.Printf("Deleted link %s\n", linkToDelete)
			return
		}
		// Handle file and snippet deletion
		err := os.Remove(filepath.Join("data", id))
		if err != nil {
			log.Printf("Failed to delete %s: %v", id, err)
			http.Error(w, "Failed to delete file", http.StatusInternalServerError)
			return
		}
		expirationTracker.mu.Lock()
		delete(expirationTracker.Expirations, id)
		expirationTracker.saveToFile()
		expirationTracker.mu.Unlock()
		notifyContentChange()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
		log.Printf("Deleted %s\n", id)
	})

	http.HandleFunc("/edit/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/edit/")
		if !strings.HasPrefix(id, "text/") {
			http.Error(w, "Can only edit text snippets", http.StatusBadRequest)
			return
		}
		content := r.FormValue("content")
		if content == "" {
			http.Error(w, "Content cannot be empty", http.StatusBadRequest)
			return
		}
		err := os.WriteFile(filepath.Join("data", id), []byte(content), 0644)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		notifyContentChange()
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Printf("Edited %s\n", id)
	})

	// SSE Updates for content refresh
	http.HandleFunc("/api/updates", handleContentUpdates)

	// Start server
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

// Helper function to create files if they don't exist
func createFileIfNotExists(filename string, defaultContent string) {
	dir := filepath.Dir(filepath.Join("data", filename))
	if dir != "." && dir != "data" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Error creating directory %s: %v\n", dir, err)
		}
	}
	filePath := filepath.Join("data", filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		err := os.WriteFile(filePath, []byte(defaultContent), 0644)
		if err != nil {
			log.Printf("Error creating file %s: %v\n", filename, err)
		} else {
			log.Printf("Created file %s with default content\n", filename)
		}
	}
}

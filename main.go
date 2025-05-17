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
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

//go:embed templates/* static/*
var content embed.FS

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
		default:
			// Default to no expiration
			delete(t.Expirations, fileID)
			return
		}
		// Set the expiration time
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

const rtextPlaceholder = `<h1>Welcome to Rich Text Notepad</h1>
<p>Start typing here to create your document. Use the toolbar above to format your text.</p>`

func generateUniqueFilename(baseDir, baseName string) string {
	// Sanitize: allow only letters, numbers, hyphen, underscore, and space
	reg := regexp.MustCompile(`[^a-zA-Z0-9\.\-_\s]`)
	sanitizedName := reg.ReplaceAllString(baseName, "-")
	log.Printf("Sanitized name %s -TO- %s\n", baseName, sanitizedName)
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

func main() {
	flag.Parse()

	if err := os.MkdirAll(filepath.Join("data", "files"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("data", "text"), 0755); err != nil {
		log.Fatal(err)
	}
	// Create notepad directory
	if err := os.MkdirAll(filepath.Join("data", "notepad"), 0755); err != nil {
		log.Fatal(err)
	}
	log.Println("Data directory created/reused without errors.")

	// Create placeholder notepad files if they don't exist
	createNotepadFileIfNotExists("md.file", mdPlaceholder)
	createNotepadFileIfNotExists("rtext.file", rtextPlaceholder)

	// Initialize the expiration tracker
	expirationTracker = initExpirationTracker()

	// Goroutine to periodically expire files
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
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
		tmpl.ExecuteTemplate(w, "index.html", entries)
	})

	http.HandleFunc("/md", func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "md.html", nil)
	})

	http.HandleFunc("/rtext", func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "rtext.html", nil)
	})

	// Serve static files from embedded filesystem
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatalf("Failed to create static sub-filesystem: %v", err)
	}
	http.Handle("/static/", http.FileServer(http.FS(staticFS)))

	// API endpoint to load notepad content
	http.HandleFunc("/notepad/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			filename := strings.TrimPrefix(r.URL.Path, "/notepad/")
			if filename != "md.file" && filename != "rtext.file" {
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
			if filename != "md.file" && filename != "rtext.file" {
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
		if err := r.ParseMultipartForm(2 << 28); err != nil { // 256 MB
			http.Error(w, err.Error(), 500)
			return
		}
		entryType := r.FormValue("type")
		expiryOption := r.FormValue("expiry")
		if expiryOption == "" {
			expiryOption = "Never" // Default to no expiration
		}

		switch entryType {
		case "text":
			content := r.FormValue("content")
			if content == "" {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			filename := r.FormValue("filename")
			if filename == "" {
				filename = time.Now().Format("Jan-02 15-04-05")
			} else {
				filename = generateUniqueFilename("data/text", filename)
			}
			err := os.WriteFile(filepath.Join("data/text", filename), []byte(content), 0644)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			// Set expiration if needed
			if expiryOption != "Never" {
				fileID := filepath.Join("text", filename)
				expirationTracker.SetExpiration(fileID, expiryOption)
			}
			log.Printf("Saved text snippet to %s with expiry %s\n", filename, expiryOption)
		case "file":
			if err := r.ParseMultipartForm(2 << 28); err != nil {
				http.Error(w, err.Error(), 500)
				log.Println("Failed to parse multipart form")
				return
			}
			files := r.MultipartForm.File["multifile"]
			if len(files) == 0 {
				http.Error(w, "No files uploaded", 400)
				log.Println("No files uploaded")
				return
			}
			for _, fileHeader := range files {
				if err := func() error {
					file, err := fileHeader.Open()
					if err != nil {
						return fmt.Errorf("failed to open uploaded file: %v", err)
					}
					defer file.Close()
					fileName := generateUniqueFilename("data/files", fileHeader.Filename)
					f, err := os.Create(filepath.Join("data/files", fileName))
					if err != nil {
						return fmt.Errorf("failed to create file: %v", err)
					}
					defer f.Close()
					if _, err := io.Copy(f, file); err != nil {
						return fmt.Errorf("failed to save file: %v", err)
					}
					// Set expiration if needed
					if expiryOption != "Never" {
						fileID := filepath.Join("files", fileName)
						expirationTracker.SetExpiration(fileID, expiryOption)
					}
					log.Printf("Saved file %s with expiry %s\n", fileName, expiryOption)
					return nil
				}(); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
			}
		}
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
			relNewPath = strings.TrimPrefix(relNewPath, "/")
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
		log.Printf("Renamed %s to %s\n", oldPath, newName)
		http.Redirect(w, r, "/", http.StatusSeeOther)
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

	http.HandleFunc("/show/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/show/")
		if !strings.HasPrefix(id, "text/") {
			http.Error(w, "Only text files can be shown", http.StatusBadRequest)
			return
		}
		content, err := os.ReadFile(filepath.Join("data", id))
		if err != nil {
			http.Error(w, "File not found", 404)
			return
		}
		viewData := struct {
			Content  string
			Filename string
		}{
			Content:  string(content),
			Filename: filepath.Base(id),
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		err = tmpl.ExecuteTemplate(w, "show.html", viewData)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		log.Printf("Served %s for viewing\n", id)
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

		// Brute force method to determine content type (in practice seems better than content-disposition)
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
		case ".mp3":
			contentType = "audio/mpeg"
		case ".mp4":
			contentType = "video/mp4"
		case ".txt":
			contentType = "text/plain"
		case ".html", ".htm":
			contentType = "text/html"
		case ".css":
			contentType = "text/css"
		case ".js":
			contentType = "application/javascript"
		case ".json":
			contentType = "application/json"
		case ".xml":
			contentType = "application/xml"
		case ".zip":
			contentType = "application/zip"
		case ".doc", ".docx":
			contentType = "application/msword"
		case ".xls", ".xlsx":
			contentType = "application/vnd.ms-excel"
		case ".ppt", ".pptx":
			contentType = "application/vnd.ms-powerpoint"
		default:
			// If not brute forced, detect from first 512 bytes
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
		w.Header().Set("X-Content-Type-Options", "nosniff") // Prevent MIME sniffing: adding as best practice
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
		filename := strings.TrimPrefix(r.URL.Path, "/delete/")
		os.Remove(filepath.Join("data", filename))
		expirationTracker.mu.Lock()
		delete(expirationTracker.Expirations, filename)
		expirationTracker.saveToFile()
		expirationTracker.mu.Unlock()
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Printf("Deleted %s\n", filename)
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
		http.Redirect(w, r, "/", http.StatusSeeOther)
		log.Printf("Edited %s\n", id)
	})

	// Start server
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}

// Helper function to create notepad files if they don't exist
func createNotepadFileIfNotExists(filename string, defaultContent string) {
	filePath := filepath.Join("data", "notepad", filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		err := os.WriteFile(filePath, []byte(defaultContent), 0644)
		if err != nil {
			log.Printf("Error creating notepad file %s: %v\n", filename, err)
		} else {
			log.Printf("Created notepad file %s with default content\n", filename)
		}
	}
}

package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

//go:embed templates/*
var content embed.FS

//go:embed static/style.css
var styleCSS []byte

//go:embed static/favicon.ico
var faviconICO []byte

//go:embed static/manifest.json
var manifestJSON []byte

//go:embed static/sw.js
var serviceWorkerJS []byte

//go:embed static/icon-192.png
var icon192PNG []byte

//go:embed static/icon-512.png
var icon512PNG []byte

//go:embed static/md.js
var mdJS []byte

//go:embed static/rtext.js
var rtextJS []byte

type Entry struct {
	ID       string
	Content  string
	Type     string
	Filename string
}

var listenAddress = flag.String("listen", "localhost:8080", "host:port in which the server will listen")

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
	log.Println("Data directory created/reused without errors.")
	tmpl := template.Must(template.ParseFS(content, "templates/*.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

	http.HandleFunc("/md.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(mdJS)
	})

	http.HandleFunc("/rtext.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(rtextJS)
	})

	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write(styleCSS)
		// log.Println("Served style.css")
	})

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write(faviconICO)
		// log.Println("Served favicon.ico")
	})

	http.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(manifestJSON)
		// log.Println("Served manifest.json")
	})

	http.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(serviceWorkerJS)
		// log.Println("Served sw.js")
	})

	http.HandleFunc("/icon-192.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(icon192PNG)
		// log.Println("Served icon-192.png")
	})

	http.HandleFunc("/icon-512.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(icon512PNG)
		// log.Println("Served icon-512.png")
	})

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(2 << 28); err != nil { // 256 MB
			http.Error(w, err.Error(), 500)
			return
		}
		entryType := r.FormValue("type")
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
			log.Printf("Saved text snippet to %s\n", filename)
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
					log.Printf("Saved file %s\n", fileName)
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
		err := os.Rename(
			filepath.Join("data", oldPath),
			filepath.Join(baseDir, newName),
		)
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

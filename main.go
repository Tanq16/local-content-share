package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
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

type Entry struct {
	ID       string
	Content  string
	Type     string // "text" or "file"
	Filename string
}

func generateUniqueFilename(baseDir, baseName string) string {
	// First try without random prefix
	if _, err := os.Stat(filepath.Join(baseDir, baseName)); os.IsNotExist(err) {
		return baseName
	}
	// Get prefix (file or text with - separator)
	prefix := baseName[:5]
	unprefixedName := baseName[5:]
	// If file exists, add random prefix until we find a unique name
	for {
		randChars := fmt.Sprintf("%04d", rand.Intn(10000))
		newName := fmt.Sprintf("%s%s-%s", prefix, randChars, unprefixedName)
		if _, err := os.Stat(filepath.Join(baseDir, newName)); os.IsNotExist(err) {
			return newName
		}
	}
}

func main() {
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal(err)
	}
	tmpl := template.Must(template.ParseFS(content, "templates/*.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		entries := []Entry{}
		files, err := os.ReadDir("data")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, file := range files {
			data, err := os.ReadFile(filepath.Join("data", file.Name()))
			if err != nil {
				continue
			}
			entry := Entry{ID: file.Name()}
			if strings.HasPrefix(file.Name(), "file-") {
				entry.Type = "file"
				entry.Filename = strings.TrimPrefix(file.Name(), "file-")
			} else {
				entry.Type = "text"
				entry.Content = string(data)
				entry.Filename = strings.TrimPrefix(file.Name(), "text-")
			}
			entries = append(entries, entry)
		}
		tmpl.ExecuteTemplate(w, "index.html", entries)
	})

	http.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write(styleCSS)
	})

	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write(faviconICO)
	})

	http.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(manifestJSON)
	})

	http.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write(serviceWorkerJS)
	})

	http.HandleFunc("/icon-192.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(icon192PNG)
	})

	http.HandleFunc("/icon-512.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(icon512PNG)
	})

	http.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
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
			// Generate timestamp-based filename
			timestamp := time.Now().Format("2006-01-02-15-04-05")
			filename := fmt.Sprintf("text-%s", timestamp)
			err := os.WriteFile(filepath.Join("data", filename), []byte(content), 0644)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		case "file":
			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			defer file.Close()
			// Generate unique filename
			fileName := generateUniqueFilename("data", "file-"+header.Filename)
			f, err := os.Create(filepath.Join("data", fileName))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			defer f.Close()
			if _, err := io.Copy(f, file); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/rename/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		oldName := strings.TrimPrefix(r.URL.Path, "/rename/")
		newName := r.FormValue("newname")
		if newName == "" {
			http.Error(w, "New name cannot be empty", http.StatusBadRequest)
			return
		}
		// Add prefix from old name if not present
		if !strings.HasPrefix(newName, "text-") && !strings.HasPrefix(newName, "file-") {
			newName = oldName[:5] + newName
		}
		// Generate unique filename
		newName = generateUniqueFilename("data", newName)

		// Rename the file
		err := os.Rename(
			filepath.Join("data", oldName),
			filepath.Join("data", newName),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/show/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/show/")
		content, err := os.ReadFile(filepath.Join("data", id))
		if err != nil {
			http.Error(w, "File not found", 404)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write(content)
	})

	http.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/download/")
		http.ServeFile(w, r, filepath.Join("data", filename))
	})

	http.HandleFunc("/delete/", func(w http.ResponseWriter, r *http.Request) {
		filename := strings.TrimPrefix(r.URL.Path, "/delete/")
		os.Remove(filepath.Join("data", filename))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

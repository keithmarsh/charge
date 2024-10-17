package main

import (
	"embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
)

//go:embed templates/*
var resources embed.FS

var t = template.Must(template.ParseFS(resources, "templates/*"))

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"

	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]string{
			"Region": os.Getenv("FLY_REGION"),
		}

		t.ExecuteTemplate(w, "index.html.tmpl", data)
	})
	http.HandleFunc("/upload", uploadHandler)

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

const MAX_UPLOAD_SIZE = 16 * 1024 // 16K

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Println("Method not allowed", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Limit file upload size
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	// Parse the body into memory
	if err := r.ParseMultipartForm(MAX_UPLOAD_SIZE); err != nil {
		log.Println("File too large", err)
		http.Error(w, "The uploaded file is too big. Please choose an file that's less than 1MB in size", http.StatusBadRequest)
		return
	}
	// Get the first file
	mulFile, mulHdr, err := r.FormFile("file")
	if err != nil {
		errStr := fmt.Sprintf("Error reading the file %s\n", err)
		log.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}
	log.Println("Filename", mulHdr.Filename)
	csvRdr := csv.NewReader(mulFile)
	csv, err := csvRdr.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d lines", len(csv))
	for ix, line := range csv {
		if ix > 0 {
			t := line[1]
			//wh := line[25]
			wh, _ := strconv.ParseFloat(line[25], 32)
			if wh > 0.0 {
				fmt.Fprintf(w, "%d %s %f<br>\n", ix, t, wh)
			}
		}
	}
}

package main

import (
	"embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	if err := r.ParseMultipartForm(MAX_UPLOAD_SIZE); err != nil {
		http.Error(w, "The uploaded file is too big. Please choose an file that's less than 1MB in size", http.StatusBadRequest)
		return
	}
	csvRdr := csv.NewReader(r.Body)
	csv, err := csvRdr.ReadAll()
	if err != nil {
		log.Fatal(err)
	}
	for ix, line := range csv {
		//if ix > 0 {
		t := line[1]
		wh := line[25]
		//wh, _ := strconv.ParseFloat(line[25], 32)
		//if wh > 0.0 {
		fmt.Fprintf(w, "%d %s %s<br>\n", ix, t, wh)
		//}
		//}
	}
}

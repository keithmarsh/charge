package main

import (
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"
)

//go:embed templates/*
var resources embed.FS

var t = template.Must(template.ParseFS(resources, "templates/*"))
var HourlyRates []Price

type Price struct {
	//ValueExcVat   float32 `json:"value_exc_vat"`
	ValueIncVat float64 `json:"value_inc_vat"`
	ValidFrom   string  `json:"valid_from"`
	FromTime    time.Time
	ToTime      time.Time
	ValidTo     string `json:"valid_to"`
	//PaymentMethod string `json:"payment_method"`
}

type Page struct {
	Count    int32   `json:"count"`
	Next     string  `json:"next"`
	Previous string  `json:"previous"`
	Results  []Price `json:"results"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"

	}

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/upload", uploadHandler)

	log.Println("listening on", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

const MAX_UPLOAD_SIZE = 16 * 1024 // 16K

func indexHandler(w http.ResponseWriter, r *http.Request) {
	url := "https://api.octopus.energy/v1/products/AGILE-FLEX-22-11-25/electricity-tariffs/E-1R-AGILE-FLEX-22-11-25-H/standard-unit-rates/"
	client := http.Client{Timeout: time.Second * 2}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println("Can't create HTTP Request", err)
		http.Error(w, "Can't create request", http.StatusInternalServerError)
		return
	}
	// To set env var in powershell
	// $env:OCTOKEY='sk...............'
	req.SetBasicAuth(os.Getenv("OCTOKEY"), "")
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Can't do HTTP Request", err)
		http.Error(w, "Can't do request", http.StatusInternalServerError)
		return
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		log.Println("HTTP Request wasn't 200 OK", err)
		http.Error(w, "Octopus request wasn't 200 OK", http.StatusBadGateway)
		return
	}
	resp.Body = http.MaxBytesReader(w, resp.Body, 1024*1024)
	//body, err :=
	dec := json.NewDecoder(resp.Body)

	var page Page
	err = dec.Decode(&page)
	if err != nil {
		log.Println("Can't decode JSON", err)
		http.Error(w, "Can't decode JSON", http.StatusInternalServerError)
		return
	}

	layout := "2006-01-02T15:04:05Z"
	for ix, p := range page.Results {
		page.Results[ix].FromTime, err = time.Parse(layout, p.ValidFrom)
		if err != nil {
			log.Println("Error parsing time", err)
		}
		page.Results[ix].ToTime, _ = time.Parse(layout, p.ValidTo)
	}

	sort.Slice(page.Results, func(i, j int) bool {
		return page.Results[i].FromTime.Before(page.Results[j].FromTime)
	})

	var prev_p Price
	HourlyRates = make([]Price, len(page.Results)/2)
	h_ix := 0
	for _, p := range page.Results {
		if prev_p.ValueIncVat != 0.0 && prev_p.ToTime.Equal(p.FromTime) && prev_p.FromTime.Minute() == 0 {
			hour := Price{
				FromTime:    prev_p.FromTime,
				ToTime:      p.ToTime,
				ValueIncVat: (prev_p.ValueIncVat + p.ValueIncVat) / 2.0,
			}
			HourlyRates[h_ix] = hour
			h_ix++
		}
		prev_p = p
	}

	log.Printf("Page %+v", HourlyRates)

	// data := map[string]string{
	// 	"Region": os.Getenv("FLY_REGION"),
	// }

	t.ExecuteTemplate(w, "index.html.tmpl", HourlyRates)
}

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
	layout := "2006-01-02T15:04:05.000Z"
	log.Printf("%d lines", len(csv))
	sumCost := 0.0
	sumKwh := 0.0
	for ix, line := range csv {
		if ix > 0 {
			t, _ := time.Parse(layout, line[1])
			//wh := line[25]
			wh, _ := strconv.ParseFloat(line[25], 32)
			if wh > 0.0 {
				rate, err := findRate(HourlyRates, t)
				if err != nil {
					fmt.Fprintf(w, "%v %.1f\n", t, wh)
				} else {
					total := wh * rate / 100000.0
					fmt.Fprintf(w, "%v %.1fWh * %.2fp = £%.2f\n", t, wh, rate, total)
					sumCost += total
				}
				sumKwh += wh
			}
		}
	}
	fmt.Fprintf(w, "Total £%.2f\n", sumCost)
	fmt.Fprintf(w, "Power %.3f kWh\n", sumKwh/1000.0)
}

func findRate(rates []Price, candidate time.Time) (float64, error) {
	for i := 0; i < len(rates)-1; i++ {
		// Check if candidate is equal to or after times[i] and before times[i+1]
		if (candidate.Equal(rates[i].FromTime) || candidate.After(rates[i].FromTime)) && candidate.Before(rates[i+1].FromTime) {
			return rates[i].ValueIncVat, nil
		}
	}
	return -1, fmt.Errorf("candidate time is out of range")
}

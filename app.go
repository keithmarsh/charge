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
	//
	// Get a page or two of octopus rates
	//
	url := "https://api.octopus.energy/v1/products/AGILE-FLEX-22-11-25/electricity-tariffs/E-1R-AGILE-FLEX-22-11-25-H/standard-unit-rates/"
	var rates []Price
	for i := 0; i < 2; i++ {
		page, err := fetchOctoPage(url)
		if err != nil {
			http.Error(w, "Could fetch Octopus rates", http.StatusBadGateway)
			return
		}
		rates = append(rates, page.Results[:]...)
		url = page.Next
	}

	//
	// Convert the timestamp strings to Time and sort
	//
	layout := "2006-01-02T15:04:05Z"
	var err error
	for ix, p := range rates {
		rates[ix].FromTime, err = time.Parse(layout, p.ValidFrom)
		if err != nil {
			log.Println("Error parsing time", err)
		}
		rates[ix].ToTime, _ = time.Parse(layout, p.ValidTo)
	}
	sort.Slice(rates, func(i, j int) bool {
		return rates[i].FromTime.Before(rates[j].FromTime)
	})
	//
	// Average to hourly rates
	//
	var prev_p Price
	HourlyRates = make([]Price, len(rates)/2)
	h_ix := 0
	for _, p := range rates {
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

	t.ExecuteTemplate(w, "index.html.tmpl", HourlyRates)
}

func fetchOctoPage(url string) (*Page, error) {
	client := http.Client{Timeout: time.Second * 2}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println("Can't create HTTP Request", err)
		return nil, err
	}
	// To set env var in powershell
	// $env:OCTOKEY='sk...............'
	req.SetBasicAuth(os.Getenv("OCTOKEY"), "")
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Can't do HTTP Request", err)
		return nil, err
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		log.Println("HTTP Request wasn't 200 OK", err)
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)

	var page Page
	err = dec.Decode(&page)
	if err != nil {
		log.Println("Can't decode JSON", err)
		return nil, err
	}
	return &page, nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Println("Method not allowed", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	//
	// Get the multipart form into a file
	//
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_SIZE)
	// Parse the body into memory
	if err := r.ParseMultipartForm(MAX_UPLOAD_SIZE); err != nil {
		log.Println("File too large", err)
		http.Error(w, "The uploaded file is too big. Please choose an file that's less than 1MB in size", http.StatusBadRequest)
		return
	}
	mulFile, mulHdr, err := r.FormFile("file")
	if err != nil {
		errStr := fmt.Sprintf("Error reading the file %s\n", err)
		log.Println(errStr)
		http.Error(w, errStr, http.StatusInternalServerError)
		return
	}
	//
	// Read the CSV
	//
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
			//
			// For each line, get the time and power
			//
			t, _ := time.Parse(layout, line[1])
			wh, _ := strconv.ParseFloat(line[25], 32)
			//
			// For entries with a matching rate, show the rate and cost
			//
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
	//
	// And show the totals
	//
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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var wineCategory = regexp.MustCompile(`(?i)(^|[^a-z])(wine|wines|vin|vins|wein|weine|vino|vini|champagne)([^a-z]|$)`)

func normalizeBarcode(input string) string {
	code := strings.NewReplacer(" ", "", "-", "", "\t", "", "\n", "").Replace(input)
	if len(code) != 8 && len(code) != 12 && len(code) != 13 {
		return ""
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			return ""
		}
	}
	sum, weight := 0, 3
	for i := len(code) - 2; i >= 0; i-- {
		sum += int(code[i]-'0') * weight
		weight = 4 - weight
	}
	if (10-sum%10)%10 != int(code[len(code)-1]-'0') {
		return ""
	}
	if len(code) == 12 {
		code = "0" + code
	}
	return code
}

type BarcodeMatch struct {
	WineRecognition
	Barcode   string `json:"barcode"`
	Source    string `json:"source"`
	SourceURL string `json:"sourceUrl"`
}
type BarcodeLookup struct {
	client      *http.Client
	endpoint    string
	userAgent   string
	mu          sync.Mutex
	lastRequest time.Time
}

func newBarcodeLookup() *BarcodeLookup {
	agent := os.Getenv("OPENFOODFACTS_USER_AGENT")
	if agent == "" {
		agent = "WineVault/0.1 (personal wine cellar organizer)"
	}
	return &BarcodeLookup{client: &http.Client{Timeout: 12 * time.Second}, endpoint: "https://world.openfoodfacts.org/api/v3/product/", userAgent: agent}
}
func (s *Store) lookupBarcode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	code := normalizeBarcode(r.PathValue("code"))
	if code == "" {
		http.Error(w, "Enter a valid 8-, 12-, or 13-digit EAN/UPC barcode, including its check digit.", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if s.db != nil {
		match := BarcodeMatch{Barcode: code, Source: "Your cellar"}
		match.Status = "recognized"
		match.Confidence = "medium"
		match.Notes = "Matched your most recently saved bottle with this barcode. Confirm the vintage on this bottle; barcodes can be shared across vintages."
		err := s.db.QueryRow(ctx, "SELECT name,region,wine_type FROM bottles WHERE barcode=$1 ORDER BY id DESC LIMIT 1", code).Scan(&match.Name, &match.Region, &match.Type)
		if err == nil {
			writeJSON(w, 200, match)
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			databaseError(w, err)
			return
		}
	}
	if s.barcodeLookup == nil {
		http.Error(w, "Barcode lookup is unavailable. Try a label photo or enter the wine manually.", 503)
		return
	}
	match, err := s.barcodeLookup.lookup(ctx, code)
	if err != nil {
		respondEditError(w, err)
		return
	}
	writeJSON(w, 200, match)
}
func (lookup *BarcodeLookup) lookup(ctx context.Context, code string) (BarcodeMatch, error) {
	result := BarcodeMatch{Barcode: code, Source: "Open Food Facts", SourceURL: "https://world.openfoodfacts.org/product/" + code}
	lookup.mu.Lock()
	if time.Since(lookup.lastRequest) < 4*time.Second {
		lookup.mu.Unlock()
		return result, reject(429, "Please wait a few seconds before looking up another barcode.")
	}
	lookup.lastRequest = time.Now()
	lookup.mu.Unlock()
	fields := "product_name,product_name_en,brands,categories,categories_tags,origins"
	request, err := http.NewRequestWithContext(ctx, "GET", lookup.endpoint+code+"?fields="+url.QueryEscape(fields), nil)
	if err != nil {
		return result, err
	}
	request.Header.Set("User-Agent", lookup.userAgent)
	response, err := lookup.client.Do(request)
	if err != nil {
		return result, reject(502, "The barcode catalogue could not be reached. Try again or use a label photo.")
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return result, reject(404, "This barcode is not in the catalogue yet. Try a label photo or enter the wine manually.")
	}
	if response.StatusCode == 429 {
		return result, reject(429, "The barcode catalogue is busy. Try again shortly.")
	}
	if response.StatusCode != 200 {
		return result, reject(502, "The barcode catalogue is unavailable. Try a label photo instead.")
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 131073))
	if err != nil || len(raw) > 131072 {
		return result, reject(502, "The barcode catalogue returned an invalid response.")
	}
	var product struct {
		Product struct {
			Name       string   `json:"product_name"`
			English    string   `json:"product_name_en"`
			Brands     string   `json:"brands"`
			Categories string   `json:"categories"`
			Tags       []string `json:"categories_tags"`
			Origins    string   `json:"origins"`
		} `json:"product"`
	}
	if json.Unmarshal(raw, &product) != nil {
		return result, reject(502, "The barcode catalogue returned an invalid response.")
	}
	p := product.Product
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = strings.TrimSpace(p.English)
	}
	if name == "" {
		return result, reject(404, "This barcode has no usable product details yet. Try a label photo or enter the wine manually.")
	}
	categories := strings.ToLower(p.Categories + " " + strings.Join(p.Tags, " "))
	wine := wineCategory.MatchString(categories)
	if !wine || strings.Contains(categories, "vinegar") || strings.Contains(categories, "vinaigre") {
		return result, reject(422, "The catalogue does not identify this product as wine. Check the barcode or use a label photo.")
	}
	if p.Brands != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(p.Brands)) {
		name = strings.TrimSpace(p.Brands) + " " + name
	}
	result.Status = "recognized"
	result.Name = limitText(name, 150)
	result.Region = limitText(strings.TrimSpace(p.Origins), 200)
	result.Confidence = "medium"
	switch {
	case strings.Contains(categories, "sparkling") || strings.Contains(categories, "champagne") || strings.Contains(categories, "mousseux"):
		result.Type = "Sparkling"
	case strings.Contains(categories, "red-wine") || strings.Contains(categories, "red wine") || strings.Contains(categories, "rouge"):
		result.Type = "Red"
	case strings.Contains(categories, "white-wine") || strings.Contains(categories, "white wine") || strings.Contains(categories, "blanc"):
		result.Type = "White"
	case strings.Contains(categories, "rose-wine") || strings.Contains(categories, "rosé"):
		result.Type = "Rosé"
	case strings.Contains(categories, "dessert-wine"):
		result.Type = "Dessert"
	}
	result.Notes = "Catalogue suggestion, not a verified bottle match. Confirm the region, wine type and vintage from your label; barcodes can be shared across vintages."
	return result, nil
}
func limitText(value string, bytes int) string {
	for len(value) > bytes {
		runes := []rune(value)
		value = string(runes[:len(runes)-1])
	}
	return value
}

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBarcodeNormalization(t *testing.T) {
	for input, want := range map[string]string{"5901234123457": "5901234123457", "036000291452": "0036000291452", "96385074": "96385074", "5901234123458": "", "bad": ""} {
		if got := normalizeBarcode(input); got != want {
			t.Errorf("%s: got %s want %s", input, got, want)
		}
	}
}
func TestBarcodeCatalogueLookup(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "WineVault/") {
			t.Error("missing app identity")
		}
		if !strings.HasSuffix(r.URL.Path, "5901234123457") {
			t.Error("wrong barcode query")
		}
		writeJSON(w, 200, map[string]any{"status": "success", "product": map[string]any{"product_name": "Estate Reserve 2018", "brands": "Test Estate", "categories_tags": []string{"en:wines", "en:red-wines"}, "origins": "France"}})
	}))
	defer provider.Close()
	lookup := &BarcodeLookup{client: provider.Client(), endpoint: provider.URL + "/", userAgent: "WineVault/test"}
	result, err := lookup.lookup(context.Background(), "5901234123457")
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "Red" || result.Vintage != nil || result.NonVintage || result.Source != "Open Food Facts" {
		t.Fatal("incorrect catalogue match", result)
	}
	if _, err = lookup.lookup(context.Background(), "5901234123457"); err == nil {
		t.Fatal("rapid repeated requests should be limited")
	}
}
func TestBarcodeNotFoundAndUnrelatedProduct(t *testing.T) {
	for _, name := range []string{"not found", "not wine"} {
		t.Run(name, func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if name == "not found" {
					w.WriteHeader(404)
					return
				}
				writeJSON(w, 200, map[string]any{"product": map[string]any{"product_name": "Chocolate spread", "categories_tags": []string{"en:spreads"}}})
			}))
			defer provider.Close()
			lookup := &BarcodeLookup{client: provider.Client(), endpoint: provider.URL + "/"}
			_, err := lookup.lookup(context.Background(), "5901234123457")
			if err == nil {
				t.Fatal("unrelated product accepted")
			}
		})
	}
}
func TestBarcodeStoredAndReusedWithoutProvider(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	w := call(s, "POST", "/api/bottles", `{"name":"Saved wine","vintage":2017,"region":"Valais","type":"Red","rack":"B","slot":29,"barcode":"036000291452"}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call(s, "GET", "/api/barcodes/036000291452", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result BarcodeMatch
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Barcode != "0036000291452" || result.Name != "Saved wine" || result.Vintage != nil || result.Source != "Your cellar" {
		t.Fatal("saved barcode not reused", result)
	}
	w = call(s, "GET", "/api/barcodes/invalid", "")
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

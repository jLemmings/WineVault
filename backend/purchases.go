package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type PurchaseData struct {
	PriceMinor   *int64 `json:"priceMinor"`
	Currency     string `json:"currency"`
	PurchaseDate string `json:"purchaseDate"`
	Seller       string `json:"seller"`
}

func validatePurchase(p *PurchaseData) error {
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	p.Seller = strings.TrimSpace(p.Seller)
	if len(p.Seller) > 200 {
		return fmt.Errorf("Seller must be at most 200 characters.")
	}
	if p.PriceMinor != nil && (*p.PriceMinor < 0 || *p.PriceMinor > 1000000000000) {
		return fmt.Errorf("Enter a valid non-negative price per bottle.")
	}
	supported := p.Currency == "CHF" || p.Currency == "EUR" || p.Currency == "USD" || p.Currency == "GBP" || p.Currency == "CAD" || p.Currency == "AUD"
	if (p.Currency != "" && !supported) || (p.PriceMinor != nil && !supported) {
		return fmt.Errorf("Choose CHF, EUR, USD, GBP, CAD, or AUD for the price.")
	}
	if p.PurchaseDate != "" {
		d, err := time.Parse("2006-01-02", p.PurchaseDate)
		if err != nil || d.Year() < 1900 || d.Year() > 9999 {
			return fmt.Errorf("Enter a purchase date as YYYY-MM-DD, or leave it blank.")
		}
	}
	return nil
}
func (s *Store) updatePurchase(w http.ResponseWriter, r *http.Request) {
	id, ok := toolID(w, r)
	if !ok {
		return
	}
	var input struct {
		PurchaseData
		Revision int `json:"revision"`
	}
	if !toolInput(w, r, &input) {
		return
	}
	if err := validatePurchase(&input.PurchaseData); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var revision int
	err := s.db.QueryRow(ctx, `
		UPDATE
		    bottles
		SET
		    price_minor = $1,
		    currency = $2,
		    purchased_on = NULLIF ($3, '')::date,
		    seller = $4
		WHERE
		    id = $5
		    AND revision = $6
		RETURNING
		    revision
	`, input.PriceMinor, input.Currency, input.PurchaseDate, input.Seller, id, input.Revision).Scan(&revision)
	if err != nil {
		toolError(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"revision": revision})
}
func (s *Store) purchaseHistory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := s.db.Query(ctx, `
		SELECT
		    p.bottle_id::text,
		    p.name,
		    p.vintage,
		    p.price_minor,
		    p.currency,
		    COALESCE(p.purchased_on::text, ''),p.seller,b.id IS NOT NULL FROM wine_purchases p LEFT JOIN bottles b ON b.id=p.bottle_id WHERE p.price_minor IS NOT NULL OR p.purchased_on IS NOT NULL OR p.seller<>'' ORDER BY p.purchased_on DESC NULLS LAST, p.bottle_id DESC
	`)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	type entry struct {
		PurchaseData
		BottleID string `json:"bottleId"`
		Name     string `json:"name"`
		Vintage  int    `json:"vintage"`
		InCellar bool   `json:"inCellar"`
	}
	result := []entry{}
	for rows.Next() {
		var p entry
		if err = rows.Scan(&p.BottleID, &p.Name, &p.Vintage, &p.PriceMinor, &p.Currency, &p.PurchaseDate, &p.Seller, &p.InCellar); err != nil {
			databaseError(w, err)
			return
		}
		result = append(result, p)
	}
	if err = rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, result)
}

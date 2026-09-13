package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Save identical bottles together so a slot conflict never leaves a partial batch.
func (s *Store) addBottleBatch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Bottle Bottle `json:"bottle"`
		Slots  []int  `json:"slots"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || len(input.Slots) < 1 || len(input.Slots) > 400 {
		http.Error(w, "Choose between 1 and 400 bottle slots", 400)
		return
	}
	b := input.Bottle
	b.Name = strings.TrimSpace(b.Name)
	b.Region = strings.TrimSpace(b.Region)
	if b.Barcode != "" {
		b.Barcode = normalizeBarcode(b.Barcode)
		if b.Barcode == "" {
			http.Error(w, "Invalid EAN or UPC barcode", 400)
			return
		}
	}
	seen := map[int]bool{}
	for _, slot := range input.Slots {
		b.Slot = slot
		if seen[slot] || !validBottle(b) {
			http.Error(w, "Invalid bottle details or duplicate slots", 400)
			return
		}
		seen[slot] = true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	result := make([]Bottle, 0, len(input.Slots))
	for _, slot := range input.Slots {
		b.Slot = slot
		err = tx.QueryRow(ctx, `INSERT INTO bottles(name,vintage,region,wine_type,rack_id,slot,barcode) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id::text`, b.Name, b.Vintage, b.Region, b.Type, b.Rack, b.Slot, b.Barcode).Scan(&b.ID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				switch pgErr.Code {
				case "23505", "23503", "23514":
					http.Error(w, "A selected shelf or slot is no longer available. No bottles were added. Review your slots and try again.", 409)
					return
				}
			}
			databaseError(w, err)
			return
		}
		result = append(result, b)
	}
	if err = tx.Commit(ctx); err != nil {
		databaseError(w, err)
		return
	}
	s.enrichAfterAdd(result[0].ID)
	writeJSON(w, 201, result)
}

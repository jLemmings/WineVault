package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Store) moveBottle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "Bottle not found", 404)
		return
	}
	var input struct {
		Rack string `json:"rack"`
		Slot *int   `json:"slot"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.Rack == "" || input.Slot == nil || *input.Slot < 0 {
		http.Error(w, "Choose a rack and an empty slot.", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var b Bottle
	err = s.db.QueryRow(ctx, `UPDATE bottles SET rack_id=$1,slot=$2 WHERE id=$3 RETURNING id::text,name,vintage,region,wine_type,rack_id,slot,barcode`, input.Rack, *input.Slot, id).Scan(&b.ID, &b.Name, &b.Vintage, &b.Region, &b.Type, &b.Rack, &b.Slot, &b.Barcode)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "This bottle is no longer in your cellar.", 404)
		return
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				http.Error(w, "That slot is now occupied. Choose another empty slot.", 409)
				return
			case "23503", "23514":
				http.Error(w, "That rack or slot is no longer available.", 409)
				return
			}
		}
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, b)
}

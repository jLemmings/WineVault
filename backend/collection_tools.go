package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func toolInput(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Invalid request", 400)
		return false
	}
	return true
}
func toolID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, e := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if e != nil || id < 1 {
		http.Error(w, "Bottle not found", 404)
		return 0, false
	}
	return id, true
}
func toolError(w http.ResponseWriter, e error) {
	if errors.Is(e, pgx.ErrNoRows) {
		http.Error(w, "Bottle changed or is no longer available. Refresh and try again.", 409)
		return
	}
	var p *pgconn.PgError
	if errors.As(e, &p) && (p.Code == "23505" || p.Code == "23503" || p.Code == "23514") {
		http.Error(w, "The selected slot is unavailable. Choose another empty slot.", 409)
		return
	}
	databaseError(w, e)
}
func (s *Store) editBottle(w http.ResponseWriter, r *http.Request) {
	id, ok := toolID(w, r)
	if !ok {
		return
	}
	var in struct {
		Name     string `json:"name"`
		Vintage  int    `json:"vintage"`
		Region   string `json:"region"`
		Type     string `json:"type"`
		Revision int    `json:"revision"`
	}
	if !toolInput(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Region = strings.TrimSpace(in.Region)
	if !validBottle(Bottle{Name: in.Name, Vintage: in.Vintage, Region: in.Region, Type: in.Type, Rack: "unused"}) || in.Revision < 1 {
		http.Error(w, "Invalid bottle details", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	// Re-link catalogue information only when the wine identity changes.
	var changed bool
	err := s.db.QueryRow(ctx, `WITH old AS (SELECT * FROM bottles WHERE id=$1 AND revision=$6 FOR UPDATE), updated AS (
 UPDATE bottles b SET name=$2,vintage=$3,region=$4,wine_type=$5,information_id=CASE WHEN bottle_wine_key(old.name,old.region,old.wine_type)=bottle_wine_key($2,$4,$5) THEN old.information_id ELSE NULL END
 FROM old WHERE b.id=old.id RETURNING bottle_wine_key(old.name,old.region,old.wine_type)<>bottle_wine_key($2,$4,$5) AS changed) SELECT changed FROM updated`, id, in.Name, in.Vintage, in.Region, in.Type, in.Revision).Scan(&changed)
	if err != nil {
		toolError(w, err)
		return
	}
	if changed {
		s.enrichAfterAdd(strconv.FormatInt(id, 10))
	}
	writeJSON(w, 200, map[string]bool{"saved": true})
}
func (s *Store) drinkingWindow(w http.ResponseWriter, r *http.Request) {
	id, ok := toolID(w, r)
	if !ok {
		return
	}
	var in struct {
		Start    *int `json:"start"`
		End      *int `json:"end"`
		Revision int  `json:"revision"`
	}
	if !toolInput(w, r, &in) {
		return
	}
	if (in.Start == nil) != (in.End == nil) || in.Revision < 1 || (in.Start != nil && (*in.Start < 1900 || *in.End < *in.Start || *in.End > 9999)) {
		http.Error(w, "Enter both years with the end at or after the start, or clear both.", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	// Serialize shared window updates before taking any bottle locks.
	if _, err = tx.Exec(ctx, "LOCK TABLE drinking_windows IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		databaseError(w, err)
		return
	}
	var key string
	var vintage int
	err = tx.QueryRow(ctx, `SELECT bottle_wine_key(name,region,wine_type),vintage FROM bottles WHERE id=$1 AND revision=$2 FOR UPDATE`, id, in.Revision).Scan(&key, &vintage)
	if err != nil {
		toolError(w, err)
		return
	}
	// Lock and revise all matching bottles so concurrent window edits cannot overwrite one another.
	_, err = tx.Exec(ctx, `UPDATE bottles SET revision=revision WHERE bottle_wine_key(name,region,wine_type)=$1 AND vintage=$2`, key, vintage)
	if err == nil {
		if in.Start == nil {
			_, err = tx.Exec(ctx, `DELETE FROM drinking_windows WHERE wine_key=$1 AND vintage=$2`, key, vintage)
		} else {
			_, err = tx.Exec(ctx, `INSERT INTO drinking_windows VALUES($1,$2,$3,$4) ON CONFLICT(wine_key,vintage) DO UPDATE SET start_year=excluded.start_year,end_year=excluded.end_year`, key, vintage, *in.Start, *in.End)
		}
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		toolError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"saved": true})
}
func (s *Store) enjoyedBottles(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := s.db.Query(ctx, `SELECT id::text,snapshot->>'name',snapshot->>'vintage',snapshot->>'rack_id',snapshot->>'slot' FROM enjoyed_bottles ORDER BY id DESC`)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	result := []map[string]string{}
	for rows.Next() {
		var id, name, vintage, rack, slot string
		if err = rows.Scan(&id, &name, &vintage, &rack, &slot); err != nil {
			databaseError(w, err)
			return
		}
		result = append(result, map[string]string{"id": id, "name": name, "vintage": vintage, "rack": rack, "slot": slot})
	}
	if err = rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (s *Store) restoreBottle(w http.ResponseWriter, r *http.Request) {
	id, ok := toolID(w, r)
	if !ok {
		return
	}
	var in struct {
		Rack string `json:"rack"`
		Slot *int   `json:"slot"`
	}
	if !toolInput(w, r, &in) {
		return
	}
	if in.Rack == "" || in.Slot == nil || *in.Slot < 0 {
		http.Error(w, "Choose an empty slot", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var snapshot []byte
	err = tx.QueryRow(ctx, `DELETE FROM enjoyed_bottles WHERE id=$1 RETURNING snapshot`, id).Scan(&snapshot)
	if err == nil {
		_, err = tx.Exec(ctx, `SELECT set_config('winevault.restoring','yes',true)`)
	}
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO bottles(id,name,vintage,region,wine_type,rack_id,slot,barcode,information_id,revision,price_minor,currency,purchased_on,seller) OVERRIDING SYSTEM VALUE
 SELECT $1,b.name,b.vintage,b.region,b.wine_type,$2,$3,b.barcode,b.information_id,b.revision+1,b.price_minor,COALESCE(b.currency,''),b.purchased_on,COALESCE(b.seller,'') FROM jsonb_populate_record(NULL::bottles,$4::jsonb) b`, id, in.Rack, *in.Slot, string(snapshot))
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		toolError(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"restored": true})
}

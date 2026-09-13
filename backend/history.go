package main

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

type HistoryEntry struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	OccurredAt time.Time `json:"occurredAt"`
	BottleID   string    `json:"bottleId"`
	Name       string    `json:"name"`
	Vintage    int       `json:"vintage"`
	Region     string    `json:"region"`
	Type       string    `json:"type"`
	Rack       string    `json:"rack"`
	RackName   string    `json:"rackName"`
	Slot       int       `json:"slot"`
	Columns    int       `json:"columns"`
}

func (s *Store) history(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	if action != "" && action != "added" && action != "enjoyed" {
		http.Error(w, "Invalid history filter", 400)
		return
	}
	var before int64
	if value := r.URL.Query().Get("before"); value != "" {
		var err error
		before, err = strconv.ParseInt(value, 10, 64)
		if err != nil || before < 1 {
			http.Error(w, "Invalid history cursor", 400)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	rows, err := s.db.Query(ctx, `SELECT id::text,action,occurred_at,bottle_id::text,name,vintage,region,wine_type,rack_id,rack_name,slot,columns FROM wine_history WHERE ($1='' OR action=$1) AND ($2::bigint=0 OR id<$2) ORDER BY id DESC LIMIT 51`, action, before)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer rows.Close()
	entries := []HistoryEntry{}
	for rows.Next() {
		var e HistoryEntry
		if err = rows.Scan(&e.ID, &e.Action, &e.OccurredAt, &e.BottleID, &e.Name, &e.Vintage, &e.Region, &e.Type, &e.Rack, &e.RackName, &e.Slot, &e.Columns); err != nil {
			databaseError(w, err)
			return
		}
		entries = append(entries, e)
	}
	if err = rows.Err(); err != nil {
		databaseError(w, err)
		return
	}
	next := ""
	if len(entries) > 50 {
		entries = entries[:50]
		next = entries[49].ID
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"entries": entries, "nextCursor": next})
}

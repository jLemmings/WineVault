package main

import (
	"context"
	"net/http"
	"time"
)

type CellarPreferences struct {
	ViewMode  string            `json:"viewMode"`
	TypeRacks map[string]string `json:"typeRacks"`
}

func (s *Store) updatePreferences(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision  int               `json:"revision"`
		ViewMode  string            `json:"viewMode"`
		TypeRacks map[string]string `json:"typeRacks"`
	}
	if !decodeEdit(w, r, &input) {
		return
	}
	if input.ViewMode != "floor-plan" && input.ViewMode != "racks-only" {
		http.Error(w, "Choose floor plan or racks only.", 400)
		return
	}
	if input.TypeRacks == nil {
		input.TypeRacks = map[string]string{}
	}
	for wineType := range input.TypeRacks {
		if !validBottle(Bottle{Name: "Wine", Region: "Region", Rack: "Rack", Type: wineType}) {
			http.Error(w, "Choose a supported wine type.", 400)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	c, err := lockCellar(ctx, tx, input.Revision)
	if err != nil {
		respondEditError(w, err)
		return
	}
	for _, rack := range input.TypeRacks {
		if rack == "" {
			continue
		}
		var exists bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM racks WHERE id=$1 AND cellar_id=$2)", rack, c.ID).Scan(&exists); err != nil {
			databaseError(w, err)
			return
		}
		if !exists {
			http.Error(w, "A preferred section no longer exists. Reopen settings and choose another shelf.", 400)
			return
		}
	}
	preferences := CellarPreferences{ViewMode: input.ViewMode, TypeRacks: input.TypeRacks}
	_, err = tx.Exec(ctx, "UPDATE cellars SET preferences=$1,revision=revision+1 WHERE id=$2", preferences, c.ID)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"preferences": preferences, "revision": c.Revision + 1})
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type wineInfo struct {
	ID          int64            `json:"-"`
	Name        string           `json:"-"`
	Type        string           `json:"-"`
	Status      string           `json:"status"`
	SourceID    *int64           `json:"sourceId"`
	Data        json.RawMessage  `json:"data"`
	FetchedAt   *time.Time       `json:"fetchedAt"`
	AttemptedAt *time.Time       `json:"attemptedAt"`
	Message     string           `json:"message"`
	Candidates  []grapeCandidate `json:"candidates"`
	SearchQuery string           `json:"-"`
}

func (s *Store) readWineInfo(ctx context.Context, bottleID string) (wineInfo, error) {
	var info wineInfo
	err := s.db.QueryRow(ctx, `SELECT i.id,i.name,i.wine_type,i.status,i.source_id,i.payload,i.fetched_at,i.attempted_at,i.message,i.candidates,i.search_query FROM wine_information i JOIN bottles b ON b.information_id=i.id WHERE b.id=$1`, bottleID).Scan(&info.ID, &info.Name, &info.Type, &info.Status, &info.SourceID, &info.Data, &info.FetchedAt, &info.AttemptedAt, &info.Message, &info.Candidates, &info.SearchQuery)
	return info, err
}
func validInfoBottleID(w http.ResponseWriter, r *http.Request) bool {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "Bottle not found.", 404)
		return false
	}
	return true
}
func infoReadError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "Bottle not found.", 404)
	} else {
		databaseError(w, err)
	}
}
func (s *Store) wineInformation(w http.ResponseWriter, r *http.Request) {
	if !validInfoBottleID(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	info, err := s.readWineInfo(ctx, r.PathValue("id"))
	if err != nil {
		infoReadError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, info)
}

func (s *Store) enrichAfterAdd(bottleID string) {
	// Inventory has already committed. Provider failure must never turn a saved batch into a failed add.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := s.enrichWine(ctx, bottleID, false, 0); err != nil {
		log.Print("GrapeMinds: wine saved, but enrichment state could not be updated; use Retry in bottle details")
	}
}
func (s *Store) enrichWine(ctx context.Context, bottleID string, force bool, chosenID int64) (wineInfo, error) {
	info, err := s.readWineInfo(ctx, bottleID)
	if err != nil {
		return info, err
	}
	if !force && info.Status != "not_fetched" {
		return info, nil
	}
	lease, err := s.db.Exec(ctx, `UPDATE wine_information SET status='fetching',message='',attempted_at=now() WHERE id=$1 AND (status!='fetching' OR attempted_at<now()-interval '1 minute') AND ($2 OR status='not_fetched')`, info.ID, force)
	if err != nil {
		return info, err
	}
	if lease.RowsAffected() == 0 {
		return s.readWineInfo(ctx, bottleID)
	}
	if chosenID == 0 && info.SourceID != nil {
		chosenID = *info.SourceID
	}
	sourceID, payload, fetchErr := s.grapeMinds.fetch(ctx, info.Name, info.Type, chosenID)
	// Use a fresh bounded context so timeouts can still be recorded and retried after restart.
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if fetchErr != nil {
		status, message := failureDetails(fetchErr)
		var failure *infoFailure
		if errors.As(fetchErr, &failure) && failure.candidates != nil {
			candidates, _ := json.Marshal(failure.candidates)
			_, err = s.db.Exec(saveCtx, "UPDATE wine_information SET status=$2,message=$3,candidates=$4,search_query=name WHERE id=$1", info.ID, status, message, candidates)
		} else {
			_, err = s.db.Exec(saveCtx, "UPDATE wine_information SET status=$2,message=$3 WHERE id=$1", info.ID, status, message)
		}
		log.Printf("GrapeMinds enrichment status=%s information_id=%d", status, info.ID)
	} else {
		_, err = s.db.Exec(saveCtx, `UPDATE wine_information SET status='ready',source_id=$2,payload=$3,fetched_at=now(),message='' WHERE id=$1`, info.ID, sourceID, payload)
	}
	if err != nil {
		return info, err
	}
	return s.readWineInfo(saveCtx, bottleID)
}
func (s *Store) retryWineInformation(w http.ResponseWriter, r *http.Request) {
	if !validInfoBottleID(w, r) {
		return
	}
	var input struct {
		SourceID int64 `json:"sourceId"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || input.SourceID < 0 {
		http.Error(w, "Invalid wine selection.", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	info, err := s.enrichWine(ctx, r.PathValue("id"), true, input.SourceID)
	if err != nil {
		infoReadError(w, err)
		return
	}
	writeJSON(w, 200, info)
}
func (s *Store) searchWineInformation(w http.ResponseWriter, r *http.Request) {
	if !validInfoBottleID(w, r) {
		return
	}
	var input struct {
		Query string `json:"query"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || len(input.Query) > 200 {
		http.Error(w, "Enter a wine search of up to 200 characters.", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	info, err := s.readWineInfo(ctx, r.PathValue("id"))
	if err != nil {
		infoReadError(w, err)
		return
	}
	if strings.TrimSpace(input.Query) == "" {
		input.Query = info.Name
	}
	if info.Candidates != nil && strings.TrimSpace(input.Query) == info.SearchQuery {
		writeJSON(w, 200, map[string]any{"candidates": info.Candidates})
		return
	}
	candidates, err := s.grapeMinds.search(ctx, input.Query)
	if err != nil {
		_, message := failureDetails(err)
		http.Error(w, message, 502)
		return
	}
	raw, _ := json.Marshal(candidates)
	if _, err = s.db.Exec(ctx, "UPDATE wine_information SET candidates=$2,search_query=$3 WHERE id=$1", info.ID, raw, strings.TrimSpace(input.Query)); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"candidates": candidates})
}

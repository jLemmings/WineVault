package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestWineHistoryPersistsAndRollsBack(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	var initial int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM wine_history").Scan(&initial); err != nil || initial != 0 {
		t.Fatal("fabricated prior history", initial, err)
	}
	w := call(s, "POST", "/api/bottles/batch", `{"bottle":{"name":"History wine","region":"France","type":"Red","rack":"C","vintage":2018},"slots":[28,29]}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var added []Bottle
	if err := json.Unmarshal(w.Body.Bytes(), &added); err != nil {
		t.Fatal(err)
	}
	w = call(s, "POST", "/api/bottles/batch", `{"bottle":{"name":"Failed history","region":"France","type":"Red","rack":"C","vintage":2018},"slots":[27,28]}`)
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	w = call(s, "DELETE", "/api/bottles?id="+added[1].ID, "")
	if w.Code != 204 {
		t.Fatal(w.Code)
	}
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	w = call(s, "GET", "/api/history", "")
	var history struct {
		Entries []HistoryEntry `json:"entries"`
	}
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &history); err != nil {
		t.Fatal(err)
	}
	if len(history.Entries) != 3 || history.Entries[0].Action != "enjoyed" || history.Entries[0].BottleID != added[1].ID || history.Entries[0].Slot != 29 || history.Entries[0].OccurredAt.IsZero() {
		t.Fatalf("unexpected history: %+v", history)
	}
	w = call(s, "GET", "/api/history?action=added&before="+history.Entries[0].ID, "")
	json.Unmarshal(w.Body.Bytes(), &history)
	if len(history.Entries) != 2 {
		t.Fatal("filter/cursor failed", w.Body.String())
	}
}

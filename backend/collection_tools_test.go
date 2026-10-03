package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func TestCollectionTools(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := migrateFixture(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	if err := migrateFixture(ctx, pool, ""); err != nil {
		t.Fatal("idempotent migration", err)
	}
	s := &Store{db: pool}
	list := func() []Bottle {
		t.Helper()
		w := call(s, "GET", "/api/bottles", "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var result []Bottle
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	b := list()[0]
	edit := fmt.Sprintf(`{"name":"Corrected wine","vintage":2020,"region":"Test region","type":"Rosé","revision":%d}`, b.Revision)
	w := call(s, "PUT", "/api/bottles/"+b.ID, edit)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call(s, "PUT", "/api/bottles/"+b.ID, edit); w.Code != 409 {
		t.Fatal("stale edit accepted", w.Code)
	}
	b = list()[0]
	if b.Name != "Corrected wine" || b.Type != "Rosé" {
		t.Fatal("edit not persisted", b)
	}
	var informationName string
	if err := pool.QueryRow(ctx, `SELECT i.name FROM bottles b JOIN wine_information i ON i.id=b.information_id WHERE b.id=$1`, b.ID).Scan(&informationName); err != nil || informationName != b.Name {
		t.Fatal("catalogue identity not relinked", err, informationName)
	}
	window := fmt.Sprintf(`{"start":2020,"end":2035,"revision":%d}`, b.Revision)
	if w = call(s, "PUT", "/api/bottles/"+b.ID+"/window", window); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call(s, "PUT", "/api/bottles/"+b.ID+"/window", window); w.Code != 409 {
		t.Fatal("stale window accepted", w.Code)
	}
	b = list()[0]
	if b.DrinkStart == nil || *b.DrinkEnd != 2035 {
		t.Fatal("window missing", b)
	}
	if w = call(s, "PUT", "/api/bottles/"+b.ID+"/window", fmt.Sprintf(`{"start":2035,"end":2020,"revision":%d}`, b.Revision)); w.Code != 400 {
		t.Fatal("invalid window accepted")
	}
	// A matching bottle shares the window while other vintages do not.
	var otherID string
	if err := pool.QueryRow(ctx, `INSERT INTO bottles(name,vintage,region,wine_type,rack_id,slot) VALUES($1,$2,$3,$4,'B',24) RETURNING id::text`, b.Name, b.Vintage, b.Region, b.Type).Scan(&otherID); err != nil {
		t.Fatal(err)
	}
	for _, other := range list() {
		if other.ID == otherID && other.DrinkStart == nil {
			t.Fatal("matching wine did not share window")
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE bottles SET barcode='5901234123457' WHERE id=$1", b.ID); err != nil {
		t.Fatal(err)
	}
	b = list()[0]
	if w = call(s, "DELETE", "/api/bottles?id="+b.ID, ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Occupied destinations must retain the archive for another attempt.
	if w = call(s, "POST", "/api/bottles/"+b.ID+"/restore", `{"rack":"B","slot":24}`); w.Code != 409 {
		t.Fatal("occupied restore accepted", w.Code, w.Body.String())
	}
	if w = call(s, "GET", "/api/enjoyed", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = call(s, "POST", "/api/bottles/"+b.ID+"/restore", fmt.Sprintf(`{"rack":%q,"slot":%d}`, b.Rack, b.Slot)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	restored := list()[0]
	if restored.ID != b.ID || restored.Name != b.Name || restored.DrinkEnd == nil || restored.Barcode != b.Barcode {
		t.Fatal("restore lost identity or window", restored)
	}
	if w = call(s, "POST", "/api/bottles/"+b.ID+"/restore", fmt.Sprintf(`{"rack":%q,"slot":%d}`, b.Rack, b.Slot)); w.Code != 409 {
		t.Fatal("double restoration accepted")
	}
	w = call(s, "GET", "/api/history?action=restored", "")
	var history struct {
		Entries []HistoryEntry `json:"entries"`
	}
	json.Unmarshal(w.Body.Bytes(), &history)
	if w.Code != 200 || len(history.Entries) != 1 || history.Entries[0].BottleID != b.ID {
		t.Fatal("incorrect restoration history", w.Body.String())
	}
	b = restored
	if w = call(s, "PUT", "/api/bottles/"+b.ID+"/window", fmt.Sprintf(`{"start":null,"end":null,"revision":%d}`, b.Revision)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if list()[0].DrinkStart != nil {
		t.Fatal("window not cleared")
	}
}

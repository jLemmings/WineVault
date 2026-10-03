package main

import (
	"context"
	"testing"
)

func TestCellarPreferences(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := migrateFixture(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	before := snapshot(t, s)
	input := map[string]any{"revision": before.Revision, "viewMode": "racks-only", "typeRacks": map[string]string{"White": "C", "Red": "A", "Champagne": "C", "Dessert": ""}}
	w := editCall(t, s, "PUT", "/api/cellar/preferences", input)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = editCall(t, s, "PUT", "/api/cellar/preferences", input); w.Code != 409 {
		t.Fatal("stale preferences accepted", w.Code)
	}
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	c := snapshot(t, &Store{db: pool})
	if c.Preferences.ViewMode != "racks-only" || c.Preferences.TypeRacks["White"] != "C" || len(c.Bottles) != len(before.Bottles) {
		t.Fatal("preferences did not persist", c.Preferences)
	}
	input["revision"] = c.Revision
	input["viewMode"] = "unknown"
	if w = editCall(t, s, "PUT", "/api/cellar/preferences", input); w.Code != 400 {
		t.Fatal("invalid view accepted")
	}
	input["viewMode"] = "floor-plan"
	input["typeRacks"] = map[string]string{"Beer": "A"}
	if w = editCall(t, s, "PUT", "/api/cellar/preferences", input); w.Code != 400 {
		t.Fatal("invalid type accepted")
	}
	input["typeRacks"] = map[string]string{"White": "missing"}
	if w = editCall(t, s, "PUT", "/api/cellar/preferences", input); w.Code != 400 {
		t.Fatal("missing section accepted")
	}
	w = call(s, "POST", "/api/bottles", `{"name":"Champagne test","vintage":0,"region":"Champagne, France","type":"Champagne","rack":"C","slot":29}`)
	if w.Code != 201 {
		t.Fatal("Champagne unsupported", w.Code, w.Body.String())
	}
	// Empty shelves can be removed; their preferred types remain with no section.
	w = editCall(t, s, "POST", "/api/racks", rackUpdate{Revision: c.Revision, Name: "Temporary section", Short: "Section", Wall: "North", Temp: 12, Color: "white", Rows: 2, Columns: 2})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	c = snapshot(t, s)
	newRack := c.Racks[len(c.Racks)-1]
	input["revision"] = c.Revision
	input["typeRacks"] = map[string]string{"White": newRack.ID}
	if w = editCall(t, s, "PUT", "/api/cellar/preferences", input); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	c = snapshot(t, s)
	if w = editCall(t, s, "DELETE", "/api/racks/"+newRack.ID, map[string]int{"revision": c.Revision}); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	c = snapshot(t, s)
	if section, ok := c.Preferences.TypeRacks["White"]; !ok || section != "" {
		t.Fatal("removed section not cleared", c.Preferences)
	}
}

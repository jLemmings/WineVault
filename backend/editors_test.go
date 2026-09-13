package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func snapshot(t *testing.T, s *Store) Cellar {
	t.Helper()
	w := call(s, "GET", "/api/cellar", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var c Cellar
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	return c
}
func editCall(t *testing.T, s *Store, method, url string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return call(s, method, url, string(data))
}
func roomDraft(c Cellar) layoutUpdate {
	return layoutUpdate{Revision: c.Revision, Name: c.Name, Room: c.Room, Width: c.Width, Depth: c.Depth, Layout: c.Layout, Racks: c.Racks}
}
func shelfDraft(c Cellar, r Rack) rackUpdate {
	return rackUpdate{Revision: c.Revision, Name: r.Name, Short: r.Short, Wall: r.Wall, Grapes: r.Grapes, Temp: r.Temp, Color: r.Color, Rows: r.Rows, Columns: r.Columns}
}

func TestRoomEditorPersistenceAndValidation(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	before := snapshot(t, s)
	draft := roomDraft(before)
	draft.Name = "My configured cellar"
	draft.Room = "The vault"
	draft.Width = 6
	draft.Layout.Shape = "l-shape"
	draft.Layout.Floor = "wood"
	draft.Racks[0].X = .7
	w := editCall(t, s, "PUT", "/api/cellar/layout", draft)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	after := snapshot(t, &Store{db: pool})
	if after.Name != draft.Name || after.Width != 6 || after.Layout.Shape != "l-shape" || after.Racks[0].X != .7 || after.Revision != before.Revision+1 {
		t.Fatal("layout not persisted", after)
	}
	if len(after.Bottles) != len(before.Bottles) {
		t.Fatal("layout changed bottles")
	}
	if w = editCall(t, s, "PUT", "/api/cellar/layout", draft); w.Code != 409 {
		t.Fatal("stale editor overwrote layout", w.Code)
	}
	draft = roomDraft(after)
	draft.Racks[0].X = -.1
	if w = editCall(t, s, "PUT", "/api/cellar/layout", draft); w.Code != 400 {
		t.Fatal("out of bounds accepted", w.Code)
	}
	draft = roomDraft(snapshot(t, s))
	draft.Racks[0].X = draft.Racks[1].X
	draft.Racks[0].Y = draft.Racks[1].Y
	draft.Racks[0].Width = .5
	if w = editCall(t, s, "PUT", "/api/cellar/layout", draft); w.Code != 400 {
		t.Fatal("overlap accepted", w.Code)
	}
	draft = roomDraft(snapshot(t, s))
	draft.Layout.DoorOffset = 30
	if w = editCall(t, s, "PUT", "/api/cellar/layout", draft); w.Code != 400 {
		t.Fatal("invalid door accepted", w.Code)
	}
	if current := snapshot(t, s); current.Revision != after.Revision {
		t.Fatal("invalid edit was partially saved")
	}
}

func TestShelfEditorProtectsInventory(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	before := snapshot(t, s)
	draft := shelfDraft(before, before.Racks[0])
	draft.Rows = 6
	draft.Columns = 6
	draft.Name = "Bordeaux reserve"
	draft.Temp = 12
	w := editCall(t, s, "PUT", "/api/racks/A", draft)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	after := snapshot(t, s)
	if after.Racks[0].Capacity != 36 || after.Racks[0].Rows != 6 || after.Racks[0].Name != draft.Name || after.Racks[0].Temp != 12 {
		t.Fatal("shelf not persisted")
	}
	oldBottles, _ := json.Marshal(before.Bottles)
	newBottles, _ := json.Marshal(after.Bottles)
	if string(oldBottles) != string(newBottles) {
		t.Fatal("editing shelf moved or lost bottles")
	}
	b := Bottle{Name: "Last slot", Vintage: 2020, Region: "Valais", Type: "Red", Rack: "A", Slot: 35}
	w = editCall(t, s, "POST", "/api/bottles", b)
	if w.Code != 201 {
		t.Fatal("new capacity unavailable", w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &b)
	draft = shelfDraft(after, after.Racks[0])
	draft.Rows = 5
	if w = editCall(t, s, "PUT", "/api/racks/A", draft); w.Code != 409 {
		t.Fatal("occupied slot removed", w.Code)
	}
	if w = editCall(t, s, "DELETE", "/api/racks/A", map[string]int{"revision": after.Revision}); w.Code != 409 {
		t.Fatal("occupied shelf deleted", w.Code)
	}
	if current := snapshot(t, s); current.Racks[0].Capacity != 36 || current.Revision != after.Revision {
		t.Fatal("rejected edit changed shelf")
	}
	call(s, "DELETE", "/api/bottles?id="+b.ID, "")
	if w = editCall(t, s, "PUT", "/api/racks/A", draft); w.Code != 200 {
		t.Fatal("safe capacity reduction failed", w.Code, w.Body.String())
	}
	if w = editCall(t, s, "POST", "/api/bottles", b); w.Code != 400 {
		t.Fatal("removed slot accepted bottle", w.Code)
	}
}

func TestCreateAndRemoveShelf(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	before := snapshot(t, s)
	draft := rackUpdate{Revision: before.Revision, Name: "New shelf", Short: "New", Wall: "Center", Rows: 3, Columns: 4, Temp: 13, Color: "gold"}
	w := editCall(t, s, "POST", "/api/racks", draft)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	after := snapshot(t, s)
	if len(after.Racks) != 5 || after.Racks[4].Capacity != 12 {
		t.Fatal("shelf not created")
	}
	if err := validateLayout(roomDraft(after)); err != nil {
		t.Fatal("new shelf placed in invalid location", err)
	}
	w = editCall(t, s, "DELETE", "/api/racks/"+result.ID, map[string]int{"revision": after.Revision})
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if after = snapshot(t, s); len(after.Racks) != 4 || len(after.Bottles) != 82 {
		t.Fatal("remove changed inventory")
	}
}

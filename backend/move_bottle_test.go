package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMoveBottle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	w := call(s, "POST", "/api/bottles/batch", `{"bottle":{"name":"Moving wine","region":"France","type":"Red","rack":"C","vintage":2020},"slots":[28,29]}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var added []Bottle
	json.Unmarshal(w.Body.Bytes(), &added)
	path := "/api/bottles/" + added[0].ID + "/location"
	for _, body := range []string{`{"rack":"C","slot":29}`, `{"rack":"missing","slot":0}`, `{"rack":"C","slot":999}`} {
		if w = call(s, "PUT", path, body); w.Code != 409 {
			t.Fatal("expected conflict", w.Code, w.Body.String())
		}
	}
	if w = call(s, "PUT", path, `{"rack":"C"}`); w.Code != 400 {
		t.Fatal("missing slot accepted")
	}
	var originalSlot int
	if err := pool.QueryRow(ctx, "SELECT slot FROM bottles WHERE id=$1", added[0].ID).Scan(&originalSlot); err != nil || originalSlot != 28 {
		t.Fatal("failed move changed bottle", err)
	}
	if w = call(s, "PUT", path, `{"rack":"C","slot":27}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call(s, "PUT", path, `{"rack":"D","slot":19}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var moved Bottle
	json.Unmarshal(w.Body.Bytes(), &moved)
	if moved.ID != added[0].ID || moved.Name != added[0].Name || moved.Rack != "D" || moved.Slot != 19 {
		t.Fatal("incorrect moved bottle", moved)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM wine_history WHERE bottle_id=$1", moved.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("move created false added/enjoyed history", count, err)
	}
	if w = call(s, "PUT", "/api/bottles/999999/location", `{"rack":"C","slot":28}`); w.Code != 404 {
		t.Fatal("missing bottle accepted")
	}
}

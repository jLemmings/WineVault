package main

import (
	"context"
	"testing"
)

func TestBottleBatchValidation(t *testing.T) {
	for _, body := range []string{`{"slots":[]}`, `{"bottle":{"name":"Wine","region":"France","type":"Red","rack":"C","vintage":2018},"slots":[1,1]}`, `{"slots":[-1]}`} {
		if w := call(&Store{}, "POST", "/api/bottles/batch", body); w.Code != 400 {
			t.Fatalf("expected validation failure, got %d", w.Code)
		}
	}
}

func TestBottleBatchAtomicSave(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	body := `{"bottle":{"name":"Batch wine","region":"France","type":"Red","rack":"C","vintage":2018},"slots":[28,29]}`
	if w := call(s, "POST", "/api/bottles/batch", body); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	conflict := `{"bottle":{"name":"Rollback wine","region":"France","type":"Red","rack":"C","vintage":2018},"slots":[27,28]}`
	if w := call(s, "POST", "/api/bottles/batch", conflict); w.Code != 409 {
		t.Fatal(w.Code, w.Body.String())
	}
	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM bottles WHERE name='Rollback wine'").Scan(&count); err != nil || count != 0 {
		t.Fatal("partial batch saved", count, err)
	}
}

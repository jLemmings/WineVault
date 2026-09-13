package main

import (
	"context"
	"testing"
)

func TestSharedInformationMigrationAndNewVintages(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	// Reproduce an installation with separate per-vintage records before migration 009.
	schema := initialSchema + initialSeed + editorSchema + nonVintageSchema + barcodeSchema + historySchema + authSchema + informationSchema + informationSearchSchema +
		`CREATE TABLE schema_migrations(version integer PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now()); INSERT INTO schema_migrations(version) SELECT generate_series(1,8);`
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO bottles(name,vintage,region,wine_type,rack_id,slot) VALUES
 ('Shared Estate',2016,'France','Red','C',27),
 ('Shared Estate',2018,'France','Red','C',28),
 (' shared  estate ',2020,' france ','Red','C',29)`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE wine_information SET status='ready',payload='{"id":42,"display_name":"Shared Estate"}',source_id=42,fetched_at=now() WHERE name='Shared Estate' AND vintage=2018`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	var records int
	if err := pool.QueryRow(ctx, `SELECT count(DISTINCT information_id) FROM bottles WHERE rack_id='C' AND slot>=27`).Scan(&records); err != nil || records != 1 {
		t.Fatal("vintages did not share information", records, err)
	}
	list, err := queryBottles(ctx, pool, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range list {
		if b.Rack == "C" && b.Slot >= 27 && (!b.HasInformation || b.InformationStatus != "ready") {
			t.Fatal("existing loaded status not propagated", b)
		}
	}
	// A later vintage must reuse the cached data even with no provider configured.
	s := &Store{db: pool}
	w := call(s, "POST", "/api/bottles", `{"name":"Shared Estate","vintage":2022,"region":"France","type":"Red","rack":"C","slot":26}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT i.status FROM wine_information i JOIN bottles b ON b.information_id=i.id WHERE b.rack_id='C' AND b.slot=26`).Scan(&status); err != nil || status != "ready" {
		t.Fatal("new vintage did not reuse saved details", status, err)
	}
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal("restart migration failed", err)
	}
}

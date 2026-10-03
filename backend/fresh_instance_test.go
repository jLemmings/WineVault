package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFreshInstanceIsEmpty(t *testing.T) {
	for _, mode := range []string{"no legacy path", "missing legacy file", "empty legacy inventory"} {
		t.Run(mode, func(t *testing.T) {
			pool := testPool(t)
			ctx := context.Background()
			path := ""
			if mode != "no legacy path" {
				path = filepath.Join(t.TempDir(), "bottles.json")
			}
			if mode == "empty legacy inventory" {
				if err := os.WriteFile(path, []byte("[]"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				if err := migrate(ctx, pool, path); err != nil {
					t.Fatal(err)
				}
				c := snapshot(t, &Store{db: pool})
				if len(c.Bottles) != 0 || len(c.Racks) != 0 || c.Owner != "Personal cellar" || c.Name != "Your cellar" || c.Layout.TableEnabled {
					t.Fatalf("new instance contains sample data: %+v", c)
				}
				for _, table := range []string{"rack_slots", "wine_information", "wine_history", "enjoyed_bottles", "drinking_windows"} {
					var count int
					if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 0 {
						t.Fatalf("%s: count=%d, err=%v", table, count, err)
					}
				}
			}
			// The empty cellar is immediately usable: users can add their own first shelf.
			c := snapshot(t, &Store{db: pool})
			w := editCall(t, &Store{db: pool}, "POST", "/api/racks", rackUpdate{Revision: c.Revision, Name: "My shelf", Short: "Shelf", Wall: "North", Grapes: "My wines", Temp: 12, Color: "red", Rows: 2, Columns: 3})
			if w.Code != 201 {
				t.Fatal("cannot add first shelf", w.Code, w.Body.String())
			}
		})
	}
}

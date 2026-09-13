package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("winevault_test_%d", time.Now().UnixNano())
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	return pool
}
func call(s *Store, method, url, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	s.dataRoutes().ServeHTTP(w, httptest.NewRequest(method, url, strings.NewReader(body)))
	return w
}
func TestDatabaseLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	w := call(s, "GET", "/api/cellar", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var c Cellar
	if err := json.Unmarshal(w.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Racks) != 4 || len(c.Bottles) != 82 || c.Width != 5.4 {
		t.Fatal("incorrect database seed", c)
	}
	body := `{"name":"Integration vintage","vintage":2020,"region":"Bordeaux","type":"Red","rack":"B","slot":24}`
	w = call(s, "POST", "/api/bottles", body)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var b Bottle
	json.Unmarshal(w.Body.Bytes(), &b)
	// A new handler reads persisted data, not a shared in-memory inventory.
	other := &Store{db: pool}
	w = call(other, "GET", "/api/bottles", "")
	var stored []Bottle
	json.Unmarshal(w.Body.Bytes(), &stored)
	if len(stored) != 83 {
		t.Fatal("insert not persisted")
	}
	if w = call(other, "POST", "/api/bottles", body); w.Code != 409 {
		t.Fatal("duplicate slot accepted", w.Code)
	}
	if w = call(other, "DELETE", "/api/bottles?id="+b.ID, ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	// Database changes must be visible in the API, including cellar and rack metadata.
	if _, err := pool.Exec(ctx, "UPDATE cellars SET name='Database cellar',width_m=6.25; UPDATE racks SET temperature=11 WHERE id='B'"); err != nil {
		t.Fatal(err)
	}
	w = call(s, "GET", "/api/cellar", "")
	json.Unmarshal(w.Body.Bytes(), &c)
	if c.Name != "Database cellar" || c.Width != 6.25 || c.Racks[1].Temp != 11 {
		t.Fatal("metadata not loaded from PostgreSQL")
	}
	// Invalid rack/slot constraints are enforced by PostgreSQL.
	for _, invalid := range []string{strings.Replace(body, `"slot":24`, `"slot":30`, 1), strings.Replace(body, `"rack":"B"`, `"rack":"Z"`, 1), strings.Replace(body, `"vintage":2020`, `"vintage":1800`, 1)} {
		if w = call(s, "POST", "/api/bottles", invalid); w.Code != 400 {
			t.Fatal("invalid input accepted", w.Code, w.Body.String())
		}
	}
	if w = call(s, "GET", "/api/health", ""); w.Code != 200 {
		t.Fatal("health check", w.Code)
	}
	// Competing requests cannot put two bottles in the same slot.
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- call(s, "POST", "/api/bottles", body).Code }()
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for code := range codes {
		if code == 201 {
			success++
		}
		if code == 409 {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("concurrent slot protection failed", success, conflict)
	}
	// Restarting migrations never restores consumed bottles, even an empty cellar.
	if _, err := pool.Exec(ctx, "DELETE FROM bottles"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	w = call(s, "GET", "/api/bottles", "")
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("migration reseeded existing inventory")
	}
}
func TestLegacyImport(t *testing.T) {
	pool := testPool(t)
	path := filepath.Join(t.TempDir(), "bottles.json")
	original := `[{"id":"old-id","name":"My own wine","vintage":2019,"region":"Valais","type":"White","rack":"D","slot":19}]`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := migrate(context.Background(), pool, path); err != nil {
		t.Fatal(err)
	}
	w := call(&Store{db: pool}, "GET", "/api/bottles", "")
	var bottles []Bottle
	json.Unmarshal(w.Body.Bytes(), &bottles)
	if len(bottles) != 1 || bottles[0].Name != "My own wine" || bottles[0].Slot != 19 {
		t.Fatal("legacy import failed", w.Body.String())
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != original {
		t.Fatal("original JSON was changed")
	}
}
func TestInvalidLegacyImportRollsBack(t *testing.T) {
	pool := testPool(t)
	path := filepath.Join(t.TempDir(), "bottles.json")
	os.WriteFile(path, []byte(`[{"name":"bad"}]`), 0600)
	if err := migrate(context.Background(), pool, path); err == nil {
		t.Fatal("invalid legacy inventory accepted")
	}
	var table *string
	if err := pool.QueryRow(context.Background(), "SELECT to_regclass('cellars')::text").Scan(&table); err != nil || table != nil {
		t.Fatal("partial migration was committed", table, err)
	}
}
func TestBottleValidation(t *testing.T) {
	b := Bottle{Name: "Wine", Vintage: 2020, Region: "Valais", Type: "Red", Rack: "A", Slot: 0}
	if !validBottle(b) {
		t.Fatal("valid bottle rejected")
	}
	b.Type = "invalid"
	if validBottle(b) {
		t.Fatal("invalid type accepted")
	}
}

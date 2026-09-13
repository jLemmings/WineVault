package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testGrapeClient(server *httptest.Server) *grapeMindsClient {
	return &grapeMindsClient{key: "test-grape-secret", base: server.URL, http: server.Client()}
}
func testWineInfo(t *testing.T, s *Store, id string) wineInfo {
	t.Helper()
	w := call(s, "GET", "/api/bottles/"+id+"/information", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var info wineInfo
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	return info
}
func TestGrapeMindsFetchPersistenceAndRequestBudget(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var fail atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-grape-secret" || r.Header.Get("Accept-Language") != "en" {
			t.Error("missing server-side authentication or language")
		}
		if r.Method != "GET" {
			t.Error("unexpected billed/non-read request", r.Method)
		}
		if fail.Load() {
			w.WriteHeader(500)
			return
		}
		switch r.URL.Path {
		case "/wines/search":
			if r.URL.Query().Get("q") != "Test Estate Reserve" {
				t.Error("wrong search query")
			}
			writeJSON(w, 200, map[string]any{"data": []grapeCandidate{{ID: 42, Name: "Test Estate Reserve", Color: "red"}}})
		case "/wines/42":
			writeJSON(w, 200, map[string]any{"id": 42, "display_name": "Test Estate Reserve", "description": map[string]string{"text": "A rich wine."}, "grapes": []map[string]string{{"name": "Merlot"}}})
		default:
			t.Error("unexpected provider path", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer provider.Close()
	s := &Store{db: pool, grapeMinds: testGrapeClient(provider)}
	w := call(s, "POST", "/api/bottles/batch", `{"bottle":{"name":"Test Estate Reserve","region":"France","type":"Red","rack":"C","vintage":2020},"slots":[27,28]}`)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var added []Bottle
	json.Unmarshal(w.Body.Bytes(), &added)
	if calls.Load() != 2 {
		t.Fatal("batch should use exactly one search and one detail request", calls.Load())
	}
	info := testWineInfo(t, s, added[0].ID)
	if info.Status != "ready" || info.SourceID == nil || *info.SourceID != 42 || info.FetchedAt == nil || !strings.Contains(string(info.Data), "Merlot") {
		t.Fatal("missing persisted information", info)
	}
	restarted := &Store{db: pool, grapeMinds: testGrapeClient(provider)}
	if testWineInfo(t, restarted, added[1].ID).Status != "ready" || calls.Load() != 2 {
		t.Fatal("opening a matching bottle made provider requests")
	}
	w = call(s, "POST", "/api/bottles", `{"name":"Test Estate Reserve","region":"France","type":"Red","rack":"C","vintage":2020,"slot":29}`)
	if w.Code != 201 || calls.Load() != 2 {
		t.Fatal("adding another matching bottle did not use cache", w.Code, calls.Load())
	}
	path := "/api/bottles/" + added[0].ID + "/information"
	w = call(restarted, "POST", path, `{}`)
	if w.Code != 200 || calls.Load() != 3 {
		t.Fatal("reload should fetch details only", w.Code, calls.Load())
	}
	fail.Store(true)
	w = call(restarted, "POST", path, `{}`)
	info = testWineInfo(t, restarted, added[0].ID)
	if w.Code != 200 || info.Status != "failed" || info.FetchedAt == nil || !strings.Contains(string(info.Data), "Merlot") {
		t.Fatal("failed refresh discarded saved information", info)
	}
}

func TestGrapeMindsAmbiguityUsesCachedSearch(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/wines/search" {
			writeJSON(w, 200, map[string]any{"data": []grapeCandidate{{ID: 1, Name: "Estate Reserve Red", Color: "red"}, {ID: 2, Name: "Estate Reserve Special", Color: "red"}}})
			return
		}
		if r.URL.Path == "/wines/2" {
			writeJSON(w, 200, map[string]any{"data": map[string]any{"id": 2, "display_name": "Estate Reserve Special"}})
			return
		}
		t.Error("unexpected request", r.URL.Path)
		w.WriteHeader(404)
	}))
	defer provider.Close()
	s := &Store{db: pool, grapeMinds: testGrapeClient(provider)}
	w := call(s, "POST", "/api/bottles", `{"name":"Estate Reserve","region":"France","type":"Red","rack":"C","vintage":2020,"slot":29}`)
	var bottle Bottle
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &bottle)
	info := testWineInfo(t, s, bottle.ID)
	if info.Status != "needs_match" || len(info.Candidates) != 2 || calls.Load() != 1 {
		t.Fatal("ambiguous result incorrectly attached", info, calls.Load())
	}
	path := "/api/bottles/" + bottle.ID + "/information"
	w = call(s, "POST", path+"/search", `{"query":"Estate Reserve"}`)
	if w.Code != 200 || calls.Load() != 1 {
		t.Fatal("repeated search should use cache")
	}
	w = call(s, "POST", path, `{"sourceId":2}`)
	if w.Code != 200 || calls.Load() != 2 || testWineInfo(t, s, bottle.ID).Status != "ready" {
		t.Fatal("match selection should use just one detail request", w.Code, calls.Load())
	}
}

func TestGrapeMindsMissingKeyKeepsBottleAndExistingInventoryRetryable(t *testing.T) {
	pool := testPool(t)
	if err := migrate(context.Background(), pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	var existingID string
	if err := pool.QueryRow(context.Background(), "SELECT id::text FROM bottles LIMIT 1").Scan(&existingID); err != nil {
		t.Fatal(err)
	}
	if testWineInfo(t, s, existingID).Status != "not_fetched" {
		t.Fatal("existing inventory should be retryable")
	}
	w := call(s, "POST", "/api/bottles", `{"name":"No key wine","region":"France","type":"Red","rack":"C","vintage":2020,"slot":29}`)
	if w.Code != 201 {
		t.Fatal("API configuration failure prevented bottle save", w.Code)
	}
	var bottle Bottle
	json.Unmarshal(w.Body.Bytes(), &bottle)
	if testWineInfo(t, s, bottle.ID).Status != "not_configured" {
		t.Fatal("missing setup message")
	}
}

func TestGrapeMindsRejectsBadProviderResponses(t *testing.T) {
	for _, status := range []int{401, 403, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				w.Write([]byte("secret provider body"))
			}))
			defer provider.Close()
			_, err := testGrapeClient(provider).request(context.Background(), "GET", "/wines/search?q=wine")
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatal("provider error not sanitized", err)
			}
		})
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("not JSON")) }))
	defer provider.Close()
	if _, err := testGrapeClient(provider).request(context.Background(), "GET", "/wines/1"); err == nil {
		t.Fatal("malformed response accepted")
	}
	if exactCandidate("Château Margaux", "Red", []grapeCandidate{{ID: 1, Name: "Chateau Margaux", Color: "red"}, {ID: 2, Name: "Chateau Margaux", Color: "red"}}) != 0 {
		t.Fatal("duplicate exact matches must need selection")
	}
}

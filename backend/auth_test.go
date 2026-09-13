package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func authCall(handler http.Handler, method, path, body string, cookie *http.Cookie, csrf bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if csrf {
		r.Header.Set("X-WineVault-Request", "1")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestOwnerAuthenticationLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	a := &authService{db: pool, setupToken: "setup-test-token", secure: true}
	handler := a.routes((&Store{db: pool}).dataRoutes())
	for _, path := range []string{"/api/cellar", "/api/history", "/api/health", "/api/wine-scan/status", "/api/barcodes/123"} {
		if w := authCall(handler, "GET", path, "", nil, false); w.Code != 401 {
			t.Fatalf("unprotected %s: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/bottles", "/api/bottles/batch", "/api/wine-scan", "/api/racks", "/api/cellar/layout"} {
		if w := authCall(handler, "POST", path, "{}", nil, true); w.Code != 401 {
			t.Fatalf("unprotected write %s: %d", path, w.Code)
		}
	}
	setup := `{"username":"owner","password":"initial-password-123","setupCode":"setup-test-token"}`
	if w := authCall(handler, "POST", "/api/auth/setup", setup, nil, false); w.Code != 403 {
		t.Fatal("missing CSRF header accepted")
	}
	if w := authCall(handler, "POST", "/api/auth/setup", `{"username":"owner","password":"initial-password-123","setupCode":"wrong"}`, nil, true); w.Code != 403 {
		t.Fatal("wrong setup code accepted")
	}
	w := authCall(handler, "POST", "/api/auth/setup", setup, nil, true)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe session cookie")
	}
	var hash, passwordHash string
	if err := pool.QueryRow(ctx, "SELECT token_hash FROM owner_sessions").Scan(&hash); err != nil || hash == cookie.Value || hash != tokenHash(cookie.Value) {
		t.Fatal("session token stored in plaintext", err)
	}
	if err := pool.QueryRow(ctx, "SELECT password_hash FROM owner_account").Scan(&passwordHash); err != nil || !strings.HasPrefix(passwordHash, "$2a$") {
		t.Fatal("password not hashed", err)
	}
	if w = authCall(handler, "POST", "/api/auth/setup", setup, nil, true); w.Code != 409 {
		t.Fatal("second owner accepted", w.Code)
	}
	if w = authCall(handler, "GET", "/api/cellar", "", cookie, false); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cross := httptest.NewRequest("POST", "/api/auth/logout", nil)
	cross.Header.Set("X-WineVault-Request", "1")
	cross.Header.Set("Sec-Fetch-Site", "cross-site")
	cross.AddCookie(cookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, cross)
	if w.Code != 403 {
		t.Fatal("cross-site write accepted")
	}
	login := `{"username":"owner","password":"initial-password-123"}`
	w = authCall(handler, "POST", "/api/auth/login", login, nil, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	second := w.Result().Cookies()[0]
	if second.Value == cookie.Value {
		t.Fatal("session token reused")
	}
	change := `{"currentPassword":"wrong","newPassword":"replacement-password-123"}`
	if w = authCall(handler, "POST", "/api/auth/password", change, cookie, true); w.Code != 400 {
		t.Fatal("wrong current password accepted")
	}
	change = `{"currentPassword":"initial-password-123","newPassword":"replacement-password-123"}`
	if w = authCall(handler, "POST", "/api/auth/password", change, cookie, true); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, c := range []*http.Cookie{cookie, second} {
		if w = authCall(handler, "GET", "/api/cellar", "", c, false); w.Code != 401 {
			t.Fatal("old session survived password change")
		}
	}
	if w = authCall(handler, "POST", "/api/auth/login", login, nil, true); w.Code != 401 {
		t.Fatal("old password accepted")
	}
	w = authCall(handler, "POST", "/api/auth/login", `{"username":"owner","password":"replacement-password-123"}`, nil, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookie = w.Result().Cookies()[0]
	var state struct {
		Authenticated bool `json:"authenticated"`
		SetupRequired bool `json:"setupRequired"`
	}
	w = authCall(handler, "GET", "/api/auth/status", "", cookie, false)
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil || !state.Authenticated || state.SetupRequired {
		t.Fatal("bad auth status", w.Body.String())
	}
	if w = authCall(handler, "POST", "/api/auth/logout", "", cookie, true); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w = authCall(handler, "GET", "/api/cellar", "", cookie, false); w.Code != 401 {
		t.Fatal("logout did not revoke session")
	}
}

func TestAuthRateLimitAndExpiry(t *testing.T) {
	a := &authService{}
	for i := 0; i < 10; i++ {
		if !a.allowAttempt() {
			t.Fatal("unexpected throttle")
		}
	}
	w := httptest.NewRecorder()
	if !a.throttle(w) || w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal("missing throttle")
	}
	pool := testPool(t)
	ctx := context.Background()
	if err := migrate(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO owner_account VALUES(1,'owner','unused',now())"); err != nil {
		t.Fatal(err)
	}
	token := strings.Repeat("a", 64)
	if _, err := pool.Exec(ctx, "INSERT INTO owner_sessions VALUES($1,1,now()-interval '1 second')", tokenHash(token)); err != nil {
		t.Fatal(err)
	}
	a.db = pool
	handler := a.routes((&Store{db: pool}).dataRoutes())
	if w := authCall(handler, "GET", "/api/cellar", "", &http.Cookie{Name: sessionCookie, Value: token}, false); w.Code != 401 {
		t.Fatal("expired session accepted")
	}
}

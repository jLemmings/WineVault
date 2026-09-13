package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "winevault_session"

type authService struct {
	db         *pgxpool.Pool
	setupToken string
	secure     bool
	mu         sync.Mutex
	attempts   []time.Time
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func newAuthService(db *pgxpool.Pool, secure bool) (*authService, error) {
	a := &authService{db: db, secure: secure}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var exists bool
	if err := db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM owner_account)").Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		token, err := randomToken()
		if err != nil {
			return nil, err
		}
		a.setupToken = token
		log.Printf("WineVault owner setup required. Open the app and use this one-time setup code: %s", token)
	}
	return a, nil
}

// A global limit suits this single-owner app and cannot be bypassed by spoofing proxy IP headers.
func (a *authService) allowAttempt() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	cutoff := time.Now().Add(-time.Minute)
	kept := a.attempts[:0]
	for _, at := range a.attempts {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	a.attempts = kept
	if len(a.attempts) >= 10 {
		return false
	}
	a.attempts = append(a.attempts, time.Now())
	return true
}

func (a *authService) routes(private http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/status", a.status)
	mux.HandleFunc("POST /api/auth/setup", a.setup)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("POST /api/auth/password", a.changePassword)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := a.session(r)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "Please sign in to access your cellar.", 401)
			return
		}
		if err != nil {
			databaseError(w, err)
			return
		}
		private.ServeHTTP(w, r)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// A custom header forces cross-origin browsers to preflight. We never enable CORS.
		if r.Method != "GET" && r.Method != "HEAD" {
			if r.Header.Get("X-WineVault-Request") != "1" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				http.Error(w, "Request origin could not be verified.", 403)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func (a *authService) session(r *http.Request) (string, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || len(cookie.Value) != 64 {
		return "", pgx.ErrNoRows
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var username string
	err = a.db.QueryRow(ctx, `SELECT a.username FROM owner_sessions s JOIN owner_account a ON a.id=s.owner_id WHERE s.token_hash=$1 AND s.expires_at>now()`, tokenHash(cookie.Value)).Scan(&username)
	return username, err
}

func (a *authService) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var exists bool
	if err := a.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM owner_account)").Scan(&exists); err != nil {
		databaseError(w, err)
		return
	}
	username, err := a.session(r)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"setupRequired": !exists, "authenticated": err == nil, "username": username})
}

type authInput struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	SetupCode       string `json:"setupCode"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func readAuth(w http.ResponseWriter, r *http.Request) (authInput, bool) {
	var input authInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "Invalid account request.", 400)
		return input, false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Invalid account request.", 400)
		return input, false
	}
	return input, true
}
func validPassword(password string) bool {
	return utf8.RuneCountInString(password) >= 12 && len(password) <= 72
}
func (a *authService) throttle(w http.ResponseWriter) bool {
	if a.allowAttempt() {
		return false
	}
	w.Header().Set("Retry-After", "60")
	http.Error(w, "Too many account attempts. Wait a minute and try again.", 429)
	return true
}
func (a *authService) setCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: 7 * 24 * 60 * 60, Expires: time.Now().Add(7 * 24 * time.Hour)})
}
func (a *authService) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteStrictMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}
func createSession(ctx context.Context, tx pgx.Tx) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM owner_sessions WHERE expires_at<=now()"); err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, "INSERT INTO owner_sessions(token_hash,owner_id,expires_at) VALUES($1,1,now()+interval '7 days')", tokenHash(token))
	return token, err
}

func (a *authService) setup(w http.ResponseWriter, r *http.Request) {
	if a.throttle(w) {
		return
	}
	input, ok := readAuth(w, r)
	if !ok {
		return
	}
	if a.setupToken == "" || subtle.ConstantTimeCompare([]byte(input.SetupCode), []byte(a.setupToken)) != 1 {
		http.Error(w, "Invalid setup code, or setup is already complete.", 403)
		return
	}
	username := strings.TrimSpace(input.Username)
	if len(username) < 1 || utf8.RuneCountInString(username) > 80 || !validPassword(input.Password) {
		http.Error(w, "Enter a username (1–80 characters) and a password with at least 12 characters and at most 72 UTF-8 bytes.", 400)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		http.Error(w, "Could not secure the password.", 500)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, "INSERT INTO owner_account(id,username,password_hash) VALUES(1,$1,$2)", username, string(hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "The owner account already exists. Sign in instead.", 409)
			return
		}
		databaseError(w, err)
		return
	}
	if _, err = tx.Exec(ctx, "UPDATE cellars SET owner_name=$1", username); err != nil {
		databaseError(w, err)
		return
	}
	token, err := createSession(ctx, tx)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		databaseError(w, err)
		return
	}
	a.setCookie(w, token)
	writeJSON(w, 201, map[string]string{"username": username})
}

func (a *authService) login(w http.ResponseWriter, r *http.Request) {
	if a.throttle(w) {
		return
	}
	input, ok := readAuth(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var username, hash string
	err = tx.QueryRow(ctx, "SELECT username,password_hash FROM owner_account WHERE id=1 FOR UPDATE").Scan(&username, &hash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		databaseError(w, err)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "Complete owner setup first.", 409)
		return
	}
	passwordOK := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) == nil
	if !passwordOK || strings.TrimSpace(input.Username) != username {
		http.Error(w, "Incorrect username or password.", 401)
		return
	}
	token, err := createSession(ctx, tx)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		databaseError(w, err)
		return
	}
	a.setCookie(w, token)
	writeJSON(w, 200, map[string]string{"username": username})
}

func (a *authService) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, err = a.db.Exec(ctx, "DELETE FROM owner_sessions WHERE token_hash=$1", tokenHash(cookie.Value)); err != nil {
			databaseError(w, err)
			return
		}
	}
	a.clearCookie(w)
	w.WriteHeader(204)
}

func (a *authService) changePassword(w http.ResponseWriter, r *http.Request) {
	if a.throttle(w) {
		return
	}
	if _, err := a.session(r); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "Please sign in again.", 401)
		} else {
			databaseError(w, err)
		}
		return
	}
	input, ok := readAuth(w, r)
	if !ok {
		return
	}
	if !validPassword(input.NewPassword) {
		http.Error(w, "Use a new password with at least 12 characters and at most 72 UTF-8 bytes.", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	var hash string
	if err = tx.QueryRow(ctx, "SELECT password_hash FROM owner_account WHERE id=1 FOR UPDATE").Scan(&hash); err != nil {
		databaseError(w, err)
		return
	}
	// Recheck after the account lock in case another password change revoked this session.
	cookie, _ := r.Cookie(sessionCookie)
	var active bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM owner_sessions WHERE token_hash=$1 AND expires_at>now())", tokenHash(cookie.Value)).Scan(&active); err != nil {
		databaseError(w, err)
		return
	}
	if !active {
		http.Error(w, "Please sign in again.", 401)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.CurrentPassword)) != nil {
		http.Error(w, "The current password is incorrect.", 400)
		return
	}
	if input.NewPassword == input.CurrentPassword {
		http.Error(w, "Choose a different new password.", 400)
		return
	}
	nextHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 12)
	if err != nil {
		http.Error(w, "Could not secure the password.", 500)
		return
	}
	if _, err = tx.Exec(ctx, "UPDATE owner_account SET password_hash=$1 WHERE id=1", string(nextHash)); err != nil {
		databaseError(w, err)
		return
	}
	if _, err = tx.Exec(ctx, "DELETE FROM owner_sessions"); err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		databaseError(w, err)
		return
	}
	a.clearCookie(w)
	w.WriteHeader(204)
}

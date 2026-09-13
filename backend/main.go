package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

type Bottle struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Vintage           int    `json:"vintage"`
	Region            string `json:"region"`
	Type              string `json:"type"`
	Rack              string `json:"rack"`
	Slot              int    `json:"slot"`
	Barcode           string `json:"barcode,omitempty"`
	InformationStatus string `json:"informationStatus,omitempty"`
	HasInformation    bool   `json:"hasInformation"`
}
type Rack struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Short    string  `json:"short"`
	Wall     string  `json:"wall"`
	Grapes   string  `json:"grapes"`
	Temp     int     `json:"temp"`
	Capacity int     `json:"capacity"`
	Color    string  `json:"color"`
	Rows     int     `json:"rows"`
	Columns  int     `json:"columns"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Width    float64 `json:"width"`
	Depth    float64 `json:"depth"`
	Rotation int     `json:"rotation"`
}
type Cellar struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Owner    string     `json:"owner"`
	Room     string     `json:"room"`
	Width    float64    `json:"width"`
	Depth    float64    `json:"depth"`
	Revision int        `json:"revision"`
	Layout   RoomLayout `json:"layout"`
	Racks    []Rack     `json:"racks"`
	Bottles  []Bottle   `json:"bottles"`
}
type Store struct {
	db            *pgxpool.Pool
	scanner       *LabelScanner
	barcodeLookup *BarcodeLookup
	grapeMinds    *grapeMindsClient
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func databaseError(w http.ResponseWriter, err error) {
	log.Printf("database request failed: %v", err)
	http.Error(w, "Database unavailable. Please try again.", http.StatusServiceUnavailable)
}

// The cellar response is a consistent snapshot of all data needed by the UI.
func (s *Store) cellar(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	c := Cellar{Racks: []Rack{}, Bottles: []Bottle{}}
	err = tx.QueryRow(ctx, "SELECT id,name,owner_name,room_name,width_m,depth_m,revision,layout FROM cellars ORDER BY id LIMIT 1").Scan(&c.ID, &c.Name, &c.Owner, &c.Room, &c.Width, &c.Depth, &c.Revision, &c.Layout)
	if err != nil {
		databaseError(w, err)
		return
	}
	rows, err := tx.Query(ctx, "SELECT id,name,short_name,wall,grapes,temperature,capacity,color,rows,columns,x,y,width_m,depth_m,rotation FROM racks WHERE cellar_id=$1 ORDER BY position", c.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	for rows.Next() {
		var rack Rack
		if err = rows.Scan(&rack.ID, &rack.Name, &rack.Short, &rack.Wall, &rack.Grapes, &rack.Temp, &rack.Capacity, &rack.Color, &rack.Rows, &rack.Columns, &rack.X, &rack.Y, &rack.Width, &rack.Depth, &rack.Rotation); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		c.Racks = append(c.Racks, rack)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		databaseError(w, err)
		return
	}
	c.Bottles, err = queryBottles(ctx, tx, c.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, c)
}

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func queryBottles(ctx context.Context, db querier, cellarID string) ([]Bottle, error) {
	rows, err := db.Query(ctx, `SELECT b.id::text,b.name,b.vintage,b.region,b.wine_type,b.rack_id,b.slot,b.barcode,COALESCE(i.status,'not_fetched'),COALESCE(i.payload IS NOT NULL AND i.fetched_at IS NOT NULL,false) FROM bottles b JOIN racks r ON r.id=b.rack_id LEFT JOIN wine_information i ON i.id=b.information_id WHERE ($1='' OR r.cellar_id=$1) ORDER BY r.position,b.slot`, cellarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Bottle{}
	for rows.Next() {
		var b Bottle
		if err = rows.Scan(&b.ID, &b.Name, &b.Vintage, &b.Region, &b.Type, &b.Rack, &b.Slot, &b.Barcode, &b.InformationStatus, &b.HasInformation); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}
func validBottle(b Bottle) bool {
	validType := b.Type == "Red" || b.Type == "White" || b.Type == "Rosé" || b.Type == "Sparkling" || b.Type == "Dessert"
	return len(b.Name) > 0 && len(b.Name) <= 150 && len(b.Region) > 0 && len(b.Region) <= 200 && (b.Vintage == 0 || (b.Vintage >= 1900 && b.Vintage <= time.Now().Year()+1)) && b.Slot >= 0 && b.Rack != "" && validType
}
func (s *Store) bottles(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	switch r.Method {
	case http.MethodGet:
		result, err := queryBottles(ctx, s.db, "")
		if err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, 200, result)
	case http.MethodPost:
		var b Bottle
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&b) != nil {
			http.Error(w, "Invalid bottle", 400)
			return
		}
		b.Name = strings.TrimSpace(b.Name)
		b.Region = strings.TrimSpace(b.Region)
		if b.Barcode != "" {
			b.Barcode = normalizeBarcode(b.Barcode)
			if b.Barcode == "" {
				http.Error(w, "Invalid EAN or UPC barcode", 400)
				return
			}
		}
		if !validBottle(b) {
			http.Error(w, "Invalid bottle details", 400)
			return
		}
		err := s.db.QueryRow(ctx, `INSERT INTO bottles(name,vintage,region,wine_type,rack_id,slot,barcode) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id::text`, b.Name, b.Vintage, b.Region, b.Type, b.Rack, b.Slot, b.Barcode).Scan(&b.ID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) {
				switch pgErr.Code {
				case "23505":
					http.Error(w, "This slot is occupied", 409)
					return
				case "23503", "23514":
					http.Error(w, "Invalid rack, slot or bottle details", 400)
					return
				}
			}
			databaseError(w, err)
			return
		}
		s.enrichAfterAdd(b.ID)
		writeJSON(w, 201, b)
	case http.MethodDelete:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id < 1 {
			http.Error(w, "Bottle not found", 404)
			return
		}
		result, err := s.db.Exec(ctx, "DELETE FROM bottles WHERE id=$1", id)
		if err != nil {
			databaseError(w, err)
			return
		}
		if result.RowsAffected() == 0 {
			http.Error(w, "Bottle not found", 404)
			return
		}
		w.WriteHeader(204)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		http.Error(w, "Method not allowed", 405)
	}
}
func (s *Store) dataRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/cellar", s.cellar)
	mux.HandleFunc("GET /api/history", s.history)
	mux.HandleFunc("GET /api/wine-scan/status", s.scanStatus)
	mux.HandleFunc("POST /api/wine-scan", s.scanWine)
	mux.HandleFunc("GET /api/barcodes/{code}", s.lookupBarcode)
	mux.HandleFunc("PUT /api/cellar/layout", s.updateLayout)
	mux.HandleFunc("PUT /api/racks/{id}", s.updateRack)
	mux.HandleFunc("POST /api/racks", s.createRack)
	mux.HandleFunc("DELETE /api/racks/{id}", s.deleteRack)
	mux.HandleFunc("/api/bottles", s.bottles)
	mux.HandleFunc("POST /api/bottles/batch", s.addBottleBatch)
	mux.HandleFunc("PUT /api/bottles/{id}/location", s.moveBottle)
	mux.HandleFunc("GET /api/bottles/{id}/information", s.wineInformation)
	mux.HandleFunc("POST /api/bottles/{id}/information", s.retryWineInformation)
	mux.HandleFunc("POST /api/bottles/{id}/information/search", s.searchWineInformation)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.db.Ping(ctx); err != nil {
			databaseError(w, err)
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok", "database": "connected"})
	})
	return mux
}
func main() {
	// Load local settings before constructing the database and scanner clients.
	// Explicit process environment variables take precedence over .env values.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal("Could not load .env; check its permissions and KEY=value syntax")
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://winevault:winevault_dev@127.0.0.1:5432/winevault?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		log.Fatal("PostgreSQL unavailable; run docker compose up -d --wait: ", err)
	}
	legacyPath := os.Getenv("WINEVAULT_LEGACY_JSON")
	if legacyPath == "" {
		legacyPath = "data/bottles.json"
	}
	if err = migrate(ctx, pool, legacyPath); err != nil {
		log.Fatal("Database migration failed: ", err)
	}
	cancel()
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	store := &Store{db: pool, scanner: newLabelScanner(), barcodeLookup: newBarcodeLookup(), grapeMinds: newGrapeMindsClient()}
	auth, err := newAuthService(pool, os.Getenv("AUTH_COOKIE_SECURE") == "true")
	if err != nil {
		log.Fatal("Authentication initialization failed: ", err)
	}
	server := &http.Server{Addr: addr, Handler: auth.routes(store.dataRoutes()), ReadHeaderTimeout: 5 * time.Second}
	stop, cleanup := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cleanup()
	go func() {
		<-stop.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()
	log.Printf("WineVault API listening on %s (PostgreSQL)", addr)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

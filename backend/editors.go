package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type RoomLayout struct {
	Shape        string  `json:"shape"`
	CutoutWidth  float64 `json:"cutoutWidth"`
	CutoutDepth  float64 `json:"cutoutDepth"`
	Floor        string  `json:"floor"`
	DoorWall     string  `json:"doorWall"`
	DoorOffset   float64 `json:"doorOffset"`
	DoorWidth    float64 `json:"doorWidth"`
	TableEnabled bool    `json:"tableEnabled"`
	TableX       float64 `json:"tableX"`
	TableY       float64 `json:"tableY"`
	TableWidth   float64 `json:"tableWidth"`
	TableDepth   float64 `json:"tableDepth"`
}
type layoutUpdate struct {
	Revision int        `json:"revision"`
	Name     string     `json:"name"`
	Room     string     `json:"room"`
	Width    float64    `json:"width"`
	Depth    float64    `json:"depth"`
	Layout   RoomLayout `json:"layout"`
	Racks    []Rack     `json:"racks"`
}
type rackUpdate struct {
	Revision int    `json:"revision"`
	Name     string `json:"name"`
	Short    string `json:"short"`
	Wall     string `json:"wall"`
	Grapes   string `json:"grapes"`
	Temp     int    `json:"temp"`
	Color    string `json:"color"`
	Rows     int    `json:"rows"`
	Columns  int    `json:"columns"`
}
type editError struct {
	status  int
	message string
}

func (e *editError) Error() string            { return e.message }
func reject(status int, message string) error { return &editError{status, message} }
func respondEditError(w http.ResponseWriter, err error) {
	var e *editError
	if errors.As(err, &e) {
		http.Error(w, e.message, e.status)
	} else {
		databaseError(w, err)
	}
}
func decodeEdit(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		http.Error(w, "Invalid editor data", 400)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Expected one JSON object", 400)
		return false
	}
	return true
}
func lockCellar(ctx context.Context, tx pgx.Tx, revision int) (Cellar, error) {
	var c Cellar
	err := tx.QueryRow(ctx, "SELECT id,name,room_name,width_m,depth_m,revision,layout FROM cellars ORDER BY id LIMIT 1 FOR UPDATE").Scan(&c.ID, &c.Name, &c.Room, &c.Width, &c.Depth, &c.Revision, &c.Layout)
	if err != nil {
		return c, err
	}
	if revision != c.Revision {
		return c, reject(409, "The cellar changed in another editor. Close and reopen this editor to load the latest version.")
	}
	return c, nil
}

type rect struct {
	x, y, w, h float64
	name       string
}

func footprint(r Rack) rect {
	w, h := r.Width, r.Depth
	if r.Rotation == 90 {
		w, h = h, w
	}
	return rect{r.X, r.Y, w, h, "Shelf " + r.ID}
}
func overlap(a, b rect) bool {
	return a.x < b.x+b.w-0.001 && a.x+a.w > b.x+0.001 && a.y < b.y+b.h-0.001 && a.y+a.h > b.y+0.001
}
func inside(a rect, c layoutUpdate) bool {
	if a.x < 0 || a.y < 0 || a.x+a.w > c.Width+0.001 || a.y+a.h > c.Depth+0.001 {
		return false
	}
	return c.Layout.Shape != "l-shape" || !overlap(a, rect{c.Width - c.Layout.CutoutWidth, 0, c.Layout.CutoutWidth, c.Layout.CutoutDepth, "cutout"})
}
func doorArea(c layoutUpdate) rect {
	l := c.Layout
	switch l.DoorWall {
	case "north":
		return rect{l.DoorOffset, 0, l.DoorWidth, 0.5, "door clearance"}
	case "south":
		return rect{l.DoorOffset, c.Depth - 0.5, l.DoorWidth, 0.5, "door clearance"}
	case "west":
		return rect{0, l.DoorOffset, 0.5, l.DoorWidth, "door clearance"}
	default:
		return rect{c.Width - 0.5, l.DoorOffset, 0.5, l.DoorWidth, "door clearance"}
	}
}
func validateLayout(c layoutUpdate) error {
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 100 || strings.TrimSpace(c.Room) == "" || len(c.Room) > 80 {
		return reject(400, "Enter a cellar name and room name.")
	}
	if c.Width < 2 || c.Width > 30 || c.Depth < 2 || c.Depth > 30 {
		return reject(400, "Room dimensions must be between 2 and 30 meters.")
	}
	l := c.Layout
	if l.Shape != "rectangle" && l.Shape != "l-shape" {
		return reject(400, "Choose a rectangular or L-shaped room.")
	}
	if l.Floor != "stone" && l.Floor != "wood" && l.Floor != "concrete" {
		return reject(400, "Choose a valid floor material.")
	}
	if l.Shape == "l-shape" && (l.CutoutWidth < 0.3 || l.CutoutDepth < 0.3 || l.CutoutWidth > c.Width-1 || l.CutoutDepth > c.Depth-1) {
		return reject(400, "The cutout must leave at least a one-meter-wide room.")
	}
	wallLength, start := c.Width, 0.0
	switch l.DoorWall {
	case "north":
		if l.Shape == "l-shape" {
			wallLength -= l.CutoutWidth
		}
	case "south":
	case "west":
		wallLength = c.Depth
	case "east":
		wallLength = c.Depth
		if l.Shape == "l-shape" {
			start = l.CutoutDepth
		}
	default:
		return reject(400, "Choose a door wall.")
	}
	if l.DoorWidth < 0.6 || l.DoorWidth > 2 || l.DoorOffset < start || l.DoorOffset+l.DoorWidth > wallLength+0.001 {
		return reject(400, "The door must fit on the selected outer wall.")
	}
	objects := []rect{}
	for _, r := range c.Racks {
		if r.Width < 0.2 || r.Depth < 0.2 || r.Width > 10 || r.Depth > 10 || (r.Rotation != 0 && r.Rotation != 90) {
			return reject(400, "Shelf footprints must be between 0.2 and 10 meters.")
		}
		objects = append(objects, footprint(r))
	}
	if l.TableEnabled {
		if l.TableWidth < 0.4 || l.TableDepth < 0.4 || l.TableWidth > 5 || l.TableDepth > 5 {
			return reject(400, "Table dimensions must be between 0.4 and 5 meters.")
		}
		objects = append(objects, rect{l.TableX, l.TableY, l.TableWidth, l.TableDepth, "Tasting table"})
	}
	for i, a := range objects {
		if !inside(a, c) {
			return reject(400, a.name+" is outside the room. Move it or enlarge the room.")
		}
		if overlap(a, doorArea(c)) {
			return reject(400, a.name+" blocks the door. Leave 0.5 meters clear inside the entrance.")
		}
		for _, b := range objects[:i] {
			if overlap(a, b) {
				return reject(400, a.name+" overlaps "+b.name+".")
			}
		}
	}
	return nil
}
func (s *Store) updateLayout(w http.ResponseWriter, r *http.Request) {
	var input layoutUpdate
	if !decodeEdit(w, r, &input) {
		return
	}
	if err := validateLayout(input); err != nil {
		respondEditError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	c, err := lockCellar(ctx, tx, input.Revision)
	if err != nil {
		respondEditError(w, err)
		return
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM racks WHERE cellar_id=$1", c.ID).Scan(&count); err != nil {
		databaseError(w, err)
		return
	}
	if len(input.Racks) != count {
		http.Error(w, "The shelf list changed. Reopen the editor.", 409)
		return
	}
	seen := map[string]bool{}
	for _, rack := range input.Racks {
		if seen[rack.ID] {
			http.Error(w, "Duplicate shelf in layout", 400)
			return
		}
		seen[rack.ID] = true
		result, err := tx.Exec(ctx, "UPDATE racks SET x=$1,y=$2,width_m=$3,depth_m=$4,rotation=$5 WHERE id=$6 AND cellar_id=$7", rack.X, rack.Y, rack.Width, rack.Depth, rack.Rotation, rack.ID, c.ID)
		if err != nil {
			databaseError(w, err)
			return
		}
		if result.RowsAffected() != 1 {
			http.Error(w, "Unknown shelf in layout", 400)
			return
		}
	}
	_, err = tx.Exec(ctx, "UPDATE cellars SET name=$1,room_name=$2,width_m=$3,depth_m=$4,layout=$5,revision=revision+1 WHERE id=$6", strings.TrimSpace(input.Name), strings.TrimSpace(input.Room), input.Width, input.Depth, input.Layout, c.ID)
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"revision": c.Revision + 1})
}
func validateRack(input *rackUpdate) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Short = strings.TrimSpace(input.Short)
	input.Wall = strings.TrimSpace(input.Wall)
	if input.Name == "" || len(input.Name) > 100 || input.Short == "" || len(input.Short) > 40 || input.Wall == "" || len(input.Wall) > 60 || len(input.Grapes) > 200 {
		return reject(400, "Enter a shelf name, short label and location within the field limits.")
	}
	if input.Rows < 1 || input.Rows > 20 || input.Columns < 1 || input.Columns > 20 {
		return reject(400, "Choose between 1 and 20 rows and columns.")
	}
	if input.Temp < 4 || input.Temp > 22 {
		return reject(400, "Temperature target must be between 4 and 22°C.")
	}
	if input.Color != "red" && input.Color != "white" && input.Color != "gold" {
		return reject(400, "Choose a valid shelf color.")
	}
	return nil
}
func (s *Store) updateRack(w http.ResponseWriter, r *http.Request) {
	var input rackUpdate
	if !decodeEdit(w, r, &input) {
		return
	}
	if err := validateRack(&input); err != nil {
		respondEditError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	c, err := lockCellar(ctx, tx, input.Revision)
	if err != nil {
		respondEditError(w, err)
		return
	}
	var id string
	err = tx.QueryRow(ctx, "SELECT id FROM racks WHERE id=$1 AND cellar_id=$2 FOR UPDATE", r.PathValue("id"), c.ID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "Shelf not found", 404)
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	// Lock the slot rows as well: new bottle inserts must not race with capacity reduction.
	if _, err = tx.Exec(ctx, "SELECT slot FROM rack_slots WHERE rack_id=$1 FOR UPDATE", id); err != nil {
		databaseError(w, err)
		return
	}
	capacity := input.Rows * input.Columns
	var occupied bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM bottles WHERE rack_id=$1 AND slot >= $2)", id, capacity).Scan(&occupied); err != nil {
		databaseError(w, err)
		return
	}
	if occupied {
		http.Error(w, "This would remove occupied slots. Keep a larger shelf or empty those slots first. No bottles were changed.", 409)
		return
	}
	_, err = tx.Exec(ctx, "UPDATE racks SET name=$1,short_name=$2,wall=$3,grapes=$4,temperature=$5,color=$6,rows=$7,columns=$8,capacity=$9 WHERE id=$10", input.Name, input.Short, input.Wall, input.Grapes, input.Temp, input.Color, input.Rows, input.Columns, capacity, id)
	if err == nil {
		_, err = tx.Exec(ctx, "DELETE FROM rack_slots WHERE rack_id=$1 AND slot >= $2", id, capacity)
	}
	if err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO rack_slots SELECT $1,generate_series(0,$2::integer-1) ON CONFLICT DO NOTHING", id, capacity)
	}
	if err == nil {
		_, err = tx.Exec(ctx, "UPDATE cellars SET revision=revision+1 WHERE id=$1", c.ID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"revision": c.Revision + 1})
}
func (s *Store) createRack(w http.ResponseWriter, r *http.Request) {
	var input rackUpdate
	if !decodeEdit(w, r, &input) {
		return
	}
	if err := validateRack(&input); err != nil {
		respondEditError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	c, err := lockCellar(ctx, tx, input.Revision)
	if err != nil {
		respondEditError(w, err)
		return
	}
	draft := layoutUpdate{Name: c.Name, Room: c.Room, Width: c.Width, Depth: c.Depth, Layout: c.Layout}
	rows, err := tx.Query(ctx, "SELECT id,x,y,width_m,depth_m,rotation FROM racks WHERE cellar_id=$1", c.ID)
	if err != nil {
		databaseError(w, err)
		return
	}
	for rows.Next() {
		var rack Rack
		if err = rows.Scan(&rack.ID, &rack.X, &rack.Y, &rack.Width, &rack.Depth, &rack.Rotation); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		draft.Racks = append(draft.Racks, rack)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		databaseError(w, err)
		return
	}
	var position int
	if err = tx.QueryRow(ctx, "SELECT coalesce(max(position),0)+1 FROM racks").Scan(&position); err != nil {
		databaseError(w, err)
		return
	}
	id := fmt.Sprintf("S%d", position)
	rack := Rack{ID: id, Width: 1.0, Depth: 0.5}
	found := false
	for y := 0.1; y < c.Depth && !found; y += 0.2 {
		for x := 0.1; x < c.Width; x += 0.2 {
			rack.X = math.Round(x*100) / 100
			rack.Y = math.Round(y*100) / 100
			candidate := draft
			candidate.Racks = append(append([]Rack{}, draft.Racks...), rack)
			if validateLayout(candidate) == nil {
				found = true
				break
			}
		}
	}
	if !found {
		http.Error(w, "No space for a new shelf. Move shelves or enlarge the room in the room editor first.", 409)
		return
	}
	_, err = tx.Exec(ctx, `INSERT INTO racks(id,cellar_id,name,short_name,wall,grapes,temperature,capacity,color,position,rows,columns,x,y,width_m,depth_m,rotation) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,0)`, id, c.ID, input.Name, input.Short, input.Wall, input.Grapes, input.Temp, input.Rows*input.Columns, input.Color, position, input.Rows, input.Columns, rack.X, rack.Y, rack.Width, rack.Depth)
	if err == nil {
		_, err = tx.Exec(ctx, "INSERT INTO rack_slots SELECT $1,generate_series(0,$2::integer-1)", id, input.Rows*input.Columns)
	}
	if err == nil {
		_, err = tx.Exec(ctx, "UPDATE cellars SET revision=revision+1 WHERE id=$1", c.ID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "revision": c.Revision + 1})
}
func (s *Store) deleteRack(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision int `json:"revision"`
	}
	if !decodeEdit(w, r, &input) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	c, err := lockCellar(ctx, tx, input.Revision)
	if err != nil {
		respondEditError(w, err)
		return
	}
	id := r.PathValue("id")
	var existing string
	err = tx.QueryRow(ctx, "SELECT id FROM racks WHERE id=$1 AND cellar_id=$2 FOR UPDATE", id, c.ID).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "Shelf not found", 404)
		return
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	if _, err = tx.Exec(ctx, "SELECT slot FROM rack_slots WHERE rack_id=$1 FOR UPDATE", id); err != nil {
		databaseError(w, err)
		return
	}
	var occupied bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM bottles WHERE rack_id=$1)", id).Scan(&occupied); err != nil {
		databaseError(w, err)
		return
	}
	if occupied {
		http.Error(w, "Only empty shelves can be removed. Your bottles have not been changed.", 409)
		return
	}
	_, err = tx.Exec(ctx, "DELETE FROM rack_slots WHERE rack_id=$1", id)
	if err == nil {
		_, err = tx.Exec(ctx, "DELETE FROM racks WHERE id=$1", id)
	}
	if err == nil {
		_, err = tx.Exec(ctx, "UPDATE cellars SET revision=revision+1 WHERE id=$1", c.ID)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		databaseError(w, err)
		return
	}
	w.WriteHeader(204)
}

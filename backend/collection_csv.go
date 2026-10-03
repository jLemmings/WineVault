package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var csvColumns = []string{"name", "vintage", "region", "type", "quantity", "rack", "slot", "barcode", "price", "currency", "purchase_date", "seller"}

func csvSafe(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if strings.HasPrefix(value, "'") || strings.ContainsAny(strings.TrimSuffix(value, trimmed), "\t\r\n") || (trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0]))) {
		return "'" + value
	}
	return value
}
func csvText(value string) string {
	if strings.HasPrefix(value, "'") {
		rest := value[1:]
		if csvSafe(rest) == value {
			return rest
		}
	}
	return value
}
func formatPrice(value *int64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%d.%02d", *value/100, *value%100)
}
func parsePrice(value string) (*int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return nil, fmt.Errorf("Price must be a non-negative decimal with at most two decimal places.")
	}
	for _, part := range parts {
		for _, c := range part {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("Price must be a non-negative decimal with at most two decimal places.")
			}
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 10000000000 {
		return nil, fmt.Errorf("Price is too large.")
	}
	cents := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) < 1 || len(parts[1]) > 2 {
			return nil, fmt.Errorf("Price must have at most two decimal places.")
		}
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		cents, _ = strconv.ParseInt(fraction, 10, 64)
	}
	total := whole*100 + cents
	return &total, nil
}
func (s *Store) exportCSV(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	bottles, err := queryBottles(ctx, tx, "")
	if err != nil {
		databaseError(w, err)
		return
	}
	rows, err := tx.Query(ctx, "SELECT id,columns FROM racks")
	if err != nil {
		databaseError(w, err)
		return
	}
	columns := map[string]int{}
	for rows.Next() {
		var id string
		var count int
		if err = rows.Scan(&id, &count); err != nil {
			rows.Close()
			databaseError(w, err)
			return
		}
		columns[id] = count
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		databaseError(w, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		databaseError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="winevault-collection.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	writer := csv.NewWriter(w)
	writer.Write(csvColumns)
	for _, b := range bottles {
		vintage := strconv.Itoa(b.Vintage)
		if b.Vintage == 0 {
			vintage = "NV"
		}
		col := columns[b.Rack]
		if col < 1 {
			col = 1
		}
		slot := fmt.Sprintf("%c%d", 'A'+b.Slot%col, b.Slot/col+1)
		values := []string{b.Name, vintage, b.Region, b.Type, "1", b.Rack, slot, b.Barcode, formatPrice(b.PriceMinor), b.Currency, b.PurchaseDate, b.Seller}
		for i, v := range values {
			values[i] = csvSafe(v)
		}
		writer.Write(values)
	}
	writer.Flush()
}

type csvReviewRow struct {
	Price       string   `json:"price"`
	VintageText string   `json:"vintageText"`
	Row         int      `json:"row"`
	Bottle      Bottle   `json:"bottle"`
	Quantity    int      `json:"quantity"`
	SlotAddress string   `json:"slotAddress"`
	Errors      []string `json:"errors"`
}

func (s *Store) previewCSV(w http.ResponseWriter, r *http.Request) {
	reader := csv.NewReader(http.MaxBytesReader(w, r.Body, 2<<20))
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		http.Error(w, "Choose a UTF-8 CSV with a header row.", 400)
		return
	}
	indices := map[string]int{}
	supported := map[string]bool{}
	for _, c := range csvColumns {
		supported[c] = true
	}
	for i, h := range headers {
		h = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		if _, ok := indices[h]; ok {
			http.Error(w, "Duplicate CSV column: "+h, 400)
			return
		}
		if !supported[h] {
			http.Error(w, "Unknown CSV column: "+h+". Use the CSV template.", 400)
			return
		}
		indices[h] = i
	}
	for _, key := range []string{"name", "vintage", "region", "type"} {
		if _, ok := indices[key]; !ok {
			http.Error(w, "Missing CSV column: "+key, 400)
			return
		}
	}
	result := []csvReviewRow{}
	total := 0
	for line := 2; ; line++ {
		record, e := reader.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			http.Error(w, fmt.Sprintf("Could not read CSV row %d: check quotes and file size.", line), 400)
			return
		}
		if len(result) >= 1000 {
			http.Error(w, "Import at most 1,000 CSV rows at a time.", 400)
			return
		}
		row := csvReviewRow{Row: line, Quantity: 1, Errors: []string{}}
		get := func(key string) string {
			idx, ok := indices[key]
			if !ok || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(csvText(record[idx]))
		}
		if len(record) != len(headers) {
			http.Error(w, fmt.Sprintf("CSV row %d has a different number of columns than the header.", line), 400)
			return
		}
		b := Bottle{Name: get("name"), Region: get("region"), Rack: get("rack"), Barcode: get("barcode"), PurchaseData: PurchaseData{Currency: get("currency"), PurchaseDate: get("purchase_date"), Seller: get("seller")}}
		for _, t := range []string{"Red", "White", "Rosé", "Champagne", "Sparkling", "Dessert"} {
			if strings.EqualFold(t, get("type")) {
				b.Type = t
			}
		}
		vintage := get("vintage")
		if strings.EqualFold(vintage, "NV") || vintage == "0" {
			b.Vintage = 0
		} else {
			year, e := strconv.Atoi(vintage)
			if e != nil {
				row.Errors = append(row.Errors, "Enter a vintage year or NV.")
			}
			b.Vintage = year
		}
		if q := get("quantity"); q != "" {
			row.Quantity, _ = strconv.Atoi(q)
		}
		if row.Quantity < 1 || row.Quantity > 400 {
			row.Errors = append(row.Errors, "Quantity must be between 1 and 400.")
		} else {
			total += row.Quantity
		}
		if !validBottle(Bottle{Name: b.Name, Vintage: b.Vintage, Region: b.Region, Type: b.Type, Rack: "review"}) {
			row.Errors = append(row.Errors, "Check name, region, vintage, and wine type.")
		}
		if b.Barcode != "" {
			originalBarcode := b.Barcode
			b.Barcode = normalizeBarcode(b.Barcode)
			if b.Barcode == "" {
				b.Barcode = originalBarcode
				row.Errors = append(row.Errors, "Invalid barcode.")
			}
		}
		b.PriceMinor, e = parsePrice(get("price"))
		if e != nil {
			row.Errors = append(row.Errors, e.Error())
		}
		if e = validatePurchase(&b.PurchaseData); e != nil {
			row.Errors = append(row.Errors, e.Error())
		}
		row.Price = get("price")
		row.VintageText = get("vintage")
		row.Bottle = b
		row.SlotAddress = strings.ToUpper(get("slot"))
		result = append(result, row)
	}
	if len(result) == 0 || total > 2000 {
		http.Error(w, "Import between 1 and 2,000 bottles at a time.", 400)
		return
	}
	writeJSON(w, 200, map[string]any{"rows": result, "totalBottles": total})
}
func (s *Store) importCSV(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Bottles []Bottle `json:"bottles"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF || len(input.Bottles) < 1 || len(input.Bottles) > 2000 {
		http.Error(w, "Review between 1 and 2,000 bottles before importing.", 400)
		return
	}
	seen := map[string]bool{}
	for i := range input.Bottles {
		b := &input.Bottles[i]
		b.Name = strings.TrimSpace(b.Name)
		b.Region = strings.TrimSpace(b.Region)
		if !validBottle(*b) {
			http.Error(w, fmt.Sprintf("Bottle %d has invalid wine details or placement.", i+1), 400)
			return
		}
		if err := validatePurchase(&b.PurchaseData); err != nil {
			http.Error(w, fmt.Sprintf("Bottle %d: %s", i+1, err), 400)
			return
		}
		if b.Barcode != "" {
			b.Barcode = normalizeBarcode(b.Barcode)
			if b.Barcode == "" {
				http.Error(w, "Invalid barcode.", 400)
				return
			}
		}
		key := fmt.Sprintf("%s:%d", b.Rack, b.Slot)
		if seen[key] {
			http.Error(w, "Two imported bottles use the same slot. Review placement.", 400)
			return
		}
		seen[key] = true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		databaseError(w, err)
		return
	}
	defer tx.Rollback(ctx)
	for _, b := range input.Bottles {
		_, err = tx.Exec(ctx, `INSERT INTO bottles(name,vintage,region,wine_type,rack_id,slot,barcode,price_minor,currency,purchased_on,seller) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::date,$11)`, b.Name, b.Vintage, b.Region, b.Type, b.Rack, b.Slot, b.Barcode, b.PriceMinor, b.Currency, b.PurchaseDate, b.Seller)
		if err != nil {
			toolError(w, err)
			return
		}
	}
	if err = tx.Commit(ctx); err != nil {
		databaseError(w, err)
		return
	}
	writeJSON(w, 201, map[string]int{"imported": len(input.Bottles)})
}

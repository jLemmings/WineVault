package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCSVPreviewValidation(t *testing.T) {
	s := &Store{}
	for _, body := range []string{"name,name,vintage,region,type\na,b,2020,France,Red\n", "name,region,type\na,France,Red\n", "name,vintage,region,type,unknown\na,2020,France,Red,x\n", "name,vintage,region,type\n\"broken,2020,France,Red\n"} {
		if w := call(s, "POST", "/api/collection/import/preview", body); w.Code != 400 {
			t.Fatal("invalid CSV accepted", w.Code, w.Body.String())
		}
	}
	body := "\ufeffname,vintage,region,type,quantity,price,currency,purchase_date,seller\n\"Estate, Reserve\",NV,France,white,2,12.35,chf,2026-01-12,\"Shop, Zurich\"\n"
	w := call(s, "POST", "/api/collection/import/preview", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var preview struct {
		Rows []csvReviewRow `json:"rows"`
	}
	json.Unmarshal(w.Body.Bytes(), &preview)
	row := preview.Rows[0]
	if row.Bottle.Name != "Estate, Reserve" || row.Bottle.Type != "White" || row.Bottle.Vintage != 0 || row.Quantity != 2 || *row.Bottle.PriceMinor != 1235 || len(row.Errors) != 0 {
		t.Fatal("bad CSV preview", row)
	}
	w = call(s, "POST", "/api/collection/import/preview", "name,vintage,region,type,price,barcode\nWine,unknown,France,Red,12.345,invalid\n")
	json.Unmarshal(w.Body.Bytes(), &preview)
	row = preview.Rows[0]
	if row.Price != "12.345" || row.VintageText != "unknown" || row.Bottle.Barcode != "invalid" || len(row.Errors) < 3 {
		t.Fatal("invalid data lost before review", row)
	}
	for _, value := range []string{"=HYPERLINK(\"x\")", " +formula", "@formula", "-formula", "\tformula", "'literal", "Normal wine"} {
		if got := csvText(csvSafe(value)); got != value {
			t.Fatalf("unsafe or lossy CSV text %q -> %q", value, got)
		}
	}
	for _, value := range []string{"-1", "NaN", "1e3", "1.234", "999999999999999999999"} {
		if _, err := parsePrice(value); err == nil {
			t.Fatal("invalid price accepted", value)
		}
	}
}

func TestCSVImportPurchaseLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if err := migrateFixture(ctx, pool, ""); err != nil {
		t.Fatal(err)
	}
	s := &Store{db: pool}
	amount := int64(1235)
	b := Bottle{Name: "=Estate, Reserve", Vintage: 2020, Region: "France", Type: "White", Rack: "C", Slot: 28, PurchaseData: PurchaseData{PriceMinor: &amount, Currency: "CHF", PurchaseDate: "2026-01-12", Seller: "Shop, Zurich"}}
	payload := func(bs []Bottle) string { data, _ := json.Marshal(map[string]any{"bottles": bs}); return string(data) }
	conflict := b
	conflict.Slot = 0
	w := call(s, "POST", "/api/collection/import", payload([]Bottle{b, conflict}))
	if w.Code != 409 {
		t.Fatal("occupied import accepted", w.Code, w.Body.String())
	}
	var count int
	pool.QueryRow(ctx, "SELECT count(*) FROM bottles WHERE name=$1", b.Name).Scan(&count)
	if count != 0 {
		t.Fatal("partial import committed")
	}
	if w = call(s, "POST", "/api/collection/import", payload([]Bottle{b, b})); w.Code != 400 {
		t.Fatal("duplicate requested slots accepted")
	}
	second := b
	second.Slot = 29
	second.Currency = "EUR"
	second.PurchaseDate = ""
	secondAmount := int64(900)
	second.PriceMinor = &secondAmount
	if w = call(s, "POST", "/api/collection/import", payload([]Bottle{b, second})); w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	list, err := queryBottles(ctx, pool, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range list {
		if stored.Rack == "C" && stored.Slot == 28 {
			b = stored
		}
	}
	if b.ID == "" || b.PriceMinor == nil || *b.PriceMinor != 1235 || b.PurchaseDate != "2026-01-12" {
		t.Fatal("purchase not persisted", b)
	}
	// Export safely quotes commas and neutralizes spreadsheet formulas, then previews losslessly.
	w = call(s, "GET", "/api/collection/export", "")
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "winevault-collection.csv") {
		t.Fatal("export failed", w.Code)
	}
	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, record := range records[1:] {
		if record[0] == "'=Estate, Reserve" {
			found = true
			if record[8] != "12.35" && record[8] != "9.00" {
				t.Fatal("price export not exact", record)
			}
		}
	}
	if !found {
		t.Fatal("formula text was not escaped")
	}
	preview := call(s, "POST", "/api/collection/import/preview", w.Body.String())
	if preview.Code != 200 {
		t.Fatal("export cannot be previewed", preview.Body.String())
	}
	// Purchase updates reject stale edits; consumption keeps costs and restore does not double count.
	update := fmt.Sprintf(`{"priceMinor":1550,"currency":"CHF","purchaseDate":"2026-02-02","seller":"Updated shop","revision":%d}`, b.Revision)
	if w = call(s, "PUT", "/api/bottles/"+b.ID+"/purchase", update); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = call(s, "PUT", "/api/bottles/"+b.ID+"/purchase", update); w.Code != 409 {
		t.Fatal("stale purchase update accepted")
	}
	if w = call(s, "DELETE", "/api/bottles?id="+b.ID, ""); w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	type purchaseEntry struct {
		PurchaseData
		InCellar bool `json:"inCellar"`
	}
	var purchases []purchaseEntry
	w = call(s, "GET", "/api/purchases", "")
	json.Unmarshal(w.Body.Bytes(), &purchases)
	if len(purchases) != 2 {
		t.Fatal("spending records lost", w.Body.String())
	}
	found = false
	for _, p := range purchases {
		if p.Currency == "CHF" {
			found = true
			if p.InCellar || *p.PriceMinor != 1550 {
				t.Fatal("enjoyment lost purchase", p)
			}
		}
	}
	if !found {
		t.Fatal("missing CHF purchase")
	}
	if w = call(s, "POST", "/api/bottles/"+b.ID+"/restore", `{"rack":"C","slot":28}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call(s, "GET", "/api/purchases", "")
	json.Unmarshal(w.Body.Bytes(), &purchases)
	if len(purchases) != 2 {
		t.Fatal("restore double-counted spending")
	}
	for _, p := range purchases {
		if p.Currency == "CHF" && (!p.InCellar || *p.PriceMinor != 1550) {
			t.Fatal("restore lost purchase details", p)
		}
	}
	// Invalid metadata cannot be persisted by direct add, batch, or import.
	bad := b
	bad.Slot = 27
	bad.Currency = "INVALID"
	if w = call(s, "POST", "/api/collection/import", payload([]Bottle{bad})); w.Code != 400 {
		t.Fatal("invalid currency accepted")
	}
	batch, _ := json.Marshal(map[string]any{"bottle": bad, "slots": []int{27}})
	if w = call(s, "POST", "/api/bottles/batch", string(batch)); w.Code != 400 {
		t.Fatal("invalid batch purchase accepted")
	}
	direct, _ := json.Marshal(bad)
	if w = call(s, "POST", "/api/bottles", string(direct)); w.Code != 400 {
		t.Fatal("invalid direct purchase accepted")
	}
}

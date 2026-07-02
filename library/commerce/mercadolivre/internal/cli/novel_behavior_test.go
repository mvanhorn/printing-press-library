// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the six novel commands against a seeded temp store.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/mlextract"
)

// seedStore points the CLI's default data dir at a temp dir and populates it
// with fixture-derived products, attributes, listings and price snapshots.
func seedStore(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MERCADOLIVRE_DATA_DIR", dir)

	s, err := openLocalStore(context.Background())
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}
	defer s.Close()

	// Two products with prices and brands.
	p1 := mlextract.ProductDetail{CatalogID: "MLB51764304", Name: "Notebook Gamer Rog Strix", Brand: "Asus", Price: 13349.11, Currency: "BRL", ReviewCount: 71}
	p2 := mlextract.ProductDetail{CatalogID: "MLB40287816", Name: "Notebook Gamer Lenovo LOQ", Brand: "Lenovo", Price: 5794.90, Currency: "BRL"}
	if _, _, err := s.UpsertBatch("products", []json.RawMessage{p1.StoreJSON(), p2.StoreJSON()}); err != nil {
		t.Fatalf("seed products: %v", err)
	}

	// Listings (used by cheapest); names carry "notebook".
	l1 := mlextract.Listing{CatalogID: "MLB51764304", Name: "Notebook Gamer Rog Strix", Brand: "Asus", Price: 13349.11, Currency: "BRL"}
	l2 := mlextract.Listing{CatalogID: "MLB40287816", Name: "Notebook Gamer Lenovo LOQ", Brand: "Lenovo", Price: 5794.90, Currency: "BRL"}
	if _, _, err := s.UpsertBatch("listings", []json.RawMessage{l1.StoreJSON(), l2.StoreJSON()}); err != nil {
		t.Fatalf("seed listings: %v", err)
	}

	// Attributes: shared value on brand-graphics, differing on RAM + SSD.
	if err := s.UpsertAttributes("MLB51764304", []mlextract.Attribute{
		{Name: "Marca de placa gráfica dedicada", Value: "NVIDIA"},
		{Name: "Memória RAM", Value: "32 GB"},
		{Name: "Capacidade de disco SSD", Value: "1 TB"},
	}); err != nil {
		t.Fatalf("seed attrs 1: %v", err)
	}
	if err := s.UpsertAttributes("MLB40287816", []mlextract.Attribute{
		{Name: "Marca de placa gráfica dedicada", Value: "NVIDIA"},
		{Name: "Memória RAM", Value: "8 GB"},
		{Name: "Capacidade de disco SSD", Value: "512 GB"},
	}); err != nil {
		t.Fatalf("seed attrs 2: %v", err)
	}

	// Price snapshots. MLB51764304 fresh (now); a dedicated dispersion id with
	// three snapshots; a history id with three timed snapshots; a stale id.
	now := time.Now()
	must := func(err error) {
		if err != nil {
			t.Fatalf("seed snapshot: %v", err)
		}
	}
	must(s.InsertPriceSnapshotAt("MLB51764304", 13349.11, "BRL", now))
	must(s.InsertPriceSnapshotAt("DISP1", 100, "BRL", now.Add(-72*time.Hour)))
	must(s.InsertPriceSnapshotAt("DISP1", 200, "BRL", now.Add(-48*time.Hour)))
	must(s.InsertPriceSnapshotAt("DISP1", 300, "BRL", now.Add(-24*time.Hour)))
	must(s.InsertPriceSnapshotAt("HIST1", 100, "BRL", now.Add(-72*time.Hour)))
	must(s.InsertPriceSnapshotAt("HIST1", 150, "BRL", now.Add(-48*time.Hour)))
	must(s.InsertPriceSnapshotAt("HIST1", 120, "BRL", now.Add(-24*time.Hour)))
	// MLB40287816 has an OLD snapshot -> stale.
	must(s.InsertPriceSnapshotAt("MLB40287816", 5794.90, "BRL", now.Add(-10*24*time.Hour)))
}

// runNovelCLI runs the CLI with args, returning stdout only (stderr captured
// separately so JSON on stdout stays clean).
func runNovelCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := RootCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestCompareBehavior(t *testing.T) {
	seedStore(t)
	out, err := runNovelCLI(t, "compare", "MLB51764304", "MLB40287816", "--json")
	if err != nil {
		t.Fatalf("compare error: %v\noutput:\n%s", err, out)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("compare output not JSON array: %v\n%s", err, out)
	}
	if len(rows) == 0 {
		t.Fatalf("compare returned no rows")
	}
	// Every row must carry the attribute label plus a column per product.
	sawPrice := false
	sawRAM := false
	for _, r := range rows {
		attr, _ := r["attribute"].(string)
		if _, ok := r["MLB51764304"]; !ok {
			t.Errorf("row %q missing MLB51764304 column", attr)
		}
		if _, ok := r["MLB40287816"]; !ok {
			t.Errorf("row %q missing MLB40287816 column", attr)
		}
		switch attr {
		case "price":
			sawPrice = true
			if r["MLB51764304"] != "13349.11" {
				t.Errorf("price row MLB51764304 = %v, want 13349.11", r["MLB51764304"])
			}
		case "Memória RAM":
			sawRAM = true
			if r["MLB51764304"] != "32 GB" || r["MLB40287816"] != "8 GB" {
				t.Errorf("RAM row = %v / %v, want 32 GB / 8 GB", r["MLB51764304"], r["MLB40287816"])
			}
		}
	}
	if !sawPrice {
		t.Error("compare matrix missing price row")
	}
	if !sawRAM {
		t.Error("compare matrix missing RAM attribute row")
	}

	// --diff must hide the shared graphics-brand row (both NVIDIA).
	outDiff, err := runNovelCLI(t, "compare", "MLB51764304", "MLB40287816", "--diff", "--json")
	if err != nil {
		t.Fatalf("compare --diff error: %v", err)
	}
	if strings.Contains(outDiff, "Marca de placa gráfica dedicada") {
		t.Errorf("--diff should hide the identical graphics-brand row:\n%s", outDiff)
	}
	if !strings.Contains(outDiff, "Memória RAM") {
		t.Errorf("--diff should keep the differing RAM row:\n%s", outDiff)
	}
}

func TestCompareTooFewArgs(t *testing.T) {
	seedStore(t)
	// One id -> help (no error, no crash).
	if _, err := runNovelCLI(t, "compare", "MLB51764304"); err != nil {
		t.Fatalf("compare with 1 id should render help, got error: %v", err)
	}
}

func TestCheapestBehavior(t *testing.T) {
	seedStore(t)
	out, err := runNovelCLI(t, "cheapest", "--query", "notebook", "--spec", "RAM>=16", "--json")
	if err != nil {
		t.Fatalf("cheapest error: %v\n%s", err, out)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("cheapest output not JSON array: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("cheapest --spec RAM>=16 expected 1 match, got %d: %s", len(rows), out)
	}
	if rows[0]["catalog_id"] != "MLB51764304" {
		t.Errorf("cheapest match = %v, want MLB51764304 (only one with >=16GB RAM)", rows[0]["catalog_id"])
	}

	// No spec -> both listings, cheapest first.
	outAll, err := runNovelCLI(t, "cheapest", "--query", "notebook", "--json")
	if err != nil {
		t.Fatalf("cheapest all error: %v", err)
	}
	var allRows []map[string]any
	if err := json.Unmarshal([]byte(outAll), &allRows); err != nil {
		t.Fatalf("cheapest all not JSON: %v\n%s", err, outAll)
	}
	if len(allRows) != 2 {
		t.Fatalf("cheapest no-spec expected 2 rows, got %d", len(allRows))
	}
	if allRows[0]["catalog_id"] != "MLB40287816" {
		t.Errorf("cheapest first row = %v, want MLB40287816 (lowest price)", allRows[0]["catalog_id"])
	}

	// Impossible floor -> honest empty [].
	outNone, err := runNovelCLI(t, "cheapest", "--query", "notebook", "--spec", "RAM>=999", "--json")
	if err != nil {
		t.Fatalf("cheapest none error: %v", err)
	}
	var noneRows []map[string]any
	if err := json.Unmarshal([]byte(outNone), &noneRows); err != nil {
		t.Fatalf("cheapest none not JSON: %v\n%s", err, outNone)
	}
	if len(noneRows) != 0 {
		t.Errorf("cheapest RAM>=999 should be empty, got %d rows", len(noneRows))
	}
}

func TestDispersionBehavior(t *testing.T) {
	seedStore(t)
	out, err := runNovelCLI(t, "dispersion", "DISP1", "--json")
	if err != nil {
		t.Fatalf("dispersion error: %v\n%s", err, out)
	}
	var stats struct {
		Count  int      `json:"count"`
		Min    *float64 `json:"min"`
		Max    *float64 `json:"max"`
		Median *float64 `json:"median"`
		Mean   *float64 `json:"mean"`
		Stddev *float64 `json:"stddev"`
	}
	if err := json.Unmarshal([]byte(out), &stats); err != nil {
		t.Fatalf("dispersion output not JSON object: %v\n%s", err, out)
	}
	if stats.Count != 3 {
		t.Fatalf("dispersion count = %d, want 3", stats.Count)
	}
	if stats.Min == nil || *stats.Min != 100 {
		t.Errorf("dispersion min = %v, want 100", stats.Min)
	}
	if stats.Max == nil || *stats.Max != 300 {
		t.Errorf("dispersion max = %v, want 300", stats.Max)
	}
	if stats.Median == nil || *stats.Median != 200 {
		t.Errorf("dispersion median = %v, want 200", stats.Median)
	}
	if stats.Mean == nil || *stats.Mean != 200 {
		t.Errorf("dispersion mean = %v, want 200", stats.Mean)
	}

	// Unknown id -> honest empty (count 0, non-empty note).
	outEmpty, _ := runNovelCLI(t, "dispersion", "NOPE", "--json")
	if !strings.Contains(outEmpty, `"count": 0`) {
		t.Errorf("dispersion for unknown id should report count 0:\n%s", outEmpty)
	}
}

func TestPriceHistoryBehavior(t *testing.T) {
	seedStore(t)
	out, err := runNovelCLI(t, "price-history", "HIST1", "--json")
	if err != nil {
		t.Fatalf("price-history error: %v\n%s", err, out)
	}
	var rows []struct {
		Price      float64 `json:"price"`
		DeltaPrev  float64 `json:"delta_prev"`
		DeltaFirst float64 `json:"delta_first"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("price-history output not JSON array: %v\n%s", err, out)
	}
	if len(rows) != 3 {
		t.Fatalf("price-history expected 3 points, got %d", len(rows))
	}
	if rows[0].Price != 100 || rows[0].DeltaPrev != 0 {
		t.Errorf("point0 = %+v, want price 100 delta_prev 0", rows[0])
	}
	if rows[1].DeltaPrev != 50 || rows[1].DeltaFirst != 50 {
		t.Errorf("point1 deltas = %v/%v, want 50/50", rows[1].DeltaPrev, rows[1].DeltaFirst)
	}
	if rows[2].DeltaPrev != -30 || rows[2].DeltaFirst != 20 {
		t.Errorf("point2 deltas = %v/%v, want -30/20", rows[2].DeltaPrev, rows[2].DeltaFirst)
	}
}

func TestCotacaoBehavior(t *testing.T) {
	seedStore(t)
	// JSON structured form.
	out, err := runNovelCLI(t, "cotacao", "MLB51764304", "--json")
	if err != nil {
		t.Fatalf("cotacao error: %v\n%s", err, out)
	}
	var raw []map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("cotacao output not JSON array: %v\n%s", err, out)
	}
	if len(raw) != 1 {
		t.Fatalf("cotacao expected 1 item, got %d", len(raw))
	}
	if raw[0]["name"] != "Notebook Gamer Rog Strix" {
		t.Errorf("cotacao name = %v", raw[0]["name"])
	}
	specs, ok := raw[0]["specs"].(map[string]any)
	if !ok || len(specs) == 0 {
		t.Errorf("cotacao item missing specs: %v", raw[0]["specs"])
	}

	// Markdown default form.
	outMD, err := runNovelCLI(t, "cotacao", "MLB51764304")
	if err != nil {
		t.Fatalf("cotacao md error: %v", err)
	}
	if !strings.Contains(outMD, "| Catalog ID |") || !strings.Contains(outMD, "MLB51764304") {
		t.Errorf("cotacao markdown missing table/row:\n%s", outMD)
	}
}

func TestStaleBehavior(t *testing.T) {
	seedStore(t)
	out, err := runNovelCLI(t, "stale", "--older-than", "7d", "--json")
	if err != nil {
		t.Fatalf("stale error: %v\n%s", err, out)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("stale output not JSON array: %v\n%s", err, out)
	}
	staleIDs := map[string]bool{}
	for _, r := range rows {
		if id, ok := r["catalog_id"].(string); ok {
			staleIDs[id] = true
		}
	}
	// MLB40287816 snapshot is 10 days old -> stale.
	if !staleIDs["MLB40287816"] {
		t.Errorf("stale should include MLB40287816 (10-day-old snapshot); got %v", staleIDs)
	}
	// MLB51764304 snapshot is fresh (now) -> not stale.
	if staleIDs["MLB51764304"] {
		t.Errorf("stale must NOT include the freshly-snapshotted MLB51764304")
	}
}

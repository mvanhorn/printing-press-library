package scraper

import (
	"os"
	"path/filepath"
	"testing"
)

func readSample(t *testing.T, name string) string {
	t.Helper()
	// Use absolute path relative to the run's discovery samples.
	candidates := []string{
		filepath.Join("..", "..", "..", "..", "discovery", "samples", name),
		filepath.Join("/Users/blake/printing-press/.runstate/cli-printing-press-65bbd03c/runs/20260519-104537/discovery/samples", name),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil {
			return string(b)
		}
	}
	t.Skipf("sample %s not found", name)
	return ""
}

func TestParseRacquetDetail(t *testing.T) {
	html := readSample(t, "new-detail.html")
	if html == "" {
		return
	}
	r, err := ParseRacquetDetail(html, "WB9810", "https://www.tennis-warehouse.com/Wilson_Blade_98_16x19_v10/descpageRCWILSON-WB9810.html", "Wilson")
	if err != nil {
		t.Fatalf("ParseRacquetDetail: %v", err)
	}
	if r.SKU != "WB9810" {
		t.Errorf("SKU: got %q", r.SKU)
	}
	if r.Brand != "Wilson" {
		t.Errorf("Brand: got %q", r.Brand)
	}
	if r.Model == "" {
		t.Errorf("Model: empty")
	}
	if r.HeadSizeIn2 != 98 {
		t.Errorf("HeadSizeIn2: got %v, want 98", r.HeadSizeIn2)
	}
	if r.StrungWeight < 11.0 || r.StrungWeight > 12.0 {
		t.Errorf("StrungWeight: got %v, want ~11.4", r.StrungWeight)
	}
	if r.Swingweight < 300 || r.Swingweight > 340 {
		t.Errorf("Swingweight: got %v, want ~322", r.Swingweight)
	}
	if r.Stiffness < 50 || r.Stiffness > 80 {
		t.Errorf("Stiffness: got %v, want ~61", r.Stiffness)
	}
	if r.Composition == "" {
		t.Errorf("Composition: empty")
	}
}

func TestParseUsedDetail(t *testing.T) {
	html := readSample(t, "used-detail.html")
	if html == "" {
		return
	}
	m, units, err := ParseUsedDetail(html, "WB9816", "https://www.tennis-warehouse.com/orderusedproduct.html?pcode=WB9816", "Wilson")
	if err != nil {
		t.Fatalf("ParseUsedDetail: %v", err)
	}
	if m.PCode != "WB9816" {
		t.Errorf("PCode: got %q", m.PCode)
	}
	if m.Model == "" {
		t.Errorf("Model: empty")
	}
	if m.HeadSizeIn2 != 98 {
		t.Errorf("HeadSizeIn2: got %v", m.HeadSizeIn2)
	}
	if len(units) == 0 {
		t.Errorf("Units: empty (expected >=1 grade-tagged tr)")
	}
	for _, u := range units {
		if u.Grade == "" {
			t.Errorf("unit %s: empty grade", u.StockCode)
		}
		if u.Price <= 0 {
			t.Errorf("unit %s: zero price", u.StockCode)
		}
	}
	t.Logf("Parsed %d units for %s", len(units), m.PCode)
}

func TestParseUsedCatalog(t *testing.T) {
	html := readSample(t, "used-wilson.html")
	if html == "" {
		return
	}
	models, err := ParseUsedCatalog(html)
	if err != nil {
		t.Fatalf("ParseUsedCatalog: %v", err)
	}
	if len(models) == 0 {
		t.Fatalf("expected >0 models")
	}
	t.Logf("Parsed %d models", len(models))
	hasPrice := 0
	for _, m := range models {
		if m.PCode == "" {
			t.Errorf("model missing PCode")
		}
		if m.PriceLow > 0 || m.PriceHigh > 0 {
			hasPrice++
		}
	}
	if hasPrice == 0 {
		t.Errorf("no models carried price data — extractor regressed")
	}
}

func TestParseRacquetCatalog(t *testing.T) {
	html := readSample(t, "new-wilson.html")
	if html == "" {
		return
	}
	rs, err := ParseRacquetCatalog(html, "Wilson")
	if err != nil {
		t.Fatalf("ParseRacquetCatalog: %v", err)
	}
	if len(rs) == 0 {
		t.Fatalf("expected >0 racquets")
	}
	t.Logf("Parsed %d racquets", len(rs))
	hasPrice := 0
	for _, r := range rs {
		if r.SKU == "" {
			t.Errorf("racquet missing SKU")
		}
		if r.Price > 0 {
			hasPrice++
		}
	}
	if hasPrice == 0 {
		t.Errorf("no racquets carried price data — extractor regressed")
	}
}

package adsanalytics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBreakEvenACOS(t *testing.T) {
	t.Parallel()
	got, err := BreakEvenACOS(32.99, 8.50, 30)
	if err != nil {
		t.Fatalf("BreakEvenACOS returned error: %v", err)
	}
	want := (32.99 - 8.50 - 9.897) / 32.99
	if got < want-0.000001 || got > want+0.000001 {
		t.Fatalf("BreakEvenACOS = %v, want %v", got, want)
	}
}

func TestBreakEvenACOSRejectsZeroPrice(t *testing.T) {
	t.Parallel()
	if _, err := BreakEvenACOS(0, 8.50, 30); err == nil {
		t.Fatal("BreakEvenACOS with zero price returned nil error")
	}
}

func TestTrueProfit(t *testing.T) {
	t.Parallel()
	got, err := TrueProfit(32.99, 8.50, 30, 4.20)
	if err != nil {
		t.Fatalf("TrueProfit returned error: %v", err)
	}
	want := 32.99 - 8.50 - 9.897 - 4.20
	if got < want-0.000001 || got > want+0.000001 {
		t.Fatalf("TrueProfit = %v, want %v", got, want)
	}
}

func TestACOSAndTACOS(t *testing.T) {
	t.Parallel()
	acos, err := ACOS(25, 100)
	if err != nil {
		t.Fatalf("ACOS returned error: %v", err)
	}
	if acos != 0.25 {
		t.Fatalf("ACOS = %v, want 0.25", acos)
	}
	tacos, err := TACOS(25, 250)
	if err != nil {
		t.Fatalf("TACOS returned error: %v", err)
	}
	if tacos != 0.10 {
		t.Fatalf("TACOS = %v, want 0.10", tacos)
	}
}

func TestLoadCOGSAndResolveProductCost(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cogs.toml")
	if err := os.WriteFile(path, []byte(`[B0TEST]
name = "Test Product"
cogs = 8.50
selling_price = 32.99
`), 0o600); err != nil {
		t.Fatalf("write cogs: %v", err)
	}
	items, err := LoadCOGS(path)
	if err != nil {
		t.Fatalf("LoadCOGS returned error: %v", err)
	}
	item, err := ResolveProductCost(items, "B0TEST", 0, 0)
	if err != nil {
		t.Fatalf("ResolveProductCost returned error: %v", err)
	}
	if item.Name != "Test Product" || item.COGS != 8.50 || item.SellingPrice != 32.99 {
		t.Fatalf("resolved item = %+v", item)
	}
	override, err := ResolveProductCost(items, "B0TEST", 30, 7)
	if err != nil {
		t.Fatalf("ResolveProductCost override returned error: %v", err)
	}
	if override.SellingPrice != 30 || override.COGS != 7 {
		t.Fatalf("override item = %+v", override)
	}
}

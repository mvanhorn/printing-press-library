// internal/catalog/catalog_test.go
package catalog

import "testing"

func TestLoadReturnsSeedEntries(t *testing.T) {
	entries, err := Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected seed entries")
	}
	var sawMika bool
	for _, e := range entries {
		if e.Provider == "mika" && e.Year != 0 && e.Race != "" {
			sawMika = true
		}
	}
	if !sawMika {
		t.Fatal("expected at least one mika entry with race+year")
	}
}

package haven

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCapturedPublicResponses(t *testing.T) {
	dir := os.Getenv("HAVEN_CAPTURE_DIR")
	if dir == "" {
		t.Skip("set HAVEN_CAPTURE_DIR to replay independently captured public API responses")
	}
	raw, err := os.ReadFile(filepath.Join(dir, "locations-probe.json"))
	if err != nil {
		t.Fatal(err)
	}
	ls, err := ParseLocations(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ls) != 10 {
		t.Fatalf("expected captured ten locations, got %d", len(ls))
	}
	raw, err = os.ReadFile(filepath.Join(dir, "menu_categories_location_id_14208.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseMenu(raw, 14208, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Items) == 0 {
		t.Fatal("empty capture")
	}
	t.Logf("Parsed %d locations and %d distinct North Haven items", len(ls), len(s.Items))
}

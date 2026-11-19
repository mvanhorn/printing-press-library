package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every file carrying an etsy-ext marker must be listed by at least one
// .printing-press-patches record, so a reprint can re-apply or review it.
func TestEveryEtsyExtFileHasPatchRecord(t *testing.T) {
	root := filepath.Join("..", "..")
	covered := map[string]bool{}
	records, err := filepath.Glob(filepath.Join(root, ".printing-press-patches", "*.json"))
	if err != nil || len(records) == 0 {
		t.Fatalf("no patch records: %v", err)
	}
	for _, rec := range records {
		raw, err := os.ReadFile(rec)
		if err != nil {
			t.Fatal(err)
		}
		var d struct {
			Files   []string `json:"files"`
			Markers []string `json:"markers"`
		}
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatalf("%s: %v", rec, err)
		}
		for _, marker := range d.Markers {
			found := false
			for _, f := range d.Files {
				data, err := os.ReadFile(filepath.Join(root, f))
				if err != nil {
					t.Fatal(err)
				}
				found = found || strings.Contains(string(data), marker)
			}
			if !found {
				t.Errorf("%s: marker %q missing from recorded files", rec, marker)
			}
		}
		for _, f := range d.Files {
			covered[filepath.ToSlash(f)] = true
		}
	}
	for _, dir := range []string{"internal", "cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, dir), func(p string, e os.DirEntry, err error) error {
			if err != nil || e.IsDir() || !strings.HasSuffix(p, ".go") || filepath.Base(p) == "etsy_patch_records_test.go" {
				return nil
			}
			b, _ := os.ReadFile(p)
			if strings.Contains(string(b), "etsy"+"-ext") {
				rel, _ := filepath.Rel(root, p)
				if !covered[filepath.ToSlash(rel)] {
					t.Errorf("%s has etsy-ext edits but no .printing-press-patches record lists it", rel)
				}
			}
			return nil
		})
	}
}

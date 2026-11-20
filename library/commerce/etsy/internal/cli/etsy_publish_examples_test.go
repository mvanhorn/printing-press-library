package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func TestAnalyticsHelpExamplesRun(t *testing.T) {
	syncCLIIsolate(t)
	example := newAnalyticsCmd(&rootFlags{}).Example
	var examples [][]string
	for _, line := range strings.Split(example, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "etsy-pp-cli" && fields[1] == "analytics" {
			examples = append(examples, fields[1:])
		}
	}
	if len(examples) != 3 {
		t.Fatalf("expected three analytics examples, got %d", len(examples))
	}
	for _, state := range []string{"missing", "empty", "synced"} {
		t.Run(state, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "data.db")
			if state != "missing" {
				db, err := store.Open(path)
				s2Must(t, err)
				if state == "synced" {
					s2Must(t, db.Upsert("listings", "1000000001", json.RawMessage(`{"listing_id":1000000001,"state":"active","taxonomy_id":123}`)))
				}
				s2Must(t, db.Close())
			}
			for _, example := range examples {
				args := append(append([]string{}, example...), "--db", path)
				_, stderr, err := s2Run(t, args...)
				if err != nil {
					t.Errorf("%v: exit %d: %v; stderr: %s", args, ExitCode(err), err, stderr)
				}
			}
		})
	}
}

func TestLocalDataHelpExamplesArePreviewOnly(t *testing.T) {
	syncCLIIsolate(t)
	for _, name := range []string{"prune", "erase-buyer"} {
		t.Run(name, func(t *testing.T) {
			out, _, err := s2Run(t, "local-data", name, "--help")
			s2Must(t, err)
			_, examples, ok := strings.Cut(out, "Examples:\n")
			if !ok {
				t.Fatalf("missing Examples section: %s", out)
			}
			examples, _, _ = strings.Cut(examples, "\nFlags:")
			count := 0
			for _, line := range strings.Split(examples, "\n") {
				if strings.Contains(line, "--yes") {
					t.Errorf("unsafe example: %s", line)
				}
				if strings.Contains(line, "etsy-pp-cli local-data "+name) {
					count++
				}
			}
			if count != 3 {
				t.Errorf("expected three examples, got %d: %s", count, examples)
			}
		})
	}
}

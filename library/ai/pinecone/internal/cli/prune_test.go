// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/pinecone/internal/store"
)

// TestNovelPruneHelpWires smoke-tests that the prune command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPruneHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"prune", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("prune --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "prune"} {
		if !strings.Contains(help, want) {
			t.Fatalf("prune --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestLoadScopedPruneVectorsExcludesForeignAndUnscopedRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "prune.db")
	mirror, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer mirror.Close()

	rows := []map[string]any{
		{"id": "target", "index": "target-index", "namespace": "target-ns", "metadata": map[string]any{"timestamp": "2020-01-01T00:00:00Z"}},
		{"id": "foreign-index", "index": "other-index", "namespace": "target-ns", "metadata": map[string]any{"timestamp": "2020-01-01T00:00:00Z"}},
		{"id": "foreign-namespace", "index": "target-index", "namespace": "other-ns", "metadata": map[string]any{"timestamp": "2020-01-01T00:00:00Z"}},
		{"id": "missing-index", "namespace": "target-ns", "metadata": map[string]any{"timestamp": "2020-01-01T00:00:00Z"}},
		{"id": "missing-namespace", "index": "target-index", "metadata": map[string]any{"timestamp": "2020-01-01T00:00:00Z"}},
	}
	for _, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			t.Fatalf("marshal row: %v", err)
		}
		if err := mirror.Upsert("vectors", row["id"].(string), data); err != nil {
			t.Fatalf("upsert %s: %v", row["id"], err)
		}
	}

	got, err := loadScopedPruneVectors(context.Background(), mirror.DB(), "target-index", "target-ns")
	if err != nil {
		t.Fatalf("load scoped vectors: %v", err)
	}
	gotIDs := make([]string, 0, len(got))
	for _, vector := range got {
		gotIDs = append(gotIDs, vector.ID)
	}
	if want := []string{"target"}; !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("scoped ids = %v, want %v", gotIDs, want)
	}
}

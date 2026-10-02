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

func TestDeleteConfirmedAbsentVectorCleansOnlyMatchingLocalScope(t *testing.T) {
	mirror, err := store.Open(filepath.Join(t.TempDir(), "scoped-prune.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer mirror.Close()
	items := []json.RawMessage{
		json.RawMessage(`{"id":"shared","index_name":"target-index","namespace":"target-ns","metadata":{"timestamp":"2020-01-01T00:00:00Z"}}`),
		json.RawMessage(`{"id":"shared","index_name":"target-index","namespace":"other-ns","metadata":{"timestamp":"2020-01-01T00:00:00Z"}}`),
		json.RawMessage(`{"id":"shared","index_name":"other-index","namespace":"target-ns","metadata":{"timestamp":"2020-01-01T00:00:00Z"}}`),
	}
	if stored, failed, err := mirror.UpsertBatch("vectors", items); err != nil || stored != 3 || failed != 0 {
		t.Fatalf("seed scoped vectors: stored=%d failed=%d error=%v", stored, failed, err)
	}
	target, err := loadScopedPruneVectors(context.Background(), mirror.DB(), "target-index", "target-ns")
	if err != nil || len(target) != 1 {
		t.Fatalf("target rows=%v error=%v", target, err)
	}
	row := store.ScopedVectorRow{StorageID: target[0].StorageID, BareID: target[0].ID, ExpectedData: target[0].RawData}
	if _, err := mirror.DeleteScopedVectorRows(context.Background(), "other-index", "target-ns", []store.ScopedVectorRow{row}); err == nil {
		t.Fatal("wrong-scope cleanup accepted")
	}
	if deleted, err := mirror.DeleteScopedVectorRows(context.Background(), "target-index", "target-ns", []store.ScopedVectorRow{row}); err != nil || deleted != 1 {
		t.Fatalf("scoped cleanup deleted=%d error=%v", deleted, err)
	}
	for _, tc := range []struct {
		index, namespace string
		want             int
	}{
		{"target-index", "target-ns", 0},
		{"target-index", "other-ns", 1},
		{"other-index", "target-ns", 1},
	} {
		rows, err := loadScopedPruneVectors(context.Background(), mirror.DB(), tc.index, tc.namespace)
		if err != nil || len(rows) != tc.want {
			t.Fatalf("scope %s/%s rows=%d want=%d error=%v", tc.index, tc.namespace, len(rows), tc.want, err)
		}
	}
	for _, table := range []string{"resources", "vectors", "resources_fts"} {
		var count int
		query := "SELECT COUNT(*) FROM " + table
		if err := mirror.DB().QueryRow(query).Scan(&count); err != nil || count != 2 {
			t.Fatalf("%s count=%d want=2 error=%v", table, count, err)
		}
	}
}

func TestDeleteConfirmedAbsentVectorFailsIfLocalRowChanged(t *testing.T) {
	mirror, err := store.Open(filepath.Join(t.TempDir(), "changed-prune.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer mirror.Close()
	old := json.RawMessage(`{"id":"same","index_name":"index-a","namespace":"ns","metadata":{"timestamp":"2020-01-01T00:00:00Z"}}`)
	if _, _, err := mirror.UpsertBatch("vectors", []json.RawMessage{old}); err != nil {
		t.Fatal(err)
	}
	rows, err := loadScopedPruneVectors(context.Background(), mirror.DB(), "index-a", "ns")
	if err != nil || len(rows) != 1 {
		t.Fatalf("initial rows=%v error=%v", rows, err)
	}
	newer := json.RawMessage(`{"id":"same","index_name":"index-a","namespace":"ns","metadata":{"timestamp":"2030-01-01T00:00:00Z"}}`)
	if _, _, err := mirror.UpsertBatch("vectors", []json.RawMessage{newer}); err != nil {
		t.Fatal(err)
	}
	row := store.ScopedVectorRow{StorageID: rows[0].StorageID, BareID: rows[0].ID, ExpectedData: rows[0].RawData}
	if _, err := mirror.DeleteScopedVectorRows(context.Background(), "index-a", "ns", []store.ScopedVectorRow{row}); err == nil {
		t.Fatal("changed local row was removed")
	}
	remaining, err := loadScopedPruneVectors(context.Background(), mirror.DB(), "index-a", "ns")
	if err != nil || len(remaining) != 1 || remaining[0].RawData != string(newer) {
		t.Fatalf("updated row lost: rows=%v error=%v", remaining, err)
	}
}

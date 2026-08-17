// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/judgeme/internal/store"
	"github.com/spf13/cobra"
)

type fakeJudgeMeClient struct {
	get func(path string, params map[string]string) (json.RawMessage, error)
}

func (f fakeJudgeMeClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	return f.get(path, params)
}

func TestFetchJudgeMeReviewPartitionDetectsRepeatedPage(t *testing.T) {
	pageRows := make([]map[string]any, judgeMePageSize)
	for i := range pageRows {
		pageRows[i] = map[string]any{"id": i + 1, "body": fmt.Sprintf("Body %d", i+1)}
	}
	page, err := json.Marshal(pageRows)
	if err != nil {
		t.Fatal(err)
	}
	client := fakeJudgeMeClient{get: func(path string, params map[string]string) (json.RawMessage, error) {
		if path != "/reviews" {
			t.Fatalf("path = %q", path)
		}
		return page, nil
	}}

	rows, looped, err := fetchJudgeMeReviewPartition(context.Background(), client, nil, 0, 101)
	if err != nil {
		t.Fatal(err)
	}
	if !looped {
		t.Fatal("expected repeated page to be detected")
	}
	if len(rows) != judgeMePageSize {
		t.Fatalf("unique rows = %d, want %d", len(rows), judgeMePageSize)
	}
}

func TestFetchJudgeMeCorpusRefusesCountMismatch(t *testing.T) {
	client := fakeJudgeMeClient{get: func(path string, params map[string]string) (json.RawMessage, error) {
		switch path {
		case "/reviews/count":
			return json.RawMessage(`{"count":3}`), nil
		case "/reviews":
			if params["page"] == "1" {
				return json.RawMessage(`[{"id":1},{"id":2}]`), nil
			}
			return json.RawMessage(`[]`), nil
		default:
			return nil, fmt.Errorf("unexpected path %s", path)
		}
	}}

	_, err := fetchJudgeMeCorpus(context.Background(), client)
	if err == nil || !strings.Contains(err.Error(), "final unique review count mismatch") {
		t.Fatalf("error = %v, want final count mismatch", err)
	}
}

func TestNormalizedBodyHashCollapsesSyndicatedFormatting(t *testing.T) {
	first := normalizedBodyHash("  Great\nPRODUCT ")
	second := normalizedBodyHash("great product")
	if first == "" || first != second {
		t.Fatalf("hashes differ: %q %q", first, second)
	}
	if normalizedBodyHash(" \t ") != "" {
		t.Fatal("empty body should not receive a dedupe hash")
	}
}

func TestDecodeJudgeMeReviewPublicationFallback(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		published bool
	}{
		{name: "explicit published field", raw: `{"id":1,"published":true}`, published: true},
		{name: "explicit field remains authoritative", raw: `{"id":1,"published":false,"curated":"ok"}`, published: false},
		{name: "curated ok fallback", raw: `{"id":1,"curated":"ok"}`, published: true},
		{name: "curated spam fallback", raw: `{"id":1,"curated":"spam"}`, published: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := mustJudgeMeReview(t, tt.raw)
			if row.Published != tt.published {
				t.Fatalf("Published = %v, want %v", row.Published, tt.published)
			}
		})
	}
}

func TestReplaceJudgeMeReviewCorpusAddsDocumentedColumnsAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reviews.db")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows := []judgeMeReview{
		mustJudgeMeReview(t, `{"id":1,"body":"Same body","rating":5,"published":true,"hidden":false,"product_external_id":"a"}`),
		mustJudgeMeReview(t, `{"id":2,"body":"same   body","rating":5,"published":false,"hidden":true,"product_external_id":"b"}`),
	}
	corpus := judgeMeCorpus{
		Reviews:       rows,
		ExpectedTotal: 2,
		Published:     1,
		Hidden:        1,
		UniqueBodies:  1,
		SyncedAt:      time.Date(2026, 7, 29, 22, 0, 0, 0, time.UTC),
	}
	if err := replaceJudgeMeReviewCorpus(context.Background(), db.DB(), corpus); err != nil {
		t.Fatal(err)
	}
	var count, unique int
	if err := db.DB().QueryRow(`SELECT COUNT(*), COUNT(DISTINCT body_hash) FROM reviews`).Scan(&count, &unique); err != nil {
		t.Fatal(err)
	}
	if count != 2 || unique != 1 {
		t.Fatalf("rows=%d unique_hashes=%d", count, unique)
	}
	var total int
	if err := db.DB().QueryRow(`SELECT total_count FROM sync_state WHERE resource_type='reviews'`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("sync_state total_count = %d", total)
	}
}

func TestJudgeMeMutationGateRequiresApplyOrDryRun(t *testing.T) {
	flags := &rootFlags{}
	root := &cobra.Command{Use: "test"}
	called := false
	mutation := &cobra.Command{
		Use:         "update",
		Annotations: map[string]string{"pp:method": "PUT"},
		RunE: func(*cobra.Command, []string) error {
			called = true
			return nil
		},
	}
	root.AddCommand(mutation)
	installJudgeMeMutationGate(root, flags)
	root.SetArgs([]string{"update"})
	if err := root.Execute(); err == nil || called {
		t.Fatalf("without --apply: err=%v called=%v", err, called)
	}

	root = &cobra.Command{Use: "test"}
	called = false
	mutation = &cobra.Command{
		Use:         "update",
		Annotations: map[string]string{"pp:method": "PUT"},
		RunE: func(*cobra.Command, []string) error {
			called = true
			return nil
		},
	}
	root.AddCommand(mutation)
	installJudgeMeMutationGate(root, flags)
	root.SetArgs([]string{"update", "--apply"})
	if err := root.Execute(); err != nil || !called {
		t.Fatalf("with --apply: err=%v called=%v", err, called)
	}
}

func mustJudgeMeReview(t *testing.T, raw string) judgeMeReview {
	t.Helper()
	row, err := decodeJudgeMeReview(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return row
}

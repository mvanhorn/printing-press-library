// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

func TestCollectionSaveInputs(t *testing.T) {
	name, inputs, err := collectionSaveInputs("research", []string{"123"}, "")
	if err != nil {
		t.Fatalf("collectionSaveInputs documented form error: %v", err)
	}
	if name != "research" || len(inputs) != 1 || inputs[0] != "123" {
		t.Fatalf("documented form = %q %v", name, inputs)
	}

	name, inputs, err = collectionSaveInputs("", []string{"research", "123"}, "")
	if err != nil {
		t.Fatalf("collectionSaveInputs positional form error: %v", err)
	}
	if name != "research" || len(inputs) != 1 || inputs[0] != "123" {
		t.Fatalf("positional form = %q %v", name, inputs)
	}
}

func TestCollectionSaveInputsRequiresCollection(t *testing.T) {
	if _, _, err := collectionSaveInputs("", []string{"123"}, ""); err == nil {
		t.Fatal("collectionSaveInputs missing collection returned nil error")
	}
	if _, _, err := collectionSaveInputs("research", []string{"123"}, "agentic coding"); err == nil {
		t.Fatal("collectionSaveInputs accepted --from-search with explicit inputs")
	}
}

func TestWriteCollectionExportMarkdownAndJSONL(t *testing.T) {
	items := []collectionItemSnapshot{{
		TweetID: "123",
		URL:     "https://x.com/alice/status/123",
		Author:  &postAuthorSummary{Username: "alice"},
		Text:    "A useful post",
		Note:    "source material",
		Tags:    []string{"research"},
		SavedAt: "2026-01-01T00:00:00Z",
	}}

	var md bytes.Buffer
	if err := writeCollectionExport(&md, "research", items, "markdown"); err != nil {
		t.Fatalf("markdown export error: %v", err)
	}
	for _, want := range []string{"# research", "https://x.com/alice/status/123", "- Note: source material", "A useful post"} {
		if !strings.Contains(md.String(), want) {
			t.Fatalf("markdown export missing %q:\n%s", want, md.String())
		}
	}

	var jsonl bytes.Buffer
	if err := writeCollectionExport(&jsonl, "research", items, "jsonl"); err != nil {
		t.Fatalf("jsonl export error: %v", err)
	}
	if lines := strings.Split(strings.TrimSpace(jsonl.String()), "\n"); len(lines) != 1 || !strings.Contains(lines[0], `"tweet_id":"123"`) {
		t.Fatalf("jsonl export = %q", jsonl.String())
	}
}

func TestNormalizeSearchTweetRecordUsesTweetIDAsInput(t *testing.T) {
	rec, err := normalizeSearchTweetRecord(json.RawMessage(`{"id":"123","text":"found by query"}`), nil, parseIncludeSet("refs"))
	if err != nil {
		t.Fatalf("normalizeSearchTweetRecord returned error: %v", err)
	}
	if rec.Input != "123" {
		t.Fatalf("Input = %q, want tweet ID", rec.Input)
	}
}

func TestListCollectionItemsAllowsExistingEmptyCollection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-twitter.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := db.DB().ExecContext(context.Background(), `INSERT INTO post_collections(name, created_at, updated_at) VALUES('empty', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert empty collection: %v", err)
	}
	defer db.Close()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	items, err := listCollectionItems(cmd, db, "empty", 100, false)
	if err != nil {
		t.Fatalf("listCollectionItems returned error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items len = %d, want 0", len(items))
	}
}

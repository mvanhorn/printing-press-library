// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
)

func TestSearchLocalUsesFTSWithoutResourceFilter(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.Upsert("serp-results", "one", json.RawMessage(`{"id":"one","snippet":"tree service daytona"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("backlinks", "two", json.RawMessage(`{"id":"two","anchor":"unrelated"}`)); err != nil {
		t.Fatal(err)
	}

	results, err := db.SearchByType("", "daytona", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || string(results[0]) != `{"id":"one","snippet":"tree service daytona"}` {
		t.Fatalf("unexpected unfiltered results: %s", results)
	}
}

func TestSearchLocalHonorsResourceFilter(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for resourceType, id := range map[string]string{"serp-results": "serp", "backlinks": "link"} {
		if err := db.Upsert(resourceType, id, json.RawMessage(`{"id":"`+id+`","text":"shared phrase"}`)); err != nil {
			t.Fatal(err)
		}
	}

	results, err := db.SearchByType("backlinks", "shared", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || string(results[0]) != `{"id":"link","text":"shared phrase"}` {
		t.Fatalf("unexpected filtered results: %s", results)
	}
}

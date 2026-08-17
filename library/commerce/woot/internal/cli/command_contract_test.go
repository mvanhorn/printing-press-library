// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/woot/internal/store"
	"github.com/spf13/cobra"
)

func TestSearchRejectsNonPositiveLimit(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"0", "-1"} {
		cmd := newSearchCmd(&rootFlags{dataSource: "local"})
		cmd.SetArgs([]string{"rayon", "--limit", value})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--limit must be greater than 0") {
			t.Errorf("search --limit %s error = %v, want usage error", value, err)
		}
	}
}

func TestSearchMissingDatabaseDoesNotCreateIt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "data.db")
	cmd := newSearchCmd(&rootFlags{dataSource: "local"})
	cmd.SetArgs([]string{"rayon", "--db", dbPath})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no local database") {
		t.Fatalf("missing database error = %v", err)
	}
	if _, statErr := os.Stat(dbPath); !os.IsNotExist(statErr) {
		t.Fatalf("read-only search created database %s: %v", dbPath, statErr)
	}
}

func TestSearchJoinsUnquotedQueryWords(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("create test store: %v", err)
	}
	for id, title := range map[string]string{"tent": "camping tent", "chair": "camping chair"} {
		body, err := json.Marshal(map[string]string{"id": id, "title": title})
		if err != nil {
			t.Fatalf("encode %s: %v", id, err)
		}
		if err := db.Upsert("deals", id, body); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close test store: %v", err)
	}

	flags := &rootFlags{dataSource: "local", asJSON: true}
	cmd := newSearchCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"camping", "tent", "--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("multi-word search: %v", err)
	}
	if !strings.Contains(stdout.String(), `"id": "tent"`) || strings.Contains(stdout.String(), `"id": "chair"`) {
		t.Fatalf("multi-word search did not require every word:\n%s", stdout.String())
	}
}

func TestAgentSearchOutputHasSingleLocalProvenanceEnvelope(t *testing.T) {
	flags := &rootFlags{agent: true, asJSON: true, compact: true}
	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	results := []json.RawMessage{json.RawMessage(`{"id":"offer-1","title":"Camping tent"}`)}
	if err := outputSearchResults(cmd, flags, results, 25, DataProvenance{Source: "local", ResourceType: "deals"}); err != nil {
		t.Fatalf("output agent search results: %v", err)
	}
	var envelope struct {
		Meta    map[string]any   `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode agent search output: %v\n%s", err, stdout.String())
	}
	if envelope.Meta["source"] != "local" || envelope.Meta["resource_type"] != "deals" || len(envelope.Results) != 1 {
		t.Fatalf("agent search envelope = %#v", envelope)
	}
}

func TestSyncRejectsUnsupportedSince(t *testing.T) {
	t.Parallel()
	cmd := newSyncCmd(&rootFlags{})
	cmd.SetArgs([]string{"--since", "7d"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not supported for Woot") {
		t.Fatalf("sync --since error = %v, want unsupported flag error", err)
	}
}

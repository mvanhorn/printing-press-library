// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/internal/store"
)

func TestSearchRequiresExplicitZIP(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"search", "coffee", "--data-source", "local", "--json"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--zip is required") {
		t.Fatalf("search error = %v, want required ZIP", err)
	}
}

func TestFlyerItemsRejectsLocalReadWithoutMarket(t *testing.T) {
	cmd := newFlyersItemsCmd(&rootFlags{dataSource: "local"})
	cmd.SetArgs([]string{"8005907"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "no local data source") {
		t.Fatalf("flyer items local read = %v, want live-only error", err)
	}
}

func TestSearchLocalSeparatesWriteThroughItemsByMarket(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("FLIPP_DATA_DIR", dataDir)
	for _, market := range []struct {
		postalCode, name string
	}{
		{postalCode: "10001", name: "East coffee"},
		{postalCode: "94105", name: "West coffee"},
	} {
		item, err := json.Marshal([]map[string]string{{"id": "same", "name": market.name}})
		if err != nil {
			t.Fatal(err)
		}
		writeThroughCache(context.Background(), "items", item, map[string]string{
			"postal_code": market.postalCode,
			"locale":      "en-us",
		})
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"search", "coffee", "--type", "items", "--zip", "10001", "--data-source", "local", "--json"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("local item search: %v (%s)", err, output.String())
	}
	if !strings.Contains(output.String(), "East coffee") || strings.Contains(output.String(), "West coffee") {
		t.Fatalf("local item market search = %s", output.String())
	}

	db, err := store.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type='items'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("cached item rows = %d, %v; want both markets", count, err)
	}
}

func TestSearchLocalUsesSelectedMarket(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []json.RawMessage{
		json.RawMessage(`{"id":"same","name":"East coffee","_sync_postal_code":"10001","_sync_locale":"en-us"}`),
		json.RawMessage(`{"id":"same","name":"West coffee","_sync_postal_code":"94105","_sync_locale":"en-us"}`),
	} {
		if _, _, err := db.UpsertBatch("flyers", []json.RawMessage{item}); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cmd := RootCmd()
	cmd.SetArgs([]string{"search", "coffee", "--type", "flyers", "--zip", "10001", "--data-source", "local", "--json", "--db", dbPath})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("local search: %v (%s)", err, output.String())
	}
	if !strings.Contains(output.String(), "East coffee") || strings.Contains(output.String(), "West coffee") {
		t.Fatalf("local market search = %s", output.String())
	}
}

// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/scryfall/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/scryfall/internal/store"
)

func seedScryfallLocalStore(t *testing.T) {
	t.Helper()
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	t.Cleanup(restore)

	db, err := store.Open(defaultDBPath("scryfall-pp-cli"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cards := []json.RawMessage{
		json.RawMessage(`{"id":"card-alpha","name":"Alpha Query Match","set":"lea","collector_number":"1","arena_id":123,"mtgo_id":456,"multiverse_ids":[789,790],"tcgplayer_id":321,"cardmarket_id":654}`),
		json.RawMessage(`{"id":"card-beta","name":"Beta Unrelated","set":"leb","collector_number":"2","arena_id":124,"mtgo_id":457,"multiverse_ids":[791],"tcgplayer_id":322,"cardmarket_id":655}`),
	}
	if _, _, err := db.UpsertBatch("cards", cards); err != nil {
		_ = db.Close()
		t.Fatalf("seed cards: %v", err)
	}
	sets := []json.RawMessage{
		json.RawMessage(`{"id":"set-alpha","name":"Limited Edition Alpha","code":"lea","mtgo_code":"1ED","tcgplayer_id":1}`),
		json.RawMessage(`{"id":"set-beta","name":"Limited Edition Beta","code":"leb","mtgo_code":"2ED","tcgplayer_id":2}`),
	}
	if _, _, err := db.UpsertBatch("sets", sets); err != nil {
		_ = db.Close()
		t.Fatalf("seed sets: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seeded store: %v", err)
	}
}

func TestResolveLocalRejectsFilteredScryfallSearch(t *testing.T) {
	seedScryfallLocalStore(t)

	data, _, err := resolveLocal(
		context.Background(), nil, io.Discard, "cards", true, "/cards/search",
		map[string]string{"q": "Alpha"}, "user_requested",
	)
	if err == nil {
		t.Fatalf("resolveLocal returned unrelated search data: %s", data)
	}
	if !strings.Contains(err.Error(), "cannot safely reproduce Scryfall card search query") {
		t.Fatalf("resolveLocal error = %v, want explicit unsupported-search error", err)
	}
}

func TestResolveLocalRejectsOtherUnsupportedCardOperations(t *testing.T) {
	seedScryfallLocalStore(t)
	for _, path := range []string{"/cards/autocomplete", "/cards/random"} {
		t.Run(path, func(t *testing.T) {
			data, _, err := resolveLocal(context.Background(), nil, io.Discard, "cards", true, path, nil, "user_requested")
			if err == nil || !strings.Contains(err.Error(), "cannot safely reproduce") {
				t.Fatalf("resolveLocal(%q) = %s, %v; want unsupported-operation error", path, data, err)
			}
		})
	}
}

func TestResolveLocalRejectsUnsupportedCardRepresentation(t *testing.T) {
	seedScryfallLocalStore(t)
	for _, params := range []map[string]string{
		{"format": "image"},
		{"face": "front"},
		{"version": "normal"},
	} {
		data, _, err := resolveLocal(context.Background(), nil, io.Discard, "cards", false, "/cards/card-alpha", params, "user_requested")
		if err == nil || !strings.Contains(err.Error(), "local data cannot reproduce") {
			t.Fatalf("resolveLocal with params %v = %s, %v; want unsupported-representation error", params, data, err)
		}
	}
}

func TestResolveLocalSupportsScryfallAlternateIdentifiers(t *testing.T) {
	seedScryfallLocalStore(t)

	tests := []struct {
		name         string
		resourceType string
		path         string
		params       map[string]string
		wantID       string
	}{
		{name: "card primary UUID", resourceType: "cards", path: "/cards/card-alpha", wantID: "card-alpha"},
		{name: "card exact name", resourceType: "cards", path: "/cards/named", params: map[string]string{"exact": "alpha query match"}, wantID: "card-alpha"},
		{name: "card arena ID", resourceType: "cards", path: "/cards/arena/123", wantID: "card-alpha"},
		{name: "card MTGO ID", resourceType: "cards", path: "/cards/mtgo/456", wantID: "card-alpha"},
		{name: "card multiverse ID", resourceType: "cards", path: "/cards/multiverse/790", wantID: "card-alpha"},
		{name: "card TCGplayer ID", resourceType: "cards", path: "/cards/tcgplayer/321", wantID: "card-alpha"},
		{name: "card Cardmarket ID", resourceType: "cards", path: "/cards/cardmarket/654", wantID: "card-alpha"},
		{name: "card set and collector number", resourceType: "cards", path: "/cards/LEA/1", wantID: "card-alpha"},
		{name: "set code", resourceType: "sets", path: "/sets/LEA", wantID: "set-alpha"},
		{name: "set MTGO code", resourceType: "sets", path: "/sets/1ed", wantID: "set-alpha"},
		{name: "set TCGplayer ID", resourceType: "sets", path: "/sets/tcgplayer/1", wantID: "set-alpha"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, prov, err := resolveLocal(
				context.Background(), nil, io.Discard, tt.resourceType, false,
				tt.path, tt.params, "user_requested",
			)
			if err != nil {
				t.Fatalf("resolveLocal: %v", err)
			}
			if prov.Source != "local" {
				t.Fatalf("provenance source = %q, want local", prov.Source)
			}
			var object struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(data, &object); err != nil {
				t.Fatalf("decode result: %v", err)
			}
			if object.ID != tt.wantID {
				t.Fatalf("resolved id = %q, want %q (data=%s)", object.ID, tt.wantID, data)
			}
		})
	}
}

func TestResolveLocalRejectsAmbiguousOrFuzzyNamedCards(t *testing.T) {
	seedScryfallLocalStore(t)

	_, _, err := resolveLocal(
		context.Background(), nil, io.Discard, "cards", false, "/cards/named",
		map[string]string{"fuzzy": "Alpha"}, "user_requested",
	)
	if err == nil || !strings.Contains(err.Error(), "cannot safely reproduce Scryfall fuzzy-name lookup") {
		t.Fatalf("fuzzy resolveLocal error = %v, want explicit unsupported error", err)
	}

	db, err := store.Open(defaultDBPath("scryfall-pp-cli"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, _, err := db.UpsertBatch("cards", []json.RawMessage{
		json.RawMessage(`{"id":"card-gamma","name":"Alpha Query Match","set":"another","collector_number":"3"}`),
	}); err != nil {
		_ = db.Close()
		t.Fatalf("seed duplicate card name: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	data, _, err := resolveLocal(context.Background(), nil, io.Discard, "cards", false, "/cards/named", map[string]string{"exact": "Alpha Query Match"}, "user_requested")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous resolveLocal = %s, %v; want explicit ambiguous error", data, err)
	}
}

func TestSetsGetAllListsSyncedSetsLocally(t *testing.T) {
	seedScryfallLocalStore(t)

	flags := &rootFlags{asJSON: true, dataSource: "local"}
	cmd := newSetsGetAllCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("sets get-all local: %v", err)
	}

	var envelope struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode command output: %v (output=%s)", err, stdout.String())
	}
	if len(envelope.Results) != 2 {
		t.Fatalf("sets get-all returned %d local sets, want 2 (output=%s)", len(envelope.Results), stdout.String())
	}
}

func TestCardsGetByCodeByNumberUsesLocalIndex(t *testing.T) {
	seedScryfallLocalStore(t)
	flags := &rootFlags{asJSON: true, dataSource: "local"}
	cmd := newCardsGetByCodeByNumberCmd(flags)
	cmd.SetContext(context.Background())
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, []string{"LEA", "1"}); err != nil {
		t.Fatalf("cards get-by-code-by-number local: %v", err)
	}
	if !strings.Contains(stdout.String(), `"card-alpha"`) || !strings.Contains(stdout.String(), `"local"`) {
		t.Fatalf("unexpected local command output: %s", stdout.String())
	}
}

func TestCardsGetAllRejectsNonJSONLocalFormat(t *testing.T) {
	seedScryfallLocalStore(t)
	flags := &rootFlags{asJSON: true, dataSource: "local"}
	cmd := newCardsGetAllCmd(flags)
	cmd.SetContext(context.Background())
	if err := cmd.Flags().Set("format", "image"); err != nil {
		t.Fatal(err)
	}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), `cannot reproduce Scryfall "image" format`) {
		t.Fatalf("cards get-all --format image --data-source local error = %v", err)
	}
}

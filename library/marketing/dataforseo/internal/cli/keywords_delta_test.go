// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
)

func TestComputeKeywordDeltasReadsObservationHistory(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	items := []store.IdentifiedResource{{ID: "tree service", Data: json.RawMessage(`{"keyword":"tree service","search_volume":100,"location_code":2840,"language_code":"en","search_partners":false}`)}}
	if err := db.UpsertIdentifiedBatch("keywords-data", "/v3/keywords_data/google_ads/search_volume/live", items); err != nil {
		t.Fatal(err)
	}
	items[0].Data = json.RawMessage(`{"keyword":"tree service","search_volume":125,"location_code":2840,"language_code":"en","search_partners":false}`)
	if err := db.UpsertIdentifiedBatch("keywords-data", "/v3/keywords_data/google_ads/search_volume/live", items); err != nil {
		t.Fatal(err)
	}

	rows, err := computeKeywordDeltas(db, time.Hour, "tree service")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].PreviousVolume != 100 || rows[0].CurrentVolume != 125 || rows[0].Delta != 25 {
		t.Fatalf("unexpected delta rows: %#v", rows)
	}
	if rows[0].Source != "/v3/keywords_data/google_ads/search_volume/live" || rows[0].LocationCode != "2840" || rows[0].LanguageCode != "en" || rows[0].SearchPartners != "0" {
		t.Fatalf("delta dimensions missing: %#v", rows[0])
	}
}

func TestComputeKeywordDeltasPartitionsSourcesAndDimensions(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	type series struct {
		source string
		data   [2]string
	}
	seriesToStore := []series{
		{source: "/v3/keywords_data/google_ads/search_volume/live", data: [2]string{
			`{"keyword":"shared","search_volume":10,"location_code":2840,"language_code":"en","search_partners":false}`,
			`{"keyword":"shared","search_volume":15,"location_code":2840,"language_code":"en","search_partners":false}`,
		}},
		{source: "/v3/keywords_data/google_ads/search_volume/live", data: [2]string{
			`{"keyword":"shared","search_volume":20,"location_code":2826,"language_code":"en","search_partners":true}`,
			`{"keyword":"shared","search_volume":28,"location_code":2826,"language_code":"en","search_partners":true}`,
		}},
		{source: "/v3/keywords_data/google_ads/search_volume/task_get", data: [2]string{
			`{"keyword":"shared","search_volume":30,"location_code":2840,"language_code":"es","search_partners":false}`,
			`{"keyword":"shared","search_volume":42,"location_code":2840,"language_code":"es","search_partners":false}`,
		}},
	}
	for _, s := range seriesToStore {
		for _, data := range s.data {
			if err := db.UpsertIdentifiedBatch("keywords-data", s.source, []store.IdentifiedResource{{ID: "shared", Data: json.RawMessage(data)}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Same JSON shape under unrelated provider/resource history must never join.
	for _, data := range []string{
		`{"keyword":"shared","search_volume":1000,"location_code":2840,"language_code":"en"}`,
		`{"keyword":"shared","search_volume":2000,"location_code":2840,"language_code":"en"}`,
	} {
		if err := db.UpsertIdentifiedBatch("serp-results", "/v3/serp/google/organic/live", []store.IdentifiedResource{{ID: "shared", Data: json.RawMessage(data)}}); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := computeKeywordDeltas(db, time.Hour, "shared")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d partitioned rows, want 3: %#v", len(rows), rows)
	}
	got := map[string]float64{}
	for _, row := range rows {
		key := row.Source + "|" + row.LocationCode + "|" + row.LanguageCode + "|" + row.SearchPartners
		got[key] = row.Delta
	}
	want := map[string]float64{
		"/v3/keywords_data/google_ads/search_volume/live|2840|en|0":     5,
		"/v3/keywords_data/google_ads/search_volume/live|2826|en|1":     8,
		"/v3/keywords_data/google_ads/search_volume/task_get|2840|es|0": 12,
	}
	for key, delta := range want {
		if got[key] != delta {
			t.Errorf("delta[%q] = %v, want %v (all rows %#v)", key, got[key], delta, rows)
		}
	}
}

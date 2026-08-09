// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/config"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/store"
)

func TestAutoReadFallsBackToLocalForPlaceholderCredential(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride() error = %v", err)
	}
	defer restore()

	db, err := store.Open(defaultDBPath("movieglu-pp-cli"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	if err := db.Upsert("films", "42", json.RawMessage(`{"film_id":42,"film_name":"Local Feature"}`)); err != nil {
		db.Close()
		t.Fatalf("Upsert() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	c := client.New(&config.Config{MoviegluCredentials: "<your-token>"}, 0, 0)
	c.EnableMovieGluHeaderValidation()
	data, provenance, err := resolveReadWithStrategyAndResponsePath(
		context.Background(), c, &rootFlags{dataSource: "auto"}, "auto", "films", true,
		"/filmsNowShowing/", nil, nil, "films", nil,
	)
	if err != nil {
		t.Fatalf("auto read with local data error = %v", err)
	}
	if provenance.Source != "local" || provenance.Reason != configurationFallbackReason {
		t.Fatalf("provenance = %+v, want local %q", provenance, configurationFallbackReason)
	}
	if !json.Valid(data) || !containsJSONText(data, "Local Feature") {
		t.Fatalf("data = %s, want local film", data)
	}
}

func TestLocalFallbackReasonDoesNotMaskProviderHTTPError(t *testing.T) {
	err := &client.APIError{Method: "GET", Path: "/filmsNowShowing/", StatusCode: 401}
	if reason, ok := localFallbackReason(err); ok {
		t.Fatalf("localFallbackReason(APIError) = %q, true; want no fallback", reason)
	}
}

func TestAddMovieGluQueryScopeStampsFilmAndDateForReadsAndSync(t *testing.T) {
	items := []json.RawMessage{json.RawMessage(`{"cinema_id":9,"cinema_name":"Scoped"}`)}
	// The params shape is shared by resolve reads and sync after sync's user
	// --param/--resource-param overrides have been applied.
	got := addMovieGluQueryScope("film-show-times", map[string]string{"film_id": "77", "date": "2026-07-24"}, items)
	if len(got) != 1 || !containsJSONText(got[0], "77") || !containsJSONText(got[0], "2026-07-24") {
		t.Fatalf("addMovieGluQueryScope() = %s, want film/date scope", got)
	}
}

func TestSyncedFilmShowTimesRetainScopeForMovieNight(t *testing.T) {
	db, err := store.Open(t.TempDir() + "/sync.db")
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	defer db.Close()

	apiItems := []json.RawMessage{json.RawMessage(`{"cinema_id":9,"cinema_name":"Synced Cinema","showings":{}}`)}
	syncParams := map[string]string{"film_id": "77", "date": "2026-07-24"}
	scopedItems := addMovieGluQueryScope("film-show-times", syncParams, apiItems)
	if stored, _, err := db.UpsertBatch("film-show-times", scopedItems); err != nil || stored != 1 {
		t.Fatalf("UpsertBatch() = stored %d, error %v; want one synced row", stored, err)
	}
	otherScope := addMovieGluQueryScope("film-show-times", map[string]string{"film_id": "88", "date": "2026-07-25"}, apiItems)
	if stored, _, err := db.UpsertBatch("film-show-times", otherScope); err != nil || stored != 1 {
		t.Fatalf("UpsertBatch(other scope) = stored %d, error %v; want one synced row", stored, err)
	}
	rows, err := db.List("film-show-times", 0)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("Marshal(rows) error = %v", err)
	}
	var cinemas []movieGluCinema
	if err := json.Unmarshal(encoded, &cinemas); err != nil {
		t.Fatalf("Unmarshal(synced rows) error = %v", err)
	}
	if len(cinemas) != 2 {
		t.Fatalf("synced rows = %#v, want same cinema retained under two film/date scopes", cinemas)
	}
	filtered := filterLocalMovieNightCinemas(cinemas, 77, "2026-07-24")
	if len(filtered) != 1 || filtered[0].CinemaID != 9 {
		t.Fatalf("synced rows after exact filter = %#v, want cinema 9", filtered)
	}
}

func containsJSONText(data []byte, want string) bool {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return false
	}
	return containsText(decoded, want)
}

func containsText(value any, want string) bool {
	switch typed := value.(type) {
	case string:
		return typed == want
	case []any:
		for _, item := range typed {
			if containsText(item, want) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if containsText(item, want) {
				return true
			}
		}
	}
	return false
}

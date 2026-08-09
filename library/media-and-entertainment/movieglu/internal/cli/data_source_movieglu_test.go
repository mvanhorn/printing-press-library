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

func TestAutoReadFallsBackToLocalWhenLiveConfigurationIsMissing(t *testing.T) {
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

	c := client.New(&config.Config{}, 0, 0)
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

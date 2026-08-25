// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseSearchEnvelope(t *testing.T) {
	t.Run("value data is re-wrapped under data with counts", func(t *testing.T) {
		in := json.RawMessage(`{"value":{"data":[{"id":"scr_1"}],"hasNextPage":true,"totalCount":861,"searchRequestId":"req_9"}}`)
		out, err := parseSearchEnvelope(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got struct {
			Data            []map[string]any `json:"data"`
			TotalCount      int              `json:"total_count"`
			HasNextPage     bool             `json:"has_next_page"`
			SearchRequestID string           `json:"search_request_id"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(got.Data) != 1 || got.Data[0]["id"] != "scr_1" {
			t.Fatalf("data = %#v", got.Data)
		}
		if got.TotalCount != 861 || !got.HasNextPage || got.SearchRequestID != "req_9" {
			t.Fatalf("meta = %#v", got)
		}
	})

	t.Run("error body becomes exit-3 non-success", func(t *testing.T) {
		in := json.RawMessage(`{"error":{"message":"unauthenticated"}}`)
		_, err := parseSearchEnvelope(in)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var ce *cliError
		if !errors.As(err, &ce) || ce.code != 3 {
			t.Fatalf("want cliError code 3, got %v", err)
		}
	})

	t.Run("absent value becomes exit-3 non-success", func(t *testing.T) {
		in := json.RawMessage(`{"something":"else"}`)
		_, err := parseSearchEnvelope(in)
		var ce *cliError
		if !errors.As(err, &ce) || ce.code != 3 {
			t.Fatalf("want cliError code 3, got %v", err)
		}
	})
}

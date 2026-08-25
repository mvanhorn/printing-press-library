// Copyright 2026 dhilip-subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"testing"
)

type graphPaginationClient struct {
	requests []map[string]string
}

func (c *graphPaginationClient) GetWithHeaders(_ context.Context, _ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	request := make(map[string]string, len(params))
	for key, value := range params {
		request[key] = value
	}
	c.requests = append(c.requests, request)

	if params["after"] == "cursor-2" {
		return json.RawMessage(`{"data":[{"id":"ad-3"}],"paging":{"cursors":{}}}`), nil
	}
	return json.RawMessage(`{"data":[{"id":"ad-1"},{"id":"ad-2"}],"paging":{"cursors":{"after":"cursor-2"},"next":"https://graph.facebook.test/ads?after=cursor-2"}}`), nil
}

func TestPaginatedGetAutoDetectsMetaGraphCursor(t *testing.T) {
	client := &graphPaginationClient{}

	data, err := paginatedGet(
		context.Background(),
		client,
		"/act_123/ads",
		map[string]string{},
		nil,
		true,
		"",
		"offset",
		"limit",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("paginatedGet returned error: %v", err)
	}

	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	if len(client.requests) != 2 {
		t.Fatalf("got %d requests, want 2", len(client.requests))
	}
	if got := client.requests[1]["after"]; got != "cursor-2" {
		t.Fatalf("second request after cursor = %q, want cursor-2", got)
	}
}

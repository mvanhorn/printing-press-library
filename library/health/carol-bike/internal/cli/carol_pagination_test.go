// Copyright 2026 bricenice17 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingPageClient struct {
	params []map[string]string
}

func (c *recordingPageClient) GetWithHeaders(_ context.Context, _ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	copyParams := make(map[string]string, len(params))
	for key, value := range params {
		copyParams[key] = value
	}
	c.params = append(c.params, copyParams)
	return json.RawMessage(`[]`), nil
}

func TestPaginatedGetPreservesZeroBasedFirstPage(t *testing.T) {
	c := &recordingPageClient{}
	_, err := paginatedGet(context.Background(), c, "/rides", map[string]string{
		"page": "0",
		"size": "40",
	}, nil, false, "page", "page", "size", 40, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.params) != 1 {
		t.Fatalf("request count = %d, want 1", len(c.params))
	}
	if got := c.params[0]["page"]; got != "0" {
		t.Fatalf("page param = %q, want zero-based first page", got)
	}
}

func TestPageCursorAdvancesFromZero(t *testing.T) {
	next, ok := nextClientSidePaginationCursor(map[string]string{"page": "0"}, "page", "page", 40)
	if !ok || next != "1" {
		t.Fatalf("next cursor = %q, %v; want 1, true", next, ok)
	}
}

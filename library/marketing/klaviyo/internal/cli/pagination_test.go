// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fakeCursorRecordingClient serves canned responses and records the params of
// every request so tests can assert exactly which cursors were sent.
type fakeCursorRecordingClient struct {
	responses []string
	seen      []map[string]string
}

func (f *fakeCursorRecordingClient) GetWithHeaders(ctx context.Context, path string, params map[string]string, headers map[string]string) (json.RawMessage, error) {
	copied := map[string]string{}
	for k, v := range params {
		copied[k] = v
	}
	f.seen = append(f.seen, copied)
	if len(f.seen) > len(f.responses) {
		return nil, fmt.Errorf("unexpected request %d", len(f.seen))
	}
	return json.RawMessage(f.responses[len(f.seen)-1]), nil
}

// An explicitly passed --page-cursor must reach the first request and be
// seeded into the repeat-detection set (a dropped cursor silently re-fetches
// page 1 forever).
func TestPaginatedGetSendsExplicitInitialCursor(t *testing.T) {
	client := &fakeCursorRecordingClient{responses: []string{
		`{"data":[{"id":"a"}],"links":{"next":null}}`,
	}}
	params := map[string]string{"page[cursor]": "cursor-2"}

	result, err := paginatedGet(context.Background(), client, "/api/forms", params, nil, true, "page[cursor]", "", "page[size]", 50, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(client.seen) != 1 {
		t.Fatalf("requests = %d, want 1", len(client.seen))
	}
	if got := client.seen[0]["page[cursor]"]; got != "cursor-2" {
		t.Fatalf("page[cursor] = %q, want cursor-2", got)
	}
	if !strings.Contains(string(result), `"a"`) {
		t.Fatalf("result missing item: %s", result)
	}
}

// A has_more=true response that carries no next cursor must fail loudly
// instead of re-requesting the same page in an infinite loop.
func TestPaginatedGetRejectsHasMoreWithoutCursor(t *testing.T) {
	client := &fakeCursorRecordingClient{responses: []string{
		`{"data":[{"id":"a"}],"has_more":true}`,
	}}

	_, err := paginatedGet(context.Background(), client, "/api/items", map[string]string{}, nil, true, "page[cursor]", "", "page[size]", 50, "", "has_more")
	if err == nil || !strings.Contains(err.Error(), "provides no next cursor") {
		t.Fatalf("err = %v, want has_more-without-cursor error", err)
	}
}

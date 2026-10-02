// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/rundown/internal/store"
)

type rundownCommentPaginationClient struct {
	t      *testing.T
	params []map[string]string
}

func (c *rundownCommentPaginationClient) Get(_ context.Context, path string, params map[string]string) (json.RawMessage, error) {
	c.t.Helper()
	if path != "/posts/post-1/comments" {
		c.t.Fatalf("path = %q, want dependent comments path", path)
	}
	copyParams := make(map[string]string, len(params))
	for key, value := range params {
		copyParams[key] = value
	}
	c.params = append(c.params, copyParams)
	switch len(c.params) {
	case 1:
		return json.RawMessage(`{"comments":[{"id":"comment-1","body":"first"}],"nextCursor":"cursor-2"}`), nil
	case 2:
		return json.RawMessage(`{"comments":[{"id":"comment-2","body":"second"}],"nextCursor":null}`), nil
	default:
		c.t.Fatalf("unexpected request %d", len(c.params))
		return nil, nil
	}
}

func (c *rundownCommentPaginationClient) RateLimit() float64 { return 0 }

func TestSyncOneParent_CommentsFollowsCursorPagination(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	client := &rundownCommentPaginationClient{t: t}
	dep := dependentResourceDefs()[0]
	report := syncOneParent(
		context.Background(),
		client,
		db,
		dep,
		map[string]string{"id": "post-1"},
		dep.PathParams,
		determinePaginationDefaults("posts/comments"),
		"",
		"",
		0,
		false,
		false,
		nil,
		&bytes.Buffer{},
	)
	if report.failure != nil {
		t.Fatalf("syncOneParent failure: %v", report.failure)
	}
	if report.stored != 2 {
		t.Fatalf("stored = %d, want 2", report.stored)
	}
	if len(client.params) != 2 {
		t.Fatalf("requests = %d, want 2", len(client.params))
	}
	if got := client.params[1]["cursor"]; got != "cursor-2" {
		t.Fatalf("second request cursor = %q, want cursor-2 (params=%v)", got, client.params[1])
	}
	if _, undocumented := client.params[0]["limit"]; undocumented {
		t.Fatalf("comments request sent undocumented limit parameter: %v", client.params[0])
	}
	if _, wrong := client.params[1]["after"]; wrong {
		t.Fatalf("second request used generic after parameter: %v", client.params[1])
	}
}

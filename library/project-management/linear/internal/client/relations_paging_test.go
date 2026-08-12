// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/config"
)

// A relation set that never exhausts must not come back as a short read: the
// callers read "absent" as "no such relation", so a partial set silently drops
// links from `relations list`, duplicates an existing relation on idempotent
// create, and lets `unblocked` clear an issue whose blocker was never seen.
func TestFetchIssueRelationsRefusesToTruncate(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := requests.Add(1)
		// Every page claims another page after it, so the loop can only end
		// at the page bound.
		fmt.Fprintf(w, `{"data":{"issue":{
			"id":"11111111-1111-1111-1111-111111111111",
			"identifier":"ENG-1",
			"team":{"id":"team-1","key":"ENG"},
			"relations":{"nodes":[{"id":"rel-%d","type":"blocks"}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-%d"}},
			"inverseRelations":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}
		}}}`, n, n)
	}))
	defer srv.Close()

	c := New(&config.Config{BaseURL: srv.URL}, 0, 0)
	_, err := c.FetchIssueRelations("11111111-1111-1111-1111-111111111111", 50)
	if err == nil {
		t.Fatal("a relation set that never exhausted must be an error, not a partial result")
	}
	if !strings.Contains(err.Error(), "partial relation set") {
		t.Fatalf("error does not name the cause: %v", err)
	}
	if got := requests.Load(); got != int64(maxRelationPages) {
		t.Fatalf("paged %d times, want the %d page bound", got, maxRelationPages)
	}
}

// The ordinary case still returns on exhaustion, with both sides collected.
func TestFetchIssueRelationsReturnsOnExhaustion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":{"issue":{
			"id":"11111111-1111-1111-1111-111111111111",
			"identifier":"ENG-1",
			"team":{"id":"team-1","key":"ENG"},
			"relations":{"nodes":[{"id":"rel-1","type":"blocks"}],"pageInfo":{"hasNextPage":false,"endCursor":""}},
			"inverseRelations":{"nodes":[{"id":"rel-2","type":"blocks"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}
		}}}`)
	}))
	defer srv.Close()

	c := New(&config.Config{BaseURL: srv.URL}, 0, 0)
	res, err := c.FetchIssueRelations("11111111-1111-1111-1111-111111111111", 50)
	if err != nil {
		t.Fatalf("FetchIssueRelations: %v", err)
	}
	if len(res.Relations) != 1 || len(res.InverseRelations) != 1 {
		t.Fatalf("collected %d outgoing and %d incoming relations, want 1 and 1", len(res.Relations), len(res.InverseRelations))
	}
	if res.TeamKey != "ENG" {
		t.Fatalf("TeamKey = %q, want ENG", res.TeamKey)
	}
}

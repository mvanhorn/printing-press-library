// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: auto_refresh.go bounded-refresh tests.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/cliutil"
	"github.com/spf13/cobra"
)

// newAutoRefreshTestFixture mirrors newSyncTestFixture but returns no
// *store.Store — autoRefreshIfStale opens its own store internally.
func newAutoRefreshTestFixture(t *testing.T, handler http.HandlerFunc) (*cobra.Command, *rootFlags) {
	t.Helper()

	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	t.Cleanup(restore)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("RAPIDAPI_BASE_URL", server.URL)

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.Flags().String("query", "", "")
	cmd.Flags().String("variables", "", "")

	return cmd, &rootFlags{}
}

// TestAutoRefreshIfStale_AllResourcesSucceedMarksFresh is the happy-path
// regression test for R1/R7: once every default resource's bounded sync
// succeeds, the freshness marker must actually advance.
func TestAutoRefreshIfStale_AllResourcesSucceedMarksFresh(t *testing.T) {
	cmd, flags := newAutoRefreshTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		op := operationName(t, r)
		switch op {
		case "getCategoriesByCtx":
			writeGraphQLResponse(w, `{"data":{"categoriesByCtx":[{"id":"cat-1"}]}}`)
		case "GetCollectionsCollapsed":
			writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c1"}]}}`)
		default:
			writeGraphQLResponse(w, `{"data":{"products":{"nodes":[{"id":"api-1"}],"pageInfo":{"endCursor":"c1","hasNextPage":false}}}}`)
		}
	})

	fresh, err := cliutil.EnsureFresh(cmd.Context(), storePath(nil))
	if err != nil {
		t.Fatalf("EnsureFresh (baseline): %v", err)
	}
	if fresh {
		t.Fatal("baseline store reported fresh with no marker; want stale")
	}

	if err := autoRefreshIfStale(cmd, flags); err != nil {
		t.Fatalf("autoRefreshIfStale: %v", err)
	}

	fresh, err = cliutil.EnsureFresh(cmd.Context(), storePath(nil))
	if err != nil {
		t.Fatalf("EnsureFresh (after refresh): %v", err)
	}
	if !fresh {
		t.Fatal("freshness marker was not advanced after a fully successful refresh")
	}
}

// TestAutoRefreshIfStale_OneResourceFailsLeavesMarkerStale is the core
// regression test for "freshness advances without refresh": if any
// resource's bounded sync errors, the marker must NOT advance.
func TestAutoRefreshIfStale_OneResourceFailsLeavesMarkerStale(t *testing.T) {
	cmd, flags := newAutoRefreshTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		op := operationName(t, r)
		if op == "getCategoriesByCtx" {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"errors":[{"message":"boom"}]}`)
			return
		}
		if op == "GetCollectionsCollapsed" {
			writeGraphQLResponse(w, `{"data":{"collections":[]}}`)
			return
		}
		writeGraphQLResponse(w, `{"data":{"products":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":false}}}}`)
	})

	if err := autoRefreshIfStale(cmd, flags); err != nil {
		t.Fatalf("autoRefreshIfStale must never return an error itself (advisory): %v", err)
	}

	fresh, err := cliutil.EnsureFresh(cmd.Context(), storePath(nil))
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
	if fresh {
		t.Fatal("freshness marker was advanced despite a resource sync failure")
	}
}

// TestAutoRefreshIfStale_AlreadyFreshSkipsSync guards the existing
// short-circuit: when the store is already fresh, no sync attempt (and no
// GraphQL request) should happen at all.
func TestAutoRefreshIfStale_AlreadyFreshSkipsSync(t *testing.T) {
	requests := 0
	cmd, flags := newAutoRefreshTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeGraphQLResponse(w, `{"data":{}}`)
	})

	if err := cliutil.MarkFresh(cmd.Context(), storePath(nil), time.Now()); err != nil {
		t.Fatalf("seed MarkFresh: %v", err)
	}

	if err := autoRefreshIfStale(cmd, flags); err != nil {
		t.Fatalf("autoRefreshIfStale: %v", err)
	}
	if requests != 0 {
		t.Fatalf("made %d GraphQL requests for an already-fresh store, want 0", requests)
	}
}

// TestAutoRefreshIfStale_FailedRefreshRetriesOnNextAttempt is the
// integration test tying R1 to R7: after a failed auto-refresh leaves the
// marker stale, the next attempt must still see the store as stale (i.e.
// it retries rather than waiting out the freshness window).
func TestAutoRefreshIfStale_FailedRefreshRetriesOnNextAttempt(t *testing.T) {
	cmd, flags := newAutoRefreshTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"errors":[{"message":"boom"}]}`)
	})

	if err := autoRefreshIfStale(cmd, flags); err != nil {
		t.Fatalf("autoRefreshIfStale (first attempt): %v", err)
	}
	fresh, err := cliutil.EnsureFresh(cmd.Context(), storePath(nil))
	if err != nil {
		t.Fatalf("EnsureFresh: %v", err)
	}
	if fresh {
		t.Fatal("store reported fresh after a failed refresh; want stale so the next invocation retries")
	}
}

// TestAutoRefreshIfStale_InvokingCommandsQueryFlagDoesNotLeakIntoRefresh is
// the regression test for a real bug ce-code-review's correctness pass
// caught: autoRefreshIfStale takes the actually-invoked cobra.Command (e.g.
// `teach`, whose own --query flag is a required, unrelated natural-language
// field) and must NOT pass that command straight through to gqlExec, which
// inspects cmd.Flags().Changed("query"/"variables") as a raw-GraphQL-
// override escape hatch. Simulates `teach --query "<question>"` being the
// invoking command while the store is stale, and asserts the internal
// getCategoriesByCtx/GetCollectionsCollapsed/searchApis calls still carry
// their real baked GraphQL documents, not the teach question text.
func TestAutoRefreshIfStale_InvokingCommandsQueryFlagDoesNotLeakIntoRefresh(t *testing.T) {
	const injectedTeachQuestion = "how do I get pricing for stripe"

	var gotQueries []string
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	t.Cleanup(restore)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		_ = decodeJSONBody(r, &body)
		gotQueries = append(gotQueries, body.Query)
		writeGraphQLResponse(w, `{"data":{"categoriesByCtx":[],"collections":[],"products":{"nodes":[],"pageInfo":{"endCursor":"","hasNextPage":false}}}}`)
	}))
	t.Cleanup(server.Close)
	t.Setenv("RAPIDAPI_BASE_URL", server.URL)

	// Mirror teach.go's flag registration: a required --query flag with an
	// unrelated meaning, set (and therefore Changed) on the invoking command.
	teachCmd := &cobra.Command{}
	teachCmd.SetContext(t.Context())
	teachCmd.Flags().String("query", "", "User's original natural-language question (required)")
	if err := teachCmd.Flags().Set("query", injectedTeachQuestion); err != nil {
		t.Fatalf("setting teach --query: %v", err)
	}

	if err := autoRefreshIfStale(teachCmd, &rootFlags{}); err != nil {
		t.Fatalf("autoRefreshIfStale: %v", err)
	}

	if len(gotQueries) == 0 {
		t.Fatal("no GraphQL requests were captured; expected at least one from the default resources")
	}
	for i, q := range gotQueries {
		if q == injectedTeachQuestion {
			t.Fatalf("request %d sent the invoking command's --query value as the GraphQL document: %q", i, q)
		}
		if !strings.Contains(q, "query ") {
			t.Fatalf("request %d's GraphQL document does not look like a baked query: %q", i, q)
		}
	}
}

func decodeJSONBody(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

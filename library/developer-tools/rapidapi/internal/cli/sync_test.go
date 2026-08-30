// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: sync.go pagination and failure-propagation tests.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

// The real client transport uses a Chrome-fingerprint uTLS/HTTP2 dialer
// (client.go's newHTTPClient) to pass the hub's Cloudflare bot gate — it
// can't complete a handshake against a plain httptest.NewServer. This
// hook swaps in a plain HTTP transport whenever RAPIDAPI_BASE_URL is set
// (the documented "point at mock/test servers" signal, config.go), which
// is exactly and only when these tests point the client at a fixture
// server. Production behavior is untouched: the hook is a no-op unless a
// caller has already opted into RAPIDAPI_BASE_URL.
func init() {
	registerClientHook(func(c *client.Client) error {
		if os.Getenv("RAPIDAPI_BASE_URL") == "" {
			return nil
		}
		c.HTTPClient = &http.Client{Transport: http.DefaultTransport}
		return nil
	})
}

// newSyncTestFixture opens a temp store and builds a cobra.Command/rootFlags
// pair pointed at the given GraphQL handler via RAPIDAPI_BASE_URL, isolated
// from the real user config via cliutil.SetHomeOverride.
func newSyncTestFixture(t *testing.T, handler http.HandlerFunc) (*cobra.Command, *rootFlags, *store.Store) {
	t.Helper()

	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	t.Cleanup(restore)

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("RAPIDAPI_BASE_URL", server.URL)

	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())
	cmd.Flags().String("query", "", "")
	cmd.Flags().String("variables", "", "")
	flags := &rootFlags{}

	return cmd, flags, s
}

// operationName extracts the GraphQL operationName from a request body.
func operationName(t *testing.T, r *http.Request) string {
	t.Helper()
	var body struct {
		OperationName string `json:"operationName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decoding GraphQL request body: %v", err)
	}
	return body.OperationName
}

func writeGraphQLResponse(w http.ResponseWriter, payload string) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, payload)
}

// TestSyncAPIResource_PaginatesUntilHasNextPageFalse is the regression test
// for "sync cursor never advances": a 2-page searchApis response sequence
// must land both pages' items and persist an empty cursor at the end.
func TestSyncAPIResource_PaginatesUntilHasNextPageFalse(t *testing.T) {
	var calls []string
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, operationName(t, r))
		switch len(calls) {
		case 1:
			writeGraphQLResponse(w, `{"data":{"products":{"nodes":[{"id":"api-1"},{"id":"api-2"}],"pageInfo":{"endCursor":"cursor-1","hasNextPage":true}}}}`)
		case 2:
			writeGraphQLResponse(w, `{"data":{"products":{"nodes":[{"id":"api-3"}],"pageInfo":{"endCursor":"cursor-2","hasNextPage":false}}}}`)
		default:
			t.Fatalf("unexpected extra request %d", len(calls))
		}
	})

	count, _, err := syncResource(cmd, flags, s, "api", 50, maxSyncPages)
	if err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3 (both pages)", count)
	}
	if len(calls) != 2 {
		t.Fatalf("made %d GraphQL calls, want 2", len(calls))
	}

	cursor, _, storedCount, err := s.GetSyncState("api")
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	if cursor != "" {
		t.Fatalf("cursor = %q after full sync, want empty (fully synced)", cursor)
	}
	if storedCount != 3 {
		t.Fatalf("stored count = %d, want 3", storedCount)
	}
}

// TestSyncAPIResource_SinglePageDoesNotLoop guards the single-page happy
// path: hasNextPage:false on the first response must not trigger a second
// request, and must persist an empty cursor.
func TestSyncAPIResource_SinglePageDoesNotLoop(t *testing.T) {
	calls := 0
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		writeGraphQLResponse(w, `{"data":{"products":{"nodes":[{"id":"api-1"}],"pageInfo":{"endCursor":"cursor-1","hasNextPage":false}}}}`)
	})

	count, _, err := syncResource(cmd, flags, s, "api", 50, maxSyncPages)
	if err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d, want 1", count)
	}
	if calls != 1 {
		t.Fatalf("made %d calls, want 1 (single page must not loop)", calls)
	}
	cursor, _, _, _ := s.GetSyncState("api")
	if cursor != "" {
		t.Fatalf("cursor = %q, want empty", cursor)
	}
}

// TestSyncAPIResource_ResumesFromPersistedCursor confirms a previously-saved
// cursor is sent as `after` on the very first request of the next call.
func TestSyncAPIResource_ResumesFromPersistedCursor(t *testing.T) {
	var gotAfter []string
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables struct {
				PaginationInput struct {
					After string `json:"after"`
				} `json:"paginationInput"`
			} `json:"variables"`
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &body)
		gotAfter = append(gotAfter, body.Variables.PaginationInput.After)
		writeGraphQLResponse(w, `{"data":{"products":{"nodes":[{"id":"api-x"}],"pageInfo":{"endCursor":"cursor-final","hasNextPage":false}}}}`)
	})

	if err := s.SaveSyncState("api", "resume-cursor", 5); err != nil {
		t.Fatalf("seed SaveSyncState: %v", err)
	}

	if _, _, err := syncResource(cmd, flags, s, "api", 50, maxSyncPages); err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if len(gotAfter) != 1 || gotAfter[0] != "resume-cursor" {
		t.Fatalf("first request's after = %v, want [\"resume-cursor\"]", gotAfter)
	}
}

// TestSyncAPIResource_CapHitPreservesCursor is the boundary test for KTD3's
// deliberate deviation from the Shopify pattern: hitting maxPages must stop
// the loop but leave the real, resumable cursor in place (not clear it).
func TestSyncAPIResource_CapHitPreservesCursor(t *testing.T) {
	calls := 0
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		cursor := fmt.Sprintf("cursor-%d", calls)
		writeGraphQLResponse(w, fmt.Sprintf(`{"data":{"products":{"nodes":[{"id":"api-%d"}],"pageInfo":{"endCursor":%q,"hasNextPage":true}}}}`, calls, cursor))
	})

	count, capped, err := syncResource(cmd, flags, s, "api", 50, 2 /* maxPages */)
	if err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 (cap hit after 2 pages)", count)
	}
	if calls != 2 {
		t.Fatalf("made %d calls, want 2 (cap must stop the loop)", calls)
	}
	if !capped {
		t.Fatal("capped = false, want true (the loop stopped because maxPages was hit, not because data ran out)")
	}
	cursor, _, _, _ := s.GetSyncState("api")
	if cursor != "cursor-2" {
		t.Fatalf("cursor after cap hit = %q, want %q (must be preserved, not cleared)", cursor, "cursor-2")
	}
}

// TestSyncAPIResource_RequestErrorPropagates is the regression test for
// "sync failures report success": a hard GraphQL error must be returned to
// the caller, not swallowed.
func TestSyncAPIResource_RequestErrorPropagates(t *testing.T) {
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"errors":[{"message":"internal error"}]}`)
	})

	_, _, err := syncResource(cmd, flags, s, "api", 50, maxSyncPages)
	if err == nil {
		t.Fatal("syncResource returned nil error for a hard GraphQL failure, want non-nil")
	}
}

// TestSyncAPIResource_MalformedNodesPropagatesError guards against silently
// treating a malformed `nodes` field as "no more data": the sync must
// error out rather than exit the loop and report success on partial data.
func TestSyncAPIResource_MalformedNodesPropagatesError(t *testing.T) {
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		// "nodes" is an object, not an array — malformed relative to the
		// expected shape.
		writeGraphQLResponse(w, `{"data":{"products":{"nodes":{"unexpected":"shape"},"pageInfo":{"endCursor":"","hasNextPage":false}}}}`)
	})

	_, _, err := syncResource(cmd, flags, s, "api", 50, maxSyncPages)
	if err == nil {
		t.Fatal("syncResource returned nil error for a malformed nodes field, want non-nil")
	}
}

// TestSyncCollectionResource_PaginatesUntilPartialPage covers the
// collection branch's page-fill heuristic: a full page followed by a
// partial page must land both pages and clear the cursor.
func TestSyncCollectionResource_PaginatesUntilPartialPage(t *testing.T) {
	var calls []string
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, operationName(t, r))
		switch len(calls) {
		case 1:
			writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c1"},{"id":"c2"}]}}`)
		case 2:
			writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c3"}]}}`)
		default:
			t.Fatalf("unexpected extra request %d", len(calls))
		}
	})

	count, _, err := syncResource(cmd, flags, s, "collection", 2, maxSyncPages)
	if err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
	cursor, _, _, _ := s.GetSyncState("collection")
	if cursor != "" {
		t.Fatalf("cursor = %q after full sync, want empty", cursor)
	}
}

// TestSyncCollectionResource_ResumesFromPersistedPage confirms a saved page
// number is sent as-is on the next call, not restarted at page 1.
func TestSyncCollectionResource_ResumesFromPersistedPage(t *testing.T) {
	var gotPages []float64
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables struct {
				Page float64 `json:"page"`
			} `json:"variables"`
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &body)
		gotPages = append(gotPages, body.Variables.Page)
		writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c1"}]}}`)
	})

	if err := s.SaveSyncState("collection", "3", 10); err != nil {
		t.Fatalf("seed SaveSyncState: %v", err)
	}
	if _, _, err := syncResource(cmd, flags, s, "collection", 2, maxSyncPages); err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if len(gotPages) != 1 || gotPages[0] != 3 {
		t.Fatalf("first request's page = %v, want [3]", gotPages)
	}
}

// TestSyncCollectionResource_LegacyTimestampCursorTreatedAsNoCursor is the
// edge case for a pre-existing "page:<timestamp>" cursor left over from
// before this fix: it must resume from page 1, not error or misparse.
func TestSyncCollectionResource_LegacyTimestampCursorTreatedAsNoCursor(t *testing.T) {
	var gotPages []float64
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables struct {
				Page float64 `json:"page"`
			} `json:"variables"`
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &body)
		gotPages = append(gotPages, body.Variables.Page)
		writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c1"}]}}`)
	})

	if err := s.SaveSyncState("collection", "page:1735689600", 10); err != nil {
		t.Fatalf("seed SaveSyncState: %v", err)
	}
	if _, _, err := syncResource(cmd, flags, s, "collection", 2, maxSyncPages); err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if len(gotPages) != 1 || gotPages[0] != 1 {
		t.Fatalf("first request's page = %v, want [1] (legacy cursor must not misparse)", gotPages)
	}
}

// TestSyncCollectionResource_NonPositiveCursorTreatedAsNoCursor covers the
// `p > 0` guard's other branch: a syntactically valid but non-positive page
// number (e.g. a corrupted or manually-edited sync_state row) must also
// resume from page 1, not request page 0 or a negative page.
func TestSyncCollectionResource_NonPositiveCursorTreatedAsNoCursor(t *testing.T) {
	for _, cursor := range []string{"0", "-1"} {
		t.Run(cursor, func(t *testing.T) {
			var gotPages []float64
			cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Variables struct {
						Page float64 `json:"page"`
					} `json:"variables"`
				}
				bodyBytes, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(bodyBytes, &body)
				gotPages = append(gotPages, body.Variables.Page)
				writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c1"}]}}`)
			})

			if err := s.SaveSyncState("collection", cursor, 10); err != nil {
				t.Fatalf("seed SaveSyncState: %v", err)
			}
			if _, _, err := syncResource(cmd, flags, s, "collection", 2, maxSyncPages); err != nil {
				t.Fatalf("syncResource: %v", err)
			}
			if len(gotPages) != 1 || gotPages[0] != 1 {
				t.Fatalf("first request's page = %v, want [1] (non-positive cursor %q must not be used)", gotPages, cursor)
			}
		})
	}
}

// TestSyncCollectionResource_CapHitPreservesCursor mirrors
// TestSyncAPIResource_CapHitPreservesCursor for the collection branch's
// independent capped-cursor implementation (page-number persistence, not an
// opaque GraphQL cursor) — the two are separately reachable bugs. The
// persisted cursor must be the NEXT page to fetch (page 3, after fetching
// pages 1 and 2), not the page just consumed — a page-consumed cursor would
// make a resumed sync re-fetch the same page forever. This is the
// regression test for a real bug ce-code-review's Greptile re-review pass
// caught: the resume test below actually drives a second syncResource call
// and asserts it requests page 3, not merely that the persisted string
// value looks plausible.
func TestSyncCollectionResource_CapHitPreservesCursor(t *testing.T) {
	calls := 0
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		// A full page (2 items, matching limit) each time signals more data.
		writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c1"},{"id":"c2"}]}}`)
	})

	count, capped, err := syncResource(cmd, flags, s, "collection", 2, 2 /* maxPages */)
	if err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if count != 4 {
		t.Fatalf("count = %d, want 4 (cap hit after 2 full pages)", count)
	}
	if calls != 2 {
		t.Fatalf("made %d calls, want 2 (cap must stop the loop)", calls)
	}
	if !capped {
		t.Fatal("capped = false, want true (the loop stopped because maxPages was hit, not because data ran out)")
	}
	cursor, _, _, _ := s.GetSyncState("collection")
	if cursor != "3" {
		t.Fatalf("cursor after cap hit = %q, want %q (must be the NEXT page to fetch, not the page just consumed)", cursor, "3")
	}
}

// TestSyncCollectionResource_ResumeAfterCapFetchesNextPageNotSamePage is the
// end-to-end regression test: after a capped run, a second syncResource
// call must request the page AFTER the last one fetched — proving the
// resume cursor actually advances pagination instead of stalling on a
// repeatedly-refetched page.
func TestSyncCollectionResource_ResumeAfterCapFetchesNextPageNotSamePage(t *testing.T) {
	var gotPages []float64
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Variables struct {
				Page float64 `json:"page"`
			} `json:"variables"`
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(bodyBytes, &body)
		gotPages = append(gotPages, body.Variables.Page)
		if body.Variables.Page < 3 {
			// Pages 1 and 2: full pages (more data signaled).
			writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c1"},{"id":"c2"}]}}`)
			return
		}
		// Page 3 onward: a partial (final) page.
		writeGraphQLResponse(w, `{"data":{"collections":[{"id":"c5"}]}}`)
	})

	// First call: caps after pages 1 and 2 (maxPages=2).
	if _, capped, err := syncResource(cmd, flags, s, "collection", 2, 2); err != nil {
		t.Fatalf("first syncResource call: %v", err)
	} else if !capped {
		t.Fatal("first call: capped = false, want true")
	}
	if len(gotPages) != 2 || gotPages[0] != 1 || gotPages[1] != 2 {
		t.Fatalf("first call's requested pages = %v, want [1 2]", gotPages)
	}

	// Second call: must resume at page 3, not re-fetch page 1 or 2.
	if _, _, err := syncResource(cmd, flags, s, "collection", 2, maxSyncPages); err != nil {
		t.Fatalf("second syncResource call: %v", err)
	}
	if len(gotPages) != 3 || gotPages[2] != 3 {
		t.Fatalf("second call's requested pages = %v, want a 3rd request for page 3 (resume must not re-fetch an already-consumed page)", gotPages)
	}
}

// TestSyncAPIResource_PartialUpsertFailureDoesNotStallPagination proves the
// loop-continuation decision is driven by the raw GraphQL response shape
// (rawNodes/hasNextPage), not by cacheDomainRows' successful-upsert count —
// so a page where some items fail to upsert still advances to the next page
// instead of being mistaken for a short/final page.
func TestSyncAPIResource_PartialUpsertFailureDoesNotStallPagination(t *testing.T) {
	var calls []string
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, operationName(t, r))
		switch len(calls) {
		case 1:
			// Two nodes, but one has no extractable ID (no id/name/uuid/...
			// field) — cacheDomainRows will silently drop it, returning 1
			// stored despite 2 raw nodes. hasNextPage is still true.
			writeGraphQLResponse(w, `{"data":{"products":{"nodes":[{"id":"api-1"},{"unrelated_field":"no-id-here"}],"pageInfo":{"endCursor":"cursor-1","hasNextPage":true}}}}`)
		case 2:
			writeGraphQLResponse(w, `{"data":{"products":{"nodes":[{"id":"api-2"}],"pageInfo":{"endCursor":"cursor-2","hasNextPage":false}}}}`)
		default:
			t.Fatalf("unexpected extra request %d", len(calls))
		}
	})

	count, _, err := syncResource(cmd, flags, s, "api", 50, maxSyncPages)
	if err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("made %d calls, want 2 (a partial-upsert page must not be mistaken for the final page)", len(calls))
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 (1 successful upsert per page x 2 pages)", count)
	}
	cursor, _, storedCount, _ := s.GetSyncState("api")
	if cursor != "" {
		t.Fatalf("cursor = %q after full sync, want empty", cursor)
	}
	if storedCount != 2 {
		t.Fatalf("stored count = %d, want 2 (successful-only total, not corrupted by the dropped item)", storedCount)
	}
}

// TestSyncCategoryResource_PersistsEmptyCursor is the regression test for
// the "category" branch of "sync cursor never advances": no fake
// timestamp-shaped cursor should ever land in sync_state.last_cursor.
func TestSyncCategoryResource_PersistsEmptyCursor(t *testing.T) {
	cmd, flags, s := newSyncTestFixture(t, func(w http.ResponseWriter, r *http.Request) {
		writeGraphQLResponse(w, `{"data":{"categoriesByCtx":[{"id":"cat-1"},{"id":"cat-2"}]}}`)
	})

	count, _, err := syncResource(cmd, flags, s, "category", 50, maxSyncPages)
	if err != nil {
		t.Fatalf("syncResource: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	cursor, lastSynced, _, err := s.GetSyncState("category")
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	if cursor != "" {
		t.Fatalf("cursor = %q, want empty (no fake page:<timestamp> value)", cursor)
	}
	if strings.HasPrefix(cursor, "page:") {
		t.Fatalf("cursor %q still looks like the old fake timestamp cursor", cursor)
	}
	if lastSynced.IsZero() {
		t.Fatal("last_synced_at was not recorded")
	}
}

// TestSyncCmd_ReturnsErrorWhenAResourceHardFails is the end-to-end
// regression test for "sync failures report success": `sync`'s RunE must
// return a non-nil error when any resource hard-fails, while still
// attempting every other resource.
func TestSyncCmd_ReturnsErrorWhenAResourceHardFails(t *testing.T) {
	restore, err := cliutil.SetHomeOverride(t.TempDir())
	if err != nil {
		t.Fatalf("SetHomeOverride: %v", err)
	}
	t.Cleanup(restore)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	}))
	t.Cleanup(server.Close)
	t.Setenv("RAPIDAPI_BASE_URL", server.URL)
	t.Setenv("RAPIDAPI_HOME", t.TempDir())

	cmd := newSyncCmd(&rootFlags{})
	cmd.SetContext(t.Context())
	var stdout, stderr strings.Builder
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	err = cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("sync RunE returned nil error when a resource hard-failed, want non-nil")
	}
	if !strings.Contains(stdout.String(), "synced collection") {
		t.Fatalf("expected the collection resource to still be attempted after category failed; stdout=%q", stdout.String())
	}
}

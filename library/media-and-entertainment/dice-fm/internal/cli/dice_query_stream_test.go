// Tests for the fetchConnectionStream pagination loop.
//
// Response envelope shape exercised here:
//
//	{"data": {"viewer": {"events": {"edges": [{"node": {...}}], "pageInfo": {"hasNextPage": bool, "endCursor": string}}}}}
//
// diceQuery unwraps the outer GraphQL {"data":...} envelope and returns the
// inner object; parseConnectionPage then reads viewer.<field> from that inner
// object. The httptest server must return the full GraphQL envelope.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/config"
)

// makeNode builds a minimal JSON node with an id field.
func makeNode(id string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"id":%q}`, id))
}

// eventsPage builds a full GraphQL envelope for the events connection.
// edges is a slice of node JSON blobs, hasNext/endCursor are the pageInfo values.
func eventsPage(edges []json.RawMessage, hasNext bool, endCursor string) []byte {
	edgeParts := make([]string, len(edges))
	for i, node := range edges {
		edgeParts[i] = fmt.Sprintf(`{"node":%s}`, string(node))
	}
	edgesJSON := "[" + strings.Join(edgeParts, ",") + "]"
	body := fmt.Sprintf(
		`{"data":{"viewer":{"events":{"edges":%s,"pageInfo":{"hasNextPage":%v,"endCursor":%q}}}}}`,
		edgesJSON, hasNext, endCursor,
	)
	return []byte(body)
}

// newTestClient constructs a *client.Client pointed at srv with NoCache=true
// and a non-placeholder auth token so the placeholder-credential check passes.
func newTestClient(t *testing.T, srv *httptest.Server) *client.Client {
	t.Helper()
	cfg := &config.Config{
		BaseURL:     srv.URL,
		DiceFmToken: "test-token",
	}
	c := client.New(cfg, 5*time.Second, 0)
	c.NoCache = true
	return c
}

// TestFetchConnectionStream_MultiPageEarlyStop exercises the forward-pagination
// loop across 3 pages where the last page carries hasNextPage:false.
// Asserts: onPage called 3 times, cumulative node count is correct,
// cursors advance page-to-page, and truncated==false.
func TestFetchConnectionStream_MultiPageEarlyStop(t *testing.T) {
	// Page responses keyed by cursor arriving in the request ("" = first page).
	pages := map[string]struct {
		nodes     []json.RawMessage
		hasNext   bool
		endCursor string
	}{
		"": {
			nodes:     []json.RawMessage{makeNode("a"), makeNode("b")},
			hasNext:   true,
			endCursor: "cursor1",
		},
		"cursor1": {
			nodes:     []json.RawMessage{makeNode("c"), makeNode("d")},
			hasNext:   true,
			endCursor: "cursor2",
		},
		"cursor2": {
			nodes:     []json.RawMessage{makeNode("e")},
			hasNext:   false,
			endCursor: "",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.Unmarshal(body, &req)
		cursor, _ := req.Variables["after"].(string)
		pg, ok := pages[cursor]
		if !ok {
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(eventsPage(pg.nodes, pg.hasNext, pg.endCursor))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	var callCount int
	var totalNodes int
	seenCursors := []string{}

	truncated, err := fetchConnectionStream(
		context.Background(), c, "events", nil, 50, 0, "", false, false,
		func(pageNodes []json.RawMessage, endCursor string, fetched int) error {
			callCount++
			totalNodes += len(pageNodes)
			seenCursors = append(seenCursors, endCursor)
			if fetched != totalNodes {
				return fmt.Errorf("page %d: totalFetched=%d but accumulated=%d", callCount, fetched, totalNodes)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("fetchConnectionStream returned error: %v", err)
	}
	if truncated {
		t.Errorf("truncated = true; want false (all pages delivered without max cap)")
	}
	if callCount != 3 {
		t.Errorf("onPage called %d times; want 3", callCount)
	}
	if totalNodes != 5 {
		t.Errorf("total nodes = %d; want 5", totalNodes)
	}
	// Cursors from page 1 and 2 are "cursor1" and "cursor2"; page 3 is "".
	wantCursors := []string{"cursor1", "cursor2", ""}
	for i, want := range wantCursors {
		if i >= len(seenCursors) {
			t.Errorf("cursor[%d] missing; want %q", i, want)
			continue
		}
		if seenCursors[i] != want {
			t.Errorf("cursor[%d] = %q; want %q", i, seenCursors[i], want)
		}
	}
}

// TestFetchConnectionStream_MaxCapTruncation exercises the per-page max cap.
// The server has 3 pages of 3 nodes each (9 total); max=5 means the loop
// should stop mid-page-2 after delivering exactly 5 nodes and set truncated=true.
func TestFetchConnectionStream_MaxCapTruncation(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		cursor, _ := req.Variables["after"].(string)

		var nodes []json.RawMessage
		var hasNext bool
		var endCursor string

		switch cursor {
		case "":
			nodes = []json.RawMessage{makeNode("n1"), makeNode("n2"), makeNode("n3")}
			hasNext = true
			endCursor = "cur-p2"
		case "cur-p2":
			nodes = []json.RawMessage{makeNode("n4"), makeNode("n5"), makeNode("n6")}
			hasNext = true
			endCursor = "cur-p3"
		case "cur-p3":
			// Should not be reached because max=5 caps before page 3.
			nodes = []json.RawMessage{makeNode("n7"), makeNode("n8"), makeNode("n9")}
			hasNext = false
			endCursor = ""
		default:
			http.Error(w, "unexpected cursor: "+cursor, http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(eventsPage(nodes, hasNext, endCursor))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	var deliveredNodes int
	var callCount int

	truncated, err := fetchConnectionStream(
		context.Background(), c, "events", nil, 3, 5, "", false, false,
		func(pageNodes []json.RawMessage, endCursor string, fetched int) error {
			callCount++
			deliveredNodes += len(pageNodes)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("fetchConnectionStream returned error: %v", err)
	}
	if !truncated {
		t.Errorf("truncated = false; want true (max=5 hit while more records remain)")
	}
	if deliveredNodes != 5 {
		t.Errorf("delivered %d nodes; want exactly 5 (max cap)", deliveredNodes)
	}
	// Page 1 (3 nodes) + page 2 capped at 2 = 2 onPage calls.
	if callCount != 2 {
		t.Errorf("onPage called %d times; want 2", callCount)
	}
	// Third page must never be requested.
	if requestCount >= 3 {
		t.Errorf("server received %d requests; want ≤2 (page 3 should not be fetched)", requestCount)
	}
}

// TestFetchConnectionStream_OnPageErrorStopsLoop checks that an error returned
// by onPage stops the pagination loop immediately and is surfaced to the caller.
func TestFetchConnectionStream_OnPageErrorStopsLoop(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		nodes := []json.RawMessage{makeNode("x1"), makeNode("x2")}
		// Always advertise hasNext:true with a stable cursor so the loop would
		// continue forever if onPage did not stop it.
		w.Header().Set("Content-Type", "application/json")
		w.Write(eventsPage(nodes, true, "next-cursor"))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	sentinel := fmt.Errorf("stop-sentinel")
	var callCount int

	_, err := fetchConnectionStream(
		context.Background(), c, "events", nil, 50, 0, "", false, false,
		func(pageNodes []json.RawMessage, endCursor string, fetched int) error {
			callCount++
			return sentinel
		},
	)
	if err == nil {
		t.Fatalf("expected error from onPage to be propagated; got nil")
	}
	if err != sentinel {
		t.Errorf("returned error = %v; want sentinel %v", err, sentinel)
	}
	// onPage must have been called exactly once before the loop stopped.
	if callCount != 1 {
		t.Errorf("onPage called %d times after error; want 1", callCount)
	}
	// Only one HTTP request should have been made (the first page).
	if requestCount != 1 {
		t.Errorf("server received %d requests; want 1 (loop stops on onPage error)", requestCount)
	}
}

// TestFetchConnectionStream_UnknownResource checks that an unrecognised resource
// name returns an error without making any HTTP request.
func TestFetchConnectionStream_UnknownResource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler called for unknown resource; no HTTP request should be made")
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)

	_, err := fetchConnectionStream(
		context.Background(), c, "does-not-exist", nil, 50, 0, "", false, false,
		func(_ []json.RawMessage, _ string, _ int) error { return nil },
	)
	if err == nil {
		t.Fatal("expected error for unknown resource; got nil")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error %q should mention the unknown resource name", err.Error())
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
)

func TestFlexCursorUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "number", input: `53970093`, want: "53970093"},
		{name: "string", input: `"01JZABCDEFGHJKMNPQRSTVWXYZ"`, want: "01JZABCDEFGHJKMNPQRSTVWXYZ"},
		{name: "null", input: `null`, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got flexCursor
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("UnmarshalJSON() error = %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("cursor = %q, want %q", got.String(), tt.want)
			}
		})
	}
}

func TestNormalizeReadwiseExportBook(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantBookID      string
		wantBookIDKey   string
		wantBookUpdated string
		wantHLBookID    string
		wantHLUpdated   string
		wantHLCount     int
	}{
		{
			name: "maps user_book_id to id and highlights",
			input: `{
				"user_book_id": 42,
				"title": "Example",
				"updated_at": "2026-06-01T00:00:00Z",
				"highlights": [
					{"id": 7, "text": "quote", "updated_at": "2026-06-02T00:00:00Z"}
				]
			}`,
			wantBookID:      "42",
			wantBookIDKey:   "42",
			wantBookUpdated: "2026-06-01T00:00:00Z",
			wantHLBookID:    "42",
			wantHLUpdated:   "2026-06-02T00:00:00Z",
			wantHLCount:     1,
		},
		{
			name: "preserves nested highlight book id",
			input: `{
				"user_book_id": 101,
				"title": "Example",
				"highlights": [
					{"id": 9, "text": "quote", "book_id": 202}
				]
			}`,
			wantBookID:    "101",
			wantBookIDKey: "101",
			wantHLBookID:  "202",
			wantHLCount:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			book, highlights, err := normalizeReadwiseExportBook(json.RawMessage(tt.input))
			if err != nil {
				t.Fatalf("normalizeReadwiseExportBook() error = %v", err)
			}
			var bookObj map[string]any
			if err := json.Unmarshal(book, &bookObj); err != nil {
				t.Fatal(err)
			}
			if got := store.ResourceIDString(bookObj["id"]); got != tt.wantBookID {
				t.Fatalf("book id = %q, want %q", got, tt.wantBookID)
			}
			if _, ok := bookObj["highlights"]; ok {
				t.Fatalf("normalized book retained highlights array")
			}
			if got := store.ResourceIDString(bookObj["user_book_id"]); got != tt.wantBookIDKey {
				t.Fatalf("user_book_id = %q, want %q", got, tt.wantBookIDKey)
			}
			if tt.wantBookUpdated != "" {
				if got := store.ResourceIDString(bookObj["updated"]); got != tt.wantBookUpdated {
					t.Fatalf("book updated = %q, want %q", got, tt.wantBookUpdated)
				}
			}
			if len(highlights) != tt.wantHLCount {
				t.Fatalf("highlights len = %d, want %d", len(highlights), tt.wantHLCount)
			}
			if tt.wantHLCount > 0 {
				var hObj map[string]any
				if err := json.Unmarshal(highlights[0], &hObj); err != nil {
					t.Fatal(err)
				}
				if got := store.ResourceIDString(hObj["book_id"]); got != tt.wantHLBookID {
					t.Fatalf("highlight book_id = %q, want %q", got, tt.wantHLBookID)
				}
				if tt.wantHLUpdated != "" {
					if got := store.ResourceIDString(hObj["updated"]); got != tt.wantHLUpdated {
						t.Fatalf("highlight updated = %q, want %q", got, tt.wantHLUpdated)
					}
				}
			}
		})
	}
}

func TestNormalizeReadwiseExportBookSetsUpdatedToNewestBookTimestamp(t *testing.T) {
	book, _, err := normalizeReadwiseExportBook(json.RawMessage(`{
		"user_book_id": 77,
		"title": "Newest Timestamp",
		"updated_at": "2026-06-01T00:00:00Z",
		"last_highlight_at": "2026-06-03T00:00:00Z",
		"highlights": []
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(book, &obj); err != nil {
		t.Fatal(err)
	}
	if got := store.ResourceIDString(obj["updated"]); got != "2026-06-03T00:00:00Z" {
		t.Fatalf("book updated = %q, want newest timestamp", got)
	}
}

func TestReadwiseSyncRateForResource(t *testing.T) {
	tests := []struct {
		resource string
		wantMax  float64
		wantMin  float64
	}{
		{resource: "documents", wantMin: 0.32, wantMax: 1.0 / 3.0},
		{resource: "reader-tags", wantMin: 0.32, wantMax: 1.0 / 3.0},
		{resource: "readwise-books-list", wantMin: 0.32, wantMax: 1.0 / 3.0},
		{resource: "readwise-export", wantMin: 2.0, wantMax: 3.0},
		{resource: "books", wantMin: 2.0, wantMax: 3.0},
	}
	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			got := readwiseSyncRateForResource(tt.resource)
			if got < tt.wantMin || got > tt.wantMax {
				t.Fatalf("rate = %f, want between %f and %f", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

type stubReadwiseGetter struct {
	responses []json.RawMessage
	paths     []string
	calls     []map[string]string
}

func (s *stubReadwiseGetter) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	copied := map[string]string{}
	for k, v := range params {
		copied[k] = v
	}
	s.paths = append(s.paths, path)
	s.calls = append(s.calls, copied)
	if len(s.responses) == 0 {
		return nil, context.Canceled
	}
	next := s.responses[0]
	s.responses = s.responses[1:]
	return next, nil
}

func (s *stubReadwiseGetter) RateLimit() float64 { return readwiseExportRatePerSecond }

func TestFixedPacedClientFirstCallImmediateThenPaces(t *testing.T) {
	client := newFixedPacedClient(&stubReadwiseGetter{responses: []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`{}`),
		json.RawMessage(`{}`),
	}}, 10)

	start := time.Now()
	if _, err := client.Get(context.Background(), "/first", nil); err != nil {
		t.Fatal(err)
	}
	firstElapsed := time.Since(start)
	if firstElapsed > 80*time.Millisecond {
		t.Fatalf("first paced call took %s, want near-immediate", firstElapsed)
	}
	if _, err := client.Get(context.Background(), "/second", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(context.Background(), "/third", nil); err != nil {
		t.Fatal(err)
	}
	totalElapsed := time.Since(start)
	if totalElapsed < 180*time.Millisecond {
		t.Fatalf("three paced calls took %s, want at least two 100ms intervals", totalElapsed)
	}
}

func TestSyncReadwiseExportFollowsCursorAndStoresRows(t *testing.T) {
	tests := []struct {
		name                 string
		responses            []json.RawMessage
		wantCalls            int
		wantSecondPageCursor string
		wantBooks            int
		wantHighlights       int
	}{
		{
			name: "follows string nextPageCursor until null",
			responses: []json.RawMessage{
				json.RawMessage(`{"count":2,"nextPageCursor":"abc","results":[{"user_book_id":1,"title":"One","highlights":[{"id":10,"text":"A"}]}]}`),
				json.RawMessage(`{"count":2,"nextPageCursor":null,"results":[{"user_book_id":2,"title":"Two","highlights":[{"id":20,"text":"B","book_id":2}]}]}`),
			},
			wantCalls:            2,
			wantSecondPageCursor: "abc",
			wantBooks:            2,
			wantHighlights:       2,
		},
		{
			name: "follows numeric nextPageCursor until null",
			responses: []json.RawMessage{
				json.RawMessage(`{"count":2,"nextPageCursor":53970093,"results":[{"user_book_id":1,"title":"One","highlights":[{"id":10,"text":"A"}]}]}`),
				json.RawMessage(`{"count":2,"nextPageCursor":null,"results":[{"user_book_id":2,"title":"Two","highlights":[{"id":20,"text":"B","book_id":2}]}]}`),
			},
			wantCalls:            2,
			wantSecondPageCursor: "53970093",
			wantBooks:            2,
			wantHighlights:       2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()

			client := &stubReadwiseGetter{responses: tt.responses}
			results := syncReadwiseExport(context.Background(), client, nil, db, "", false, 0, false, nil, nil)
			if len(results) != 2 {
				t.Fatalf("results len = %d, want 2", len(results))
			}
			for _, result := range results {
				if result.Err != nil {
					t.Fatalf("%s error = %v", result.Resource, result.Err)
				}
			}
			if len(client.calls) != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", len(client.calls), tt.wantCalls)
			}
			if tt.wantSecondPageCursor != "" {
				if got := client.calls[1]["pageCursor"]; got != tt.wantSecondPageCursor {
					t.Fatalf("second pageCursor = %q, want %q", got, tt.wantSecondPageCursor)
				}
			}
			bookCount, err := db.Count("books")
			if err != nil {
				t.Fatal(err)
			}
			if bookCount != tt.wantBooks {
				t.Fatalf("books count = %d, want %d", bookCount, tt.wantBooks)
			}
			highlightCount, err := db.Count("highlights")
			if err != nil {
				t.Fatal(err)
			}
			if highlightCount != tt.wantHighlights {
				t.Fatalf("highlights count = %d, want %d", highlightCount, tt.wantHighlights)
			}
			cursor, _, count, err := db.GetSyncState("readwise-export")
			if err != nil {
				t.Fatal(err)
			}
			if cursor != "" {
				t.Fatalf("readwise-export cursor = %q, want cleared cursor after completion", cursor)
			}
			if count != tt.wantBooks+tt.wantHighlights {
				t.Fatalf("readwise-export sync count = %d, want %d", count, tt.wantBooks+tt.wantHighlights)
			}
		})
	}
}

func TestSyncReadwiseExportSkipsMalformedBookAndContinues(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	client := &stubReadwiseGetter{responses: []json.RawMessage{
		json.RawMessage(`{"count":2,"nextPageCursor":"next","results":[{"user_book_id":1,"title":"Good"},{"title":"Missing ID"}]}`),
		json.RawMessage(`{"count":2,"nextPageCursor":null,"results":[{"user_book_id":2,"title":"Second"}]}`),
	}}

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	results := syncReadwiseExport(context.Background(), client, nil, db, "", false, 0, false, nil, io.Discard)
	_ = w.Close()
	os.Stderr = oldStderr
	warnRaw, _ := io.ReadAll(r)
	_ = r.Close()

	for _, result := range results {
		if result.Err != nil {
			t.Fatalf("%s error = %v", result.Resource, result.Err)
		}
	}
	if len(client.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(client.calls))
	}
	bookCount, err := db.Count("books")
	if err != nil {
		t.Fatal(err)
	}
	if bookCount != 2 {
		t.Fatalf("books count = %d, want 2", bookCount)
	}
	warn := string(warnRaw)
	if !strings.Contains(warn, "skipped 1 malformed readwise export book") {
		t.Fatalf("stderr warning = %q, want malformed-book warning", warn)
	}
}

func TestSyncReadwiseExportAlsoUpsertsBooksListZeroHighlightAndEnrichment(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	exportClient := &stubReadwiseGetter{responses: []json.RawMessage{
		json.RawMessage(`{"count":1,"nextPageCursor":null,"results":[{"user_book_id":1,"title":"Highlighted","highlights":[{"id":10,"text":"A"}]}]}`),
	}}
	booksListClient := &stubReadwiseGetter{responses: []json.RawMessage{
		json.RawMessage(`{"count":2,"next":null,"previous":null,"results":[
			{"id":1,"title":"Highlighted","author":"A","category":"books","source":"kindle","num_highlights":1,"last_highlight_at":"2026-06-01T00:00:00Z","updated":"2026-06-02T00:00:00Z","cover_image_url":"https://example.com/cover.jpg","highlights_url":"https://readwise.io/bookreview/1","source_url":"https://example.com/source","tags":["tag"],"document_note":"note"},
			{"id":2,"title":"Zero Highlight","author":"B","category":"articles","source":"reader","num_highlights":0,"last_highlight_at":null,"updated":"2026-06-03T00:00:00Z","highlights_url":"https://readwise.io/bookreview/2","source_url":"https://example.com/zero"}
		]}`),
	}}

	results := syncReadwiseExport(context.Background(), exportClient, booksListClient, db, "", false, 0, false, nil, io.Discard)
	for _, result := range results {
		if result.Err != nil {
			t.Fatalf("%s error = %v", result.Resource, result.Err)
		}
	}

	bookCount, err := db.Count("books")
	if err != nil {
		t.Fatal(err)
	}
	if bookCount != 2 {
		t.Fatalf("books count = %d, want 2", bookCount)
	}
	if got := results[0].Count; got != 3 {
		t.Fatalf("books sync count = %d, want export plus list upserts", got)
	}

	var zeroTitle string
	var zeroHighlights int
	if err := db.DB().QueryRow(`SELECT title, num_highlights FROM books WHERE id = ?`, "2").Scan(&zeroTitle, &zeroHighlights); err != nil {
		t.Fatal(err)
	}
	if zeroTitle != "Zero Highlight" || zeroHighlights != 0 {
		t.Fatalf("zero-highlight book = (%q, %d), want stored with 0 highlights", zeroTitle, zeroHighlights)
	}

	var lastHighlightAt, updated, highlightsURL string
	if err := db.DB().QueryRow(`SELECT last_highlight_at, updated, highlights_url FROM books WHERE id = ?`, "1").Scan(&lastHighlightAt, &updated, &highlightsURL); err != nil {
		t.Fatal(err)
	}
	if lastHighlightAt != "2026-06-01T00:00:00Z" {
		t.Fatalf("last_highlight_at = %q, want /books/ value", lastHighlightAt)
	}
	if updated != "2026-06-02T00:00:00Z" {
		t.Fatalf("updated = %q, want /books/ value", updated)
	}
	if highlightsURL != "https://readwise.io/bookreview/1" {
		t.Fatalf("highlights_url = %q, want /books/ value", highlightsURL)
	}
}

func TestSyncReadwiseBooksListFollowsPageNextAndStopsOnEmpty(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	client := &stubReadwiseGetter{responses: []json.RawMessage{
		json.RawMessage(`{"count":3,"next":"https://readwise.io/api/v2/books/?page=2&page_size=1000","previous":null,"results":[{"id":1,"title":"One"}]}`),
		json.RawMessage(`{"count":3,"next":"https://readwise.io/api/v2/books/?page=3&page_size=1000","previous":null,"results":[{"id":2,"title":"Two"}]}`),
		json.RawMessage(`{"count":3,"next":"https://readwise.io/api/v2/books/?page=4&page_size=1000","previous":null,"results":[]}`),
	}}

	count, err := syncReadwiseBooksList(context.Background(), client, db, "2026-06-01T00:00:00Z", 0, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if len(client.calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(client.calls))
	}
	for i, wantPage := range []string{"1", "2", "3"} {
		if got := client.calls[i]["page"]; got != wantPage {
			t.Fatalf("call %d page = %q, want %q", i+1, got, wantPage)
		}
		if got := client.calls[i]["page_size"]; got != "1000" {
			t.Fatalf("call %d page_size = %q, want 1000", i+1, got)
		}
		if got := client.calls[i]["updated__gt"]; got != "2026-06-01T00:00:00Z" {
			t.Fatalf("call %d updated__gt = %q, want incremental timestamp", i+1, got)
		}
	}
	if client.paths[0] != "/books/" {
		t.Fatalf("path = %q, want /books/", client.paths[0])
	}
}

func TestSyncErrorJSONRedactsNonAPIErrors(t *testing.T) {
	line := syncErrorJSON("books", "", fmt.Errorf("decoding export page: %w", errors.New("secret highlight text")))
	if strings.Contains(line, "secret highlight text") || strings.Contains(line, "decoding export page") {
		t.Fatalf("syncErrorJSON leaked raw non-API error: %s", line)
	}
	if !strings.Contains(line, "sync failed") {
		t.Fatalf("syncErrorJSON = %s, want generic diagnostic", line)
	}
}

func TestSyncReadwiseCursorResourceFollowsCursorAndStopsOnEmpty(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	client := &stubReadwiseGetter{responses: []json.RawMessage{
		json.RawMessage(`{"count":3,"nextPageCursor":"01JZABCDEFGHJKMNPQRSTVWXYZ","results":[{"id":"doc-1","title":"One"}]}`),
		json.RawMessage(`{"count":3,"nextPageCursor":"","results":[{"id":"doc-2","title":"Two"}]}`),
	}}
	result := syncReadwiseCursorResource(context.Background(), client, db, "documents", "https://readwise.io/api/v3/list/", "", false, 0, false, nil, io.Discard, true)
	if result.Err != nil {
		t.Fatalf("syncReadwiseCursorResource() error = %v", result.Err)
	}
	if len(client.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(client.calls))
	}
	if got := client.calls[0]["pageCursor"]; got != "" {
		t.Fatalf("first pageCursor = %q, want empty", got)
	}
	if got := client.calls[1]["pageCursor"]; got != "01JZABCDEFGHJKMNPQRSTVWXYZ" {
		t.Fatalf("second pageCursor = %q, want ULID cursor", got)
	}
	if result.Count != 2 {
		t.Fatalf("result count = %d, want 2", result.Count)
	}
	docCount, err := db.Count("documents")
	if err != nil {
		t.Fatal(err)
	}
	if docCount != 2 {
		t.Fatalf("documents count = %d, want 2", docCount)
	}
}

func TestE2ResumePreservesFilter(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	since := "2020-01-01T00:00:00Z"
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"nextPageCursor":"next","results":[]}`)}}
	for _, res := range syncReadwiseExport(context.Background(), c, nil, db, since, false, 1, false, nil, nil) {
		if res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	_, at, _, err := db.GetSyncState("readwise-export")
	if err != nil || !at.IsZero() {
		t.Fatalf("partial watermark=%v err=%v", at, err)
	}
	c = &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[]}`)}}
	for _, res := range syncReadwiseExport(context.Background(), c, nil, db, "", false, 0, false, nil, nil) {
		if res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	q := c.calls[0]
	if q["pageCursor"] != "next" || q["updatedAfter"] != since || q["includeDeleted"] != "true" {
		t.Fatalf("resume query=%v", q)
	}
}
func TestE2DefaultAndFullTombstones(t *testing.T) {
	for _, full := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "full"}[full], func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.UpsertHighlights(json.RawMessage(`{"id":2,"text":"vanished"}`)); err != nil {
				t.Fatal(err)
			}
			if err := db.SaveSyncState("readwise-export", "", 1); err != nil {
				t.Fatal(err)
			}
			c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[{"user_book_id":1,"highlights":[{"id":2,"is_deleted":true}]}]}`)}}
			results := syncReadwiseExport(context.Background(), c, nil, db, "", full, 0, false, nil, nil)
			if c.calls[0]["includeDeleted"] != "true" {
				t.Fatalf("tombstone query=%v", c.calls[0])
			}
			if !full && c.calls[0]["updatedAfter"] == "" {
				t.Fatal("default sync lost watermark")
			}
			n, err := db.Count("highlights")
			if err != nil || n != 0 || results[1].Deleted != 1 {
				t.Fatalf("ghost count=%d results=%v err=%v", n, results, err)
			}
		})
	}
}
func TestE2ReaderReconciliation(t *testing.T) {
	for _, capPages := range []int{0, 1} {
		t.Run(map[int]string{0: "uncapped", 1: "capped"}[capPages], func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, raw := range []string{`{"id":"old","title":"gone"}`, `{"id":"child","parent_id":"old","category":"highlight"}`, `{"id":"new","title":"kept"}`} {
				if err := db.UpsertDocuments(json.RawMessage(raw)); err != nil {
					t.Fatal(err)
				}
			}
			next := ""
			if capPages > 0 {
				next = "next"
			}
			c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(fmt.Sprintf(`{"nextPageCursor":%q,"results":[{"id":"new","title":"kept"}]}`, next))}}
			res := syncReadwiseCursorResource(context.Background(), c, db, "documents", "/list/", "", true, capPages, false, nil, io.Discard, true)
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			n, err := db.Count("documents")
			want := 1
			if capPages > 0 {
				want = 3
			}
			if err != nil || n != want {
				t.Fatalf("documents=%d want=%d err=%v", n, want, err)
			}
		})
	}
}
func TestE2ArchivePartialTimeout(t *testing.T) {
	err := finishReadwiseSyncResults([]syncResult{{Resource: "books"}, {Resource: "documents", Err: context.DeadlineExceeded}}, false, time.Now(), io.Discard)
	err = workflowArchiveTimeoutError(time.Millisecond, err)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "timed out after 1ms") {
		t.Fatalf("archive timeout lost: %v", err)
	}
}

func TestE2FailedFullClearsWatermark(t *testing.T) {
	fail := true
	cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"detail":"invalid"}`)
			return
		}
		if r.URL.Query().Has("updatedAfter") || r.URL.Query().Has("updated__gt") {
			t.Errorf("failed full advanced watermark: %s", r.URL)
		}
		fmt.Fprint(w, `{"results":[]}`)
	})
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSyncState("readwise-export", "", 1); err != nil {
		t.Fatal(err)
	}
	db.Close()
	_, _, code := sliceBExecute(t, cfg, "sync", "--resources", "books", "--db", path, "--full", "--json")
	if code == 0 {
		t.Fatal("failed full succeeded")
	}
	fail = false
	out, stderr, code := sliceBExecute(t, cfg, "sync", "--resources", "books", "--db", path, "--json")
	if code != 0 {
		t.Fatalf("retry exit=%d out=%s err=%s", code, out, stderr)
	}
}
func TestE2ArchiveCommandTimeout(t *testing.T) {
	cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		fmt.Fprint(w, `{"results":[]}`)
	})
	_, stderr, code := sliceBExecute(t, cfg, "workflow", "archive", "--db", path, "--timeout", "1ms", "--json")
	if code == 0 || !strings.Contains(stderr, "timed out after 1ms") {
		t.Fatalf("archive exit=%d stderr=%s", code, stderr)
	}
}
func TestE2LatestOnlyClearsCursor(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SaveReadwiseSyncProgress("readwise-export", "stale", 1, "2020-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"nextPageCursor":"next","results":[]}`)}}
	for _, res := range syncReadwiseExport(context.Background(), c, nil, db, "", false, 1, true, nil, nil) {
		if res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	cursor, _, _, err := db.GetSyncState("readwise-export")
	if err != nil || cursor != "" || c.calls[0]["pageCursor"] != "" {
		t.Fatalf("latestOnly resumed or retained cursor: %s %v %v", cursor, c.calls, err)
	}
}
func TestE2FullSummaryAndIncrementalBookCount(t *testing.T) {
	cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/export/":
			fmt.Fprint(w, `{"results":[{"user_book_id":1,"title":"book","highlights":[{"id":2,"text":"new quote"}]}]}`)
		case "/books/":
			fmt.Fprint(w, `{"results":[]}`)
		default:
			fmt.Fprint(w, `{"results":[{"id":"doc","title":"article"}]}`)
		}
	})
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertBooks(json.RawMessage(`{"id":1,"num_highlights":30}`)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	out, stderr, code := sliceBExecute(t, cfg, "sync", "--resources", "books,documents", "--db", path, "--full", "--json")
	if code != 0 || strings.Count(out, `"event":"sync_summary"`) != 1 || !strings.Contains(out, `"resource":"documents"`) || strings.Contains(stderr, `"event":"sync_`) {
		t.Fatalf("mixed output: exit=%d out=%s stderr=%s", code, out, stderr)
	}
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.DB().QueryRow(`SELECT num_highlights FROM books WHERE id='1'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 30 {
		t.Fatalf("partial export overwrote total: %d", count)
	}
}

func TestE2WatermarkUsesRunStart(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"nextPageCursor":"next","results":[]}`), json.RawMessage(`{"results":[]}`)}}
	start := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	ctx := context.WithValue(context.Background(), readwiseRunStartKey{}, start)
	results := syncReadwiseExport(ctx, c, nil, db, "", false, 0, false, nil, nil)
	for _, res := range results {
		if res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	_, at, _, err := db.GetSyncState("readwise-export")
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(start) {
		t.Fatalf("watermark=%s want run start=%s", at, start)
	}
}

func TestE2SyncPreservesCauseAndDefaultWarning(t *testing.T) {
	cause := &client.APIError{StatusCode: 401}
	results := []syncResult{{Resource: "books", Err: cause}, {Resource: "documents"}}
	var out bytes.Buffer
	if err := finishReadwiseSyncResults(results, false, time.Now(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"reason":"exit_policy_default_changed"`) {
		t.Fatalf("partial sync missing warning: %s", out.String())
	}
	for _, strict := range []bool{true, false} {
		selected := results
		if !strict {
			selected = results[:1]
		}
		err := finishReadwiseSyncResults(selected, strict, time.Now(), io.Discard)
		var apiErr *client.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
			t.Fatalf("auth cause lost: %v", err)
		}
	}
}
func TestE2TombstoneErrorEvent(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[{"user_book_id":1,"highlights":[{"is_deleted":true}]}]}`)}}
	var out bytes.Buffer
	results := syncReadwiseExport(context.Background(), c, nil, db, "", false, 0, false, nil, &out)
	if len(results) != 1 || results[0].Err == nil || !strings.Contains(out.String(), `"event":"sync_error"`) {
		t.Fatalf("silent tombstone error: %v %s", results, out.String())
	}
}
func TestE2FilteredReaderNeverPrunes(t *testing.T) {
	for _, mode := range []string{"location", "no_prune", "since"} {
		db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
		if err != nil {
			t.Fatal(err)
		}
		if err := db.UpsertDocuments(json.RawMessage(`{"id":"old","title":"kept"}`)); err != nil {
			t.Fatal(err)
		}
		params := &syncUserParams{flatGlobal: map[string]string{}}
		since := ""
		prune := true
		if mode == "location" {
			params.flatGlobal["location"] = "new"
		}
		if mode == "no_prune" {
			prune = false
		}
		if mode == "since" {
			since = "2020-01-01T00:00:00Z"
		}
		c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[]}`)}}
		res := syncReadwiseCursorResource(context.Background(), c, db, "documents", "/list/", since, true, 0, false, params, io.Discard, prune)
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		n, err := db.Count("documents")
		db.Close()
		if err != nil || n != 1 {
			t.Fatalf("unsafe prune mode=%s count=%d err=%v", mode, n, err)
		}
	}
}

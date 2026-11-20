package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func cursorEnvelope(t *testing.T, out string) (map[string]any, []map[string]any) {
	t.Helper()
	var envelope struct {
		Meta    map[string]any   `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatal(err, out)
	}
	return envelope.Meta, envelope.Results
}

func TestLiveListReaderCursorRoundTrip(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v3/list/" {
			t.Errorf("path: %s", r.URL)
		}
		switch calls {
		case 1:
			if r.URL.Query().Get("pageCursor") != "" {
				t.Errorf("unexpected cursor: %s", r.URL)
			}
			fmt.Fprint(w, `{"count":2,"nextPageCursor":"abc","results":[{"id":"doc-1"}]}`)
		case 2:
			if r.URL.Query().Get("pageCursor") != "abc" {
				t.Errorf("cursor not forwarded: %s", r.URL)
			}
			fmt.Fprint(w, `{"count":2,"nextPageCursor":null,"results":[{"id":"doc-2"}]}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	})
	out, stderr, code := sliceBExecute(t, cfg, "documents", "list", "--json")
	if code != 0 {
		t.Fatal(code, out, stderr)
	}
	if !strings.Contains(stderr, "pass --all, or --page-cursor abc") {
		t.Fatal(stderr)
	}
	meta, items := cursorEnvelope(t, out)
	if meta["next_cursor"] != "abc" || meta["next_cursor_flag"] != "--page-cursor" || meta["has_more"] != true || meta["count"] != nil || meta["api_count"] != float64(2) || len(items) != 1 {
		t.Fatal(out)
	}
	out, stderr, code = sliceBExecute(t, cfg, "documents", "list", "--data-source", "live", "--json", meta["next_cursor_flag"].(string), meta["next_cursor"].(string))
	if code != 0 {
		t.Fatal(code, out, stderr)
	}
	if strings.Contains(stderr, "truncated") {
		t.Fatal(stderr)
	}
	meta, items = cursorEnvelope(t, out)
	if _, ok := meta["next_cursor"]; ok {
		t.Fatal(out)
	}
	if _, ok := meta["next_cursor_flag"]; ok {
		t.Fatal(out)
	}
	if meta["has_more"] != false || len(items) != 1 || items[0]["id"] != "doc-2" || calls != 2 {
		t.Fatal(out, calls)
	}
}

func TestLiveListV2PageCursor(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		page := "2"
		if calls == 2 {
			page = "7"
		}
		if r.URL.Query().Get("page") != page || r.URL.Query().Get("page_size") != "2" {
			t.Errorf("query: %s", r.URL)
		}
		if calls == 1 {
			fmt.Fprint(w, `{"count":5,"next":"https://example.test/api/v2/books/?page=7&page_size=2","results":[{"id":1},{"id":2}]}`)
		} else {
			fmt.Fprint(w, `{"count":5,"next":null,"results":[{"id":3}]}`)
		}
	})
	out, stderr, code := sliceBExecute(t, cfg, "books", "list", "--page", "2", "--page-size", "2", "--data-source", "live", "--json")
	if code != 0 {
		t.Fatal(code, out, stderr)
	}
	var event map[string]string
	if json.Unmarshal([]byte(stderr), &event) != nil || event["next_cursor"] != "7" || event["next_cursor_flag"] != "--page" || !strings.Contains(event["hint"], "--page 7") {
		t.Fatal(stderr)
	}
	meta, items := cursorEnvelope(t, out)
	if meta["next_cursor"] != "7" || meta["next_cursor_flag"] != "--page" || meta["count"] != nil || meta["api_count"] != float64(5) || meta["has_more"] != true || len(items) != 2 {
		t.Fatal(out)
	}
	out, stderr, code = sliceBExecute(t, cfg, "books", "list", "--page", meta["next_cursor"].(string), "--page-size", "2", "--data-source", "live", "--json")
	meta, _ = cursorEnvelope(t, out)
	if code != 0 || meta["has_more"] != false || meta["next_cursor"] != nil || meta["next_cursor_flag"] != nil || strings.Contains(stderr, "truncated") || calls != 2 {
		t.Fatal(out, stderr, calls)
	}

}

func TestLiveListAllBytesUnchanged(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			fmt.Fprint(w, `{"count":2,"nextPageCursor":"abc","results":[{"id":"doc-1"}]}`)
			return
		}
		if calls != 2 || r.URL.Query().Get("pageCursor") != "abc" {
			t.Errorf("request %d: %s", calls, r.URL)
		}
		fmt.Fprint(w, `{"count":2,"nextPageCursor":null,"results":[{"id":"doc-2"}]}`)
	})
	out, stderr, code := sliceBExecute(t, cfg, "documents", "list", "--all", "--data-source", "live", "--json")
	const before = "{\n  \"meta\": {\n    \"source\": \"live\"\n  },\n  \"results\": [\n    {\n      \"id\": \"doc-1\"\n    },\n    {\n      \"id\": \"doc-2\"\n    }\n  ]\n}\n"
	if code != 0 || out != before || calls != 2 || strings.Contains(stderr, "truncated") {
		t.Fatalf("exit=%d calls=%d\n%s\n%s", code, calls, out, stderr)
	}
}

func TestLiveListTruncationHint(t *testing.T) {
	for _, human := range []bool{false, true} {
		t.Run(fmt.Sprint(human), func(t *testing.T) {
			// humanFriendly is process-global, so these subtests stay serial.
			oldHuman := humanFriendly
			humanFriendly = human
			defer func() { humanFriendly = oldHuman }()
			var capture bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetErr(&capture)
			ctx := context.WithValue(t.Context(), liveListCommandKey{}, cmd)
			emitTruncationWarning(ctx, map[string]any{"has_more": true, "next_cursor": "abc&123", "next_cursor_flag": "--page-cursor"}, true)
			raw := capture.Bytes()
			if !strings.Contains(string(raw), "pass --all, or --page-cursor 'abc&123'") {
				t.Fatal(string(raw))
			}
			if human && string(raw) != "warning: results truncated; pass --all, or --page-cursor 'abc&123'\n" {
				t.Fatal(string(raw))
			}
			if !human {
				var event map[string]string
				if json.Unmarshal(raw, &event) != nil || event["next_cursor"] != "abc&123" || event["next_cursor_flag"] != "--page-cursor" {
					t.Fatal(string(raw))
				}
			}
		})
	}
}

func TestLiveListTagStrategy(t *testing.T) {
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("tag") != "sample" {
			t.Errorf("tag missing: %s", r.URL)
		}
		fmt.Fprint(w, `{"nextPageCursor":"abc","results":[]}`)
	})
	out, stderr, code := sliceBExecute(t, cfg, "documents", "list", "--tag", "sample", "--json")
	meta, _ := cursorEnvelope(t, out)
	if code != 0 || meta["next_cursor"] != "abc" || meta["next_cursor_flag"] != "--page-cursor" || meta["has_more"] != true {
		t.Fatal(out, stderr)
	}
}

func TestLiveListCursorFlagsExist(t *testing.T) {
	for _, resource := range []string{"documents", "reader-tags", "books", "highlights"} {
		t.Run(resource, func(t *testing.T) {
			cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"nextPageCursor":"abc","next":"https://example.test/?page=7","results":[]}`)
			})
			args := []string{resource, "list"}
			// Reader tags is promoted to a leaf command in this generated CLI.
			if resource == "reader-tags" {
				args = []string{resource}
			}
			out, stderr, code := sliceBExecute(t, cfg, append(args, "--json")...)
			meta, _ := cursorEnvelope(t, out)
			flag, ok := meta["next_cursor_flag"].(string)
			var flags rootFlags
			cmd, _, err := newRootCmd(&flags).Find(args)
			if code != 0 || err != nil || !ok || cmd.Flags().Lookup(strings.TrimPrefix(flag, "--")) == nil {
				t.Fatal(out, stderr, err)
			}
		})
	}
}

func TestLiveListTerminalAndUnpaginatedMetadata(t *testing.T) {
	for _, body := range []string{`{"nextPageCursor":null}`, `{}`} {
		meta := liveListPageMetadata(json.RawMessage(body), "nextPageCursor", "", "cursor", "pageCursor", nil)
		if meta["has_more"] != false {
			t.Fatal(meta)
		}
	}
	meta := liveListPageMetadata(json.RawMessage(`{}`), "", "", "", "", nil)
	if _, ok := meta["has_more"]; ok {
		t.Fatal(meta)
	}
}

type cursorFixtureClient struct{}

func (cursorFixtureClient) GetWithHeaders(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error) {
	return json.RawMessage(`{"nextPageCursor":"abc","results":[]}`), nil
}

func TestLiveListMissingCursorFlag(t *testing.T) {
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)
	paging := map[string]any{}
	ctx := context.WithValue(t.Context(), liveListCommandKey{}, cmd)
	ctx = context.WithValue(ctx, liveListPagingKey{}, paging)
	_, err := paginatedGet(ctx, cursorFixtureClient{}, "/", nil, nil, false, "pageCursor", "cursor", "", 0, "nextPageCursor", "")
	if err != nil || paging["has_more"] != true || paging["next_cursor_flag"] != nil {
		t.Fatal(paging, err)
	}
	var event map[string]string
	if json.Unmarshal(stderr.Bytes(), &event) != nil || event["hint"] != "pass --all" || event["next_cursor"] != "" || event["next_cursor_flag"] != "" {
		t.Fatal(stderr.String())
	}
}

// captureLiveListAll also captures pagination progress written to process stderr.
func captureLiveListAll(t *testing.T, cfg string, args ...string) (string, string, int) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()
	out, stderr, code := sliceBExecute(t, cfg, args...)
	w.Close()
	os.Stderr = old
	progress, _ := io.ReadAll(r)
	r.Close()
	return out, stderr + string(progress), code
}

func TestLiveListV2AllPages(t *testing.T) {
	for _, tc := range []struct{ resource, start string }{
		{"books", ""}, {"highlights", ""}, {"highlights", "2"},
	} {
		t.Run(tc.resource+tc.start, func(t *testing.T) {
			calls := 0
			first := 1
			if tc.start != "" {
				first = 2
			}
			cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				want := fmt.Sprint(first + calls - 1)
				if calls == 1 && tc.start == "" {
					want = ""
				}
				if calls > 3 || r.URL.Path != "/"+tc.resource+"/" || r.URL.Query().Get("page") != want || r.URL.Query().Get("page_size") != "2" {
					t.Errorf("request %d: %s", calls, r.URL)
				}
				if calls < 3 {
					fmt.Fprintf(w, `{"count":5,"next":"https://example.test/?page=%d","results":[{"id":%d},{"id":%d}]}`, first+calls, calls*2-1, calls*2)
				} else {
					fmt.Fprint(w, `{"count":5,"next":null,"results":[{"id":5}]}`)
				}
			})
			args := []string{tc.resource, "list", "--page-size", "2", "--all", "--data-source", "live", "--json"}
			if tc.start != "" {
				args = append(args, "--page", tc.start)
			}
			out, stderr, code := captureLiveListAll(t, cfg, args...)
			_, items := cursorEnvelope(t, out)
			if code != 0 || calls != 3 || len(items) != 5 || !strings.Contains(stderr, `"pages":3`) {
				t.Fatal(out, stderr, calls)
			}
			for i, item := range items {
				if item["id"] != float64(i+1) {
					t.Fatal(items)
				}
			}
		})
	}
}

func TestLiveListV2RepeatedNext(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"count":9,"next":"https://x/?page=2","results":[{"id":1}]}`)
	})
	out, stderr, code := captureLiveListAll(t, cfg, "highlights", "list", "--page-size", "1", "--all", "--json")
	_, items := cursorEnvelope(t, out)
	if code != 0 || calls != 2 || len(items) != 2 || !strings.Contains(stderr, "pagination_cursor_repeated") {
		t.Fatal(out, stderr, calls)
	}
}

func TestLiveListV2MissingNext(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			fmt.Fprint(w, `{"count":9,"results":[{"id":1}]}`)
		} else {
			if r.URL.Query().Get("page") != "2" {
				t.Errorf("query: %s", r.URL)
			}
			fmt.Fprint(w, `{"count":9,"results":[]}`)
		}
	})
	out, stderr, code := captureLiveListAll(t, cfg, "highlights", "list", "--page-size", "1", "--all", "--json")
	_, items := cursorEnvelope(t, out)
	if code != 0 || calls != 2 || len(items) != 1 || strings.Contains(stderr, "truncated") || !strings.Contains(stderr, `"pages":2`) {
		t.Fatal(out, stderr, calls)
	}
}

func TestLiveListQuotedCursorHint(t *testing.T) {
	var capture bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&capture)
	ctx := context.WithValue(t.Context(), liveListCommandKey{}, cmd)
	old := humanFriendly
	humanFriendly = true
	defer func() { humanFriendly = old }()
	emitTruncationWarning(ctx, map[string]any{"has_more": true, "next_cursor": "a'b", "next_cursor_flag": "--page-cursor"}, true)
	if capture.String() != "warning: results truncated; pass --all, or --page-cursor 'a'\"'\"'b'\n" {
		t.Fatal(capture.String())
	}
}

func TestLiveListUnadvanceableHint(t *testing.T) {
	for _, tc := range []struct{ body, next, more string }{
		{`{"has_more":true,"results":[]}`, "", "has_more"},
		{`{"nextPageCursor":"abc","results":[]}`, "nextPageCursor", ""},
	} {
		t.Run(tc.next+tc.more, func(t *testing.T) {
			var capture bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetErr(&capture)
			ctx := context.WithValue(t.Context(), liveListCommandKey{}, cmd)
			old := humanFriendly
			defer func() { humanFriendly = old }()
			humanFriendly = false
			_, err := paginatedGet(ctx, cursorBodyClient(tc.body), "/", nil, nil, false, "", "cursor", "", 0, tc.next, tc.more)
			if err != nil {
				t.Fatal(err)
			}
			if capture.String() != "{\"event\":\"truncated\"}\n" {
				t.Fatal(capture.String())
			}
			capture.Reset()
			humanFriendly = true
			_, err = paginatedGet(ctx, cursorBodyClient(tc.body), "/", nil, nil, false, "", "cursor", "", 0, tc.next, tc.more)
			if err != nil {
				t.Fatal(err)
			}
			if capture.String() != "warning: results truncated; more pages available.\n" {
				t.Fatal(capture.String())
			}
		})
	}
}

func TestLiveListPageTerminalSignals(t *testing.T) {
	for _, kind := range []string{"page", "offset"} {
		for _, body := range []string{`{}`, `{"next":"https://x/?unrelated=2"}`, `{"next":null}`, `{"links":{"next":null}}`, `{"paging":{"next":null}}`} {
			meta := liveListPageMetadata(json.RawMessage(body), "", "", kind, "page", nil)
			if body == `{}` || strings.Contains(body, "unrelated") {
				if _, ok := meta["has_more"]; ok {
					t.Fatal(kind, body, meta)
				}
			} else if meta["has_more"] != false {
				t.Fatal(kind, body, meta)
			}
		}
	}
}

func TestLiveListLocalGoldenBytes(t *testing.T) {
	seedNovelCommandStore(t)
	out := runNovelCommand(t, "documents", "list", "--data-source", "local", "--id", "doc-1-child")
	const want = "{\n  \"meta\": {\n    \"reason\": \"user_requested\",\n    \"resource_type\": \"documents\",\n    \"source\": \"local\"\n  },\n  \"results\": [\n    {\n      \"id\": \"doc-1-child\",\n      \"title\": \"Child Highlight\",\n      \"parent_id\": \"doc-1\",\n      \"saved_at\": \"2026-06-11T00:00:00Z\"\n    }\n  ]\n}\n"
	if out != want {
		t.Fatalf("local bytes changed:\n%s", out)
	}
}

type cursorBodyClient string

func (c cursorBodyClient) GetWithHeaders(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error) {
	return json.RawMessage(c), nil
}

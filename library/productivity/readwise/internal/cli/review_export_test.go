package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestReviewExportPages(t *testing.T) {
	for _, resource := range []string{"books", "highlights", "documents"} {
		for _, limit := range []int{0, 3} {
			name := resource + "_unbounded"
			if limit > 0 {
				name = fmt.Sprintf("%s_limit%d", resource, limit)
			}
			t.Run(name, func(t *testing.T) {
				testenv.Isolate(t)
				calls := 0
				cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					if calls > 3 {
						http.Error(w, "past terminal page", 400)
						return
					}
					page := strconv.Itoa(calls)
					if resource == "documents" {
						want := ""
						if calls > 1 {
							want = "cursor" + page
						}
						if r.URL.Query().Get("pageCursor") != want {
							t.Errorf("cursor: %s", r.URL.RawQuery)
						}
						next := "null"
						if calls < 3 {
							next = fmt.Sprintf("%q", "cursor"+strconv.Itoa(calls+1))
						}
						fmt.Fprintf(w, `{"nextPageCursor":%s,"results":[{"id":%s}]}`, next, page)
					} else {
						if r.URL.Query().Get("page_size") != "100" {
							t.Errorf("page_size: %s want 100", r.URL.RawQuery)
						}
						if r.URL.Query().Get("page") != page {
							t.Errorf("page: %s want %s", r.URL.RawQuery, page)
						}
						next := "null"
						if calls < 3 {
							next = fmt.Sprintf("%q", "http://example.test/?page="+strconv.Itoa(calls+1))
						}
						fmt.Fprintf(w, `{"next":%s,"results":[{"id":%s}]}`, next, page)
					}
				})
				// Reader uses an absolute endpoint; point only this test's resource at the fixture.
				old := resourceReadPaths[resource]
				resourceReadPaths[resource] = "/" + resource + "/"
				defer func() { resourceReadPaths[resource] = old }()
				out, stderr, code := sliceBExecute(t, cfg, "export", resource, "--format", "jsonl", "--limit", strconv.Itoa(limit), "--no-learn")
				if code != 0 || calls != 3 || strings.TrimSpace(out) != "{\"id\":1}\n{\"id\":2}\n{\"id\":3}" {
					t.Fatalf("calls=%d code=%d out=%s err=%s", calls, code, out, stderr)
				}
			})
		}
	}
}

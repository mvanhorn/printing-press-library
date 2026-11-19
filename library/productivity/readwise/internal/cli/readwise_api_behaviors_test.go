package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
)

func sliceBSetup(t *testing.T, handler http.HandlerFunc) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("READWISE_API_TOKEN", "")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("READWISE_BASE_URL", srv.URL)
	cfg := filepath.Join(home, "fixture.toml")
	if err := os.WriteFile(cfg, []byte("api_token = \"slice-b-fixture-token\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return cfg, defaultDBPath("readwise-pp-cli")
}

func sliceBExecute(t *testing.T, cfg string, args ...string) (string, string, int) {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config", cfg, "--no-cache"}, args...))
	err := cmd.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
		stderr.WriteString(err.Error())
	}
	return out.String(), stderr.String(), code
}

func TestSliceBBulkUpdate(t *testing.T) {
	for _, status := range []int{200, 207, 400} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			cfg, dbPath := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "PATCH" || r.URL.Path != "/api/v3/bulk_update/" {
					t.Errorf("request %s %s", r.Method, r.URL)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body["updates"].([]any)) != 3 {
					t.Errorf("body %v: %v", body, err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status == 400 {
					fmt.Fprint(w, `{"detail":"bad payload"}`)
					return
				}
				fmt.Fprintf(w, `{"results":[{"id":"a","success":true},{"id":"b","success":%t,"error":"cannot update"},{"id":"c","success":true}]}`, status == 200)
			})
			for _, mode := range [][]string{{"--json"}, {"--json", "--select", "id,success,error"}, {"--agent"}} {
				args := append([]string{"documents", "bulk-update", "--updates", `[{"id":"a","title":"new"},{"id":"b","title":"new"},{"id":"c","title":"new"}]`, "--yes"}, mode...)
				out, stderr, code := sliceBExecute(t, cfg, args...)
				want := 0
				if status == 400 {
					want = 5
				}
				if status == 207 {
					want = 6
				}
				if code != want {
					t.Fatalf("exit %d want %d: %s %s", code, want, out, stderr)
				}
				if status != 400 && (!strings.Contains(out, `"a"`) || !strings.Contains(out, `"success"`)) {
					t.Fatalf("lost results: %s", out)
				}
				if status == 207 && (!strings.Contains(stderr, "1 of 3 updates failed") || !strings.Contains(out, "cannot update")) {
					t.Fatalf("partial result: %s %s", out, stderr)
				}
			}
			if calls != 3 {
				t.Fatalf("calls=%d", calls)
			}
			if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
				t.Fatalf("unexpected store: %v", err)
			}
		})
	}
}

func TestSliceBReview(t *testing.T) {
	for _, detail := range []string{"Review not found.", "Other missing resource."} {
		t.Run(detail, func(t *testing.T) {
			cfg, dbPath := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/review/" {
					t.Errorf("path=%s", r.URL.Path)
				}
				w.WriteHeader(404)
				json.NewEncoder(w).Encode(map[string]string{"detail": detail})
			})
			for _, mode := range [][]string{{"--json"}, {"--agent"}, {"--json", "--select", "review_completed,highlights,note"}} {
				out, stderr, code := sliceBExecute(t, cfg, append([]string{"review", "get", "--data-source", "live"}, mode...)...)
				if detail == "Review not found." {
					if code != 0 || !strings.Contains(out, "no daily review available today") || !strings.Contains(out, `"review_completed": false`) || !strings.Contains(out, `"highlights": []`) {
						t.Fatalf("review: exit=%d %s %s", code, out, stderr)
					}
				} else if code != 3 {
					t.Fatalf("real 404: exit=%d %s %s", code, out, stderr)
				}
			}
			if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
				t.Fatalf("unexpected store: %v", err)
			}
		})
	}
}

func TestSliceBDocumentTags(t *testing.T) {
	for _, tags := range [][]string{nil, {"one"}, {"one", "two", "three"}, {"1", "2", "3", "4", "5"}, {"1", "2", "3", "4", "5", "6"}, {""}} {
		t.Run(fmt.Sprintf("%q", tags), func(t *testing.T) {
			calls := 0
			cfg, dbPath := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if !reflect.DeepEqual(r.URL.Query()["tag"], tags) {
					t.Errorf("tags=%q want %q; %s", r.URL.Query()["tag"], tags, r.URL)
				}
				fmt.Fprint(w, `{"count":1,"results":[{"id":"doc","title":"kept"}],"nextPageCursor":null}`)
			})
			args := []string{"documents", "list", "--json", "--data-source", "live"}
			for _, tag := range tags {
				args = append(args, "--tag", tag)
			}
			out, stderr, code := sliceBExecute(t, cfg, args...)
			if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
				t.Fatalf("list created store: %v", err)
			}
			if len(tags) > 5 {
				if code != 2 || !strings.Contains(stderr, "--tag") || calls != 0 {
					t.Fatalf("usage: %d %s calls=%d", code, stderr, calls)
				}
				return
			}
			if code != 0 || !strings.Contains(out, "kept") || calls != 1 {
				t.Fatalf("list: %d %s %s calls=%d", code, out, stderr, calls)
			}
			if len(tags) > 0 {
				args = append(args, "--data-source", "local")
				_, stderr, code = sliceBExecute(t, cfg, args...)
				if code != 2 || !strings.Contains(stderr, "--tag filtering requires live data") || calls != 1 {
					t.Fatalf("local must fail closed: %d %s calls=%d", code, stderr, calls)
				}
			}
		})
	}
}

func TestSliceBTagResponseShapes(t *testing.T) {
	for _, resource := range []string{"book-tags", "highlight-tags"} {
		t.Run(resource, func(t *testing.T) {
			body := `[{"id":7,"name":"fixture-tag"}]`
			if resource == "highlight-tags" {
				body = `{"count":1,"next":null,"previous":null,"results":` + body + `}`
			}
			cfg, dbPath := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
			out, stderr, code := sliceBExecute(t, cfg, resource, "list", "42", "--json")
			if code != 0 || !strings.Contains(out, "fixture-tag") {
				t.Fatalf("tags: %d %s %s", code, out, stderr)
			}
			cfgData, err := config.Load(cfg)
			if err != nil {
				t.Fatal(err)
			}
			c := client.New(cfgData, time.Second, 0)
			c.NoCache = true
			wireData, err := c.Get(context.Background(), "/fixture-tags/", nil)
			if err != nil {
				t.Fatal(err)
			}
			items, _, _ := extractPageItems(wireData, "page", responsePathForResource(resource, "")...)
			if len(items) != 1 {
				t.Fatalf("sync decoded %d items", len(items))
			}
			db, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			n, _, _, err := upsertResourceBatch(db, resource, items)
			if err != nil || n != 1 {
				t.Fatalf("sync: %d %v", n, err)
			}
			count, err := db.Count(resource)
			if err != nil || count != 1 {
				t.Fatalf("stored: %d %v", count, err)
			}
		})
	}
}

func TestSliceBExportDeletion(t *testing.T) {
	for _, incremental := range []bool{true, false} {
		t.Run(fmt.Sprint(incremental), func(t *testing.T) {
			cfgPath, dbPath := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/books/" {
					fmt.Fprint(w, `{"count":0,"next":null,"results":[]}`)
					return
				}
				if r.URL.Path != "/export/" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				q := r.URL.Query()
				if incremental {
					if q.Get("includeDeleted") != "true" || q.Get("updatedAfter") == "" {
						t.Errorf("incremental query: %s", r.URL)
					}
				} else if q.Get("includeDeleted") != "true" {
					t.Errorf("full query: %s", r.URL)
				}
				if q.Get("includeDeleted") != "true" {
					fmt.Fprint(w, `{"results":[]}`)
					return
				}
				fmt.Fprint(w, `{"results":[{"user_book_id":42,"title":"fixture","highlights":[{"id":1,"text":"living passage"},{"id":2,"text":"vanishing passage","is_deleted":true}]}]}`)
			})
			db, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err = db.UpsertHighlights(json.RawMessage(`{"id":2,"text":"vanishing passage"}`)); err != nil {
				t.Fatal(err)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			args := []string{"sync", "--resources", "books", "--db", dbPath, "--json", "--strict"}
			if incremental {
				args = append(args, "--since", "7d")
			} else {
				args = append(args, "--full")
			}
			out, stderr, code := sliceBExecute(t, cfgPath, args...)
			if code != 0 || !strings.Contains(out, `"deleted":1`) || !strings.Contains(out, `"resource":"highlights","total":1`) {
				t.Fatalf("sync exit=%d output=%s stderr=%s", code, out, stderr)
			}
			db, err = store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			count, err := db.Count("highlights")
			if err != nil || count != 1 {
				t.Fatalf("count=%d err=%v", count, err)
			}
			for _, table := range []string{"highlights_fts", "resources_fts"} {
				var n int
				if err = db.DB().QueryRow(`SELECT count(*) FROM ` + table + ` WHERE ` + table + ` MATCH 'vanishing'`).Scan(&n); err != nil || n != 0 {
					t.Fatalf("%s stale FTS=%d err=%v", table, n, err)
				}
			}
			var text string
			if err = db.DB().QueryRow(`SELECT text FROM highlights WHERE id='1'`).Scan(&text); err != nil || text != "living passage" {
				t.Fatalf("live text %q %v", text, err)
			}
		})
	}
}

func TestE3BulkAllowPartial(t *testing.T) {
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(207)
		fmt.Fprint(w, `{"results":[{"id":"a","success":true},{"id":"b","success":false},{"id":"c","success":true}]}`)
	})
	for _, allow := range []bool{false, true} {
		args := []string{"documents", "bulk-update", "--updates", `[{"id":"a"},{"id":"b"},{"id":"c"}]`, "--yes", "--json"}
		if allow {
			args = append(args, "--allow-partial-failure")
		}
		out, stderr, code := sliceBExecute(t, cfg, args...)
		want := 6
		if allow {
			want = 0
		}
		if code != want || !strings.Contains(stderr, "1 of 3 updates failed") || !strings.Contains(out, `"c"`) {
			t.Fatalf("code=%d want=%d out=%s err=%s", code, want, out, stderr)
		}
	}
}

func TestE3BareReview(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/review/" {
			t.Errorf("path: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"review_id":42,"highlights":[]}`)
	})
	out, stderr, code := sliceBExecute(t, cfg, "review", "--json", "--data-source", "live")
	if code != 0 || calls != 1 || !strings.Contains(out, `42`) || !strings.Contains(stderr, "deprecated: bare review; use review get") {
		t.Fatalf("%d %d %s %s", code, calls, out, stderr)
	}
}

func TestE3SaveInputGuard(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `{"id":"saved"}`) })
	out, stderr, code := sliceBExecute(t, cfg, "documents", "save", "--yes")
	if code != 0 || calls != 0 || !strings.Contains(out, "Usage:") {
		t.Fatalf("bare: code=%d calls=%d %s %s", code, calls, out, stderr)
	}
	_, stderr, code = sliceBExecute(t, cfg, "documents", "save", "--title", "missing url", "--yes", "--json")
	if code != 2 || calls != 0 || !strings.Contains(stderr, "--url is required") {
		t.Fatalf("guard: code=%d calls=%d %s", code, calls, stderr)
	}
	_, stderr, code = sliceBExecute(t, cfg, "documents", "save", "--parent-id", "parent", "--content", "highlight", "--yes", "--json")
	if code != 0 || calls != 1 {
		t.Fatalf("parent: code=%d calls=%d %s", code, calls, stderr)
	}
}

func TestE3LocalTagUsage(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	_, stderr, code := sliceBExecute(t, cfg, "documents", "list", "--tag", "X", "--data-source", "local", "--json")
	if code != 2 || calls != 0 || !strings.Contains(stderr, "--tag filtering requires live data") {
		t.Fatalf("code=%d calls=%d %s", code, calls, stderr)
	}
}

func TestE3FiveTags(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if len(r.URL.Query()["tag"]) != 5 {
			t.Errorf("tags=%v", r.URL.Query()["tag"])
		}
		fmt.Fprint(w, `{"results":[]}`)
	})
	_, stderr, code := sliceBExecute(t, cfg, "documents", "list", "--data-source", "live", "--json", "--tag", "a", "--tag", "b", "--tag", "c", "--tag", "d", "--tag", "e")
	if code != 0 || calls != 1 {
		t.Fatalf("code=%d calls=%d %s", code, calls, stderr)
	}
}

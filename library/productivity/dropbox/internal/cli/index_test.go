package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func runIndexTestCommand(t *testing.T, args ...string) indexResult {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("index: %v\nstderr: %s", err, stderr.String())
	}
	var result indexResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output %q: %v", out.String(), err)
	}
	return result
}

func TestIndexFullIncrementalAndReset(t *testing.T) {
	testenv.Isolate(t)
	phase := 0
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/users/get_current_account" {
			io.WriteString(w, `{"account_type":{".tag":"pro"},"root_info":{"root_namespace_id":"root-ns","home_namespace_id":"home-ns"}}`)
			return
		}
		if r.Header.Get("Dropbox-API-Path-Root") != `{".tag":"root","root":"root-ns"}` {
			t.Errorf("missing path root header on %s", r.URL.Path)
		}
		var body struct {
			Path      string `json:"path"`
			Cursor    string `json:"cursor"`
			Recursive bool   `json:"recursive"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("request body: %v", err)
			return
		}
		switch {
		case r.URL.Path == "/files/list_folder" && body.Path == "" && !body.Recursive:
			io.WriteString(w, `{"entries":[{".tag":"file","id":"id:a","name":"a.txt","path_lower":"/a.txt","path_display":"/a.txt"},{".tag":"folder","id":"id:p","name":"Photos","path_lower":"/photos","path_display":"/Photos"}],"cursor":"root1","has_more":false}`)
		case r.URL.Path == "/files/list_folder" && strings.EqualFold(body.Path, "/photos") && body.Recursive && phase == 0:
			io.WriteString(w, `{"entries":[{".tag":"folder","name":"2019","path_lower":"/photos/2019"},{".tag":"file","name":"one.jpg","path_lower":"/photos/one.jpg"}],"cursor":"p1","has_more":true}`)
		case r.URL.Path == "/files/list_folder" && strings.EqualFold(body.Path, "/photos") && body.Recursive && phase == 2:
			io.WriteString(w, `{"entries":[{".tag":"file","name":"rebuilt.jpg","path_lower":"/photos/rebuilt.jpg"}],"cursor":"p4","has_more":false}`)
		case r.URL.Path == "/files/list_folder/continue" && body.Cursor == "p1":
			io.WriteString(w, `{"entries":[{".tag":"file","name":"old.jpg","path_lower":"/photos/2019/old.jpg"}],"cursor":"p2","has_more":false}`)
		case r.URL.Path == "/files/list_folder/continue" && body.Cursor == "root1":
			io.WriteString(w, `{"entries":[],"cursor":"root2","has_more":false}`)
		case r.URL.Path == "/files/list_folder/continue" && body.Cursor == "root2":
			io.WriteString(w, `{"entries":[],"cursor":"root3","has_more":false}`)
		case r.URL.Path == "/files/list_folder/continue" && body.Cursor == "p2":
			io.WriteString(w, `{"entries":[{".tag":"deleted","path_lower":"/photos/2019"},{".tag":"file","name":"new.jpg","path_lower":"/photos/new.jpg"}],"cursor":"p3","has_more":false}`)
		case r.URL.Path == "/files/list_folder/continue" && body.Cursor == "p3":
			w.WriteHeader(http.StatusConflict)
			io.WriteString(w, `{"error_summary":"reset/..."}`)
		default:
			t.Errorf("unexpected request %s body %+v phase %d", r.URL.Path, body, phase)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := filepath.Join(t.TempDir(), "dropbox.db")
	args := []string{"index", "--db", dbPath, "--json"}
	first := runIndexTestCommand(t, args...)
	if len(first.Roots) != 2 || first.Roots[0].Mode != "full" || first.Roots[1].Pages != 2 || first.TotalUpserted != 5 || !first.Roots[1].Complete || first.AccountType != "pro" {
		t.Fatalf("full result = %+v", first)
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.DB().QueryRow(`SELECT count(*) FROM dbx_files`).Scan(&n); err != nil || n != 5 {
		t.Fatalf("full rows = %d, %v", n, err)
	}
	var parent string
	if err := db.DB().QueryRow(`SELECT parent_lower FROM dbx_files WHERE path_lower='/photos/2019/old.jpg'`).Scan(&parent); err != nil || parent != "/photos/2019" {
		t.Fatalf("parent = %q, %v", parent, err)
	}
	state, found, err := db.GetDropboxIndexState(context.Background(), "/photos")
	if err != nil || !found || !state.Complete || state.Cursor != "p2" {
		t.Fatalf("state = %+v, %t, %v", state, found, err)
	}
	phase = 1
	second := runIndexTestCommand(t, args...)
	if second.Roots[1].Mode != "incremental" || second.TotalDeleted != 2 || second.TotalUpserted != 1 {
		t.Fatalf("incremental result = %+v", second)
	}
	for _, path := range []string{"/photos/2019", "/photos/2019/old.jpg"} {
		if err := db.DB().QueryRow(`SELECT count(*) FROM dbx_files WHERE path_lower=?`, path).Scan(&n); err != nil || n != 0 {
			t.Fatalf("deleted %s still present: %d, %v", path, n, err)
		}
	}
	if err := db.DB().QueryRow(`SELECT count(*) FROM dbx_files WHERE path_lower='/photos/new.jpg'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("new file missing: %d, %v", n, err)
	}
	phase = 2
	third := runIndexTestCommand(t, args...)
	if third.Roots[1].Mode != "full" || !third.Roots[1].Complete {
		t.Fatalf("reset result = %+v", third)
	}
	state, _, err = db.GetDropboxIndexState(context.Background(), "/photos")
	if err != nil || state.Cursor != "p4" {
		t.Fatalf("reset cursor = %+v, %v", state, err)
	}
	if err := db.DB().QueryRow(`SELECT count(*) FROM dbx_files WHERE path_lower='/photos/rebuilt.jpg'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("recrawl missing: %d, %v", n, err)
	}
	if requests != 11 {
		t.Fatalf("requests = %d, want 11", requests)
	}
}

func TestIndexDryRunDoesNotUseNetwork(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; fmt.Fprintln(w, `{}`) }))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	cmd := RootCmd()
	cmd.SetArgs([]string{"index", "--root", "/Photos", "--dry-run", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), err)
	}
	if string(payload["dry_run"]) != "true" || requests != 0 {
		t.Fatalf("dry-run output %s, requests %d", out.String(), requests)
	}
}

func TestIndexDogfoodCurtailsTopLevelRoots(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	var crawled []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			_, _ = io.WriteString(w, `{"account_type":{".tag":"basic"}}`)
		case "/files/list_folder":
			var body struct {
				Path string `json:"path"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			if body.Path == "" {
				entries := make([]map[string]string, 0, 10)
				for i := 9; i >= 0; i-- {
					p := fmt.Sprintf("/root-%02d", i)
					entries = append(entries, map[string]string{".tag": "folder", "name": p[1:], "path_lower": p, "path_display": p})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"entries": entries, "cursor": "root-cursor", "has_more": false})
				return
			}
			crawled = append(crawled, body.Path)
			_ = json.NewEncoder(w).Encode(map[string]any{"entries": []any{}, "cursor": body.Path + "-cursor", "has_more": true})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	data, err := runRead(t, "index", "--db", filepath.Join(t.TempDir(), "index.db"), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Roots        []indexRootResult `json:"roots"`
		Curtailed    bool              `json:"curtailed"`
		SkippedRoots int               `json:"skipped_roots"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if !result.Curtailed || result.SkippedRoots != 7 || len(result.Roots) != 4 {
		t.Fatalf("dogfood result = %+v", result)
	}
	if got, want := strings.Join(crawled, ","), "/root-00,/root-01,/root-02"; got != want {
		t.Fatalf("crawled roots = %q, want %q", got, want)
	}
	for _, root := range result.Roots {
		if root.Pages != 1 {
			t.Fatalf("root %q pages = %d, want 1", root.Root, root.Pages)
		}
	}
}

func TestIndexIncrementalDropsFormerTopLevelRoot(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"basic"}}`)
		case "/files/list_folder":
			var body struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Path == "" {
				io.WriteString(w, `{"entries":[{".tag":"folder","name":"Gone","path_lower":"/gone","path_display":"/Gone"}],"cursor":"root1","has_more":false}`)
			} else {
				io.WriteString(w, `{"entries":[{".tag":"file","name":"inside.txt","path_lower":"/gone/inside.txt","path_display":"/Gone/inside.txt"}],"cursor":"gone1","has_more":false}`)
			}
		case "/files/list_folder/continue":
			var body struct {
				Cursor string `json:"cursor"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Cursor == "root1" {
				io.WriteString(w, `{"entries":[{".tag":"file","name":"Gone","path_lower":"/gone","path_display":"/Gone"}],"cursor":"root2","has_more":false}`)
			} else {
				t.Errorf("unexpected cursor %q", body.Cursor)
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := filepath.Join(t.TempDir(), "index.db")
	runIndexTestCommand(t, "index", "--db", dbPath, "--json")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, found, err := db.GetDropboxIndexState(context.Background(), "/gone"); err != nil || !found {
		t.Fatalf("initial state found=%t err=%v", found, err)
	}
	runIndexTestCommand(t, "index", "--db", dbPath, "--json")
	if _, found, err := db.GetDropboxIndexState(context.Background(), "/gone"); err != nil || found {
		t.Fatalf("stale state found=%t err=%v", found, err)
	}
	var count int
	if err := db.DB().QueryRow(`SELECT count(*) FROM dbx_files WHERE path_lower='/gone' OR path_lower LIKE '/gone/%'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("stale rows=%d err=%v", count, err)
	}
}

func TestIndexDefaultTimeoutDoesNotCapWholeCrawl(t *testing.T) {
	testenv.Isolate(t)
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/users/get_current_account" {
			io.WriteString(w, `{"account_type":{".tag":"basic"}}`)
			return
		}
		time.Sleep(60 * time.Millisecond)
		page++
		fmt.Fprintf(w, `{"entries":[],"cursor":"c%d","has_more":%t}`, page, page < 5)
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	// Inject the request timeout without marking --timeout as explicitly supplied.
	// Five fast pages exceed the whole-command duration of 200ms.
	flags := &rootFlags{asJSON: true, timeout: 200 * time.Millisecond}
	cmd := newIndexCmd(flags)
	cmd.SetArgs([]string{"--db", filepath.Join(t.TempDir(), "index.db")})
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("index: %v; stderr: %s", err, stderr.String())
	}
	var result indexResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if page != 5 || len(result.Roots) != 1 || result.Roots[0].Pages != 5 {
		t.Fatalf("pages=%d result=%+v", page, result)
	}
}

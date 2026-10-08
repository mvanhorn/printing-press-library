package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func auditTestDB(t *testing.T, oldComplete bool) string {
	t.Helper()
	p := seedIndex(t, fixtureRow("/Taxes/2019.pdf", "file", "", "", 2), fixtureRow("/Docs/a.docx", "file", "", "", 3))
	db, err := store.OpenWithContext(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SetDropboxMeta(context.Background(), "account_id", "audit-account"); err != nil {
		t.Fatal(err)
	}
	if err := db.SetDropboxIndexState(context.Background(), store.DropboxIndexState{Root: "/old", Complete: oldComplete, LastFullAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	return p
}

func auditTestServer(t *testing.T, fail bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_id":"audit-account","account_type":{".tag":"pro"},"root_info":{"root_namespace_id":"root","home_namespace_id":"home"}}`)
		case "/sharing/list_shared_links":
			var header map[string]string
			if err := json.Unmarshal([]byte(r.Header.Get("Dropbox-API-Path-Root")), &header); err != nil || header["root"] != "root" {
				t.Errorf("path root header %q", r.Header.Get("Dropbox-API-Path-Root"))
			}
			if fail {
				w.WriteHeader(500)
				io.WriteString(w, `{"error_summary":"server_error"}`)
				return
			}
			var body struct {
				Cursor string `json:"cursor"`
				Path   string `json:"path"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Path != "" {
				t.Errorf("unexpected path filter %q", body.Path)
			}
			if body.Cursor == "" {
				io.WriteString(w, `{"links":[{"url":"https://d/tax","path_lower":"/taxes/2019.pdf","name":"2019.pdf","link_permissions":{"resolved_visibility":{".tag":"public"}}},{"url":"https://d/doc","path_lower":"/docs/a.docx","name":"a.docx","expires":"2028-01-01T00:00:00Z","link_permissions":{"resolved_visibility":{".tag":"team_only"}}}],"has_more":true,"cursor":"next"}`)
			} else if body.Cursor == "next" {
				io.WriteString(w, `{"links":[{"url":"https://d/gone","path_lower":"/old/gone.pdf","name":"gone.pdf","link_permissions":{"resolved_visibility":{".tag":"public"}}}],"has_more":false}`)
			} else {
				t.Errorf("cursor %q", body.Cursor)
			}
		case "/sharing/list_folders":
			io.WriteString(w, `{"entries":[{"name":"Taxes","path_display":"/Taxes","shared_folder_id":"sf1","access_type":{".tag":"owner"}}]}`)
		case "/sharing/list_folder_members":
			io.WriteString(w, `{"users":[{"user":{"display_name":"Alice","email":"alice@example.test"},"access_type":{".tag":"editor"}}],"groups":[],"invitees":[]}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
}

func TestLinksAuditFlagsAndPlans(t *testing.T) {
	testenv.Isolate(t)
	server := auditTestServer(t, false)
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	db := auditTestDB(t, true)
	plan := filepath.Join(t.TempDir(), "dangling.json")
	data, err := runRead(t, "links", "audit", "--db", db, "--plan", plan, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got linksAuditResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "live" || got.Counts != (auditCounts{Total: 3, Public: 2, NoExpiry: 2, Dangling: 1}) || got.FoldersScanned != 1 || got.FoldersTotal != 1 {
		t.Fatalf("result: %+v", got)
	}
	want := map[string][]string{"https://d/tax": {"public", "no_expiry"}, "https://d/doc": {}, "https://d/gone": {"public", "no_expiry", "dangling"}}
	for _, link := range got.Links {
		if !reflect.DeepEqual(link.Flags, want[link.URL]) {
			t.Errorf("%s flags=%v", link.URL, link.Flags)
		}
	}
	if len(got.SharedFolders[0].Users) != 1 || got.SharedFolders[0].Users[0].Email != "alice@example.test" {
		t.Fatalf("folders=%+v", got.SharedFolders)
	}
	p, err := dropbox.ReadPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Ops) != 1 || p.Ops[0].URL != "https://d/gone" || p.Ops[0].Op != "revoke_link" || !p.Ops[0].ExpectDangling {
		t.Fatalf("plan=%+v", p)
	}
	plan = filepath.Join(t.TempDir(), "public.json")
	if _, err := runRead(t, "links", "audit", "--db", db, "--revoke", "public", "--plan", plan, "--json"); err != nil {
		t.Fatal(err)
	}
	p, err = dropbox.ReadPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Ops) != 2 {
		t.Fatalf("public plan=%+v", p)
	}
	for _, op := range p.Ops {
		if op.ExpectDangling {
			t.Fatalf("public selection incorrectly requires dangling: %+v", op)
		}
	}
}

func TestLinksAuditIncompleteRootAndFallback(t *testing.T) {
	testenv.Isolate(t)
	server := auditTestServer(t, false)
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	db := auditTestDB(t, false)
	data, err := runRead(t, "links", "audit", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got linksAuditResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Counts.Dangling != 0 || got.Counts.Unknown != 1 || !reflect.DeepEqual(got.Links[2].Flags, []string{"public", "no_expiry", "unknown"}) {
		t.Fatalf("incomplete=%+v", got)
	}
	server.Close()
	failing := auditTestServer(t, true)
	defer failing.Close()
	t.Setenv("DROPBOX_BASE_URL", failing.URL)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	data, err = runRead(t, "links", "audit", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "cache" || got.Counts.Total != 3 || got.Counts.Unknown != 1 {
		t.Fatalf("fallback=%+v", got)
	}
}

func TestLinksAuditStaleIndexDoesNotClaimDangling(t *testing.T) {
	testenv.Isolate(t)
	if newNovelLinksAuditCmd(&rootFlags{}).Annotations["mcp:local-write"] != "" {
		t.Fatal("links audit must use open-world MCP annotations")
	}
	dbPath := auditTestDB(t, true)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-25 * time.Hour).UTC().Format(time.RFC3339)
	if err := db.SetDropboxIndexState(context.Background(), store.DropboxIndexState{Root: "/old", Complete: true, LastFullAt: old}); err != nil {
		t.Fatal(err)
	}
	if err := replaceAuditLinks(context.Background(), db, []auditLink{{URL: "https://d/gone", Path: "/old/gone.pdf"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelLinksAuditCmd(&rootFlags{asJSON: true, dataSource: "local"})
	cmd.SetArgs([]string{"--db", dbPath})
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	err = cmd.Execute()
	data := out.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "run: dropbox-pp-cli index") {
		t.Fatalf("stale hint=%q", stderr.String())
	}
	var got linksAuditResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Counts.Dangling != 0 || got.Counts.StaleIndex != 1 || !reflect.DeepEqual(got.Links[0].Flags, []string{"no_expiry", "stale_index"}) {
		t.Fatalf("stale result=%+v", got)
	}
}

func TestLinksAuditRefusesDifferentAccount(t *testing.T) {
	testenv.Isolate(t)
	server := auditTestServer(t, false)
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := auditTestDB(t, true)
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetDropboxMeta(context.Background(), "account_id", "other-account"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	planPath := filepath.Join(t.TempDir(), "links.json")
	_, err = runRead(t, "links", "audit", "--db", dbPath, "--plan", planPath, "--json")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "index --rebind") {
		t.Fatalf("mismatch error=%v", err)
	}
	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Fatalf("plan written despite account mismatch: %v", err)
	}
}

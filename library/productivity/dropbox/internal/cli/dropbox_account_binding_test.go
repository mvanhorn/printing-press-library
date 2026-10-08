package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

func TestIndexBindsAccountAndApplyUndoRejectDifferentAccount(t *testing.T) {
	testenv.Isolate(t)
	accountID := "account-a"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/get_current_account":
			fmt.Fprintf(w, `{"account_id":%q,"account_type":{".tag":"pro"},"root_info":{"root_namespace_id":"root","home_namespace_id":"root"}}`, accountID)
		case "/files/list_folder":
			fmt.Fprint(w, `{"entries":[],"cursor":"cursor","has_more":false}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	dbPath := filepath.Join(t.TempDir(), "index.db")
	if _, err := runRead(t, "index", "--db", dbPath, "--root", "/Docs", "--json"); err != nil {
		t.Fatal(err)
	}
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	bound, ok, err := db.GetDropboxMeta(context.Background(), "account_id")
	if err != nil || !ok || bound != "account-a" {
		t.Fatalf("bound account=%q ok=%t err=%v", bound, ok, err)
	}
	if err := db.UpsertDropboxEntries(context.Background(), "/docs", []store.DropboxRow{fixtureRow("/Docs/a.txt", "file", "H", "", 1)}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateDropboxJournalBatch(context.Background(), store.DropboxJournalBatch{ID: "batch", CreatedAt: time.Now().UTC().Format(time.RFC3339), Status: "complete"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/Docs/a.txt"}}}); err != nil {
		t.Fatal(err)
	}
	accountID = "account-b"
	for _, args := range [][]string{
		{"apply", planPath, "--yes", "--db", dbPath, "--json"},
		{"undo", "batch", "--yes", "--db", dbPath, "--json"},
	} {
		if _, err := runRead(t, args...); err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "this index/journal belongs to another Dropbox account") {
			t.Fatalf("%v mismatch error=%v", args, err)
		}
	}
}

// Hand-authored coverage for the velocity "no current yearly counts" notice. Not generated.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/store"
)

func syncedStore(t *testing.T, results ...string) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "mixed.db")
	ctx := context.Background()
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.DB().SetMaxOpenConns(1)
	page := `{"meta":{"next_cursor":""},"results":[` + strings.Join(results, ",") + `]}`
	if _, err := lancet.Refresh(ctx, pageFetcher{json.RawMessage(page)}, st.DB(), []lancet.Journal{{Slug: "lancet", ISSN: "0140-6736", Display: "The Lancet"}}, 0, 0, 1, nil); err != nil {
		t.Fatal(err)
	}
	return dbPath
}

func syncedWork(id, title string, counts string) string {
	c := ""
	if counts != "" {
		c = `,"counts_by_year":` + counts
	}
	return `{"id":"https://openalex.org/W` + id + `","title":"` + title + `","publication_year":2015,"cited_by_count":10,"open_access":{"is_oa":false},"primary_topic":{"display_name":"Rate"}` + c + `}`
}

func runVelocity(t *testing.T, dbPath string) (string, string, error) {
	t.Helper()
	cmd := newNovelCurateCmd(&rootFlags{dataSource: "local", asJSON: true})
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs([]string{"--topic", "rate", "--sort", "velocity", "--db", dbPath})
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

// c) some synced, some not: exit 0 plus exactly one stderr notice with N and M.
func TestCurateVelocityMixedStorePrintsNotice(t *testing.T) {
	dbPath := syncedStore(t,
		syncedWork("1", "Synced work", "[]"),
		syncedWork("2", "Unsynced one", ""),
		syncedWork("3", "Unsynced two", ""),
	)
	out, errs, err := runVelocity(t, dbPath)
	if err != nil {
		t.Fatalf("err = %v, want exit 0", err)
	}
	if !strings.Contains(out, "Synced work") {
		t.Fatalf("stdout = %q, want the rows", out)
	}
	want := "2 of 3 matched works have no current yearly citation counts; run thelancet-pp-cli refresh to include them"
	if strings.Count(errs, want) != 1 {
		t.Fatalf("stderr = %q, want exactly one %q", errs, want)
	}
}

func TestCurateVelocityAllSyncedPrintsNoNotice(t *testing.T) {
	dbPath := syncedStore(t, syncedWork("1", "Synced work", "[]"))
	_, errs, err := runVelocity(t, dbPath)
	if err != nil || strings.Contains(errs, "no current yearly citation counts") {
		t.Fatalf("err = %v, stderr = %q; want exit 0 and no notice", err, errs)
	}
}

// c) no work synced at all: non-zero exit with the refresh hint.
func TestCurateVelocityAllUnsyncedFailsWithRefreshHint(t *testing.T) {
	dbPath := syncedStore(t, syncedWork("1", "Unsynced one", ""), syncedWork("2", "Unsynced two", ""))
	out, _, err := runVelocity(t, dbPath)
	if err == nil || !strings.Contains(err.Error(), "refresh") {
		t.Fatalf("err = %v, want a non-zero exit naming refresh", err)
	}
	if strings.Contains(out, "Unsynced") {
		t.Fatalf("rows printed despite no synced work: %q", out)
	}
}

// staleStore backdates the marker of the given work ids to a past year.
func staleStore(t *testing.T, dbPath string, workIDs ...string) {
	t.Helper()
	st, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, id := range workIDs {
		if _, err := st.DB().Exec(`UPDATE lancet_works SET counts_synced_at = '2000-01-01T00:00:00Z' WHERE work_id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCurateVelocityStaleWorkPrintsNotice(t *testing.T) {
	dbPath := syncedStore(t, syncedWork("1", "Fresh work", "[]"), syncedWork("2", "Stale work", "[]"))
	staleStore(t, dbPath, "W2")
	out, errs, err := runVelocity(t, dbPath)
	if err != nil || !strings.Contains(out, "Fresh work") {
		t.Fatalf("err = %v, stdout = %q; want exit 0 with rows", err, out)
	}
	want := "1 of 2 matched works have no current yearly citation counts; run thelancet-pp-cli refresh to include them"
	if strings.Count(errs, want) != 1 {
		t.Fatalf("stderr = %q, want exactly one %q", errs, want)
	}
}

func TestCurateVelocityAllStaleFailsWithRefreshHint(t *testing.T) {
	dbPath := syncedStore(t, syncedWork("1", "Stale one", "[]"), syncedWork("2", "Stale two", "[]"))
	staleStore(t, dbPath, "W1", "W2")
	out, _, err := runVelocity(t, dbPath)
	if err == nil || !strings.Contains(err.Error(), "refresh") {
		t.Fatalf("err = %v, want a non-zero exit naming refresh", err)
	}
	if strings.Contains(out, "Stale") {
		t.Fatalf("rows printed despite no current work: %q", out)
	}
}

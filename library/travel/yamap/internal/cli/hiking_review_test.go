package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/yamap/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/yamap/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/yamap/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/yamap/internal/store"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReviewSourceCoursePagingIsNotCompleteAtCap(t *testing.T) {
	testenv.Isolate(t)
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("page")
		if p == "" {
			p = "1"
		}
		pages = append(pages, p)
		next := -1
		offset := 5
		if p == "1" {
			next = 2
			offset = 0
		}
		rows := []map[string]any{}
		for i := 1; i <= 5; i++ {
			rows = append(rows, map[string]any{"id": offset + i, "name": "高尾山"})
		}
		json.NewEncoder(w).Encode(map[string]any{"model_courses": rows, "meta": map[string]any{"next_page": next}})
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	c.NoCache = true
	db, e := store.Open(t.TempDir() + "/sync.db")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var out bytes.Buffer
	r := syncResource(context.Background(), c, db, "source_course", "", true, 2, false, false, nil, &out)
	if r.Err != nil || r.Warn != nil || r.Partial || r.Count != 10 || len(pages) != 2 || pages[1] != "2" {
		t.Fatalf("result=%+v pages=%v output=%s", r, pages, out.String())
	}
	cursor, at, _, err := db.GetSyncState("source_course")
	if err != nil || cursor != "" || at.IsZero() {
		t.Fatalf("full terminal page did not complete: cursor=%q at=%s err=%v", cursor, at, err)
	}
	if err := db.SaveSyncStateAt("source_course", "", 0, time.Time{}); err != nil {
		t.Fatal(err)
	}
	pages = nil
	out.Reset()
	r = syncResource(context.Background(), c, db, "source_course", "", true, 1, false, false, nil, &out)
	if r.Err != nil || !r.Partial || r.Count != 10 || len(pages) != 1 {
		t.Fatalf("cap result=%+v pages=%v output=%s", r, pages, out.String())
	}
}

func TestReviewRootUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"maps", "get", "1", "--data-source", "nonsense"}, {"source-course", "search", "--per", "1000"}} {
		testenv.Isolate(t)
		cmd := RootCmd()
		cmd.SetArgs(args)
		var b bytes.Buffer
		cmd.SetOut(&b)
		cmd.SetErr(&b)
		e := cmd.Execute()
		if ExitCode(e) != 2 {
			t.Fatalf("%v returned %v code %d", args, e, ExitCode(e))
		}
	}
}

func TestReviewValidSourceFlags(t *testing.T) {
	for _, args := range [][]string{{"source-activity", "search", "--dry-run"}, {"source-course", "search", "--page", "2", "--per", "5", "--dry-run"}, {"source-mountain", "reports", "--id", "108", "--dry-run"}} {
		testenv.Isolate(t)
		cmd := RootCmd()
		cmd.SetArgs(args)
		var b bytes.Buffer
		cmd.SetOut(&b)
		cmd.SetErr(&b)
		if e := cmd.Execute(); e != nil {
			t.Fatalf("%v returned %v: %s", args, e, b.String())
		}
	}
}

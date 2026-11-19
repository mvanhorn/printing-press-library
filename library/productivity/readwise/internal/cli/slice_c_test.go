package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"net/http"
	"testing"
)

func TestSliceCDocumentHighlightSave(t *testing.T) {
	calls := 0
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v3/save/" {
			t.Errorf("request %s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["parent_id"] != "X" || body["content"] != "passage" {
			t.Errorf("body=%v", body)
		}
		if _, ok := body["url"]; ok {
			t.Errorf("url must be omitted: %v", body)
		}
		fmt.Fprint(w, `{"id":"highlight-fixture"}`)
	})
	out, stderr, code := sliceBExecute(t, cfg, "documents", "save", "--parent-id", "X", "--content", "passage", "--json")
	if code != 0 || calls != 1 {
		t.Fatalf("code=%d calls=%d: %s %s", code, calls, out, stderr)
	}
}

func TestSliceCSyncNoPrune(t *testing.T) {
	// This spec declares no reconcilable partitions. Supply a synthetic
	// single-tenant partition to exercise the template prune gate end to end.
	previous := flatReconcileModes
	flatReconcileModes = map[string]string{"documents": "flat_global"}
	t.Cleanup(func() { flatReconcileModes = previous })
	for _, noPrune := range []bool{false, true} {
		t.Run(fmt.Sprint(noPrune), func(t *testing.T) {
			calls := 0
			cfg, dbPath := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/v3/list/" {
					t.Errorf("unexpected path %s", r.URL)
				}
				fmt.Fprint(w, `{"count":1,"nextPageCursor":null,"results":[{"id":"live","title":"present"}]}`)
			})
			db, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Upsert("documents", "gone", []byte(`{"id":"gone","title":"stale"}`)); err != nil {
				t.Fatal(err)
			}
			if err := db.UpsertDocuments(json.RawMessage(`{"id":"live","title":"present"}`)); err != nil {
				t.Fatal(err)
			}
			db.Close()
			args := []string{"sync", "--resources", "documents", "--full", "--strict", "--json"}
			if noPrune {
				args = append(args, "--no-prune")
			}
			out, stderr, code := sliceBExecute(t, cfg, args...)
			if code != 0 || calls != 1 {
				t.Fatalf("code=%d calls=%d: %s %s", code, calls, out, stderr)
			}
			db, err = store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			gone, err := db.Get("documents", "gone")
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				t.Fatal(err)
			}
			if (len(gone) > 0) != noPrune {
				t.Fatalf("stale row=%s noPrune=%t", gone, noPrune)
			}
			live, err := db.Get("documents", "live")
			if err != nil || len(live) == 0 {
				t.Fatalf("live=%s err=%v", live, err)
			}
		})
	}
}

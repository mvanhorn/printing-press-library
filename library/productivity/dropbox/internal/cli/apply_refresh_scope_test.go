// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

// A plan built from `index --root /A` must not make apply crawl the rest of
// the account: only the tracked root is refreshed before writes.
func TestApplyRefreshStaysInsideRootOnlyIndex(t *testing.T) {
	testenv.Isolate(t)
	var listed []string
	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"account_type":{".tag":"pro"},"root_info":{"root_namespace_id":"root","home_namespace_id":"home"}}`)
		case "/files/list_folder":
			var request struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			listed = append(listed, request.Path)
			switch request.Path {
			case "/a":
				io.WriteString(w, `{"cursor":"a","has_more":false,"entries":[{".tag":"folder","id":"id-a","name":"A","path_lower":"/a","path_display":"/A"},{".tag":"file","id":"id-f","name":"f.txt","path_lower":"/a/f.txt","path_display":"/A/f.txt","rev":"rev:f.txt","size":1}]}`)
			default:
				io.WriteString(w, `{"cursor":"x","has_more":false,"entries":[{".tag":"folder","id":"id-b","name":"B","path_lower":"/b","path_display":"/B"}]}`)
			}
		case "/files/delete_batch":
			deletes++
			io.WriteString(w, `{".tag":"complete","entries":[{".tag":"success"}]}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)

	dbPath := seedIndex(t, fixtureRow("/A", "folder", "", "", 0), fixtureRow("/A/f.txt", "file", "", "", 1))
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteDropboxIndexState(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	db.Close()

	planPath := filepath.Join(t.TempDir(), "plan.json")
	if err := dropbox.WritePlan(planPath, dropbox.Plan{Version: 1, CreatedAt: time.Now().UTC(), Source: "test", Ops: []dropbox.Op{{Op: "delete", Path: "/A/f.txt", Rev: "rev:f.txt"}}}); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "apply", planPath, "--yes", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("apply: %v %s", err, data)
	}
	if deletes != 1 {
		t.Fatalf("delete batches = %d", deletes)
	}
	if len(listed) != 1 || listed[0] != "/a" {
		t.Fatalf("refresh listed %q, want only the tracked root /a", listed)
	}
}

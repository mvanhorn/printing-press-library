// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestScheduleAcrossTenantsFollowsPagination(t *testing.T) {
	var afters []string
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classes" {
			http.NotFound(w, r)
			return
		}
		after := r.URL.Query().Get("after")
		afters = append(afters, after)
		w.Header().Set("Content-Type", "application/json")
		switch after {
		case "":
			fmt.Fprintf(w, `{"results":[{"id":"class-1","start_datetime":"2026-05-15T10:00:00Z"}],"links":{"next":"%s/classes?after=cursor-2"}}`, server.URL)
		case "cursor-2":
			fmt.Fprint(w, `{"results":[{"id":"class-2","start_datetime":"2026-05-15T11:00:00Z"}],"links":{"next":null}}`)
		default:
			t.Fatalf("unexpected after cursor %q", after)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	tenantDir := filepath.Join(home, ".config", "marianatek-pp-cli", "tenants")
	if err := os.MkdirAll(tenantDir, 0o700); err != nil {
		t.Fatalf("mkdir tenant dir: %v", err)
	}
	tenantConfig := fmt.Sprintf("base_url = %q\nbase_path = \"\"\noauth_authorization = \"token\"\n", server.URL)
	if err := os.WriteFile(filepath.Join(tenantDir, "tenant-one.toml"), []byte(tenantConfig), 0o600); err != nil {
		t.Fatalf("write tenant config: %v", err)
	}

	rows, err := scheduleAcrossTenants(&cobra.Command{}, &rootFlags{noCache: true, timeout: time.Second}, scheduleFilters{})
	if err != nil {
		t.Fatalf("scheduleAcrossTenants returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2: %#v", len(rows), rows)
	}
	if got, want := fmt.Sprint(afters), "[ cursor-2]"; got != want {
		t.Fatalf("after cursors = %s, want %s", got, want)
	}
}

func TestSortByStartUsesAbsoluteTime(t *testing.T) {
	rows := []map[string]any{
		{"id": "later", "start_datetime": "2026-05-15T07:00:00-05:00"},
		{"id": "earlier", "start_datetime": "2026-05-15T09:00:00+05:30"},
	}

	sortByStart(rows)

	if got, want := rows[0]["id"], "earlier"; got != want {
		t.Fatalf("first row id = %v, want %s", got, want)
	}
}

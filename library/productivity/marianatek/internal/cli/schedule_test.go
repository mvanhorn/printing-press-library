// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/store"
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

func TestScheduleEarliestSortsLocalCacheByStartTime(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	earlierStart := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	laterStart := earlierStart.Add(2 * time.Hour)
	earlier := json.RawMessage(fmt.Sprintf(`{"id":"earlier","start_datetime":%q}`, earlierStart.Format(time.RFC3339)))
	later := json.RawMessage(fmt.Sprintf(`{"id":"later","start_datetime":%q}`, laterStart.Format(time.RFC3339)))
	if err := db.Upsert("classes", "earlier", earlier); err != nil {
		t.Fatalf("upsert earlier: %v", err)
	}
	if err := db.Upsert("classes", "later", later); err != nil {
		t.Fatalf("upsert later: %v", err)
	}
	if _, err := db.DB().Exec(`UPDATE resources SET updated_at = ? WHERE resource_type = ? AND id = ?`, time.Now().Add(-time.Hour), "classes", "earlier"); err != nil {
		t.Fatalf("set earlier updated_at: %v", err)
	}
	if _, err := db.DB().Exec(`UPDATE resources SET updated_at = ? WHERE resource_type = ? AND id = ?`, time.Now(), "classes", "later"); err != nil {
		t.Fatalf("set later updated_at: %v", err)
	}

	flags := &rootFlags{asJSON: true}
	cmd := newScheduleCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--db", dbPath, "--earliest"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute schedule: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("parse output %q: %v", out.String(), err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1: %#v", len(rows), rows)
	}
	if got, want := rows[0]["id"], "earlier"; got != want {
		t.Fatalf("row id = %v, want %s", got, want)
	}
}

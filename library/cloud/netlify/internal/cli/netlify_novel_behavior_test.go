// Copyright 2026 Charles Denzel Segovia and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the cross-site novel commands. Each seeds a temp SQLite
// mirror with fixtures and asserts the aggregation produces correct non-empty
// output — proving the logic independent of a live (possibly empty) account.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/netlify/internal/store"
)

func seedMirror(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	put := func(rt, id string, obj map[string]any) {
		raw, _ := json.Marshal(obj)
		if err := st.Upsert(rt, id, raw); err != nil {
			t.Fatalf("upsert %s/%s: %v", rt, id, err)
		}
	}

	put("sites", "s1", map[string]any{
		"id": "s1", "name": "alpha", "ssl_url": "https://alpha.example.com",
		"state": "current", "account_slug": "team",
		"published_deploy": map[string]any{"published_at": "2026-07-01T10:00:00Z"},
	})
	put("sites", "s2", map[string]any{
		"id": "s2", "name": "beta", "ssl_url": "https://beta.example.com",
		"state": "current", "account_slug": "team", "published_at": "2026-06-01T10:00:00Z",
	})
	put("forms", "f1", map[string]any{"id": "f1", "site_id": "s1", "name": "contact"})

	// env vars: KEY1 on both with prod (no drift); KEY2 on s1 only (drift);
	// KEY3 on s1 branch-only (drift + missing production).
	prod := []any{map[string]any{"context": "production", "value": "x"}}
	branch := []any{map[string]any{"context": "branch-deploy", "value": "y"}}
	put("accounts-env", "e1", map[string]any{"key": "KEY1", "site_id": "s1", "values": prod})
	put("accounts-env", "e2", map[string]any{"key": "KEY1", "site_id": "s2", "values": prod})
	put("accounts-env", "e3", map[string]any{"key": "KEY2", "site_id": "s1", "values": prod})
	put("accounts-env", "e4", map[string]any{"key": "KEY3", "site_id": "s1", "values": branch})

	put("dns-zones", "z1", map[string]any{"id": "z1", "name": "example.com"})
	put("dns-records", "r1", map[string]any{"id": "r1", "dns_zone_id": "z1", "hostname": "alpha.example.com", "type": "NETLIFY", "value": "alpha"})
	put("dns-records", "r2", map[string]any{"id": "r2", "dns_zone_id": "z1", "hostname": "ghost.example.com", "type": "NETLIFY", "value": "ghost"})

	soon := time.Now().Add(5 * 24 * time.Hour).Format(time.RFC3339)
	later := time.Now().Add(200 * 24 * time.Hour).Format(time.RFC3339)
	expired := time.Now().Add(-12 * time.Hour).Format(time.RFC3339)
	put("sni-certificates", "c1", map[string]any{"domain": "alpha.example.com", "expires_at": soon})
	put("sni-certificates", "c2", map[string]any{"domain": "beta.example.com", "expires_at": later})
	put("sni-certificates", "c3", map[string]any{"domain": "expired.example.com", "expires_at": expired})

	recent := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	old := time.Now().Add(-72 * time.Hour).Format(time.RFC3339)
	put("deploys", "d1", map[string]any{"id": "d1", "site_id": "s1", "state": "ready", "context": "production", "commit_ref": "aaa111", "created_at": recent})
	put("deploys", "d2", map[string]any{"id": "d2", "site_id": "s2", "state": "error", "context": "production", "commit_ref": "bbb222", "created_at": old})

	put("submissions", "sub1", map[string]any{"id": "sub1", "email": "lead@acme.com", "name": "Jane", "form_name": "contact", "site_name": "alpha", "created_at": "2026-07-01T00:00:00Z"})
	put("submissions", "sub2", map[string]any{"id": "sub2", "email": "x@other.com", "name": "Bob", "form_name": "contact", "site_name": "beta", "created_at": "2026-06-01T00:00:00Z"})

	return dbPath
}

func runCmd(t *testing.T, args ...string) []byte {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v error: %v", args, err)
	}
	return out.Bytes()
}

func TestOverviewAggregates(t *testing.T) {
	db := seedMirror(t)
	var rows []siteOverview
	if err := json.Unmarshal(runCmd(t, "overview", "--json", "--db", db), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 sites, got %d", len(rows))
	}
	if rows[0].Name != "alpha" { // most-recent deploy sorts first
		t.Fatalf("want alpha first, got %q", rows[0].Name)
	}
	if rows[0].FormCount != 1 {
		t.Fatalf("want alpha form_count 1, got %d", rows[0].FormCount)
	}
}

func TestTimestampAfterHandlesOffsets(t *testing.T) {
	olderWithOffset := "2026-07-08T09:00:00+05:30" // 03:30 UTC
	newerUTC := "2026-07-08T04:00:00Z"
	if timestampAfter(olderWithOffset, newerUTC) {
		t.Fatalf("expected %s to sort before %s chronologically", olderWithOffset, newerUTC)
	}
	if !timestampAfter(newerUTC, olderWithOffset) {
		t.Fatalf("expected %s to sort after %s chronologically", newerUTC, olderWithOffset)
	}
}

func TestMissingMirrorPreservesJSONShape(t *testing.T) {
	db := filepath.Join(t.TempDir(), "missing.db")
	tests := []struct {
		name string
		args []string
		want byte
	}{
		{name: "overview array", args: []string{"overview", "--json", "--db", db}, want: '['},
		{name: "env drift object", args: []string{"env-drift", "--json", "--db", db}, want: '{'},
		{name: "dns audit object", args: []string{"dns-audit", "--json", "--db", db}, want: '{'},
		{name: "since object", args: []string{"since", "24h", "--json", "--db", db}, want: '{'},
		{name: "submissions search object", args: []string{"submissions", "search", "acme", "--json", "--db", db}, want: '{'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := bytes.TrimSpace(runCmd(t, tt.args...))
			if len(out) == 0 || out[0] != tt.want {
				t.Fatalf("want JSON starting with %q, got %q", tt.want, out)
			}
		})
	}
}

func TestEnvDriftDetects(t *testing.T) {
	db := seedMirror(t)
	var view envDriftView
	if err := json.Unmarshal(runCmd(t, "env-drift", "--json", "--db", db), &view); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := map[string]envDriftRow{}
	for _, d := range view.Drift {
		got[d.Key] = d
	}
	if _, ok := got["KEY1"]; ok {
		t.Fatalf("KEY1 present on both with prod value; should not be drift")
	}
	k2, ok := got["KEY2"]
	if !ok || len(k2.MissingOn) != 1 || k2.MissingOn[0] != "s2" {
		t.Fatalf("KEY2 should be missing on s2, got %+v", k2)
	}
	if k3, ok := got["KEY3"]; !ok || !k3.MissingProd {
		t.Fatalf("KEY3 should flag missing production value, got %+v", k3)
	}
}

func TestDNSAuditFlags(t *testing.T) {
	db := seedMirror(t)
	var view dnsAuditView
	if err := json.Unmarshal(runCmd(t, "dns-audit", "--json", "--db", db, "--cert-days", "30"), &view); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(view.Dangling) != 1 || view.Dangling[0].Value != "ghost" {
		t.Fatalf("want 1 dangling record 'ghost', got %+v", view.Dangling)
	}
	if len(view.ExpiringCert) != 1 || view.ExpiringCert[0].Domain != "alpha.example.com" {
		t.Fatalf("want 1 expiring cert alpha, got %+v", view.ExpiringCert)
	}
}

func TestSinceWindow(t *testing.T) {
	db := seedMirror(t)
	var view sinceView
	if err := json.Unmarshal(runCmd(t, "since", "24h", "--json", "--db", db), &view); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if view.Count != 1 || view.Deploys[0].ID != "d1" {
		t.Fatalf("want only d1 within 24h, got %+v", view.Deploys)
	}
}

func TestSubmissionsSearchMatches(t *testing.T) {
	db := seedMirror(t)
	var view submissionSearchView
	if err := json.Unmarshal(runCmd(t, "submissions", "search", "acme", "--json", "--db", db), &view); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if view.Count != 1 || view.Submissions[0].Email != "lead@acme.com" {
		t.Fatalf("want 1 match lead@acme.com, got %+v", view.Submissions)
	}
	if view.Scanned != 2 {
		t.Fatalf("want scanned 2, got %d", view.Scanned)
	}
}

func TestSubmissionsSearchSortsOffsetsBeforeLimit(t *testing.T) {
	db := seedMirror(t)
	st, err := store.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()

	for id, createdAt := range map[string]string{
		"older-offset": "2026-07-08T09:00:00+05:30", // 03:30 UTC
		"newer-utc":    "2026-07-08T04:00:00Z",
	} {
		raw, _ := json.Marshal(map[string]any{
			"id": id, "summary": "offset-order", "created_at": createdAt,
		})
		if err := st.Upsert("submissions", id, raw); err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
	}

	var view submissionSearchView
	if err := json.Unmarshal(runCmd(t, "submissions", "search", "offset-order", "--json", "--limit", "1", "--db", db), &view); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if view.Count != 1 || view.Submissions[0].ID != "newer-utc" {
		t.Fatalf("want newest chronological match after limit, got %+v", view.Submissions)
	}
}

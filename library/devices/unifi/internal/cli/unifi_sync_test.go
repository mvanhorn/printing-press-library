// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

const fixtureSiteID = "11111111-2222-3333-4444-555555555555"

func TestSyncIteratesConsoleSitesAndKeysRowsByConsole(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	// Both consoles deliberately use one site UUID: this is the collision the
	// console key exists to prevent.
	for _, c := range []string{"CONSOLE-A", "CONSOLE-B"} {
		api.set(c, "/network/integration/v1/sites", pageOf(map[string]any{"id": fixtureSiteID, "name": "Default", "internalReference": "default"}))
		api.set(c, "/network/integration/v1/sites/"+fixtureSiteID+"/firewall/policies", pageOf(map[string]any{"id": "pol-1", "name": "policy on " + c, "index": 1}))
	}
	// Protect is flat and not site-scoped, so it must not be iterated per site.
	api.set("CONSOLE-A", "/protect/integration/v1/cameras", []map[string]any{{"id": "cam-1", "name": "Porch", "mac": "AA:BB:CC:00:00:01"}})
	// Site Manager's id family must not leak into the integration path.
	api.set("", "/v1/sites", map[string]any{"data": []map[string]any{{"siteId": "cloud-site-id", "hostId": "CONSOLE-A"}}})

	for _, c := range []string{"CONSOLE-A", "CONSOLE-B"} {
		run := runUniFiCLI(t, "sync", "--console", c, "--db", db, "--json")
		if run.err != nil {
			t.Fatalf("sync %s: %v\n%s\n%s", c, run.err, run.stdout, run.stderr)
		}
	}
	for _, req := range api.requested() {
		if strings.Contains(req, "cloud-site-id") {
			t.Errorf("Site Manager site id used against the integration API: %s", req)
		}
		if strings.Contains(req, "/connector/consoles/") && !strings.Contains(req, "/proxy/") {
			t.Errorf("connector request missing proxy segment: %s", req)
		}
	}
	if !contains(api.requested(), "GET /v1/connector/consoles/CONSOLE-A/proxy/network/integration/v1/sites/"+fixtureSiteID+"/firewall/policies") {
		t.Errorf("site-scoped resource was not iterated:\n%s", strings.Join(api.requested(), "\n"))
	}

	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	store.SetActiveConsole("")
	rows, err := sdb.DB().Query(`SELECT json_extract(data,'$._console_id'), json_extract(data,'$.name') FROM resources WHERE resource_type='network_integration_v1_sites_firewall_policies' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var c, n string
		if err := rows.Scan(&c, &n); err != nil {
			t.Fatal(err)
		}
		got = append(got, c+"="+n)
	}
	want := []string{"CONSOLE-A=policy on CONSOLE-A", "CONSOLE-B=policy on CONSOLE-B"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	var cams int
	if err := sdb.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type='protect-integration-v1-cameras'`).Scan(&cams); err != nil || cams != 1 {
		t.Fatalf("cameras = %d err %v", cams, err)
	}
}

func TestSyncPaginatesNetworkByOffset(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	api.set("C1", "/network/integration/v1/sites", pageOf(map[string]any{"id": fixtureSiteID, "name": "Default"}))
	// Serves 60 clients in pages of the limit the caller asked for, honoring offset.
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		if !strings.HasSuffix(r.URL.Path, "/clients") {
			return false
		}
		var offset, limit int
		_, _ = fmt.Sscan(r.URL.Query().Get("offset"), &offset)
		_, _ = fmt.Sscan(r.URL.Query().Get("limit"), &limit)
		if limit == 0 {
			limit = 25
		}
		items := []map[string]any{}
		for i := offset; i < 60 && i < offset+limit; i++ {
			items = append(items, map[string]any{"id": fmt.Sprintf("client-%d", i), "name": fmt.Sprintf("c%d", i)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"offset": offset, "limit": limit, "count": len(items), "totalCount": 60, "data": items})
		return true
	}
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatalf("sync: %v\n%s", run.err, run.stderr)
	}
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	var n int
	if err := sdb.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type='network_integration_v1_sites_clients'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 60 {
		t.Fatalf("stored %d clients of 60: pagination stopped early", n)
	}
}

func TestLocalReadsAreConsoleScoped(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	t.Setenv("UNIFI_NO_LEARN", "1")
	for _, c := range []string{"CONSOLE-A", "CONSOLE-B"} {
		api.set(c, "/protect/integration/v1/cameras", []map[string]any{{"id": "cam-" + c, "name": "cam on " + c}})
		if run := runUniFiCLI(t, "sync", "--console", c, "--db", db, "--resources", "protect-integration-v1-cameras", "--json"); run.err != nil {
			t.Fatalf("sync: %v %s", run.err, run.stderr)
		}
	}
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	store.SetActiveConsole("CONSOLE-B")
	defer store.SetActiveConsole("")
	items, _, _, err := scanLocalList(sdb, "protect-integration-v1-cameras", nil, localListQuery{limit: 10}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.Contains(string(items[0]), "cam-CONSOLE-B") {
		t.Fatalf("scoped read = %s", items)
	}
}

func TestConsolePageIsNotAnAuthFailure(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>UniFi OS</title></head><body>unifi-portal</body></html>`))
		return true
	}
	run := runUniFiCLI(t, "network", "list-sites", "--console", "C1", "--data-source", "live", "--json")
	if run.err == nil {
		t.Fatal("expected an error for an HTML console page")
	}
	if code := ExitCode(run.err); code != 5 {
		t.Fatalf("exit %d (%v), want 5: a login page is not proof of a bad key", code, run.err)
	}
}

func TestReadOnlyAndRoutingErrorsUseConfigExit(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	t.Setenv("UNIFI_READ_ONLY", "1")
	run := runUniFiCLI(t, "network", "delete-acl-rule", fixtureSiteID, "acl-1", "--console", "C1", "--yes")
	if run.err == nil || ExitCode(run.err) != 10 {
		t.Fatalf("exit %d err %v", ExitCode(run.err), run.err)
	}
	for _, r := range api.requested() {
		if strings.HasPrefix(r, "DELETE") {
			t.Fatalf("mutation reached the API: %s", r)
		}
	}
}

func TestDeleteRemovesMirrorRow(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	db := defaultDBPath("unifi-pp-cli")
	api.set("C1", "/network/integration/v1/sites", pageOf(map[string]any{"id": fixtureSiteID, "name": "Default"}))
	api.set("C1", "/network/integration/v1/sites/"+fixtureSiteID+"/acl-rules", pageOf(map[string]any{"id": "acl-1", "name": "doomed"}))
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--json"); run.err != nil {
		t.Fatalf("sync: %v %s", run.err, run.stderr)
	}
	db = defaultDBPath("unifi-pp-cli")
	count := func() int {
		sdb, err := store.OpenWithContext(context.Background(), db)
		if err != nil {
			t.Fatal(err)
		}
		defer sdb.Close()
		var n int
		if err := sdb.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type='network_integration_v1_sites_acl_rules'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if count() != 1 {
		t.Fatalf("mirror before delete at %s has %d rows", db, count())
	}
	run := runUniFiCLI(t, "network", "delete-acl-rule", fixtureSiteID, "acl-1", "--console", "C1", "--yes")
	if run.err != nil {
		t.Fatalf("delete: %v %s", run.err, run.stderr)
	}
	if count() != 0 {
		t.Fatal("mirror row survived a successful delete")
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestSearchSpansConsolesUnlessNarrowed(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	for _, c := range []string{"CONSOLE-A", "CONSOLE-B"} {
		api.set(c, "/protect/integration/v1/cameras", []map[string]any{{"id": "cam-" + c, "name": "gazebo camera"}})
		if run := runUniFiCLI(t, "sync", "--console", c, "--db", db, "--resources", "protect-integration-v1-cameras", "--json"); run.err != nil {
			t.Fatalf("sync: %v", run.err)
		}
	}
	all := runUniFiCLI(t, "search", "gazebo", "--db", db, "--data-source", "local", "--json")
	if all.err != nil || !strings.Contains(all.stdout, "cam-CONSOLE-A") || !strings.Contains(all.stdout, "cam-CONSOLE-B") || !strings.Contains(all.stdout, "_console_id") {
		t.Fatalf("cross-console search: %v %s", all.err, all.stdout)
	}
	one := runUniFiCLI(t, "search", "gazebo", "--console", "CONSOLE-B", "--db", db, "--data-source", "local", "--json")
	if one.err != nil || strings.Contains(one.stdout, "cam-CONSOLE-A") || !strings.Contains(one.stdout, "cam-CONSOLE-B") {
		t.Fatalf("narrowed search: %v %s", one.err, one.stdout)
	}
	none := runUniFiCLI(t, "search", "nosuchwordanywhere", "--db", db, "--data-source", "local", "--json")
	if none.err != nil || strings.Contains(none.stdout, "cam-") {
		t.Fatalf("negative search: %v %s", none.err, none.stdout)
	}
}

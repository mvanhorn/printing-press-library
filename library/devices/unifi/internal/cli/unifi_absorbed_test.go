// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
)

func TestNormalizeRawPath(t *testing.T) {
	good := map[string]string{
		"network/api/s/default/stat/health":          "/proxy/network/api/s/default/stat/health",
		"/network/api/s/default/stat/health":         "/proxy/network/api/s/default/stat/health",
		"/proxy/network/integration/v1/sites":        "/proxy/network/integration/v1/sites",
		"proxy/protect/integration/v1/cameras":       "/proxy/protect/integration/v1/cameras",
		"network/v2/api/site/default/system-log/all": "/proxy/network/v2/api/site/default/system-log/all",
	}
	for in, want := range good {
		got, _, err := normalizeRawPath(in)
		if err != nil || got != want {
			t.Errorf("normalizeRawPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{
		"", "../../etc/passwd", "network/../../v1/hosts", "network/%2e%2e/%2e%2e/v1/hosts",
		"network/%252e%252e/x", "https://evil.example/network/x", "//evil.example/network",
		"network\\..\\x", "network/a#frag", "v1/hosts", "network", "NETWORK/x", "network//x",
		"network/./x", "network/%00/x",
	}
	for _, in := range bad {
		if got, _, err := normalizeRawPath(in); err == nil {
			t.Errorf("normalizeRawPath(%q) accepted as %q, should be refused", in, got)
		}
	}
	_, query, err := normalizeRawPath("network/api/s/default/stat/event?_limit=5")
	if err != nil || query.Get("_limit") != "5" {
		t.Errorf("query not preserved: %v %v", query, err)
	}
}

func TestRawGetRunsAndWriteNeedsConfirm(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	api.set("C1", "/network/api/s/default/stat/health", map[string]any{"data": []map[string]any{{"subsystem": "wan", "status": "ok"}}})

	run := runUniFiCLI(t, "raw", "network/api/s/default/stat/health", "--console", "C1", "--json")
	if run.err != nil || !strings.Contains(run.stdout, "wan") {
		t.Fatalf("GET: %v\n%s", run.err, run.stdout)
	}

	before := len(api.requested())
	run = runUniFiCLI(t, "raw", "POST", "network/api/s/default/cmd/stamgr", "--body", `{"cmd":"block-sta"}`, "--console", "C1", "--json")
	if run.err != nil {
		t.Fatalf("preview errored: %v", run.err)
	}
	var preview map[string]any
	if err := json.Unmarshal([]byte(run.stdout), &preview); err != nil {
		t.Fatalf("preview not JSON: %v\n%s", err, run.stdout)
	}
	if preview["sent"] != false || preview["confirmation_required"] != true {
		t.Fatalf("preview = %v", preview)
	}
	if len(api.requested()) != before {
		t.Fatal("POST without --confirm reached the API")
	}

	run = runUniFiCLI(t, "raw", "POST", "network/api/s/default/cmd/stamgr", "--body", `{"cmd":"block-sta"}`, "--console", "C1", "--dry-run", "--confirm", "--json")
	if run.err != nil || len(api.requested()) != before {
		t.Fatalf("--dry-run must win over --confirm: %v, %d requests", run.err, len(api.requested())-before)
	}

	run = runUniFiCLI(t, "raw", "POST", "network/api/s/default/cmd/stamgr", "--body", `{"cmd":"block-sta"}`, "--console", "C1", "--confirm", "--json")
	if run.err != nil {
		t.Fatalf("confirmed POST: %v", run.err)
	}
	if !contains(api.requested(), "POST /v1/connector/consoles/C1/proxy/network/api/s/default/cmd/stamgr") {
		t.Fatalf("confirmed POST did not route through the connector: %v", api.requested())
	}
}

func TestRawRefusesEscapesWithUsageExit(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	run := runUniFiCLI(t, "raw", "network/../../v1/hosts", "--console", "C1")
	if run.err == nil || ExitCode(run.err) != 2 {
		t.Fatalf("exit %d err %v", ExitCode(run.err), run.err)
	}
	if len(api.requested()) != 0 {
		t.Fatalf("escaping path reached the API: %v", api.requested())
	}
}

func TestConsoleAllowlistBlocksOtherConsoles(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	t.Setenv("UNIFI_CONSOLE_ALLOWLIST", "ALLOWED")
	run := runUniFiCLI(t, "raw", "network/integration/v1/info", "--console", "OTHER", "--json")
	if run.err == nil || ExitCode(run.err) != 10 {
		t.Fatalf("exit %d err %v", ExitCode(run.err), run.err)
	}
	if len(api.requested()) != 0 {
		t.Fatalf("blocked console was contacted: %v", api.requested())
	}
	if run := runUniFiCLI(t, "raw", "network/integration/v1/info", "--console", "ALLOWED", "--json"); run.err != nil {
		t.Fatalf("allowed console refused: %v", run.err)
	}
}

func fleetFixture(t *testing.T) *fakeConsoleAPI {
	t.Helper()
	api := newFakeConsoleAPI(t)
	api.set("", "/v1/hosts", map[string]any{"data": []map[string]any{
		{"id": "HOST-A", "isBlocked": false, "lastConnectionStateChange": "2026-10-30T10:00:00Z", "reportedState": map[string]any{"name": "Alpha", "state": "connected", "version": "5.1.0"}},
		{"id": "HOST-B", "isBlocked": false, "reportedState": map[string]any{"hostname": "beta", "state": "disconnected"}},
	}})
	api.set("", "/v1/devices", map[string]any{"data": []map[string]any{
		{"hostId": "HOST-A", "hostName": "Alpha", "devices": []map[string]any{
			{"id": "d1", "name": "gw", "model": "UDM", "status": "online", "version": "5.1.0", "updateAvailable": "5.2.0"},
			{"id": "d2", "name": "ap", "model": "U7", "status": "offline", "version": "8.0.0"},
		}},
		{"hostId": "HOST-B", "hostName": "beta", "devices": []map[string]any{{"id": "d3", "name": "gw2", "model": "UDR", "status": "online"}}},
	}})
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/consoles/HOST-B/") {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"code":"offline"}`))
			return true
		}
		return false
	}
	return api
}

func TestFleetReportsPartialFailureWithoutCountingItHealthy(t *testing.T) {
	api := fleetFixture(t)
	fixtureEnv(t, api.server.URL)
	t.Setenv("UNIFI_MAX_RETRIES", "0")
	run := runUniFiCLI(t, "fleet", "--json")
	if run.err != nil {
		t.Fatalf("partial failure must not fail the command: %v\n%s", run.err, run.stderr)
	}
	var view fleetView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, run.stdout)
	}
	if len(view.Consoles) != 2 {
		t.Fatalf("consoles = %+v", view.Consoles)
	}
	byID := map[string]fleetConsole{}
	for _, c := range view.Consoles {
		byID[c.ConsoleID] = c
	}
	if byID["HOST-A"].Connector != "ok" || byID["HOST-B"].Connector != "failed" {
		t.Fatalf("connector states: %+v", byID)
	}
	if byID["HOST-A"].DevicesOnline != 1 || byID["HOST-A"].DevicesOffline != 1 || byID["HOST-A"].UpdatesPending != 1 {
		t.Fatalf("HOST-A counts: %+v", byID["HOST-A"])
	}
	if len(view.FetchFailures) != 1 || view.FetchFailures[0].ConsoleID != "HOST-B" {
		t.Fatalf("fetch_failures = %+v", view.FetchFailures)
	}
	if view.Totals["connector_failed"] != 1 {
		t.Fatalf("totals = %+v", view.Totals)
	}
	if len(view.Firmware) != 1 || view.Firmware[0].UpdateAvailable != "5.2.0" {
		t.Fatalf("firmware = %+v", view.Firmware)
	}
}

func TestFleetUnknownDevicesDropTotalsInsteadOfReportingZero(t *testing.T) {
	view := buildFleetView("all", []fleetHost{{id: "H"}}, nil, false, nil, []fleetFailure{{Source: "/v1/devices", Error: "boom"}}, 0)
	if _, present := view.Totals["devices"]; present {
		t.Fatalf("unknown device list must not report a zero total: %v", view.Totals)
	}
}

func TestFleetFirmwareNoUpdatesIsEmptyNotFabricated(t *testing.T) {
	view := buildFleetView("firmware", []fleetHost{{id: "H"}}, []fleetDevice{{ConsoleID: "H", Name: "gw", Status: "online"}}, true, nil, nil, 0)
	if view.Firmware == nil || len(view.Firmware) != 0 {
		t.Fatalf("firmware = %#v, want empty non-nil", view.Firmware)
	}
}

func TestFleetBadReportIsUsageError(t *testing.T) {
	api := fleetFixture(t)
	fixtureEnv(t, api.server.URL)
	if run := runUniFiCLI(t, "fleet", "nonsense"); run.err == nil || ExitCode(run.err) != 2 {
		t.Fatalf("exit %d err %v", ExitCode(run.err), run.err)
	}
}

func TestAuditCommandReadsScrubbedLog(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	body := `{"name":"guest","passphrase":"super-secret-value"}`
	if run := runUniFiCLI(t, "raw", "POST", "network/integration/v1/sites/s/wifi/broadcasts", "--body", body, "--console", "C1", "--confirm", "--json"); run.err != nil {
		t.Fatalf("write: %v", run.err)
	}
	run := runUniFiCLI(t, "audit", "--json")
	if run.err != nil {
		t.Fatalf("audit: %v", run.err)
	}
	if strings.Contains(run.stdout, "super-secret-value") {
		t.Fatal("audit output leaked a secret")
	}
	var view struct {
		Matched int                  `json:"matched"`
		Records []client.AuditRecord `json:"records"`
	}
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, run.stdout)
	}
	if view.Matched != 1 || view.Records[0].Outcome != "applied" || view.Records[0].Console != "C1" {
		t.Fatalf("view = %+v", view)
	}
	// A filter that matches nothing returns an empty list rather than the log.
	run = runUniFiCLI(t, "audit", "--method", "DELETE", "--json")
	if run.err != nil {
		t.Fatal(run.err)
	}
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || view.Matched != 0 || len(view.Records) != 0 {
		t.Fatalf("negative filter: %+v %v", view, err)
	}
}

func TestFilterAuditRecordsSinceAndLimit(t *testing.T) {
	now := time.Now().UTC()
	records := []client.AuditRecord{
		{Time: now.Add(-48 * time.Hour).Format(time.RFC3339Nano), Method: "POST", Path: "/a"},
		{Time: now.Add(-1 * time.Hour).Format(time.RFC3339Nano), Method: "POST", Path: "/b"},
		{Time: now.Format(time.RFC3339Nano), Method: "POST", Path: "/c"},
	}
	got, total := filterAuditRecords(records, now.Add(-24*time.Hour), "", "", "", "", 1)
	if total != 2 || len(got) != 1 || got[0].Path != "/c" {
		t.Fatalf("got %+v total %d", got, total)
	}
}

func TestDoctorReportsResolvedTransport(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	run := runUniFiCLI(t, "doctor", "--console", "C1", "--json")
	if !strings.Contains(run.stdout, `"transport"`) || !strings.Contains(run.stdout, `"connector"`) || !strings.Contains(run.stdout, "C1") {
		t.Fatalf("doctor output missing transport: err=%v\n%s", run.err, run.stdout)
	}
}

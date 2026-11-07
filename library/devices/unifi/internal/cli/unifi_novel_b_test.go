// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

func hydratedDevice(id, name, uplink string) mirrorRow {
	row := deviceRow(id, name, uplink)
	row.Data[deviceHydratedField] = true
	return row
}

func impactFixture() impactData {
	return impactData{
		Transport: "connector",
		Devices:   []mirrorRow{hydratedDevice(fixGateway, "gw", ""), hydratedDevice(fixSwitch, "sw", fixGateway), hydratedDevice(fixAP, "ap", fixSwitch), hydratedDevice("leaf", "leaf", fixGateway)},
		Clients: []mirrorRow{
			{ID: "cl1", Parent: "s", Data: map[string]any{"name": "laptop", "type": "WIRED", "macAddress": "00:11:22:33:44:01", "uplinkDeviceId": fixAP}},
			{ID: "cam-client", Parent: "s", Data: map[string]any{"name": "porch-cam", "type": "WIRED", "macAddress": "00:11:22:33:44:02", "uplinkDeviceId": fixSwitch}},
			{ID: "vpn1", Parent: "s", Data: map[string]any{"name": "remote", "type": "VPN"}},
			{ID: "wl1", Parent: "s2", Data: map[string]any{"name": "phone", "type": "WIRELESS"}},
		},
		Cameras: []mirrorRow{{ID: "cam1", Data: map[string]any{"name": "Porch", "mac": "00:11:22:33:44:02"}}},
		WANs:    []mirrorRow{{ID: "wan1", Parent: "s", Data: map[string]any{"name": "Internet 1"}}, {ID: "wan2", Parent: "s2", Data: map[string]any{"name": "Internet A"}}, {ID: "wan3", Parent: "s2", Data: map[string]any{"name": "Internet B"}}},
		Wifi:    []mirrorRow{{ID: "wifi-busy", Parent: "s2", Data: map[string]any{"name": "Home"}}, {ID: "wifi-idle", Parent: "s", Data: map[string]any{"name": "Guest"}}},
		Zones:   []mirrorRow{{ID: "z-ext", Parent: "s", Data: map[string]any{"name": "External"}}, {ID: "z-lan", Parent: "s", Data: map[string]any{"name": "LAN"}}, {ID: "z-iot", Parent: "s", Data: map[string]any{"name": "IoT"}}},
		Policies: []mirrorRow{
			{ID: "pol-ext", Parent: "s", Data: map[string]any{"name": "to-ext", "source": map[string]any{"zoneId": "z-lan"}, "destination": map[string]any{"zoneId": "z-ext"}}},
			{ID: "pol-int", Parent: "s", Data: map[string]any{"name": "lan-iot", "source": map[string]any{"zoneId": "z-lan"}, "destination": map[string]any{"zoneId": "z-iot"}}},
			{ID: "pol-unknown", Parent: "s", Data: map[string]any{"name": "mystery", "source": map[string]any{"zoneId": "gone"}, "destination": map[string]any{"zoneId": "z-lan"}}},
		},
	}
}

func TestImpactVerdicts(t *testing.T) {
	cases := []struct {
		name                 string
		kind, action, target string
		port                 string
		mutate               func(*impactData)
		want                 string
		wantDevices          int
		wantClients          int
		wantCameras          int
	}{
		{name: "gateway alias blocks path", kind: "device", action: "restart", target: "gateway", want: verdictBlocksPath, wantDevices: 3, wantClients: 2, wantCameras: 1},
		{name: "switch with downstream is at risk", kind: "device", action: "restart", target: "sw", want: verdictAtRisk, wantDevices: 1, wantClients: 2, wantCameras: 1},
		{name: "leaf with nothing below", kind: "device", action: "restart", target: "leaf", want: verdictNoKnownRisk},
		{name: "unknown device", kind: "device", action: "restart", target: "nope", want: verdictCannotDetermine},
		{name: "unhydrated mirror cannot determine", kind: "device", action: "restart", target: "leaf", want: verdictCannotDetermine, mutate: func(d *impactData) {
			d.Devices = append(d.Devices, deviceRow("unknown-uplink", "x", ""))
		}},
		{name: "ambiguous name cannot determine", kind: "device", action: "restart", target: "dup", want: verdictCannotDetermine, mutate: func(d *impactData) {
			d.Devices = append(d.Devices, hydratedDevice("d1", "dup", fixGateway), hydratedDevice("d2", "dup", fixGateway))
		}},
		{name: "port power-cycle always cannot determine", kind: "port", action: "power-cycle", target: "sw", port: "4", want: verdictCannotDetermine},
		{name: "client over connector", kind: "client", action: "block", target: "laptop", want: verdictNoKnownRisk, wantClients: 1},
		{name: "client over local may be me", kind: "client", action: "block", target: "laptop", want: verdictAtRisk, wantClients: 1, mutate: func(d *impactData) { d.Transport = "local" }},
		{name: "VPN client may be my session", kind: "client", action: "block", target: "remote", want: verdictAtRisk, wantClients: 1},
		{name: "client that is a camera", kind: "client", action: "block", target: "porch-cam", want: verdictAtRisk, wantClients: 1, wantCameras: 1},
		{name: "client not in mirror", kind: "client", action: "block", target: "stranger", want: verdictCannotDetermine},
		{name: "wifi with clients on site", kind: "wifi", action: "disable", target: "Home", want: verdictAtRisk},
		{name: "wifi with none on site", kind: "wifi", action: "disable", target: "Guest", want: verdictNoKnownRisk},
		{name: "only WAN blocks path", kind: "wan", action: "disable", target: "Internet 1", want: verdictBlocksPath},
		{name: "one of two WANs at risk, failover untested", kind: "wan", action: "disable", target: "Internet A", want: verdictAtRisk},
		{name: "unknown WAN", kind: "wan", action: "disable", target: "nope", want: verdictCannotDetermine},
		{name: "policy touching External", kind: "firewall-policy", action: "enable", target: "to-ext", want: verdictAtRisk},
		{name: "policy between inside zones", kind: "firewall-policy", action: "enable", target: "lan-iot", want: verdictNoKnownRisk},
		{name: "policy with unsynced zone", kind: "firewall-policy", action: "enable", target: "mystery", want: verdictCannotDetermine},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := impactFixture()
			if tc.mutate != nil {
				tc.mutate(&d)
			}
			got := assessImpact(tc.kind, tc.action, tc.target, tc.port, d)
			if got.Verdict != tc.want {
				t.Fatalf("verdict %q (%v), want %q", got.Verdict, got.Reasons, tc.want)
			}
			if got.AffectedCounts["devices"] != tc.wantDevices || got.AffectedCounts["clients"] != tc.wantClients || got.AffectedCounts["cameras"] != tc.wantCameras {
				t.Fatalf("counts %v, want d=%d c=%d cam=%d", got.AffectedCounts, tc.wantDevices, tc.wantClients, tc.wantCameras)
			}
			if len(got.Reasons) == 0 || !strings.Contains(got.Limits, "cannot see every cause") {
				t.Fatalf("must explain itself and state limits: %+v", got)
			}
			encoded, _ := json.Marshal(got)
			if strings.Contains(strings.ToLower(string(encoded)), `"safe"`) || got.Verdict == "safe" {
				t.Fatal("no verdict may claim safe")
			}
		})
	}
}

func TestImpactEndToEnd(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	run := runUniFiCLI(t, "impact", "device", "restart", "gateway", "--console", "C1", "--db", db, "--json")
	if run.err != nil {
		t.Fatalf("%v %s", run.err, run.stderr)
	}
	var got impactResult
	if err := json.Unmarshal([]byte(run.stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.Verdict != verdictBlocksPath || got.ControlPath.Gateway != "gw" || got.ControlPath.Transport != "connector" || got.AffectedCounts["clients"] != 1 || got.MirrorTakenAt == "" {
		t.Fatalf("result = %s", run.stdout)
	}
	for _, bad := range [][]string{{"impact", "device"}, {"impact", "toaster", "x", "y"}, {"impact", "device", "explode", "gw"}} {
		if run := runUniFiCLI(t, append(bad, "--db", db)...); run.err == nil || ExitCode(run.err) != 2 {
			t.Errorf("%v: exit %d err %v", bad, ExitCode(run.err), run.err)
		}
	}
}

func TestDigestTimelineAcrossCategories(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	dip := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	healthy := time.Now().UTC().Add(-90 * time.Minute).Format(time.RFC3339)
	api.set("", "/v1/isp-metrics/5m", map[string]any{"data": []map[string]any{{"metricType": "5m", "hostId": "C1", "siteId": "s", "periods": []map[string]any{
		{"metricTime": dip, "data": map[string]any{"wan": map[string]any{"avgLatency": 250.0, "packetLoss": 0.0, "downtime": 0.0}}},
		{"metricTime": healthy, "data": map[string]any{"wan": map[string]any{"avgLatency": 10.0, "packetLoss": 0.0, "downtime": 0.0}}},
	}}}})
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	base := "/network/integration/v1/sites/" + fixtureSiteID
	api.set("C1", base+"/devices", pageOf(
		map[string]any{"id": fixGateway, "name": "gw", "model": "UDM", "state": "ONLINE"},
		map[string]any{"id": fixSwitch, "name": "sw", "model": "USW", "state": "OFFLINE"},
	))
	api.set("C1", base+"/devices/"+fixSwitch, map[string]any{"id": fixSwitch, "name": "sw", "model": "USW", "state": "OFFLINE", "uplink": map[string]any{"deviceId": fixGateway}})
	api.set("C1", base+"/clients", pageOf(
		map[string]any{"id": "client-1", "name": "laptop", "type": "WIRED", "uplinkDeviceId": fixSwitch},
		map[string]any{"id": "client-2", "name": "tablet", "type": "WIRELESS", "uplinkDeviceId": fixSwitch},
	))
	api.set("C1", base+"/firewall/policies", pageOf(map[string]any{"id": "pol-1", "name": "block iot", "enabled": false, "index": 1, "action": map[string]any{"type": "BLOCK"}}))
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	run := runUniFiCLI(t, "digest", "--since", "2h", "--console", "C1", "--db", db, "--json")
	if run.err != nil {
		t.Fatalf("%v %s", run.err, run.stderr)
	}
	var view digestView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatal(err)
	}
	for _, category := range []string{"device-state", "drift", "newcomer", "isp"} {
		if view.Counts[category] == 0 {
			t.Errorf("missing %s: %s", category, run.stdout)
		}
	}
	if view.Counts["isp"] != 1 {
		t.Errorf("only the bad ISP sample is an event: %+v", view.Counts)
	}
	for i := 1; i < len(view.Events); i++ {
		if view.Events[i-1].Time > view.Events[i].Time {
			t.Fatalf("not time ordered: %+v", view.Events)
		}
	}
	joined := strings.ToLower(strings.Join(view.Notes, " ") + run.stdout)
	if !strings.Contains(joined, "flap") {
		t.Error("must disclose the sync-point limit")
	}
	if strings.Contains(strings.ToLower(run.stdout), "caused by") {
		t.Error("must not assert causal blame")
	}
	// Negative filter: a category with nothing returns an empty list.
	run = runUniFiCLI(t, "digest", "--since", "2h", "--console", "C1", "--db", db, "--category", "camera-state", "--json")
	view = digestView{}
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || view.Events == nil || len(view.Events) != 0 {
		t.Fatalf("%v %s", err, run.stdout)
	}
}

func TestDigestOneSnapshotHasNoTransitions(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	run := runUniFiCLI(t, "digest", "--since", "1h", "--console", "C1", "--db", db, "--json")
	var view digestView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || run.err != nil {
		t.Fatalf("%v %v", err, run.err)
	}
	if len(view.Events) != 0 {
		t.Fatalf("baseline fabricated events: %+v", view.Events)
	}
	missing := filepath.Join(t.TempDir(), "absent.db")
	if run := runUniFiCLI(t, "digest", "--db", missing, "--json"); run.err != nil || !json.Valid([]byte(run.stdout)) {
		t.Fatalf("missing mirror: %v %s", run.err, run.stdout)
	}
}

// statefulObject is one REST object behind the fake connector, with scriptable failures.
type statefulObject struct {
	mu          sync.Mutex
	path        string
	collection  string
	obj         map[string]any
	writes      []string
	writeStatus int
	applyOnFail bool
	mangle      bool
	dieAfter    bool
	dead        bool
	onWrite     func()
	nextID      string
}

func (s *statefulObject) handle(w http.ResponseWriter, r *http.Request) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := r.URL.Path
	if i := strings.Index(path, "/proxy"); i >= 0 {
		path = path[i+len("/proxy"):]
	}
	if s.dead {
		w.WriteHeader(http.StatusBadGateway)
		return true
	}
	if !strings.HasPrefix(path, s.collection) {
		return false
	}
	if r.Method == http.MethodGet {
		if s.obj == nil || path != s.path {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		_ = json.NewEncoder(w).Encode(s.obj)
		return true
	}
	s.writes = append(s.writes, r.Method+" "+path)
	if s.onWrite != nil {
		s.onWrite()
	}
	var body map[string]any
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	apply := func() {
		switch r.Method {
		case http.MethodDelete:
			s.obj = nil
		case http.MethodPost:
			body["id"] = s.nextID
			s.obj = body
			s.path = s.collection + "/" + s.nextID
		case http.MethodPut:
			body["id"] = lastSegment(s.path)
			s.obj = body
		case http.MethodPatch:
			for k, v := range body {
				s.obj[k] = v
			}
		}
		if s.mangle && s.obj != nil {
			s.obj["enabled"] = "server-changed-it"
		}
	}
	if s.writeStatus != 0 {
		if s.applyOnFail {
			apply()
		}
		w.WriteHeader(s.writeStatus)
		return true
	}
	apply()
	if s.dieAfter {
		s.dead = true
	}
	_ = json.NewEncoder(w).Encode(s.obj)
	return true
}

func (s *statefulObject) writeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.writes)
}

func changeFixture(t *testing.T) (*fakeConsoleAPI, *statefulObject, string, string) {
	t.Helper()
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	collection := "/network/integration/v1/sites/" + fixtureSiteID + "/firewall/policies"
	obj := &statefulObject{collection: collection, path: collection + "/pol-1", nextID: "pol-new", obj: map[string]any{"id": "pol-1", "name": "base", "enabled": true}}
	api.handler = obj.handle
	return api, obj, db, collection
}

func writeChangeFile(t *testing.T, spec map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(spec)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "change.json")
	if err := os.WriteFile(p, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func parseChange(t *testing.T, run cliRun) changeResult {
	t.Helper()
	var result changeResult
	if err := json.Unmarshal([]byte(run.stdout), &result); err != nil {
		t.Fatalf("not JSON: %v\nstdout=%s\nerr=%v\nstderr=%s", err, run.stdout, run.err, run.stderr)
	}
	return result
}

func journalOf(t *testing.T, db, id string) store.JournalEntry {
	t.Helper()
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	entry, err := sdb.GetJournalEntry(context.Background(), id)
	if err != nil {
		t.Fatalf("journal %s: %v", id, err)
	}
	return entry
}

func patchSpec(path string) map[string]any {
	return map[string]any{"method": "PATCH", "path": path, "body": map[string]any{"enabled": false}}
}

func TestChangeApplyPreviewSendsNothing(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	file := writeChangeFile(t, patchSpec(collection+"/pol-1"))
	for _, args := range [][]string{{}, {"--dry-run", "--confirm"}} {
		run := runUniFiCLI(t, append([]string{"change", "apply", file, "--console", "C1", "--db", db, "--json"}, args...)...)
		result := parseChange(t, run)
		if run.err != nil || result.Sent {
			t.Fatalf("%v: %v %+v", args, run.err, result)
		}
		if !strings.Contains(result.URL, "/v1/connector/consoles/C1/proxy/network/integration") {
			t.Fatalf("preview url = %s", result.URL)
		}
	}
	if obj.writeCount() != 0 {
		t.Fatalf("preview wrote: %v", obj.writes)
	}
	if _, err := os.Stat(db); err == nil {
		t.Fatal("preview created a journal")
	}
}

func TestChangeApplyVerifiedThenUndo(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	file := writeChangeFile(t, patchSpec(collection+"/pol-1"))
	var pendingSeen string
	obj.onWrite = func() {
		// At the moment the write arrives, the intent must already be durable.
		sdb, err := store.OpenWithContext(context.Background(), db)
		if err != nil {
			pendingSeen = "open: " + err.Error()
			return
		}
		defer sdb.Close()
		entries, err := sdb.ListJournalEntries(context.Background(), "", time.Time{})
		if err != nil || len(entries) == 0 {
			pendingSeen = "no journal row before the write"
			return
		}
		pendingSeen = entries[len(entries)-1].State
	}
	run := runUniFiCLI(t, "change", "apply", file, "--console", "C1", "--db", db, "--confirm", "--json")
	result := parseChange(t, run)
	if run.err != nil || result.State != journalVerified || !result.Verified || !result.Undoable {
		t.Fatalf("%v %+v", run.err, result)
	}
	if pendingSeen != journalPending {
		t.Fatalf("at write time the journal said %q", pendingSeen)
	}
	obj.onWrite = nil
	entry := journalOf(t, db, result.JournalID)
	if !strings.Contains(string(entry.BeforeImage), `"enabled":true`) || !strings.Contains(string(entry.AfterImage), `"enabled":false`) || entry.ConsoleID != "C1" || entry.EntityID != "pol-1" {
		t.Fatalf("journal = %+v", entry)
	}

	run = runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--db", db, "--json")
	if preview := parseChange(t, run); preview.Sent || preview.Method != "PUT" {
		t.Fatalf("undo preview: %+v", preview)
	}
	if n := obj.writeCount(); n != 1 {
		t.Fatalf("undo preview wrote: %v", obj.writes)
	}
	run = runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
	undo := parseChange(t, run)
	if run.err != nil || undo.State != journalVerified {
		t.Fatalf("%v %+v", run.err, undo)
	}
	if obj.obj["enabled"] != true {
		t.Fatalf("not restored: %v", obj.obj)
	}
	if journalOf(t, db, result.JournalID).State != journalUndone || journalOf(t, db, undo.JournalID).Undoes != result.JournalID {
		t.Fatal("journal not linked")
	}
	run = runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
	if run.err == nil || ExitCode(run.err) != 2 {
		t.Fatalf("second undo: exit %d %v", ExitCode(run.err), run.err)
	}
}

func TestChangeApplyRefusedWriteFailsClearly(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	obj.writeStatus = http.StatusForbidden
	file := writeChangeFile(t, patchSpec(collection+"/pol-1"))
	run := runUniFiCLI(t, "change", "apply", file, "--console", "C1", "--db", db, "--confirm", "--json")
	if run.err == nil || ExitCode(run.err) != 4 || !strings.Contains(run.err.Error(), "read-only") {
		t.Fatalf("exit %d err %v", ExitCode(run.err), run.err)
	}
	result := parseChange(t, run)
	if result.State != journalFailed || journalOf(t, db, result.JournalID).State != journalFailed {
		t.Fatalf("%+v", result)
	}
}

func TestChangeApplyAmbiguousIsReadBackNeverReplayed(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		applied     bool
		method      string
		wantState   string
		wantNonZero bool
	}{
		{"408 but it landed", 408, true, "PATCH", journalVerified, false},
		{"408 and it did not land", 408, false, "PATCH", journalUnresolved, true},
		{"502 and it did not land", 502, false, "PATCH", journalUnresolved, true},
		{"408 on create cannot be located", 408, true, "POST", journalUnresolved, true},
		{"408 on delete that landed", 408, true, "DELETE", journalVerified, false},
		{"408 on delete that did not", 408, false, "DELETE", journalUnresolved, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, obj, db, collection := changeFixture(t)
			obj.writeStatus, obj.applyOnFail = tc.status, tc.applied
			spec := patchSpec(collection + "/pol-1")
			spec["method"] = tc.method
			if tc.method == "POST" {
				spec["path"] = collection
			}
			if tc.method == "DELETE" {
				delete(spec, "body")
			}
			run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json")
			result := parseChange(t, run)
			if result.State != tc.wantState || (run.err != nil) != tc.wantNonZero {
				t.Fatalf("state %q err %v (%s)", result.State, run.err, result.Detail)
			}
			if n := obj.writeCount(); n != 1 {
				t.Fatalf("write sent %d times, must be exactly once: %v", n, obj.writes)
			}
			if got := journalOf(t, db, result.JournalID).State; got != tc.wantState {
				t.Fatalf("journal state %q", got)
			}
		})
	}
}

func TestChangeApplyPathDiesAfterWriteIsUnresolved(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	obj.dieAfter = true
	t.Setenv("UNIFI_MAX_RETRIES", "0")
	run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, patchSpec(collection+"/pol-1")), "--console", "C1", "--db", db, "--confirm", "--auto-revert", "--json")
	result := parseChange(t, run)
	if run.err == nil || result.State != journalUnresolved || !strings.Contains(result.Detail, "cannot be verified or reverted") {
		t.Fatalf("err %v result %+v", run.err, result)
	}
	if obj.writeCount() != 1 {
		t.Fatalf("attempted a revert with no path: %v", obj.writes)
	}
}

func TestChangeApplyMismatchWithAndWithoutAutoRevert(t *testing.T) {
	t.Run("no auto revert leaves it and exits non-zero", func(t *testing.T) {
		_, obj, db, collection := changeFixture(t)
		obj.mangle = true
		run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, patchSpec(collection+"/pol-1")), "--console", "C1", "--db", db, "--confirm", "--json")
		result := parseChange(t, run)
		if run.err == nil || result.State != journalMismatch || len(result.Mismatch) != 1 || result.Mismatch[0] != "enabled" {
			t.Fatalf("%v %+v", run.err, result)
		}
		if obj.writeCount() != 1 {
			t.Fatalf("writes %v", obj.writes)
		}
	})
	t.Run("auto revert restores while the path lives", func(t *testing.T) {
		_, obj, db, collection := changeFixture(t)
		obj.mangle = true
		// Only the first write is mangled; the revert PUT lands cleanly.
		obj.onWrite = func() {
			if len(obj.writes) > 1 {
				obj.mangle = false
			}
		}
		run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, patchSpec(collection+"/pol-1")), "--console", "C1", "--db", db, "--confirm", "--auto-revert", "--json")
		result := parseChange(t, run)
		if run.err == nil || result.State != journalReverted || !result.Reverted {
			t.Fatalf("%v %+v", run.err, result)
		}
		if obj.obj["enabled"] != true || obj.writeCount() != 2 {
			t.Fatalf("obj %v writes %v", obj.obj, obj.writes)
		}
	})
}

func TestChangeJournalHoldsNoSecretsAndUndoRefusesThem(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	obj.obj["passphrase"] = "before-secret-value"
	spec := patchSpec(collection + "/pol-1")
	spec["body"] = map[string]any{"enabled": false, "passphrase": "after-secret-value"}
	run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json")
	result := parseChange(t, run)
	if run.err != nil || result.Undoable {
		t.Fatalf("%v %+v", run.err, result)
	}
	if strings.Contains(run.stdout, "secret-value") {
		t.Fatalf("output leaked: %s", run.stdout)
	}
	raw, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	var body, before, after string
	if err := sdb.DB().QueryRow(`SELECT request_body, before_image, after_image FROM unifi_change_journal WHERE id = ?`, result.JournalID).Scan(&body, &before, &after); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{body, before, after} {
		if strings.Contains(column, "secret-value") {
			t.Fatalf("journal holds a secret: %s", column)
		}
	}
	_ = raw
	run = runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
	if run.err == nil || !strings.Contains(run.err.Error(), "secret") {
		t.Fatalf("undo should refuse: %v", run.err)
	}
	// The audit log for the same write must also be clean.
	audit := runUniFiCLI(t, "audit", "--json")
	if strings.Contains(audit.stdout, "secret-value") {
		t.Fatalf("audit leaked: %s", audit.stdout)
	}
}

func TestChangeUndoRefusesWhenStateMoved(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, patchSpec(collection+"/pol-1")), "--console", "C1", "--db", db, "--confirm", "--json"))
	obj.mu.Lock()
	obj.obj["name"] = "edited by someone else"
	obj.mu.Unlock()
	run := runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
	if run.err == nil || !strings.Contains(run.err.Error(), "after-image") {
		t.Fatalf("err %v", run.err)
	}
	if obj.writeCount() != 1 {
		t.Fatalf("undo wrote over someone else's change: %v", obj.writes)
	}
}

func TestChangeUndoCreateAndDelete(t *testing.T) {
	t.Run("create undone by delete", func(t *testing.T) {
		_, obj, db, collection := changeFixture(t)
		spec := map[string]any{"method": "POST", "path": collection, "body": map[string]any{"name": "new", "enabled": true}}
		result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json"))
		if result.State != journalVerified || result.EntityID != "pol-new" {
			t.Fatalf("%+v", result)
		}
		run := runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
		if undo := parseChange(t, run); run.err != nil || undo.State != journalVerified || undo.Method != "DELETE" {
			t.Fatalf("%v %+v", run.err, undo)
		}
		if obj.obj != nil {
			t.Fatalf("object survives: %v", obj.obj)
		}
	})
	t.Run("delete undone by recreate, warning of a new id", func(t *testing.T) {
		_, obj, db, collection := changeFixture(t)
		spec := map[string]any{"method": "DELETE", "path": collection + "/pol-1"}
		result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json"))
		if result.State != journalVerified || obj.obj != nil {
			t.Fatalf("%+v", result)
		}
		run := runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
		undo := parseChange(t, run)
		if run.err != nil || undo.Method != "POST" || !strings.Contains(undo.Detail, "new id") || undo.EntityID != "pol-new" {
			t.Fatalf("%v %+v", run.err, undo)
		}
		if obj.obj["name"] != "base" {
			t.Fatalf("not recreated: %v", obj.obj)
		}
	})
}

func TestUndoRefusalRules(t *testing.T) {
	cases := []struct {
		entry store.JournalEntry
		want  string
	}{
		{store.JournalEntry{State: journalVerified, Kind: changeKindAction}, "cannot be undone"},
		{store.JournalEntry{State: journalUnresolved, Kind: changeKindUpdate, BeforeImage: json.RawMessage(`{}`)}, "unknown"},
		{store.JournalEntry{State: journalPending, Kind: changeKindUpdate, BeforeImage: json.RawMessage(`{}`)}, "unknown"},
		{store.JournalEntry{State: journalFailed, Kind: changeKindUpdate}, "refused"},
		{store.JournalEntry{State: journalUndone, Kind: changeKindUpdate}, "already"},
		{store.JournalEntry{State: journalReverted, Kind: changeKindUpdate}, "already"},
		{store.JournalEntry{State: journalVerified, Kind: changeKindUpdate, SecretsScrubbed: true, BeforeImage: json.RawMessage(`{}`)}, "secret"},
		{store.JournalEntry{State: journalVerified, Kind: changeKindUpdate}, "no before-image"},
		{store.JournalEntry{State: journalVerified, Kind: changeKindUndo}, "itself an undo"},
		{store.JournalEntry{State: journalVerified, Kind: changeKindUpdate, BeforeImage: json.RawMessage(`{}`)}, ""},
		// Deleting a created object needs no before-image, so scrubbing does not block it.
		{store.JournalEntry{State: journalVerified, Kind: changeKindCreate, SecretsScrubbed: true}, ""},
	}
	for _, tc := range cases {
		got := undoRefusal(tc.entry)
		if (tc.want == "" && got != "") || !strings.Contains(got, tc.want) {
			t.Errorf("%+v: got %q, want %q", tc.entry, got, tc.want)
		}
	}
}

func TestChangeUndoUnknownIDExitsNotFound(t *testing.T) {
	_, _, db, _ := changeFixture(t)
	run := runUniFiCLI(t, "change", "undo", "chg_absent", "--console", "C1", "--db", db, "--confirm")
	if run.err == nil || ExitCode(run.err) != 3 {
		t.Fatalf("exit %d err %v", ExitCode(run.err), run.err)
	}
}

func TestChangeReadOnlyModeRefusesBeforeJournaling(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	t.Setenv("UNIFI_READ_ONLY", "1")
	run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, patchSpec(collection+"/pol-1")), "--console", "C1", "--db", db, "--confirm", "--json")
	if run.err == nil || ExitCode(run.err) != 10 || obj.writeCount() != 0 {
		t.Fatalf("exit %d err %v writes %v", ExitCode(run.err), run.err, obj.writes)
	}
	if _, err := os.Stat(db); err == nil {
		t.Fatal("refused write created a journal")
	}
}

func TestChangeApplyRejectsPathsOutsideTheOfficialAPI(t *testing.T) {
	_, obj, db, _ := changeFixture(t)
	for _, path := range []string{"/proxy/network/api/s/default/cmd/stamgr", "/v1/hosts", "/network/integration/v1/../../x", "https://evil.example/x"} {
		run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, map[string]any{"method": "POST", "path": path, "body": map[string]any{}}), "--console", "C1", "--db", db, "--confirm")
		if run.err == nil || ExitCode(run.err) != 2 {
			t.Errorf("%s: exit %d err %v", path, ExitCode(run.err), run.err)
		}
	}
	if obj.writeCount() != 0 {
		t.Fatalf("writes: %v", obj.writes)
	}
}

func TestTrialSchedule(t *testing.T) {
	collection := "/network/integration/v1/sites/s1/firewall/policies"
	noon := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	t.Run("rewrites to ONE_TIME_ONLY in the given zone", func(t *testing.T) {
		spec := changeSpec{Method: "POST", Path: collection, Body: json.RawMessage(`{"name":"trial","schedule":{"mode":"ALWAYS"}}`)}
		trial, err := applyTrialSchedule(&spec, 45, "America/New_York", noon)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		_ = json.Unmarshal(spec.Body, &body)
		schedule := body["schedule"].(map[string]any)
		window := schedule["timeFilter"].(map[string]any)
		if schedule["mode"] != "ONE_TIME_ONLY" || schedule["date"] != "2026-11-01" || window["startTime"] != "07:00" || window["stopTime"] != "07:45" {
			t.Fatalf("schedule = %v", schedule)
		}
		if trial.ExpiryVerified {
			t.Fatal("expiry must be reported as unverified")
		}
	})
	t.Run("refuses to cross midnight", func(t *testing.T) {
		spec := changeSpec{Method: "POST", Path: collection, Body: json.RawMessage(`{"name":"t"}`)}
		_, err := applyTrialSchedule(&spec, 60, "UTC", time.Date(2026, 11, 1, 23, 30, 0, 0, time.UTC))
		if err == nil || !strings.Contains(err.Error(), "midnight") {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("refuses edits", func(t *testing.T) {
		for _, spec := range []changeSpec{
			{Method: "PUT", Path: collection + "/p1", Body: json.RawMessage(`{}`)},
			{Method: "PATCH", Path: collection + "/p1", Body: json.RawMessage(`{}`)},
			{Method: "POST", Path: collection, Body: json.RawMessage(`{"id":"existing"}`)},
			{Method: "POST", Path: "/network/integration/v1/sites/s1/networks", Body: json.RawMessage(`{}`)},
		} {
			if _, err := applyTrialSchedule(&spec, 10, "UTC", noon); err == nil {
				t.Errorf("%s %s accepted", spec.Method, spec.Path)
			}
		}
	})
}

func TestChangeApplyTrialEndToEnd(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	changeNow = func() time.Time { return time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC) }
	defer func() { changeNow = time.Now }()
	file := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(file, []byte(`{"name":"trial block","enabled":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	run := runUniFiCLI(t, "change", "apply", file, "--trial", "--site", fixtureSiteID, "--trial-timezone", "UTC", "--console", "C1", "--db", db, "--confirm", "--json")
	result := parseChange(t, run)
	if run.err != nil || result.State != journalVerified || result.Trial == nil || result.Trial.ExpiryVerified {
		t.Fatalf("%v %+v", run.err, result)
	}
	if obj.writes[0] != "POST "+collection {
		t.Fatalf("writes %v", obj.writes)
	}
	if schedule := obj.obj["schedule"].(map[string]any); schedule["mode"] != "ONE_TIME_ONLY" {
		t.Fatalf("sent %v", obj.obj)
	}
	// A bare policy without --trial is refused: nothing says where it goes.
	run = runUniFiCLI(t, "change", "apply", file, "--console", "C1", "--db", db, "--confirm")
	if run.err == nil || ExitCode(run.err) != 2 {
		t.Fatalf("exit %d err %v", ExitCode(run.err), run.err)
	}
}

func TestVerifiedChangeAttributesDriftAndReachYes(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	collection := "/network/integration/v1/sites/" + fixtureSiteID + "/firewall/policies"
	obj := &statefulObject{collection: collection, path: collection + "/pol-1", obj: map[string]any{"id": "pol-1", "name": "block iot", "enabled": true, "index": float64(1), "action": map[string]any{"type": "BLOCK"}}}
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		trimmed := r.URL.Path[strings.Index(r.URL.Path, "/proxy")+len("/proxy"):]
		if strings.Contains(r.URL.Path, "/proxy") && trimmed == collection && r.Method == http.MethodGet {
			obj.mu.Lock()
			defer obj.mu.Unlock()
			_ = json.NewEncoder(w).Encode(pageOf(obj.obj))
			return true
		}
		return obj.handle(w, r)
	}
	result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, patchSpec(collection+"/pol-1")), "--console", "C1", "--db", db, "--confirm", "--json"))
	if result.State != journalVerified {
		t.Fatalf("%+v", result)
	}
	time.Sleep(1100 * time.Millisecond) // snapshot stamps have one second resolution
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	run := runUniFiCLI(t, "drift", "--db", db, "--console", "C1", "--json")
	var view driftView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Changes) != 1 || view.Changes[0].Attribution != "attributed" || view.Changes[0].JournalIDs[0] != result.JournalID {
		t.Fatalf("drift = %s", run.stdout)
	}
}

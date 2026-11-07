// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

const (
	fixGateway = "aaaaaaaa-0000-0000-0000-000000000001"
	fixSwitch  = "aaaaaaaa-0000-0000-0000-000000000002"
	fixAP      = "aaaaaaaa-0000-0000-0000-000000000003"
)

func deviceRow(id, name, uplink string) mirrorRow {
	data := map[string]any{"name": name, "model": "M", "state": "ONLINE"}
	if uplink != "" {
		data["uplink"] = map[string]any{"deviceId": uplink}
	}
	return mirrorRow{Type: rtDevices, ID: id, Parent: fixtureSiteID, Console: "C1", Data: data}
}

func TestTopologyBuildsTreeAndCountsClients(t *testing.T) {
	devices := []mirrorRow{deviceRow(fixGateway, "gw", ""), deviceRow(fixSwitch, "sw", fixGateway), deviceRow(fixAP, "ap", fixSwitch)}
	clients := []mirrorRow{
		{Type: rtClients, ID: "c1", Parent: fixtureSiteID, Console: "C1", Data: map[string]any{"name": "laptop", "uplinkDeviceId": fixAP}},
		{Type: rtClients, ID: "c2", Parent: fixtureSiteID, Console: "C1", Data: map[string]any{"name": "ghost", "uplinkDeviceId": "not-in-mirror"}},
	}
	site := buildTopologySite(fixtureSiteID, devices, clients, true)
	if len(site.Roots) != 1 || site.Roots[0].Name != "gw" {
		t.Fatalf("roots = %+v", site.Roots)
	}
	sw := site.Roots[0].Children[0]
	if sw.Name != "sw" || sw.Children[0].Name != "ap" || sw.Children[0].ClientCount != 1 || sw.Children[0].Clients[0].Name != "laptop" {
		t.Fatalf("tree wrong: %+v", sw)
	}
	if site.UnparentedClients != 1 {
		t.Fatalf("unparented = %d", site.UnparentedClients)
	}
}

func TestTopologyToleratesOrphansAndCycles(t *testing.T) {
	devices := []mirrorRow{deviceRow("a", "a", "b"), deviceRow("b", "b", "a"), deviceRow("orphan", "orphan", "gone")}
	site := buildTopologySite("s", devices, nil, false)
	names := map[string]bool{}
	for _, root := range site.Roots {
		names[root.Name] = true
	}
	// Terminating at all is the first assertion; a cycle must not hang or lose nodes.
	if !names["orphan"] || !names["a"] || !names["b"] {
		t.Fatalf("roots = %v", names)
	}
}

func seedConsole(api *fakeConsoleAPI, console string) {
	api.set(console, "/network/integration/v1/sites", pageOf(map[string]any{"id": fixtureSiteID, "name": "Default"}))
	base := "/network/integration/v1/sites/" + fixtureSiteID
	api.set(console, base+"/devices", pageOf(
		map[string]any{"id": fixGateway, "name": "gw", "model": "UDM", "state": "ONLINE", "macAddress": "02:00:00:00:00:01"},
		map[string]any{"id": fixSwitch, "name": "sw", "model": "USW", "state": "ONLINE", "macAddress": "02:00:00:00:00:02"},
	))
	api.set(console, base+"/devices/"+fixGateway, map[string]any{"id": fixGateway, "name": "gw", "model": "UDM", "state": "ONLINE", "macAddress": "02:00:00:00:00:01", "uplink": nil})
	api.set(console, base+"/devices/"+fixSwitch, map[string]any{"id": fixSwitch, "name": "sw", "model": "USW", "state": "ONLINE", "macAddress": "02:00:00:00:00:02", "uplink": map[string]any{"deviceId": fixGateway}})
	api.set(console, base+"/clients", pageOf(map[string]any{"id": "client-1", "name": "laptop", "type": "WIRED", "macAddress": "00:11:22:33:44:01", "uplinkDeviceId": fixSwitch}))
	api.set(console, base+"/firewall/policies", pageOf(map[string]any{"id": "pol-1", "name": "block iot", "enabled": true, "index": 1, "action": map[string]any{"type": "BLOCK"}}))
}

func TestTopologyEndToEndUsesHydratedUplink(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatalf("sync: %v\n%s", run.err, run.stderr)
	}
	run := runUniFiCLI(t, "topology", "--db", db, "--console", "C1", "--json")
	if run.err != nil {
		t.Fatalf("topology: %v", run.err)
	}
	var view topologyView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, run.stdout)
	}
	if len(view.Consoles) != 1 || len(view.Consoles[0].Sites) != 1 {
		t.Fatalf("view = %+v", view)
	}
	roots := view.Consoles[0].Sites[0].Roots
	if len(roots) != 1 || roots[0].Name != "gw" || len(roots[0].Children) != 1 || roots[0].Children[0].ClientCount != 1 {
		t.Fatalf("tree = %s", run.stdout)
	}
}

func TestTopologyWithoutHydrationSaysSoInsteadOfGuessing(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--no-hydrate", "--json"); run.err != nil {
		t.Fatalf("sync: %v", run.err)
	}
	run := runUniFiCLI(t, "topology", "--db", db, "--console", "C1", "--json")
	if run.err != nil || !strings.Contains(run.stdout, "no device has uplink data") {
		t.Fatalf("err=%v out=%s", run.err, run.stdout)
	}
}

func TestLocalCommandsOnMissingMirrorReturnEmptyAndExitZero(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	missing := t.TempDir() + "/absent.db"
	for _, args := range [][]string{
		{"topology", "--db", missing, "--json"},
		{"drift", "--db", missing, "--json"},
		{"newcomer", "--db", missing, "--json"},
		{"rule-predict", "--db", missing, "--src-zone", "a", "--dst-zone", "b", "--json"},
	} {
		run := runUniFiCLI(t, args...)
		if run.err != nil {
			t.Errorf("%v: %v", args, run.err)
		}
		if !json.Valid([]byte(run.stdout)) {
			t.Errorf("%v: stdout not JSON: %s", args, run.stdout)
		}
		if _, err := os.Stat(missing); err == nil {
			t.Fatalf("%v created the mirror", args)
		}
	}
}

func snapRow(rt, id, body string) (string, store.SnapshotRow) {
	return rt + "|" + id, store.SnapshotRow{ResourceType: rt, EntityID: id, Data: json.RawMessage(body)}
}

func snapMap(rows ...[3]string) map[string]store.SnapshotRow {
	out := map[string]store.SnapshotRow{}
	for _, r := range rows {
		k, v := snapRow(r[0], r[1], r[2])
		out[k] = v
	}
	return out
}

func TestDriftNoChangeReturnsEmptyNotFabricated(t *testing.T) {
	same := snapMap([3]string{rtFirewall, "s/p1", `{"name":"a","enabled":true}`})
	got := computeDrift("C1", same, same, nil, "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", driftResourceTypes)
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want empty non-nil", got)
	}
}

func TestDriftFieldLevelAndAttribution(t *testing.T) {
	from := snapMap(
		[3]string{rtFirewall, "s/p1", `{"name":"a","enabled":true,"action":{"type":"ALLOW"}}`},
		[3]string{rtFirewall, "s/p2", `{"name":"b","enabled":true}`},
		[3]string{rtWifi, "s/w1", `{"name":"guest","passphrase":"old-secret"}`},
		[3]string{rtClients, "s/c1", `{"name":"laptop","ipAddress":"192.0.2.1"}`},
	)
	to := snapMap(
		[3]string{rtFirewall, "s/p1", `{"name":"a","enabled":false,"action":{"type":"BLOCK"}}`},
		[3]string{rtWifi, "s/w1", `{"name":"guest","passphrase":"new-secret"}`},
		[3]string{rtNetworks, "s/n1", `{"name":"iot"}`},
		[3]string{rtClients, "s/c1", `{"name":"laptop","ipAddress":"192.0.2.2"}`},
	)
	journal := []store.JournalEntry{
		{ID: "j-in", ConsoleID: "C1", EntityID: "p1", State: journalVerified, CreatedAt: "2026-01-01T12:00:00Z"},
		{ID: "j-other-console", ConsoleID: "C2", EntityID: "w1", State: journalVerified, CreatedAt: "2026-01-01T12:00:00Z"},
		{ID: "j-before-window", ConsoleID: "C1", EntityID: "n1", State: journalVerified, CreatedAt: "2025-12-01T00:00:00Z"},
		{ID: "j-failed", ConsoleID: "C1", EntityID: "p2", State: journalFailed, CreatedAt: "2026-01-01T12:00:00Z"},
	}
	got := computeDrift("C1", from, to, journal, "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", driftResourceTypes)
	by := map[string]driftChange{}
	for _, c := range got {
		by[c.EntityID] = c
	}
	if _, present := by["c1"]; present {
		t.Fatal("client state is not config and must not appear in drift")
	}
	p1 := by["p1"]
	if p1.Change != "changed" || p1.Attribution != "attributed" || p1.JournalIDs[0] != "j-in" {
		t.Fatalf("p1 = %+v", p1)
	}
	paths := map[string]driftField{}
	for _, f := range p1.Fields {
		paths[f.Path] = f
	}
	if paths["enabled"].After != false || paths["action.type"].After != "BLOCK" {
		t.Fatalf("fields = %+v", p1.Fields)
	}
	if by["p2"].Change != "removed" || by["p2"].Attribution != "out-of-band" {
		t.Fatalf("a failed journal entry must not attribute: %+v", by["p2"])
	}
	if by["n1"].Change != "added" || by["n1"].Attribution != "out-of-band" {
		t.Fatalf("journal entry outside the window must not attribute: %+v", by["n1"])
	}
	if by["w1"].Attribution != "out-of-band" {
		t.Fatalf("another console's journal must not attribute: %+v", by["w1"])
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "new-secret") || strings.Contains(string(encoded), "old-secret") {
		t.Fatalf("secret leaked: %s", encoded)
	}
	if by["w1"].Fields[0].Before != "[scrubbed]" {
		t.Fatalf("secret change should still be reported: %+v", by["w1"])
	}
}

func TestDriftEndToEndBetweenTwoSyncs(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	// One snapshot: empty changes and an honest reason.
	run := runUniFiCLI(t, "drift", "--db", db, "--console", "C1", "--json")
	var view driftView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || run.err != nil {
		t.Fatalf("%v %v %s", err, run.err, run.stdout)
	}
	if len(view.Changes) != 0 || !strings.Contains(view.Note, "one snapshot") {
		t.Fatalf("view = %+v", view)
	}
	api.set("C1", "/network/integration/v1/sites/"+fixtureSiteID+"/firewall/policies", pageOf(map[string]any{"id": "pol-1", "name": "block iot", "enabled": false, "index": 1, "action": map[string]any{"type": "BLOCK"}}))
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	run = runUniFiCLI(t, "drift", "--db", db, "--console", "C1", "--json")
	view = driftView{}
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || run.err != nil {
		t.Fatalf("%v %v %s", err, run.err, run.stdout)
	}
	if len(view.Changes) != 1 || view.Changes[0].EntityID != "pol-1" || view.Changes[0].Attribution != "out-of-band" || view.Changes[0].Fields[0].Path != "enabled" {
		t.Fatalf("drift = %s", run.stdout)
	}
}

func TestDriftRejectsLiveOnlyBeforeTouchingLocalState(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	missing := t.TempDir() + "/absent.db"
	run := runUniFiCLI(t, "drift", "--data-source", "live", "--db", missing, "--json")
	if run.err == nil {
		t.Fatal("drift --data-source live must be refused")
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("refused command created local state")
	}
	if len(api.requested()) != 0 {
		t.Fatalf("refused command called the API: %v", api.requested())
	}
}

func TestSnapshotsAreIsolatedPerConsole(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	seedConsole(api, "C2")
	for _, c := range []string{"C1", "C2", "C1"} {
		if run := runUniFiCLI(t, "sync", "--console", c, "--db", db, "--json"); run.err != nil {
			t.Fatal(run.err)
		}
	}
	// C1 and C2 have one identical policy id. C2's snapshot must not count as drift
	// on C1, and C2 has only one snapshot, so nothing compares there.
	run := runUniFiCLI(t, "drift", "--db", db, "--json")
	var view driftView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || run.err != nil {
		t.Fatalf("%v %v", err, run.err)
	}
	if len(view.Changes) != 0 {
		t.Fatalf("cross-console contamination: %s", run.stdout)
	}
	if len(view.Windows) != 1 || view.Windows[0].ConsoleID != "C1" {
		t.Fatalf("windows = %+v", view.Windows)
	}
}

func TestNewcomerExcludesBaselineAndFindsNew(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	var view newcomerView
	run := runUniFiCLI(t, "newcomer", "--db", db, "--console", "C1", "--json")
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || run.err != nil {
		t.Fatalf("%v %v", err, run.err)
	}
	if len(view.Newcomers) != 0 {
		t.Fatalf("baseline reported as new: %+v", view.Newcomers)
	}
	api.set("C1", "/network/integration/v1/sites/"+fixtureSiteID+"/clients", pageOf(
		map[string]any{"id": "client-1", "name": "laptop", "type": "WIRED", "macAddress": "00:11:22:33:44:01", "uplinkDeviceId": fixSwitch},
		map[string]any{"id": "client-new", "name": "phone", "type": "WIRELESS", "macAddress": "02:ab:00:00:00:09", "ipAddress": "192.0.2.50", "uplinkDeviceId": fixSwitch},
	))
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	view = newcomerView{}
	run = runUniFiCLI(t, "newcomer", "--db", db, "--console", "C1", "--since", "1h", "--json")
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || run.err != nil {
		t.Fatalf("%v %v", err, run.err)
	}
	if len(view.Newcomers) != 1 || view.Newcomers[0].EntityID != "client-new" || !view.Newcomers[0].RandomizedMAC || view.Newcomers[0].Kind != "client" || view.Newcomers[0].SiteID != fixtureSiteID {
		t.Fatalf("newcomers = %+v", view.Newcomers)
	}
	// Negative filters: another kind and another console find nothing.
	view = newcomerView{}
	run = runUniFiCLI(t, "newcomer", "--db", db, "--console", "C1", "--kind", "camera", "--json")
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || len(view.Newcomers) != 0 {
		t.Fatalf("kind filter: %v %+v", err, view.Newcomers)
	}
	view = newcomerView{}
	run = runUniFiCLI(t, "newcomer", "--db", db, "--console", "NOT-THIS-ONE", "--json")
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil || len(view.Newcomers) != 0 {
		t.Fatalf("console filter: %v %+v", err, view.Newcomers)
	}
}

func TestNewcomerWindowExcludesOlder(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	ctx := context.Background()
	old := time.Now().Add(-72 * time.Hour)
	row := func(id string) []store.SnapshotRow {
		return []store.SnapshotRow{{ResourceType: rtClients, EntityID: "s/" + id, Data: json.RawMessage(`{}`)}}
	}
	if _, err := sdb.SaveSnapshot(ctx, "C1", old.Add(-time.Hour), row("base"), nil, firstSeenTracked); err != nil {
		t.Fatal(err)
	}
	if _, err := sdb.SaveSnapshot(ctx, "C1", old, row("older"), nil, firstSeenTracked); err != nil {
		t.Fatal(err)
	}
	view, err := loadNewcomers(ctx, sdb, "C1", 24*time.Hour, "", time.Now())
	if err != nil || len(view.Newcomers) != 0 {
		t.Fatalf("older than window: %+v %v", view.Newcomers, err)
	}
	view, err = loadNewcomers(ctx, sdb, "C1", 96*time.Hour, "", time.Now())
	if err != nil || len(view.Newcomers) != 1 || view.Newcomers[0].EntityID != "older" {
		t.Fatalf("inside window: %+v %v", view.Newcomers, err)
	}
}

func TestMacIsRandomized(t *testing.T) {
	cases := map[string]bool{"02:00:00:00:00:01": true, "DA:A1:19:00:00:00": true, "00:11:22:33:44:55": false, "": false, "zz": false}
	for mac, want := range cases {
		if got := macIsRandomized(mac); got != want {
			t.Errorf("%q = %v", mac, got)
		}
	}
}

func portItems(values ...map[string]any) map[string]any {
	items := make([]any, 0, len(values))
	for _, v := range values {
		items = append(items, v)
	}
	return map[string]any{"type": "PORTS", "items": items}
}

func policyFixture(id string, index int, action string, srcZone, dstZone string, dstFilter map[string]any) map[string]any {
	dst := map[string]any{"zoneId": dstZone}
	if dstFilter != nil {
		dst["trafficFilter"] = dstFilter
	}
	return map[string]any{"id": id, "name": id, "index": float64(index), "enabled": true, "action": map[string]any{"type": action}, "source": map[string]any{"zoneId": srcZone}, "destination": dst, "schedule": map[string]any{"mode": "ALWAYS"}, "ipProtocolScope": map[string]any{"ipVersion": "IPV4_AND_IPV6"}}
}

func TestRulePredictTable(t *testing.T) {
	port443 := map[string]any{"type": "PORT", "portFilter": portItems(map[string]any{"type": "PORT_NUMBER", "value": float64(443)})}
	port80 := map[string]any{"type": "PORT", "portFilter": portItems(map[string]any{"type": "PORT_NUMBER", "value": float64(80)})}
	portRange := map[string]any{"type": "PORT", "portFilter": portItems(map[string]any{"type": "PORT_NUMBER_RANGE", "start": float64(400), "stop": float64(500)})}
	opposite := map[string]any{"type": "PORT", "portFilter": map[string]any{"type": "PORTS", "matchOpposite": true, "items": []any{map[string]any{"type": "PORT_NUMBER", "value": float64(443)}}}}
	domain := map[string]any{"type": "DOMAIN", "domainFilter": map[string]any{"type": "DOMAINS", "domains": []any{"example.com"}}}
	domainPort80 := map[string]any{"type": "DOMAIN", "domainFilter": map[string]any{"type": "DOMAINS", "domains": []any{"example.com"}}, "portFilter": portItems(map[string]any{"type": "PORT_NUMBER", "value": float64(80)})}
	subnet := map[string]any{"type": "IP_ADDRESS", "ipAddressFilter": map[string]any{"type": "IP_ADDRESSES", "items": []any{map[string]any{"type": "SUBNET", "value": "192.0.2.0/24"}}}}
	tml := map[string]any{"type": "PORT", "portFilter": map[string]any{"type": "TRAFFIC_MATCHING_LIST", "trafficMatchingListId": "tml-1"}}
	tmlMissing := map[string]any{"type": "PORT", "portFilter": map[string]any{"type": "TRAFFIC_MATCHING_LIST", "trafficMatchingListId": "absent"}}
	packet := predictPacket{SrcZone: "lan", DstZone: "wan", DstIP: "192.0.2.9", DstPort: 443, Protocol: "tcp"}
	disabled := policyFixture("disabled-allow", 1, "ALLOW", "lan", "wan", port443)
	disabled["enabled"] = false
	scheduled := policyFixture("scheduled", 1, "ALLOW", "lan", "wan", port443)
	scheduled["schedule"] = map[string]any{"mode": "EVERY_DAY"}
	wrongState := policyFixture("established-only", 1, "ALLOW", "lan", "wan", port443)
	wrongState["connectionStateFilter"] = []any{"ESTABLISHED", "RELATED"}

	cases := []struct {
		name          string
		policies      []map[string]any
		acl           []map[string]any
		tmls          map[string]map[string]any
		p             predictPacket
		wantVerdict   string
		wantAction    string
		wantPolicy    string
		wantUndecided string
	}{
		{"first by index wins", []map[string]any{policyFixture("later-allow", 9, "ALLOW", "lan", "wan", port443), policyFixture("first-block", 2, "BLOCK", "lan", "wan", port443)}, nil, nil, packet, "match", "BLOCK", "first-block", ""},
		{"disabled is skipped", []map[string]any{disabled, policyFixture("next", 2, "BLOCK", "lan", "wan", nil)}, nil, nil, packet, "match", "BLOCK", "next", ""},
		{"other zone does not match", []map[string]any{policyFixture("other", 1, "ALLOW", "iot", "wan", nil), policyFixture("mine", 2, "REJECT", "lan", "wan", nil)}, nil, nil, packet, "match", "REJECT", "mine", ""},
		{"port mismatch falls through", []map[string]any{policyFixture("p80", 1, "ALLOW", "lan", "wan", port80), policyFixture("p443", 2, "BLOCK", "lan", "wan", port443)}, nil, nil, packet, "match", "BLOCK", "p443", ""},
		{"port range", []map[string]any{policyFixture("range", 1, "ALLOW", "lan", "wan", portRange)}, nil, nil, packet, "match", "ALLOW", "range", ""},
		{"match opposite inverts", []map[string]any{policyFixture("not443", 1, "ALLOW", "lan", "wan", opposite), policyFixture("next", 2, "BLOCK", "lan", "wan", nil)}, nil, nil, packet, "match", "BLOCK", "next", ""},
		{"domain is indeterminate and stops the walk", []map[string]any{policyFixture("dom", 1, "BLOCK", "lan", "wan", domain), policyFixture("next", 2, "ALLOW", "lan", "wan", nil)}, nil, nil, packet, "indeterminate", "", "", "dom"},
		{"definite port miss beats unknown domain", []map[string]any{policyFixture("dom80", 1, "BLOCK", "lan", "wan", domainPort80), policyFixture("next", 2, "ALLOW", "lan", "wan", nil)}, nil, nil, packet, "match", "ALLOW", "next", ""},
		{"schedule is indeterminate", []map[string]any{scheduled}, nil, nil, packet, "indeterminate", "", "", "scheduled"},
		{"non-new connection state skipped", []map[string]any{wrongState, policyFixture("next", 2, "BLOCK", "lan", "wan", nil)}, nil, nil, packet, "match", "BLOCK", "next", ""},
		{"subnet matches", []map[string]any{policyFixture("net", 1, "ALLOW", "lan", "wan", subnet)}, nil, nil, packet, "match", "ALLOW", "net", ""},
		{"missing packet ip is indeterminate", []map[string]any{policyFixture("net", 1, "ALLOW", "lan", "wan", subnet)}, nil, nil, predictPacket{SrcZone: "lan", DstZone: "wan", DstPort: 443, Protocol: "tcp"}, "indeterminate", "", "", "net"},
		{"traffic matching list resolves", []map[string]any{policyFixture("list", 1, "ALLOW", "lan", "wan", tml)}, nil, map[string]map[string]any{"tml-1": {"type": "PORTS", "items": []any{map[string]any{"type": "PORT_NUMBER", "value": float64(443)}}}}, packet, "match", "ALLOW", "list", ""},
		{"unsynced traffic matching list is indeterminate", []map[string]any{policyFixture("list", 1, "ALLOW", "lan", "wan", tmlMissing)}, nil, nil, packet, "indeterminate", "", "", "list"},
		{"no policy matches is indeterminate, not allow", []map[string]any{policyFixture("other", 1, "ALLOW", "iot", "dmz", nil)}, nil, nil, packet, "indeterminate", "", "", ""},
		{"empty ruleset is indeterminate", nil, nil, nil, packet, "indeterminate", "", "", ""},
		{"blocking ACL makes it indeterminate", []map[string]any{policyFixture("allow", 1, "ALLOW", "lan", "wan", nil)}, []map[string]any{{"id": "acl", "name": "acl", "type": "IPV4", "enabled": true, "action": "BLOCK", "index": float64(1)}}, nil, packet, "indeterminate", "", "allow", ""},
		{"non-matching ACL does not interfere", []map[string]any{policyFixture("allow", 1, "ALLOW", "lan", "wan", nil)}, []map[string]any{{"id": "acl", "type": "IPV4", "enabled": true, "action": "BLOCK", "protocolFilter": []any{"UDP"}}}, nil, packet, "match", "ALLOW", "allow", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := predictRule(tc.p, predictInputs{Policies: tc.policies, ACLRules: tc.acl, TMLs: tc.tmls})
			if got.Verdict != tc.wantVerdict || got.PredictedAction != tc.wantAction {
				t.Fatalf("verdict %q action %q (%s), want %q %q", got.Verdict, got.PredictedAction, got.Reason, tc.wantVerdict, tc.wantAction)
			}
			if tc.wantPolicy != "" && (got.MatchedPolicy == nil || got.MatchedPolicy.ID != tc.wantPolicy) {
				t.Fatalf("matched %+v, want %s", got.MatchedPolicy, tc.wantPolicy)
			}
			if tc.wantUndecided != "" && (got.UndecidedPolicy == nil || got.UndecidedPolicy.ID != tc.wantUndecided) {
				t.Fatalf("undecided %+v, want %s", got.UndecidedPolicy, tc.wantUndecided)
			}
			if !got.Prediction || got.ConfirmedByAPI {
				t.Fatalf("must be flagged as an unconfirmed prediction: %+v", got)
			}
		})
	}
}

func TestRulePredictEndToEndResolvesZoneNames(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	base := "/network/integration/v1/sites/" + fixtureSiteID
	api.set("C1", base+"/firewall/zones", pageOf(map[string]any{"id": "zone-lan", "name": "LAN"}, map[string]any{"id": "zone-wan", "name": "External"}))
	api.set("C1", base+"/firewall/policies", pageOf(policyFixture("drop-ssh", 3, "BLOCK", "zone-lan", "zone-wan", map[string]any{"type": "PORT", "portFilter": portItems(map[string]any{"type": "PORT_NUMBER", "value": float64(22)})})))
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	run := runUniFiCLI(t, "rule-predict", "--db", db, "--console", "C1", "--src-zone", "lan", "--dst-zone", "external", "--dst-port", "22", "--protocol", "tcp", "--json")
	if run.err != nil {
		t.Fatalf("%v %s", run.err, run.stderr)
	}
	var got predictResult
	if err := json.Unmarshal([]byte(run.stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got.Verdict != "match" || got.PredictedAction != "BLOCK" || got.MatchedPolicy.ID != "drop-ssh" {
		t.Fatalf("result = %s", run.stdout)
	}
	// Negative: another port falls off the end and must not claim anything.
	run = runUniFiCLI(t, "rule-predict", "--db", db, "--console", "C1", "--src-zone", "lan", "--dst-zone", "external", "--dst-port", "443", "--protocol", "tcp", "--json")
	if run.err != nil || !strings.Contains(run.stdout, `"indeterminate"`) || strings.Contains(run.stdout, "drop-ssh") {
		t.Fatalf("%v %s", run.err, run.stdout)
	}
	run = runUniFiCLI(t, "rule-predict", "--db", db, "--console", "C1", "--src-zone", "nope", "--dst-zone", "external", "--json")
	if run.err == nil || ExitCode(run.err) != 2 || !strings.Contains(run.err.Error(), "External") {
		t.Fatalf("unknown zone should list known zones with exit 2: %v", run.err)
	}
}

func reachFixture(t *testing.T) *fakeConsoleAPI {
	t.Helper()
	api := newFakeConsoleAPI(t)
	api.set("", "/v1/hosts", map[string]any{"data": []map[string]any{{"id": "HOST-A", "reportedState": map[string]any{"name": "Alpha"}}}})
	api.set("HOST-A", "/network/integration/v1/sites", pageOf(map[string]any{"id": fixtureSiteID, "name": "Default"}))
	api.set("HOST-A", "/network/integration/v1/sites/"+fixtureSiteID+"/wans", pageOf(map[string]any{"id": "w1", "name": "WAN"}, map[string]any{"id": "w2", "name": "WAN2"}))
	api.set("HOST-A", "/network/integration/v1/sites/"+fixtureSiteID+"/vpn/servers", pageOf(map[string]any{"id": "v1", "name": "wg"}))
	api.set("HOST-A", "/network/integration/v1/info", map[string]any{"applicationVersion": "11.0"})
	return api
}

func parseReach(t *testing.T, run cliRun) reachView {
	t.Helper()
	if run.err != nil {
		t.Fatalf("reach: %v\n%s", run.err, run.stderr)
	}
	var view reachView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, run.stdout)
	}
	return view
}

func TestReachReportsSurfacesShapeAndSecondPath(t *testing.T) {
	api := reachFixture(t)
	fixtureEnv(t, api.server.URL)
	view := parseReach(t, runUniFiCLI(t, "reach", "--json"))
	if len(view.Consoles) != 1 {
		t.Fatalf("view = %+v", view)
	}
	row := view.Consoles[0]
	cloud := row.Transports["connector"]
	if !cloud.Available || !strings.Contains(cloud.PathShape, "/proxy/") || cloud.Surfaces["network"].Status != "ok" {
		t.Fatalf("connector = %+v", cloud)
	}
	if cloud.Write != "unproven" {
		t.Fatalf("no probe must stay unproven: %+v", cloud)
	}
	if row.Transports["local"].Skipped == "" {
		t.Fatalf("local should be skipped with a reason: %+v", row.Transports["local"])
	}
	if row.SecondPath == nil || row.SecondPath.WANCount == nil || *row.SecondPath.WANCount != 2 || len(row.SecondPath.VPNServers) != 1 || !strings.Contains(row.SecondPath.Note, "not mean tested") {
		t.Fatalf("second path = %+v", row.SecondPath)
	}
	for _, r := range api.requested() {
		if !strings.HasPrefix(r, "GET ") {
			t.Fatalf("reach without --probe-writes sent a mutation: %s", r)
		}
	}
}

func TestReachFindsAlternateConnectorShape(t *testing.T) {
	api := reachFixture(t)
	fixtureEnv(t, api.server.URL)
	// This console only answers the no-proxy shape from the Site Manager narrative.
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		if strings.Contains(r.URL.Path, "/consoles/HOST-A/proxy/") {
			w.WriteHeader(http.StatusNotFound)
			return true
		}
		return false
	}
	view := parseReach(t, runUniFiCLI(t, "reach", "--json"))
	cloud := view.Consoles[0].Transports["connector"]
	if !cloud.Available || strings.Contains(cloud.PathShape, "/proxy/") || cloud.PathShape == "unknown" {
		t.Fatalf("shape not found: %+v", cloud)
	}
}

func TestReachWriteProbeVerdicts(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   string
	}{
		{"forbidden is a definite no", 403, "no"},
		{"unauthorized is a definite no", 401, "no"},
		{"method not allowed is a definite no", 405, "no"},
		{"404 may be the proxy, so unproven", 404, "unproven"},
		{"400 is unproven", 400, "unproven"},
		{"200 is unproven, never yes", 200, "unproven"},
		{"204 is unproven, never yes", 204, "unproven"},
		{"502 is unproven", 502, "unproven"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := reachFixture(t)
			fixtureEnv(t, api.server.URL)
			var writes []string
			api.handler = func(w http.ResponseWriter, r *http.Request) bool {
				if r.Method != http.MethodGet {
					writes = append(writes, r.Method+" "+r.URL.Path)
					w.WriteHeader(tc.status)
					return true
				}
				return false
			}
			view := parseReach(t, runUniFiCLI(t, "reach", "--probe-writes", "--json"))
			got := view.Consoles[0].Transports["connector"]
			if got.Write != tc.want {
				t.Fatalf("write = %q (%s), want %q", got.Write, got.WriteWhy, tc.want)
			}
			if len(writes) != 1 || !strings.HasSuffix(writes[0], "/acl-rules/"+zeroUUID) {
				t.Fatalf("probe writes = %v", writes)
			}
		})
	}
}

func TestReachWriteYesOnlyFromVerifiedJournal(t *testing.T) {
	entries := []store.JournalEntry{
		{ID: "a", ConsoleID: "C1", Transport: "connector", State: journalFailed},
		{ID: "b", ConsoleID: "C2", Transport: "connector", State: journalVerified},
		{ID: "c", ConsoleID: "C1", Transport: "local", State: journalVerified},
	}
	if got, _ := writeStatusFromJournal(entries, "C1", "connector"); got != "" {
		t.Fatalf("got %q", got)
	}
	if got, _ := writeStatusFromJournal(entries, "C1", "local"); got != "yes" {
		t.Fatalf("got %q", got)
	}
}

func TestReachLocalGatewayRow(t *testing.T) {
	api := newFakeConsoleAPI(t)
	// The fake stands in for the cloud host too, so nothing here can reach the real
	// one, and the assertion below proves a gateway-only run does not try.
	fixtureEnv(t, api.server.URL)
	t.Setenv("UNIFI_API_KEY", "")
	t.Setenv("UNIFI_LOCAL_API_KEY", "fixture-local-key-bbbb")
	api.set("", "/proxy/network/integration/v1/info", map[string]any{"applicationVersion": "11.0"})
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pageOf())
		return true
	}
	run := runUniFiCLI(t, "reach", "--gateway", api.server.URL, "--json")
	view := parseReach(t, run)
	if len(view.Consoles) != 1 || !strings.HasPrefix(view.Consoles[0].ConsoleID, "gateway:") {
		t.Fatalf("view = %s", run.stdout)
	}
	local := view.Consoles[0].Transports["local"]
	if !local.Available || local.Surfaces["network"].Status != "ok" {
		t.Fatalf("local = %+v\n%s", local, run.stdout)
	}
	if view.Consoles[0].Transports["connector"].Skipped == "" {
		t.Fatalf("connector should be skipped: %+v", view.Consoles[0].Transports)
	}
	for _, r := range api.requested() {
		if strings.Contains(r, "/v1/hosts") {
			t.Fatalf("gateway-only reach asked the cloud: %s", r)
		}
	}
}

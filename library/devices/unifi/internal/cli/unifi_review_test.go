// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression tests from the Phase 4.95 security review. They use loopback fakes only:
// fixtureEnv points the base URL at httptest, so nothing here can reach api.ui.com.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

type keySpy struct {
	mu   sync.Mutex
	seen []string
	srv  *httptest.Server
}

func newKeySpy(t *testing.T) *keySpy {
	t.Helper()
	s := &keySpy{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.seen = append(s.seen, r.Method+" "+r.URL.Path+" key="+r.Header.Get("X-API-KEY"))
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *keySpy) requests() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.seen...)
}

// asMCPChild marks this process the way the MCP mirror marks its child, with the argv
// the child would see, since the confirm refusal has to look at the raw argument.
func asMCPChild(t *testing.T, argv ...string) {
	t.Helper()
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	saved := os.Args
	os.Args = append([]string{"unifi-pp-cli"}, argv...)
	t.Cleanup(func() { os.Args = saved })
}

// Finding: any mirrored tool took a gateway argument, so a caller could name the host
// that receives UNIFI_LOCAL_API_KEY, and could route around UNIFI_CONSOLE_ALLOWLIST.
func TestMCPChildCannotChooseTheRoute(t *testing.T) {
	cloud := newKeySpy(t)
	attacker := newKeySpy(t)
	fixtureEnv(t, cloud.srv.URL)
	t.Setenv("UNIFI_LOCAL_API_KEY", "fixture-local-key-bbbb")
	t.Setenv("UNIFI_CONSOLE_ID", "A")
	t.Setenv("UNIFI_CONSOLE_ALLOWLIST", "A")
	for _, extra := range [][]string{{"--gateway=" + attacker.srv.URL}, {"--gateway=" + attacker.srv.URL, "--transport=local"}, {"--transport=cloud"}} {
		args := append([]string{"raw", "GET", "network/x"}, extra...)
		asMCPChild(t, args...)
		run := runUniFiCLI(t, args...)
		if run.err == nil {
			t.Fatalf("%v accepted", extra)
		}
	}
	if got := attacker.requests(); len(got) != 0 {
		t.Fatalf("attacker host received: %v", got)
	}
	if got := cloud.requests(); len(got) != 0 {
		t.Fatalf("refused route still sent: %v", got)
	}
}

// An operator at a terminal keeps both flags. The refusal is about who is asking.
func TestOperatorKeepsGatewayFlag(t *testing.T) {
	gateway := newKeySpy(t)
	fixtureEnv(t, "http://127.0.0.1:9")
	t.Setenv("UNIFI_LOCAL_API_KEY", "fixture-local-key-bbbb")
	run := runUniFiCLI(t, "raw", "GET", "network/x", "--gateway", gateway.srv.URL, "--json")
	if run.err != nil {
		t.Fatal(run.err)
	}
	if got := gateway.requests(); len(got) != 1 || !strings.Contains(got[0], "key=fixture-local-key-bbbb") {
		t.Fatalf("gateway saw %v", got)
	}
}

// The MCP schema must not advertise the flags, which is what the walker checks before
// it spawns anything. Annotation names are spelled out because cli cannot see cobratree.
func TestRootMarksRouteFlagsAsNotForMCP(t *testing.T) {
	root := RootCmd()
	got := root.Annotations["mcp:read-flags"]
	for _, name := range []string{"gateway", "transport"} {
		if !strings.Contains(got, name) {
			t.Fatalf("root does not block %s from MCP: %q", name, got)
		}
	}
}

// Finding: the mirror renders confirm:"true" and confirm:1 as --confirm=true and
// --confirm=1, which pflag accepts, while only a boolean true renders as bare --confirm.
func TestMCPChildRefusesConfirmThatWasNotBooleanTrue(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"raw string", []string{"raw", "POST", "network/x", "--confirm=true"}},
		{"raw number", []string{"raw", "DELETE", "network/x", "--confirm=1"}},
		{"raw word", []string{"raw", "DELETE", "network/x", "--confirm=T"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cloud := newKeySpy(t)
			fixtureEnv(t, cloud.srv.URL)
			t.Setenv("UNIFI_CONSOLE_ID", "A")
			asMCPChild(t, tc.args...)
			if run := runUniFiCLI(t, tc.args...); run.err == nil {
				t.Fatalf("accepted, stdout=%s", run.stdout)
			}
			if got := cloud.requests(); len(got) != 0 {
				t.Fatalf("write sent without boolean confirm: %v", got)
			}
		})
	}
	t.Run("boolean true still works", func(t *testing.T) {
		cloud := newKeySpy(t)
		fixtureEnv(t, cloud.srv.URL)
		t.Setenv("UNIFI_CONSOLE_ID", "A")
		args := []string{"raw", "POST", "network/x", "--confirm"}
		asMCPChild(t, args...)
		if run := runUniFiCLI(t, args...); run.err != nil {
			t.Fatal(run.err)
		}
		if got := cloud.requests(); len(got) != 1 {
			t.Fatalf("got %v", got)
		}
	})
}

// Finding: change apply opened any path the server account can read, and --trial with
// --site echoed a JSON object file back in its preview.
func TestMCPChildCannotReadFilesThroughChangeApply(t *testing.T) {
	cloud := newKeySpy(t)
	fixtureEnv(t, cloud.srv.URL)
	t.Setenv("UNIFI_CONSOLE_ID", "A")
	file := filepath.Join(t.TempDir(), "creds.json")
	if err := os.WriteFile(file, []byte(`{"client_email":"victim@example.com"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"change", "apply", file, "--trial", "--site", "s", "--json"}
	asMCPChild(t, args...)
	run := runUniFiCLI(t, args...)
	if strings.Contains(run.stdout+run.stderr, "victim@example.com") {
		t.Fatalf("file contents reached the caller: %s", run.stdout)
	}
	if run.err == nil {
		t.Fatal("file path accepted from an MCP caller")
	}
}

// Inline JSON keeps the journaled path usable over MCP once files are refused.
func TestChangeJSONInlineWorksForMCPChild(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	encoded, _ := json.Marshal(patchSpec(collection + "/pol-1"))
	args := []string{"change", "apply", "--change-json", string(encoded), "--console", "C1", "--db", db, "--confirm", "--json"}
	asMCPChild(t, args...)
	result := parseChange(t, runUniFiCLI(t, args...))
	if result.State != journalVerified || obj.writeCount() != 1 {
		t.Fatalf("%+v writes=%d", result, obj.writeCount())
	}
}

// Finding: undo trusted whichever console the client now pointed at. A journaled delete
// on console C1 reads as 404 on C2, which passes the "should be gone" check, and the
// object was then recreated on the wrong console.
func TestUndoRefusesAnotherConsole(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	spec := map[string]any{"method": "DELETE", "path": collection + "/pol-1"}
	result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json"))
	if result.State != journalVerified {
		t.Fatalf("%+v", result)
	}
	writes := obj.writeCount()
	run := runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C2", "--db", db, "--confirm", "--json")
	if run.err == nil {
		t.Fatalf("undo crossed consoles: %s", run.stdout)
	}
	if obj.writeCount() != writes {
		t.Fatalf("undo wrote to the wrong console: %v", obj.writes)
	}
	if !strings.Contains(run.err.Error(), "C1") {
		t.Fatalf("refusal does not name the journaled console: %v", run.err)
	}
}

// Finding: an ambiguous 5xx whose values already matched was journaled verified, and
// reach then reported write_status yes from a write that may have been refused.
func TestAmbiguousWriteOverIdenticalStateIsNotVerified(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	obj.writeStatus, obj.applyOnFail = 502, false
	spec := map[string]any{"method": "PATCH", "path": collection + "/pol-1", "body": map[string]any{"enabled": true}}
	run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json")
	result := parseChange(t, run)
	if result.State != journalUnresolved || run.err == nil {
		t.Fatalf("state %q err %v (%s)", result.State, run.err, result.Detail)
	}
	if verdict, _ := writeStatusFromJournal([]store.JournalEntry{journalOf(t, db, result.JournalID)}, "C1", "connector"); verdict == "yes" {
		t.Fatal("reach would claim writes work")
	}
}

// A write refused before it left has nothing to read back, so unresolved is wrong.
func TestWriteThatNeverLeftIsFailedNotUnresolved(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	gateway := newKeySpy(t)
	spec := map[string]any{"method": "POST", "path": "/network/integration/v1/sites/s/firewall/policies", "body": map[string]any{"name": "x"}}
	run := runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--gateway", gateway.srv.URL, "--db", db, "--confirm", "--json")
	result := parseChange(t, run)
	if result.State != journalFailed || run.err == nil {
		t.Fatalf("state %q err %v", result.State, run.err)
	}
	if got := gateway.requests(); len(got) != 0 {
		t.Fatalf("sent: %v", got)
	}
}

// Finding: the client's delete hook removed the mirror row before the journal settled,
// breaking "the journal can never claim less than the mirror shows".
func TestJournalSettlesBeforeMirrorDelete(t *testing.T) {
	api, obj, _, collection := changeFixture(t)
	db := defaultDBPath("unifi-pp-cli")
	if err := os.MkdirAll(filepath.Dir(db), 0o700); err != nil {
		t.Fatal(err)
	}
	seed, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	store.SetActiveConsole("C1")
	item, _ := json.Marshal(map[string]any{"id": "pol-1", "parent_id": fixtureSiteID, "name": "base"})
	if _, _, err := seed.UpsertBatch(rtFirewall, []json.RawMessage{item}); err != nil {
		t.Fatal(err)
	}
	seed.Close()
	rowGoneDuringReadBack := false
	inner := api.handler
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		// The read-back follows the DELETE; the journal is still pending here.
		if r.Method == http.MethodGet && obj.writeCount() > 0 {
			probe, err := store.OpenWithContext(context.Background(), db)
			if err == nil {
				ids, _ := probe.ListIDs(rtFirewall)
				rowGoneDuringReadBack = len(ids) == 0
				probe.Close()
			}
		}
		return inner(w, r)
	}
	spec := map[string]any{"method": "DELETE", "path": collection + "/pol-1"}
	result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--confirm", "--json"))
	if result.State != journalVerified {
		t.Fatalf("%+v", result)
	}
	if rowGoneDuringReadBack {
		t.Fatal("mirror row was removed while the journal still said pending")
	}
	after, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer after.Close()
	if ids, _ := after.ListIDs(rtFirewall); len(ids) != 0 {
		t.Fatalf("mirror row survived a verified delete: %v", ids)
	}
}

// Validator bypass: "/../" missed a trailing "/..", so the path passed as official.
func TestChangeSpecRefusesDotSegments(t *testing.T) {
	for _, path := range []string{
		"/network/integration/v1/sites/s/firewall/policies/p/..",
		"/network/integration/v1/sites/s/./firewall",
		"/network/integration/v1/sites/s/firewall/policies/p/.",
		"/network/integration/v1/sites//firewall",
		"/network/integration/v1/sites/s/firewall/",
	} {
		if err := validateChangeSpec(changeSpec{Method: http.MethodDelete, Path: path}); err == nil {
			t.Fatalf("%s accepted", path)
		}
	}
}

func TestRawRefusesDeepEncodingAndPathParams(t *testing.T) {
	for _, path := range []string{
		"network/%25252525252e%25252525252e/x",
		"network/..;/x",
		"network/%0d%0aSet-Cookie:x/y",
	} {
		if _, _, err := normalizeRawPath(path); err == nil {
			t.Fatalf("%s accepted", path)
		}
	}
	if _, _, err := normalizeRawPath("network/api/s/default/stat/health"); err != nil {
		t.Fatal(err)
	}
}

// Previews land in transcripts and logs. change apply already scrubbed; raw did not.
func TestRawPreviewScrubsSecrets(t *testing.T) {
	cloud := newKeySpy(t)
	fixtureEnv(t, cloud.srv.URL)
	t.Setenv("UNIFI_CONSOLE_ID", "A")
	run := runUniFiCLI(t, "raw", "POST", "network/x", "--body", `{"passphrase":"hunter2","name":"guest"}`, "--json")
	if run.err != nil || strings.Contains(run.stdout, "hunter2") || !strings.Contains(run.stdout, "guest") {
		t.Fatalf("err=%v stdout=%s", run.err, run.stdout)
	}
}

// Finding: reach is annotated read-only and also sends a mutating verb on request, so
// hosts could auto-approve a tool that issues DELETE.
func TestReachIsNotAdvertisedReadOnly(t *testing.T) {
	cmd := newNovelReachCmd(&rootFlags{})
	if v := cmd.Annotations["mcp:read-only"]; v == "true" {
		t.Fatal("reach claims read-only while it can send a mutating verb")
	}
	if cmd.Annotations["mcp:read-only-unless-flags"] != "probe-writes,confirm" {
		t.Fatalf("annotations = %v", cmd.Annotations)
	}
}

// Finding: a secret-named parent holding an object was walked into, and the leaf name
// alone decided, so {"credentials":{"value":"x"}} printed x.
func TestDriftHidesValuesUnderSecretNamedObjects(t *testing.T) {
	before := map[string]any{"credentials": map[string]any{"value": "old-hunter2"}, "name": "a"}
	after := map[string]any{"credentials": map[string]any{"value": "new-hunter2"}, "name": "b"}
	encoded, _ := json.Marshal(diffFields("", before, after))
	if strings.Contains(string(encoded), "hunter2") {
		t.Fatalf("secret reached drift output: %s", encoded)
	}
	if !strings.Contains(string(encoded), "credentials") {
		t.Fatalf("the change itself should still be visible: %s", encoded)
	}
}

// rule-predict must not state a verdict from a policy it cannot actually evaluate.
func TestRulePredictRefusesToAssume(t *testing.T) {
	packet := predictPacket{SrcZone: "z1", DstZone: "z2", SrcIP: "192.0.2.1", DstIP: "192.0.2.2", DstPort: 443, Protocol: "tcp"}
	full := func(extra map[string]any) map[string]any {
		policy := map[string]any{"id": "p", "index": float64(1), "enabled": true, "action": map[string]any{"type": "ALLOW"},
			"source": map[string]any{"zoneId": "z1"}, "destination": map[string]any{"zoneId": "z2"}, "ipProtocolScope": map[string]any{"ipVersion": "IPV4_AND_IPV6"}}
		for k, v := range extra {
			if v == nil {
				delete(policy, k)
			} else {
				policy[k] = v
			}
		}
		return policy
	}
	if got := predictRule(packet, predictInputs{Policies: []map[string]any{full(nil)}}); got.Verdict != "match" {
		t.Fatalf("control case: %+v", got)
	}
	for name, policy := range map[string]map[string]any{
		"no source":      full(map[string]any{"source": nil}),
		"no destination": full(map[string]any{"destination": nil}),
		"no index":       full(map[string]any{"index": nil}),
	} {
		if got := predictRule(packet, predictInputs{Policies: []map[string]any{policy}}); got.Verdict != "indeterminate" || got.PredictedAction != "" {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	// An IPv6-only ALLOW says nothing about an IPv4 packet.
	v6 := full(map[string]any{"ipProtocolScope": map[string]any{"ipVersion": "IPV6"}})
	if got := predictRule(packet, predictInputs{Policies: []map[string]any{v6}}); got.MatchedPolicy != nil {
		t.Fatalf("IPv6 policy matched an IPv4 packet: %+v", got)
	}
	noIP := packet
	noIP.SrcIP, noIP.DstIP = "", ""
	if got := predictRule(noIP, predictInputs{Policies: []map[string]any{v6}}); got.Verdict != "indeterminate" || got.UndecidedPolicy == nil {
		t.Fatalf("unknown address family: %+v", got)
	}
	// matchOpposite on a preset inverts it; ignoring that predicted the reverse.
	opposite := full(map[string]any{"ipProtocolScope": map[string]any{"ipVersion": "IPV4_AND_IPV6", "protocolFilter": map[string]any{"type": "PRESET", "matchOpposite": true, "preset": map[string]any{"name": "TCP_UDP"}}}})
	if got := predictRule(packet, predictInputs{Policies: []map[string]any{opposite}}); got.MatchedPolicy != nil {
		t.Fatalf("matchOpposite ignored on preset: %+v", got)
	}
	// Same for an inverted MAC filter.
	mac := packet
	mac.SrcMAC = "02:00:00:00:00:01"
	macPolicy := full(map[string]any{"source": map[string]any{"zoneId": "z1", "trafficFilter": map[string]any{"type": "MAC_ADDRESS", "macAddressFilter": map[string]any{"matchOpposite": true, "macAddresses": []any{"02:00:00:00:00:01"}}}}})
	if got := predictRule(mac, predictInputs{Policies: []map[string]any{macPolicy}}); got.MatchedPolicy != nil {
		t.Fatalf("matchOpposite ignored on MAC filter: %+v", got)
	}
}

// With no client rows at all, "nothing depends on it" is ignorance, not a finding.
func TestImpactDoesNotCallUnknownClientsLowRisk(t *testing.T) {
	device := mirrorRow{ID: "sw1", Data: map[string]any{"name": "sw", deviceHydratedField: true, "uplink": map[string]any{"deviceId": "gw"}}}
	gateway := mirrorRow{ID: "gw", Data: map[string]any{"name": "gw", deviceHydratedField: true}}
	wifi := mirrorRow{ID: "w1", Parent: "s", Data: map[string]any{"name": "guest"}}
	data := impactData{Devices: []mirrorRow{device, gateway}, Wifi: []mirrorRow{wifi}}
	if got := assessImpact("device", "restart", "sw1", "", data); got.Verdict != verdictCannotDetermine {
		t.Fatalf("device: %+v", got)
	}
	if got := assessImpact("wifi", "disable", "w1", "", data); got.Verdict != verdictCannotDetermine {
		t.Fatalf("wifi: %+v", got)
	}
	data.Clients = []mirrorRow{{ID: "c1", Parent: "other", Data: map[string]any{"uplinkDeviceId": "gw", "type": "WIRED"}}}
	if got := assessImpact("device", "restart", "sw1", "", data); got.Verdict != verdictNoKnownRisk {
		t.Fatalf("with client data: %+v", got)
	}
}

// fetchAllPages used to return a partial list as if complete when pagination misbehaved,
// so fleet would count a truncated inventory as the whole fleet.
func TestPaginationThatDoesNotEndIsAnError(t *testing.T) {
	for name, next := range map[string]func(int) string{
		"repeats a token": func(int) string { return "same" },
		"never ends":      func(i int) string { return "t" + strings.Repeat("x", i) },
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			api := newFakeConsoleAPI(t)
			fixtureEnv(t, api.server.URL)
			api.handler = func(w http.ResponseWriter, r *http.Request) bool {
				calls++
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "h"}}, "nextToken": next(calls)})
				return true
			}
			flags := rootFlags{timeout: 5 * time.Second}
			c, err := flags.newClient()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := fetchAllPages(context.Background(), c, "/v1/hosts", parseFleetHosts); err == nil {
				t.Fatal("incomplete pagination reported as complete")
			}
			if calls > 50 {
				t.Fatalf("pagination did not stop: %d calls", calls)
			}
		})
	}
}

// A device list that could not be fetched must not read as zero devices on each row.
func TestFleetRowsSayWhenDevicesAreUnknown(t *testing.T) {
	view := buildFleetView("health", []fleetHost{{id: "h1"}}, nil, false, nil, nil, 0)
	if view.Consoles[0].DevicesKnown {
		t.Fatalf("%+v", view.Consoles[0])
	}
	view = buildFleetView("health", []fleetHost{{id: "h1"}}, nil, true, nil, nil, 0)
	if !view.Consoles[0].DevicesKnown {
		t.Fatalf("%+v", view.Consoles[0])
	}
}

// Identity must not depend on how the gateway was spelled, or reach cannot find the
// journal entries that prove a local write worked.
func TestReachFindsLocalJournalHoweverGatewayIsSpelled(t *testing.T) {
	gateway := newKeySpy(t)
	db := fixtureEnv(t, "http://127.0.0.1:9")
	_ = db
	t.Setenv("UNIFI_LOCAL_API_KEY", "fixture-local-key-bbbb")
	key := "gateway:" + strings.TrimPrefix(gateway.srv.URL, "http://")
	path := defaultDBPath("unifi-pp-cli")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	seed, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := seed.InsertJournalEntry(context.Background(), store.JournalEntry{ID: "chg_1", ConsoleID: key, CreatedAt: now, UpdatedAt: now, State: journalVerified, Kind: "update", Method: "PATCH", Path: "/x", Transport: "local"}); err != nil {
		t.Fatal(err)
	}
	seed.Close()
	run := runUniFiCLI(t, "reach", "--gateway", gateway.srv.URL, "--json")
	if run.err != nil {
		t.Fatal(run.err)
	}
	var view reachView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Consoles) != 1 || view.Consoles[0].ConsoleID != key || view.Consoles[0].Transports["local"].Write != "yes" {
		t.Fatalf("%s", run.stdout)
	}
}

// --transport cloud means the gateway is not the route, so it cannot be the mirror scope.
func TestCloudTransportDoesNotScopeMirrorToGateway(t *testing.T) {
	fixtureEnv(t, "http://127.0.0.1:9")
	t.Setenv("UNIFI_GATEWAY", "192.0.2.1")
	t.Setenv("UNIFI_TRANSPORT", "cloud")
	store.SetActiveConsole("")
	scopeMirrorToConfiguredConsole(&rootFlags{})
	if got := store.ActiveConsole(); got != "" {
		t.Fatalf("scope = %q", got)
	}
}

// Audit lead: a local-only operator was told they had no credentials at all.
func TestAuthStatusAndDoctorAcceptLocalOnlySetup(t *testing.T) {
	fake := newKeySpy(t)
	fixtureEnv(t, fake.srv.URL)
	t.Setenv("UNIFI_API_KEY", "")
	t.Setenv("UNIFI_LOCAL_API_KEY", "fixture-local-key-bbbb")
	t.Setenv("UNIFI_GATEWAY", "192.0.2.1")
	run := runUniFiCLI(t, "auth", "status", "--json")
	if run.err != nil {
		t.Fatalf("auth status: exit %d %v\n%s", ExitCode(run.err), run.err, run.stdout)
	}
	var status map[string]any
	if err := json.Unmarshal([]byte(run.stdout), &status); err != nil {
		t.Fatal(err)
	}
	if status["authenticated"] != true || status["cloud_key"] != false || status["local_key"] != true {
		t.Fatalf("%s", run.stdout)
	}
	if strings.Contains(run.stdout, "fixture-local-key") {
		t.Fatal("status printed the key")
	}
	run = runUniFiCLI(t, "doctor", "--json")
	var report map[string]any
	if err := json.Unmarshal([]byte(run.stdout), &report); err != nil {
		t.Fatalf("%v\n%s", err, run.stdout)
	}
	if env, _ := report["env_vars"].(string); strings.Contains(env, "missing") {
		t.Fatalf("env_vars = %q", env)
	}
	if auth, _ := report["auth"].(string); auth != "configured" {
		t.Fatalf("auth = %q", auth)
	}
	if strings.Contains(run.stdout, "fixture-local-key") {
		t.Fatal("doctor printed the key")
	}
}

// With neither key the old answer stands, including its exit code.
func TestAuthStatusWithNoKeysStillFails(t *testing.T) {
	fake := newKeySpy(t)
	fixtureEnv(t, fake.srv.URL)
	t.Setenv("UNIFI_API_KEY", "")
	if run := runUniFiCLI(t, "auth", "status", "--json"); run.err == nil || ExitCode(run.err) != 4 {
		t.Fatalf("exit %d %v", ExitCode(run.err), run.err)
	}
}

// Audit lead: "reachable (HTTP 404 at /)" read as healthy, but a 404 at the root proves
// only that something answered.
func TestDoctorDoesNotCallA404Reachable(t *testing.T) {
	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer notFound.Close()
	fixtureEnv(t, notFound.URL)
	run := runUniFiCLI(t, "doctor", "--json")
	var report map[string]any
	if err := json.Unmarshal([]byte(run.stdout), &report); err != nil {
		t.Fatalf("%v\n%s", err, run.stdout)
	}
	api, _ := report["api"].(string)
	if strings.Contains(api, "reachable") || !strings.Contains(api, "404") || !strings.Contains(api, "not verified") {
		t.Fatalf("api = %q", api)
	}
}

// Round 2: under a gateway route --console is only a label, so a caller could relabel
// the operator's gateway as another console and pass undo's console check.
func TestUndoRefusesAnotherRouteAndMCPCannotRelabelGateway(t *testing.T) {
	_, obj, db, collection := changeFixture(t)
	// Answers 404 to the precheck: the case where a deleted object looks equally gone on
	// the wrong box, so only the route comparison can stop the recreate.
	var gatewayWrites []string
	var gatewayMu sync.Mutex
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			gatewayMu.Lock()
			gatewayWrites = append(gatewayWrites, r.Method+" "+r.URL.Path)
			gatewayMu.Unlock()
		}
		http.Error(w, "{}", http.StatusNotFound)
	}))
	defer gatewayServer.Close()
	spec := map[string]any{"method": "DELETE", "path": collection + "/pol-1"}
	result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json"))
	if result.State != journalVerified {
		t.Fatalf("%+v", result)
	}
	t.Setenv("UNIFI_LOCAL_API_KEY", "fixture-local-key-bbbb")
	writes := obj.writeCount()
	if run := runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--gateway", gatewayServer.URL, "--db", db, "--confirm", "--json"); run.err == nil {
		t.Fatalf("undo crossed routes: %s", run.stdout)
	}
	gatewayMu.Lock()
	defer gatewayMu.Unlock()
	if len(gatewayWrites) != 0 {
		t.Fatalf("gateway received a write: %v", gatewayWrites)
	}
	if obj.writeCount() != writes {
		t.Fatalf("wrote: %v", obj.writes)
	}
	t.Setenv("UNIFI_GATEWAY", gatewayServer.URL)
	t.Setenv("UNIFI_CONSOLE_ID", "OPERATOR")
	args := []string{"raw", "GET", "network/x", "--console=OTHER"}
	asMCPChild(t, args...)
	if run := runUniFiCLI(t, args...); run.err == nil {
		t.Fatal("MCP caller relabeled the operator's gateway")
	}
}

// Round 2: a deadline during the write also killed the read-back and the journal update,
// so the entry stayed pending and the promised read-back never ran.
func TestTimedOutWriteStillSettlesTheJournal(t *testing.T) {
	api, obj, db, collection := changeFixture(t)
	t.Setenv("UNIFI_CONSOLE_ID", "C1")
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	flags := rootFlags{timeout: 5 * time.Second}
	c, err := flags.newClient()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inner := api.handler
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		handled := inner(w, r)
		if r.Method != http.MethodGet {
			// The write landed, and the caller's deadline ends before the answer arrives.
			cancel()
			time.Sleep(100 * time.Millisecond)
		}
		return handled
	}
	result, _ := newChangeRunner(c, sdb, os.Stderr).applyChange(ctx, changeSpec{Method: http.MethodPatch, Path: collection + "/pol-1", Body: []byte(`{"enabled":false}`)}, false, nil)
	if result.JournalID == "" {
		t.Fatalf("no journal entry: %+v", result)
	}
	entry, err := sdb.GetJournalEntry(context.Background(), result.JournalID)
	if err != nil {
		t.Fatal(err)
	}
	if entry.State != journalVerified || obj.writeCount() != 1 {
		t.Fatalf("state %q (%s), writes %d: read-back should have proved the write", entry.State, entry.Detail, obj.writeCount())
	}
}

// Round 2 regression: clearing the delete hook must not leave rows the hook used to remove.
func TestVerifiedDeletesCleanEveryMirroredType(t *testing.T) {
	for _, tc := range []struct{ rt, sub string }{
		{rtDevices, "devices/d-1"},
		{"network_integration_v1_sites_hotspot_vouchers", "hotspot/vouchers/d-1"},
		{rtACL, "acl-rules/d-1"},
	} {
		for _, ambiguous := range []bool{false, true} {
			api := newFakeConsoleAPI(t)
			db := fixtureEnv(t, api.server.URL)
			path := "/network/integration/v1/sites/" + fixtureSiteID + "/" + tc.sub
			obj := &statefulObject{collection: path[:strings.LastIndex(path, "/")], path: path, obj: map[string]any{"id": "d-1", "name": "n"}}
			if ambiguous {
				obj.writeStatus, obj.applyOnFail = 502, true
			}
			api.handler = obj.handle
			store.SetActiveConsole("C1")
			seed, err := store.OpenWithContext(context.Background(), db)
			if err != nil {
				t.Fatal(err)
			}
			item, _ := json.Marshal(map[string]any{"id": "d-1", "parent_id": fixtureSiteID, "name": "n"})
			if _, _, err := seed.UpsertBatch(tc.rt, []json.RawMessage{item}); err != nil {
				t.Fatal(err)
			}
			seed.Close()
			spec := map[string]any{"method": "DELETE", "path": path}
			result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json"))
			if result.State != journalVerified {
				t.Fatalf("%s ambiguous=%v: %+v", tc.sub, ambiguous, result)
			}
			check, err := store.OpenWithContext(context.Background(), db)
			if err != nil {
				t.Fatal(err)
			}
			ids, _ := check.ListIDs(tc.rt)
			check.Close()
			if len(ids) != 0 {
				t.Fatalf("%s ambiguous=%v: row survived: %q", tc.sub, ambiguous, ids)
			}
		}
	}
}

// Round 2: an IPv4-mapped address is the same host, so it must meet the same policy.
func TestRulePredictTreatsMappedIPv4AsIPv4(t *testing.T) {
	block := map[string]any{"id": "b", "index": float64(1), "enabled": true, "action": map[string]any{"type": "BLOCK"},
		"source":          map[string]any{"zoneId": "z1", "trafficFilter": map[string]any{"type": "IP_ADDRESS", "ipAddressFilter": map[string]any{"type": "IP_ADDRESSES", "items": []any{map[string]any{"type": "SUBNET", "value": "10.0.0.0/8"}}}}},
		"destination":     map[string]any{"zoneId": "z2"},
		"ipProtocolScope": map[string]any{"ipVersion": "IPV4"}}
	packet := predictPacket{SrcZone: "z1", DstZone: "z2", SrcIP: "::ffff:10.0.0.5", DstIP: "192.0.2.1"}
	if got := predictRule(packet, predictInputs{Policies: []map[string]any{block}}); got.PredictedAction != "BLOCK" {
		t.Fatalf("%+v", got)
	}
	acl := map[string]any{"id": "a", "enabled": true, "action": "BLOCK", "type": "IPV4", "sourceFilter": map[string]any{"type": "IP_ADDRESSES_OR_SUBNETS", "ipAddressesOrSubnets": []any{"10.0.0.0/8"}}}
	if got := evalACLRules([]map[string]any{acl}, packet, predictInputs{}); len(got) != 1 {
		t.Fatalf("ACL missed a mapped address: %v", got)
	}
}

// Round 2: local readers took --console with no fence check, so drift could show a
// console the operator had fenced off.
func TestAllowlistRefusesNamedConsoleForLocalCommands(t *testing.T) {
	cloud := newKeySpy(t)
	db := fixtureEnv(t, cloud.srv.URL)
	t.Setenv("UNIFI_CONSOLE_ALLOWLIST", "A")
	if run := runUniFiCLI(t, "drift", "--console", "B", "--db", db, "--json"); run.err == nil {
		t.Fatalf("fenced console accepted: %s", run.stdout)
	}
	if run := runUniFiCLI(t, "drift", "--console", "A", "--db", db, "--json"); run.err != nil {
		t.Fatalf("allowed console refused: %v", run.err)
	}
}

// Round 3: the follow-up budget keeps read-back alive after cancellation, but a person who
// cancelled did not agree to a new write, so auto-revert must not send one.
func TestAutoRevertSendsNoWriteAfterCancel(t *testing.T) {
	api, obj, db, collection := changeFixture(t)
	t.Setenv("UNIFI_CONSOLE_ID", "C1")
	obj.mangle = true
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	flags := rootFlags{timeout: 5 * time.Second}
	c, err := flags.newClient()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inner := api.handler
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		// Cancelled once the write has answered, while its read-back is under way: that
		// is the point at which auto-revert would decide to send a second write.
		if r.Method == http.MethodGet && obj.writeCount() > 0 {
			cancel()
		}
		return inner(w, r)
	}
	result, runErr := newChangeRunner(c, sdb, os.Stderr).applyChange(ctx, changeSpec{Method: http.MethodPatch, Path: collection + "/pol-1", Body: []byte(`{"enabled":false}`)}, true, nil)
	if obj.writeCount() != 1 {
		t.Fatalf("a write went out after cancel: %v", obj.writes)
	}
	if result.State != journalUnresolved || runErr == nil {
		t.Fatalf("state %q err %v", result.State, runErr)
	}
}

// Round 3: a config that failed to parse used to discard --console, which left the mirror
// unscoped, so local readers would show every console including fenced ones.
func TestConsoleFlagScopesMirrorEvenWithBrokenConfig(t *testing.T) {
	fixtureEnv(t, "http://127.0.0.1:9")
	bad := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(bad, []byte("not [ valid toml"), 0o600); err != nil {
		t.Fatal(err)
	}
	unifiTarget.ConsoleID = "A"
	store.SetActiveConsole("")
	scopeMirrorToConfiguredConsole(&rootFlags{configPath: bad})
	if got := store.ActiveConsole(); got != "A" {
		t.Fatalf("scope = %q", got)
	}
}

// Round 3: unmapping only the packet broke prefixes written in the mapped form.
func TestRulePredictMappedPrefix(t *testing.T) {
	items := []any{map[string]any{"type": "SUBNET", "value": "::ffff:10.0.0.0/104"}}
	for _, ip := range []string{"10.0.0.5", "::ffff:10.0.0.5"} {
		if got := matchesIPItems(items, ip); got != triYes {
			t.Fatalf("%s: %v", ip, got)
		}
	}
	filter := map[string]any{"type": "IP_ADDRESSES_OR_SUBNETS", "ipAddressesOrSubnets": []any{"::ffff:10.0.0.0/104"}}
	if got := evalACLEndpoint(filter, "10.0.0.5", 0); got != triYes {
		t.Fatalf("ACL: %v", got)
	}
}

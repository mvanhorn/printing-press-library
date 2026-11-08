// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the user's decisions on mirror secrets, console pinning and write probes,
// and for the output review. Loopback fakes only: fixtureEnv points at httptest.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
)

const (
	fixturePSK       = "plaintext-psk-Zq7vK2"
	fixturePerNetPSK = "plaintext-ppsk-Lm3xR9"
	fixtureVoucher   = "plaintext-voucher-Tn4bW8"
	fixtureRTSPToken = "plaintext-rtsp-Yc6sH1"
)

func wifiBroadcast(psk string) map[string]any {
	return map[string]any{"id": "wifi-1", "name": "guest", "enabled": true, "securityConfiguration": map[string]any{
		"type": "WPA2_PERSONAL", "passphrase": psk,
		"presharedKeys": []any{map[string]any{"network": map[string]any{"id": "n1"}, "passphrase": fixturePerNetPSK}},
	}}
}

func seedSecrets(api *fakeConsoleAPI, console, psk string) {
	seedConsole(api, console)
	base := "/network/integration/v1/sites/" + fixtureSiteID
	api.set(console, base+"/wifi/broadcasts", pageOf(wifiBroadcast(psk)))
	api.set(console, base+"/wifi/broadcasts/wifi-1", wifiBroadcast(psk))
	api.set(console, base+"/hotspot/vouchers", pageOf(map[string]any{"id": "v1", "name": "lobby", "code": fixtureVoucher, "authorizedGuestLimit": 1, "timeLimitMinutes": 60}))
}

// assertNoPlaintext reads every file under the isolated home as bytes, which catches the
// WAL, FTS shadow tables, response cache, audit log and journal without naming each one.
func assertNoPlaintext(t *testing.T, home string, secrets ...string) {
	t.Helper()
	checked := 0
	err := filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		for _, secret := range secrets {
			if bytes.Contains(data, []byte(secret)) {
				t.Errorf("%s holds %q", path, secret)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no files were checked, so this proves nothing")
	}
}

// assertNoPlaintextInTables reads each column of each table through SQL, including
// virtual FTS tables, in case a value is stored in a form the byte scan would miss.
func assertNoPlaintextInTables(t *testing.T, dbPath string, secrets ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	rows.Close()
	if len(tables) < 5 {
		t.Fatalf("only %d tables: %v", len(tables), tables)
	}
	for _, table := range tables {
		content, err := db.Query(fmt.Sprintf(`SELECT * FROM %q`, table))
		if err != nil {
			continue // shadow tables can refuse a direct read; the byte scan covers them
		}
		cols, _ := content.Columns()
		for content.Next() {
			values := make([]any, len(cols))
			holders := make([]any, len(cols))
			for i := range values {
				holders[i] = &values[i]
			}
			if err := content.Scan(holders...); err != nil {
				t.Fatal(err)
			}
			for i, value := range values {
				text := fmt.Sprintf("%s", value)
				for _, secret := range secrets {
					if strings.Contains(text, secret) {
						t.Errorf("%s.%s holds %q", table, cols[i], secret)
					}
				}
			}
		}
		content.Close()
	}
}

// The user's proof: a passphrase seeded through the real sync path is gone from every
// table and from the raw bytes of every file.
func TestSyncedSecretsNeverReachDisk(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	home := filepath.Dir(db)
	seedSecrets(api, "C1", fixturePSK)
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatalf("sync: %v\n%s", run.err, run.stderr)
	}
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := sdb.ListIDs(rtWifi)
	sdb.Close()
	if err != nil || len(ids) != 1 {
		t.Fatalf("the seeded row did not sync, so the scan proves nothing: %v %v", ids, err)
	}
	assertNoPlaintext(t, home, fixturePSK, fixturePerNetPSK, fixtureVoucher)
	assertNoPlaintextInTables(t, db, fixturePSK, fixturePerNetPSK, fixtureVoucher)
	run := runUniFiCLI(t, "search", fixturePSK, "--db", db, "--data-source", "local", "--json")
	if strings.Contains(run.stdout, "wifi-1") {
		t.Fatalf("search matched secret text: %s", run.stdout)
	}
}

// drift must still notice a rotated passphrase, or sealing would blind it, and must not
// show the value or the digest.
func TestDriftReportsSecretChangeWithoutValueOrDigest(t *testing.T) {
	// Two syncs with --no-hydrate, because more exceed the 100 per minute connector budget
	// and the throttle would then correctly make this test wait a minute. That an unchanged
	// secret does not drift is covered in the store, by TestSealingIsStableAcrossSyncs.
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedSecrets(api, "C1", fixturePSK)
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json", "--no-hydrate"); run.err != nil {
		t.Fatal(run.err)
	}
	seedSecrets(api, "C1", "rotated-psk-Pw9dF5")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json", "--no-hydrate"); run.err != nil {
		t.Fatal(run.err)
	}
	run := runUniFiCLI(t, "drift", "--db", db, "--console", "C1", "--json")
	var view driftView
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatalf("%v\n%s", err, run.stdout)
	}
	found := false
	for _, change := range view.Changes {
		for _, field := range change.Fields {
			if strings.Contains(field.Path, "passphrase") && field.SecretChanged {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("rotation not reported as a secret change: %s", run.stdout)
	}
	for _, leak := range []string{fixturePSK, "rotated-psk", "[secret:"} {
		if strings.Contains(run.stdout, leak) {
			t.Fatalf("drift shows %q: %s", leak, run.stdout)
		}
	}
	human := runUniFiCLI(t, "drift", "--db", db, "--console", "C1", "--human-friendly")
	if strings.Contains(human.stdout, "[secret:") {
		t.Fatalf("terminal drift shows a digest: %s", human.stdout)
	}
}

// Sending a digest as a passphrase would change the WiFi key to a string nobody knows and
// lock every client out, so a placeholder must never leave the client.
func TestStoredPlaceholdersCannotBeSentBack(t *testing.T) {
	for _, placeholder := range []string{"[secret:v1:0123456789abcdef]", "[scrubbed]"} {
		cloud := newKeySpy(t)
		fixtureEnv(t, cloud.srv.URL)
		body := fmt.Sprintf(`{"securityConfiguration":{"passphrase":%q}}`, placeholder)
		run := runUniFiCLI(t, "raw", "PUT", "network/integration/v1/sites/s/wifi/broadcasts/w", "--console", "C1", "--body", body, "--confirm", "--json")
		if run.err == nil {
			t.Fatalf("%s accepted", placeholder)
		}
		if got := cloud.requests(); len(got) != 0 {
			t.Fatalf("%s sent: %v", placeholder, got)
		}
	}
}

// Undo already refuses a before-image with secrets. Confirm the journal and refreshed
// mirror stay clean through a real write, and that undo still refuses.
func TestJournaledWifiChangeKeepsSecretsOutAndUndoRefuses(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	home := filepath.Dir(db)
	collection := "/network/integration/v1/sites/" + fixtureSiteID + "/wifi/broadcasts"
	obj := &statefulObject{collection: collection, path: collection + "/wifi-1", obj: wifiBroadcast(fixturePSK)}
	api.handler = obj.handle
	spec := map[string]any{"method": "PATCH", "path": collection + "/wifi-1", "body": map[string]any{"enabled": false}}
	result := parseChange(t, runUniFiCLI(t, "change", "apply", writeChangeFile(t, spec), "--console", "C1", "--db", db, "--confirm", "--json"))
	if result.State != journalVerified || result.Undoable {
		t.Fatalf("%+v", result)
	}
	assertNoPlaintext(t, home, fixturePSK, fixturePerNetPSK)
	assertNoPlaintextInTables(t, db, fixturePSK, fixturePerNetPSK)
	writes := obj.writeCount()
	run := runUniFiCLI(t, "change", "undo", result.JournalID, "--console", "C1", "--db", db, "--confirm", "--json")
	if run.err == nil || obj.writeCount() != writes {
		t.Fatalf("undo sent a scrubbed before-image: %v %v", run.err, obj.writes)
	}
}

// The API may echo a rejected value. The audit log outlives the session.
func TestAuditErrorDropsEchoedSecret(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	api.handler = func(w http.ResponseWriter, r *http.Request) bool {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"passphrase ` + fixturePSK + ` is too short"}`))
		return true
	}
	body := fmt.Sprintf(`{"passphrase":%q}`, fixturePSK)
	if run := runUniFiCLI(t, "raw", "POST", "network/x", "--console", "C1", "--body", body, "--confirm", "--json"); run.err == nil {
		t.Fatal("expected the fake's refusal")
	}
	assertNoPlaintext(t, filepath.Dir(db), fixturePSK)
}

// Decision 2: a pinned MCP server stays on its console unless the allowlist opts in.
func TestMCPChildPinnedToConfiguredConsole(t *testing.T) {
	cases := []struct {
		name, allowlist string
		wantSent        bool
	}{
		{"other console refused", "", false},
		{"allowlist without it refused", "A,C", false},
		{"allowlist opts in", "A,B", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cloud := newKeySpy(t)
			fixtureEnv(t, cloud.srv.URL)
			t.Setenv("UNIFI_CONSOLE_ID", "A")
			t.Setenv("UNIFI_CONSOLE_ALLOWLIST", tc.allowlist)
			args := []string{"raw", "GET", "network/x", "--console=B"}
			asMCPChild(t, args...)
			run := runUniFiCLI(t, args...)
			got := cloud.requests()
			if tc.wantSent {
				if run.err != nil || len(got) != 1 || !strings.Contains(got[0], "/consoles/B/") {
					t.Fatalf("err=%v sent=%v", run.err, got)
				}
				return
			}
			if run.err == nil || len(got) != 0 {
				t.Fatalf("err=%v sent=%v", run.err, got)
			}
			if !strings.Contains(run.err.Error(), "UNIFI_CONSOLE_ID") || !strings.Contains(run.err.Error(), "UNIFI_CONSOLE_ALLOWLIST") {
				t.Fatalf("error does not say how to opt in: %v", run.err)
			}
		})
	}
	t.Run("the pinned console itself works", func(t *testing.T) {
		cloud := newKeySpy(t)
		fixtureEnv(t, cloud.srv.URL)
		t.Setenv("UNIFI_CONSOLE_ID", "A")
		args := []string{"raw", "GET", "network/x", "--console=A"}
		asMCPChild(t, args...)
		if run := runUniFiCLI(t, args...); run.err != nil {
			t.Fatal(run.err)
		}
	})
	t.Run("an operator keeps --console", func(t *testing.T) {
		cloud := newKeySpy(t)
		fixtureEnv(t, cloud.srv.URL)
		t.Setenv("UNIFI_CONSOLE_ID", "A")
		if run := runUniFiCLI(t, "raw", "GET", "network/x", "--console", "B"); run.err != nil {
			t.Fatal(run.err)
		}
		if got := cloud.requests(); len(got) != 1 || !strings.Contains(got[0], "/consoles/B/") {
			t.Fatalf("%v", got)
		}
	})
}

// A pinned fleet or reach must not probe other consoles through the mirror either.
func TestPinnedFleetProbesOnlyThePinnedConsole(t *testing.T) {
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	t.Setenv("UNIFI_CONSOLE_ID", "A")
	api.set("", "/v1/hosts", map[string]any{"data": []any{map[string]any{"id": "A"}, map[string]any{"id": "B"}}})
	args := []string{"fleet", "health", "--json"}
	asMCPChild(t, args...)
	if run := runUniFiCLI(t, args...); run.err != nil {
		t.Fatal(run.err)
	}
	for _, request := range api.requested() {
		if strings.Contains(request, "/consoles/B/") {
			t.Fatalf("probed another console: %v", api.requested())
		}
	}
}

func probeFixture(t *testing.T) *fakeConsoleAPI {
	t.Helper()
	api := newFakeConsoleAPI(t)
	fixtureEnv(t, api.server.URL)
	api.set("", "/v1/hosts", map[string]any{"data": []any{map[string]any{"id": "A"}}})
	api.set("A", "/network/integration/v1/sites", pageOf(map[string]any{"id": fixtureSiteID, "name": "Default"}))
	return api
}

func probesSent(api *fakeConsoleAPI) int {
	sent := 0
	for _, request := range api.requested() {
		if strings.HasPrefix(request, "DELETE ") {
			sent++
		}
	}
	return sent
}

// Decision 3: over MCP the probe needs a real confirm; at a terminal it stays as it was.
func TestReachProbeWritesNeedsConfirmOverMCP(t *testing.T) {
	t.Run("MCP without confirm sends nothing", func(t *testing.T) {
		api := probeFixture(t)
		args := []string{"reach", "--console", "A", "--probe-writes", "--json"}
		asMCPChild(t, args...)
		run := runUniFiCLI(t, args...)
		if run.err == nil || !strings.Contains(run.err.Error(), "confirm") {
			t.Fatalf("err = %v", run.err)
		}
		if len(api.requested()) != 0 {
			t.Fatalf("sent: %v", api.requested())
		}
	})
	t.Run("MCP with a truthy string is refused", func(t *testing.T) {
		api := probeFixture(t)
		args := []string{"reach", "--console", "A", "--probe-writes", "--confirm=true", "--json"}
		asMCPChild(t, args...)
		if run := runUniFiCLI(t, args...); run.err == nil {
			t.Fatal("accepted")
		}
		if probesSent(api) != 0 {
			t.Fatalf("sent: %v", api.requested())
		}
	})
	t.Run("MCP with boolean confirm probes", func(t *testing.T) {
		api := probeFixture(t)
		args := []string{"reach", "--console", "A", "--probe-writes", "--confirm", "--json"}
		asMCPChild(t, args...)
		if run := runUniFiCLI(t, args...); run.err != nil {
			t.Fatal(run.err)
		}
		if probesSent(api) != 1 {
			t.Fatalf("sent: %v", api.requested())
		}
	})
	t.Run("MCP without the probe needs nothing", func(t *testing.T) {
		api := probeFixture(t)
		args := []string{"reach", "--console", "A", "--json"}
		asMCPChild(t, args...)
		if run := runUniFiCLI(t, args...); run.err != nil {
			t.Fatal(run.err)
		}
		if probesSent(api) != 0 {
			t.Fatalf("sent: %v", api.requested())
		}
	})
	t.Run("the CLI needs no confirm", func(t *testing.T) {
		api := probeFixture(t)
		if run := runUniFiCLI(t, "reach", "--console", "A", "--probe-writes", "--json"); run.err != nil {
			t.Fatal(run.err)
		}
		if probesSent(api) != 1 {
			t.Fatalf("sent: %v", api.requested())
		}
	})
}

func emptyStore(t *testing.T, db string) {
	t.Helper()
	sdb, err := store.OpenWithContext(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	sdb.Close()
}

// Output review: on a store that has never synced, silence reads as an all clear.
func TestNeverSyncedStoreSaysSoNotAllClear(t *testing.T) {
	for _, withDB := range []bool{false, true} {
		for _, command := range []string{"digest", "newcomer"} {
			fake := newKeySpy(t)
			db := fixtureEnv(t, fake.srv.URL)
			if withDB {
				emptyStore(t, db)
			}
			agent := runUniFiCLI(t, command, "--db", db, "--json")
			var view map[string]any
			if err := json.Unmarshal([]byte(agent.stdout), &view); err != nil {
				t.Fatalf("%s: %v\n%s", command, err, agent.stdout)
			}
			note, _ := view["note"].(string)
			if !strings.Contains(note, "sync") {
				t.Fatalf("%s withDB=%v JSON note = %q", command, withDB, note)
			}
			human := runUniFiCLI(t, command, "--db", db, "--human-friendly")
			if !strings.Contains(human.stdout, "sync") || strings.Contains(human.stdout, "Nothing recorded") || strings.Contains(human.stdout, "No newcomers") {
				t.Fatalf("%s withDB=%v terminal = %q", command, withDB, human.stdout)
			}
		}
	}
}

// Once a sync has run, "nothing happened" is a real answer again.
func TestSyncedStoreKeepsItsAllClear(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	for i := 0; i < 2; i++ {
		if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
			t.Fatal(run.err)
		}
	}
	run := runUniFiCLI(t, "newcomer", "--db", db, "--console", "C1", "--json")
	var view map[string]any
	if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
		t.Fatal(err)
	}
	if note, _ := view["note"].(string); note != "" {
		t.Fatalf("note after two syncs: %q", note)
	}
}

func TestTopologyHintsSyncWheneverNoDevicesAreMirrored(t *testing.T) {
	fake := newKeySpy(t)
	db := fixtureEnv(t, fake.srv.URL)
	emptyStore(t, db)
	run := runUniFiCLI(t, "topology", "--db", db, "--agent")
	if !strings.Contains(run.stdout, "sync") {
		t.Fatalf("no sync hint: %s", run.stdout)
	}
}

// Output review: 168h0m0s is Go's notation, not anybody's.
func TestWindowsEchoWhatTheUserTyped(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	for _, command := range []string{"digest", "newcomer"} {
		for _, mode := range []string{"--json", "--human-friendly"} {
			run := runUniFiCLI(t, command, "--db", db, "--console", "C1", "--since", "7d", mode)
			if strings.Contains(run.stdout, "168h") || strings.Contains(run.stdout, "0m0s") {
				t.Fatalf("%s %s: %s", command, mode, run.stdout)
			}
		}
		run := runUniFiCLI(t, command, "--db", db, "--console", "C1", "--since", "7d", "--json")
		var view map[string]any
		if err := json.Unmarshal([]byte(run.stdout), &view); err != nil {
			t.Fatal(err)
		}
		if view["window"] != "7d" || view["window_seconds"] != float64(604800) {
			t.Fatalf("%s: window=%v seconds=%v", command, view["window"], view["window_seconds"])
		}
	}
}

// Output review: an empty mirror used to answer with a blank list of known zones.
func TestRulePredictOnEmptyMirrorHintsSync(t *testing.T) {
	fake := newKeySpy(t)
	db := fixtureEnv(t, fake.srv.URL)
	emptyStore(t, db)
	run := runUniFiCLI(t, "rule-predict", "--db", db, "--src-zone", "LAN", "--dst-zone", "External", "--dst-port", "443", "--protocol", "tcp", "--json")
	if run.err != nil {
		t.Fatalf("%v", run.err)
	}
	var result predictResult
	if err := json.Unmarshal([]byte(run.stdout), &result); err != nil {
		t.Fatalf("%v\n%s", err, run.stdout)
	}
	if result.Verdict != "indeterminate" || !strings.Contains(result.Reason, "sync") {
		t.Fatalf("%+v", result)
	}
}

// The help example must name zones something supports, not an assumed alias.
func TestRulePredictHelpExampleUsesSupportedZoneNames(t *testing.T) {
	cmd := newNovelRulePredictCmd(&rootFlags{})
	if strings.Contains(cmd.Example, "--dst-zone wan") || strings.Contains(cmd.Annotations["pp:happy-args"], "--dst-zone=wan") {
		t.Fatalf("example = %s, happy-args = %s", cmd.Example, cmd.Annotations["pp:happy-args"])
	}
	if !strings.Contains(cmd.Example, "--dst-zone External") {
		t.Fatalf("example = %s", cmd.Example)
	}
}

func TestNewcomerLimitKeepsNewestAndSaysSo(t *testing.T) {
	api := newFakeConsoleAPI(t)
	db := fixtureEnv(t, api.server.URL)
	seedConsole(api, "C1")
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	base := "/network/integration/v1/sites/" + fixtureSiteID + "/clients"
	api.set("C1", base, pageOf(map[string]any{"id": "client-1", "name": "laptop"}, map[string]any{"id": "client-2", "name": "phone"}, map[string]any{"id": "client-3", "name": "tablet"}))
	if run := runUniFiCLI(t, "sync", "--console", "C1", "--db", db, "--json"); run.err != nil {
		t.Fatal(run.err)
	}
	var view newcomerView
	if err := json.Unmarshal([]byte(runUniFiCLI(t, "newcomer", "--db", db, "--console", "C1", "--limit", "1", "--json").stdout), &view); err != nil {
		t.Fatal(err)
	}
	if !view.Truncated || len(view.Newcomers) != 1 {
		t.Fatalf("%+v", view)
	}
}

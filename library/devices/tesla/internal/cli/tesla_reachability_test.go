// Tests for the additive fields (recommended_via, available_paths) U5
// added to tesla_reachability.go. The legacy Classification enum values
// MUST remain unchanged; the v3 README at line 25 and downstream MCP
// consumers depend on them. These tests cover the picker-driven
// annotation helper plus a round-trip that proves the legacy
// Classification still emits to JSON unchanged.
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// writeReachabilityConfig drops a minimal config.toml with Fleet tokens at a
// temp path and points TESLA_CONFIG / TESLA_PP_AUTH_HOME at it. Helps the
// reachability annotate helper find a Fleet access token without hitting
// the real ~/.config/tesla-pp-cli/.
func writeReachabilityConfig(t *testing.T, ft config.FleetConfig) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := &config.Config{
		BaseURL: "https://owner-api.teslamotors.com",
		Fleet:   ft,
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Set HOME so newRelayPaths() resolves to a fresh empty dir (no relay
	// state present unless this test writes some). Also point TESLA_CONFIG
	// at the file so config.Load("") in reachabilityFleetReady picks it up.
	t.Setenv("HOME", dir)
	t.Setenv("TESLA_CONFIG", cfgPath)
	// Clear env overrides that would short-circuit the picker.
	t.Setenv("TESLA_FLEET_TOKEN", "")
	t.Setenv(commandHermesPortEnv, "")
}

// fakeRelay writes a PID file (using the current PID so processAliveFn's
// signal-0 probe returns true) and a port file under $HOME. The tests use
// this to simulate a Hermes relay running without spawning a subprocess.
func fakeRelay(t *testing.T, port int) {
	t.Helper()
	home := os.Getenv("HOME")
	if home == "" {
		t.Fatalf("HOME not set; call writeReachabilityConfig first")
	}
	dir := filepath.Join(home, ".config", "tesla-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "relay.pid"), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644); err != nil {
		t.Fatalf("write pid: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "relay.port"), []byte(strconv.Itoa(port)+"\n"), 0o644); err != nil {
		t.Fatalf("write port: %v", err)
	}
}

func TestReachability_SignedCmd_FleetAndRelay_RecommendsFleet(t *testing.T) {
	writeReachabilityConfig(t, config.FleetConfig{
		AccessToken: "header.payload.sig", // any non-empty token (decodeJWTClaims may fail; not load-bearing here)
		TokenExpiry: time.Now().Add(time.Hour),
	})
	fakeRelay(t, 4443)

	r := &reachabilityReport{Classification: "SIGNED_COMMAND_REQ"}
	annotateReachabilityPaths(r, VehicleClassSignedCmd)

	// VCSEC's parent class needs Fleet; KD6 selects Fleet on signed-cmd
	// vehicles whenever it's ready, regardless of relay liveness.
	if r.RecommendedVia != PathFleet {
		t.Errorf("recommended_via: got %q want %q", r.RecommendedVia, PathFleet)
	}
	got := append([]string{}, r.AvailablePaths...)
	sort.Strings(got)
	want := []string{PathBLE, PathFleet, PathHermes}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("available_paths: got %v want %v", r.AvailablePaths, want)
	}
}

func TestReachability_SignedCmd_OnlyRelay_RecommendsHermes(t *testing.T) {
	writeReachabilityConfig(t, config.FleetConfig{}) // no fleet token
	fakeRelay(t, 4443)

	r := &reachabilityReport{Classification: "SIGNED_COMMAND_REQ"}
	annotateReachabilityPaths(r, VehicleClassSignedCmd)

	if r.RecommendedVia != PathHermes {
		t.Errorf("recommended_via: got %q want %q", r.RecommendedVia, PathHermes)
	}
	// Fleet absent, so available_paths is {hermes, ble}.
	got := append([]string{}, r.AvailablePaths...)
	sort.Strings(got)
	want := []string{PathBLE, PathHermes}
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("available_paths: got %v want %v", r.AvailablePaths, want)
	}
	// Critically: Fleet must NOT appear when no Fleet creds.
	for _, p := range r.AvailablePaths {
		if p == PathFleet {
			t.Errorf("available_paths leaked Fleet without Fleet creds: %v", r.AvailablePaths)
		}
	}
}

func TestReachability_SignedCmd_OnlyBLE_RecommendsBLE(t *testing.T) {
	writeReachabilityConfig(t, config.FleetConfig{})
	// No fakeRelay call: relay PID file doesn't exist, so hermes is not running.

	r := &reachabilityReport{Classification: "SIGNED_COMMAND_REQ"}
	annotateReachabilityPaths(r, VehicleClassSignedCmd)

	if r.RecommendedVia != PathBLE {
		t.Errorf("recommended_via: got %q want %q", r.RecommendedVia, PathBLE)
	}
	// Only BLE is available.
	if len(r.AvailablePaths) != 1 || r.AvailablePaths[0] != PathBLE {
		t.Errorf("available_paths: got %v want [ble]", r.AvailablePaths)
	}
}

func TestReachability_RESTFriendly_RecommendsREST(t *testing.T) {
	writeReachabilityConfig(t, config.FleetConfig{})

	r := &reachabilityReport{Classification: "REST_OK"}
	annotateReachabilityPaths(r, VehicleClassRESTFriendly)

	if r.RecommendedVia != PathREST {
		t.Errorf("recommended_via: got %q want %q", r.RecommendedVia, PathREST)
	}
	// REST + BLE always available; Fleet/Hermes only when configured.
	saw := map[string]bool{}
	for _, p := range r.AvailablePaths {
		saw[p] = true
	}
	if !saw[PathREST] {
		t.Errorf("available_paths must include REST: %v", r.AvailablePaths)
	}
	if !saw[PathBLE] {
		t.Errorf("available_paths must include BLE recipe: %v", r.AvailablePaths)
	}
}

// TestReachability_LegacyClassificationValuesPreserved is the regression
// guard for U5's "additive only" promise. Encode a report carrying every
// legacy Classification value and assert the JSON still emits each enum
// literally; downstream MCP consumers parse on these strings.
func TestReachability_LegacyClassificationValuesPreserved(t *testing.T) {
	legacy := []string{
		"REST_OK",
		"SIGNED_COMMAND_REQ",
		"TOKEN_EXPIRED",
		"TESLA_5XX",
		"VEHICLE_ASLEEP_OR_OFFLINE",
		"UNKNOWN",
	}
	for _, val := range legacy {
		t.Run(val, func(t *testing.T) {
			r := &reachabilityReport{
				Classification: val,
				Detail:         "synthetic",
				Checks:         []probeCheck{},
			}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			s := string(raw)
			if !strings.Contains(s, `"classification":"`+val+`"`) {
				t.Errorf("classification %q not emitted verbatim; got %s", val, s)
			}
		})
	}
}

// TestReachability_AdditiveFieldsOmitEmpty proves the new fields don't
// pollute JSON when annotateReachabilityPaths isn't called (e.g. the
// TOKEN_EXPIRED and TESLA_5XX branches return without annotating). v3
// consumers reading those JSON shapes still see exactly the v3 keys.
func TestReachability_AdditiveFieldsOmitEmpty(t *testing.T) {
	r := &reachabilityReport{
		Classification: "TOKEN_EXPIRED",
		Detail:         "bearer token expired",
		Checks:         []probeCheck{},
	}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	if strings.Contains(s, "recommended_via") {
		t.Errorf("recommended_via must omit when empty (v3 consumer compat): %s", s)
	}
	if strings.Contains(s, "available_paths") {
		t.Errorf("available_paths must omit when empty (v3 consumer compat): %s", s)
	}
}

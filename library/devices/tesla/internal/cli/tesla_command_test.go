// Tests for `tesla command` — covers the path picker integration, the
// default-print-vs-send rule, the Fleet token-file subprocess invocation,
// the Hermes localhost POST, and the resolution edge cases (ambiguous name,
// missing vehicle, etc.). Live Tesla servers are never touched; tests use a
// local httptest.Server reachable via TESLA_BASE_URL plus the
// runTeslaControlSubprocessFn / runHermesHTTPClientFn package-var seams.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// commandTestFlags returns a *rootFlags pointing config.Load at a fresh temp
// path. Every test calls this so no production config is ever touched.
func commandTestFlags(t *testing.T) *rootFlags {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	return &rootFlags{configPath: cfgPath, timeout: 5 * time.Second, rateLimit: 0}
}

// commandTestSetup wires up the common scaffolding used by every test:
//   - a temp HOME (so ~/.config/tesla-pp-cli/tmp/ writes are isolated)
//   - a *rootFlags pointing at a temp config.toml
//   - an httptest.Server that mocks /api/1/products
//
// Products is parameterized so tests can simulate 0, 1, 2, or N vehicles.
func commandTestSetup(t *testing.T, products []productEntry) (*rootFlags, *httptest.Server) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Mock /api/1/products. The client.Client just reads BaseURL + path; we
	// don't bother enforcing auth headers in the mock because the AuthHeader
	// is set from the config we plant below.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/1/products", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": products,
		})
	})
	srv := httptest.NewServer(mux)
	t.Setenv("TESLA_BASE_URL", srv.URL)
	t.Cleanup(srv.Close)

	flags := commandTestFlags(t)
	// Plant a non-empty iOS-app bearer so AuthHeader() returns something
	// (the Hermes path requires it). Doesn't need to be a real Tesla token.
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		t.Fatalf("Load cfg: %v", err)
	}
	if err := cfg.SaveTokens("ownerapi", "", "ios-app-bearer", "ios-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	return flags, srv
}

func newCommandCmdForTest(t *testing.T, flags *rootFlags) *cobra.Command {
	t.Helper()
	cmd := newCommandCmd(flags)
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

// runCommandForTest invokes the router with the given argv. Returns the
// stdout buffer + error so each test can assert on the JSON shape.
func runCommandForTest(t *testing.T, flags *rootFlags, argv []string) (*bytes.Buffer, error) {
	t.Helper()
	cmd := newCommandCmdForTest(t, flags)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(argv)
	err := cmd.Execute()
	return out, err
}

// ---------------------------------------------------------------------------
// Default-print (no --send)
// ---------------------------------------------------------------------------

func TestCommand_DefaultPrint_FleetUnlock(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	// Make Fleet "ready" via env so the picker prefers it for VCSEC.
	t.Setenv("TESLA_FLEET_TOKEN", "fleet-bearer-xyz")

	// Sentinel: tesla-control must NOT be invoked.
	calls := 0
	orig := runTeslaControlSubprocessFn
	t.Cleanup(func() { runTeslaControlSubprocessFn = orig })
	runTeslaControlSubprocessFn = func(ctx context.Context, bin string, args []string) (string, string, error) {
		calls++
		return "", "", nil
	}

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake"})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out.String())
	}
	if calls != 0 {
		t.Errorf("tesla-control should NOT be invoked without --send (got %d calls)", calls)
	}
	body := out.String()
	if !strings.Contains(body, `"sent": false`) && !strings.Contains(body, `"sent":false`) {
		t.Errorf("expected sent=false in output, got: %s", body)
	}
	if !strings.Contains(body, "would unlock Snowflake via fleet") {
		t.Errorf("expected intent line naming Fleet, got: %s", body)
	}
}

// ---------------------------------------------------------------------------
// Fleet happy-path (with --send)
// ---------------------------------------------------------------------------

func TestCommand_Fleet_UnlockSend_InvokesTeslaControl(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	t.Setenv("TESLA_FLEET_TOKEN", "fleet-bearer-xyz")

	// Plant a fake key file so resolveFleetKeyPath succeeds.
	keyFile := filepath.Join(t.TempDir(), "fleet-private.pem")
	if err := os.WriteFile(keyFile, []byte("-----BEGIN EC PRIVATE KEY-----\nfake\n-----END EC PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("TESLA_FLEET_KEY_FILE", keyFile)

	// Plant a fake tesla-control on PATH so detectTeslaControlBinary resolves.
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "tesla-control")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("plant tesla-control: %v", err)
	}
	t.Setenv("PATH", binDir)

	// Capture the args tesla-control would have received.
	var gotBin string
	var gotArgs []string
	orig := runTeslaControlSubprocessFn
	t.Cleanup(func() { runTeslaControlSubprocessFn = orig })
	runTeslaControlSubprocessFn = func(ctx context.Context, bin string, args []string) (string, string, error) {
		gotBin = bin
		gotArgs = append(gotArgs[:0], args...)
		return "command succeeded\n", "", nil
	}

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send"})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out.String())
	}
	if !strings.HasSuffix(gotBin, "tesla-control") {
		t.Errorf("expected bin to be tesla-control, got %q", gotBin)
	}
	// Verify the expected args are present.
	assertArgPair(t, gotArgs, "-key-file", keyFile)
	assertArgPair(t, gotArgs, "-vin", "SNOWFLAKEVIN0001")
	// Token file is in a private tmp dir; verify the -token-file arg points
	// at a mode-0o600 file under ~/.config/tesla-pp-cli/tmp/. The file is
	// removed in defer, so we capture it inside the stub before this assert.
	// Re-read with a fresh stub.
}

// TestCommand_Fleet_TokenFileShape verifies the token file is mode-0o600 under
// ~/.config/tesla-pp-cli/tmp/ at the moment tesla-control is invoked.
func TestCommand_Fleet_TokenFileShape(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	t.Setenv("TESLA_FLEET_TOKEN", "fleet-bearer-xyz")

	keyFile := filepath.Join(t.TempDir(), "fleet-private.pem")
	if err := os.WriteFile(keyFile, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("TESLA_FLEET_KEY_FILE", keyFile)

	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "tesla-control"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("plant tesla-control: %v", err)
	}
	t.Setenv("PATH", binDir)

	type capture struct {
		tokenFile string
		exists    bool
		mode      os.FileMode
		underTmp  bool
		token     string
	}
	var got capture
	orig := runTeslaControlSubprocessFn
	t.Cleanup(func() { runTeslaControlSubprocessFn = orig })
	runTeslaControlSubprocessFn = func(ctx context.Context, bin string, args []string) (string, string, error) {
		for i, a := range args {
			if a == "-token-file" && i+1 < len(args) {
				got.tokenFile = args[i+1]
				info, err := os.Stat(got.tokenFile)
				if err == nil {
					got.exists = true
					got.mode = info.Mode().Perm()
				}
				if data, err := os.ReadFile(got.tokenFile); err == nil {
					got.token = string(data)
				}
				home, _ := os.UserHomeDir()
				expectedPrefix := filepath.Join(home, ".config", relayDirName, commandTmpDirName)
				got.underTmp = strings.HasPrefix(got.tokenFile, expectedPrefix)
				break
			}
		}
		return "", "", nil
	}

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send"})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out.String())
	}
	if !got.exists {
		t.Fatalf("token file was not present at the moment tesla-control was invoked: %+v", got)
	}
	if got.mode != 0o600 {
		t.Errorf("token file mode = %o, want 0600", got.mode)
	}
	if !got.underTmp {
		t.Errorf("token file %q is not under ~/.config/tesla-pp-cli/tmp/", got.tokenFile)
	}
	if got.token != "fleet-bearer-xyz" {
		t.Errorf("token file content = %q, want fleet-bearer-xyz", got.token)
	}

	// After the call returns the cleanup defer should have removed the file.
	if _, err := os.Stat(got.tokenFile); err == nil {
		t.Errorf("token file %q was not cleaned up after dispatch", got.tokenFile)
	}
}

// ---------------------------------------------------------------------------
// Hermes happy-path (set_charge_limit through a local httptest.Server)
// ---------------------------------------------------------------------------

func TestCommand_Hermes_SetChargeLimit_SendsToLocalRelay(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})

	// Mock "relay" — a local httptest.Server we point the Hermes path at via
	// the TESLA_PP_RELAY_PORT env. The relay routes by URL path, so we
	// register the expected /api/1/vehicles/.../command/set_charge_limit
	// handler.
	var gotAuth string
	var gotBody []byte
	relayMux := http.NewServeMux()
	relayMux.HandleFunc("/api/1/vehicles/SNOWFLAKEVIN0001/command/set_charge_limit", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotBody, _ = readAll(r.Body)
		_ = json.NewEncoder(w).Encode(map[string]any{"response": map[string]any{"result": true, "reason": ""}})
	})
	relaySrv := httptest.NewServer(relayMux)
	defer relaySrv.Close()

	// Hijack the Hermes HTTP-client seam to point at the relaySrv. Easier
	// than wrestling with localhost-port-matching + self-signed certs in a
	// test.
	orig := runHermesHTTPClientFn
	t.Cleanup(func() { runHermesHTTPClientFn = orig })
	runHermesHTTPClientFn = func(ctx context.Context, endpoint, bearer string, body []byte) (int, string, error) {
		// Sanity: the endpoint built by the router targets localhost on the
		// port we advertised via env.
		u, err := url.Parse(endpoint)
		if err != nil {
			t.Errorf("router built invalid endpoint %q: %v", endpoint, err)
		} else if u.Hostname() != "localhost" {
			t.Errorf("router targeted non-localhost endpoint: %s", endpoint)
		}
		// Now actually post against the relaySrv with the router's bearer
		// and body so the relayMux handler can assert on them.
		req, _ := http.NewRequestWithContext(ctx, "POST", relaySrv.URL+u.Path, bytes.NewReader(body))
		req.Header.Set("Authorization", bearer)
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := relaySrv.Client().Do(req)
		if err != nil {
			return 0, "", err
		}
		defer resp.Body.Close()
		b, _ := readAll(resp.Body)
		return resp.StatusCode, string(b), nil
	}

	// Mark Hermes "running" by setting the override port env. The router
	// reads commandHermesRunning() which short-circuits true when the env is
	// set. The port itself is arbitrary in this seam since we hijacked the
	// HTTP client.
	t.Setenv(commandHermesPortEnv, "9999")

	out, err := runCommandForTest(t, flags, []string{"set_charge_limit", "--vehicle", "Snowflake", "--send", "--", "percent=80"})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out.String())
	}
	if gotAuth != "Bearer ios-app-bearer" {
		t.Errorf("Authorization header = %q, want Bearer ios-app-bearer", gotAuth)
	}
	if !bytes.Contains(gotBody, []byte(`"percent":"80"`)) {
		t.Errorf("body = %s, want a percent=80 entry", string(gotBody))
	}
	if !strings.Contains(out.String(), `"path": "hermes"`) && !strings.Contains(out.String(), `"path":"hermes"`) {
		t.Errorf("expected path=hermes in output, got: %s", out.String())
	}
}

// ---------------------------------------------------------------------------
// Path-picker errors at command level
// ---------------------------------------------------------------------------

func TestCommand_ViaHermes_UnlockErrors(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	t.Setenv("TESLA_FLEET_TOKEN", "fleet-bearer-xyz")
	t.Setenv(commandHermesPortEnv, "9999") // relay "running"

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--via", "hermes", "--send"})
	if err == nil {
		t.Fatalf("expected error for --via=hermes on unlock, got nil; output=%s", out.String())
	}
	if !strings.Contains(err.Error(), "Hermes does not support lock/unlock/trunk") {
		t.Errorf("expected Hermes-VCSEC rejection, got: %v", err)
	}
	if !errIsUsage(err) {
		t.Errorf("expected usage error (exit 2), got: %v", err)
	}
}

func TestCommand_ViaHermes_NoRelayErrors(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	t.Setenv("TESLA_FLEET_TOKEN", "fleet-bearer-xyz")
	// No TESLA_PP_RELAY_PORT, no relay state file under temp HOME.

	out, err := runCommandForTest(t, flags, []string{"honk_horn", "--vehicle", "Snowflake", "--via", "hermes", "--send"})
	if err == nil {
		t.Fatalf("expected error, got nil; output=%s", out.String())
	}
	if !strings.Contains(err.Error(), "Hermes relay not running") {
		t.Errorf("expected Hermes-not-running error, got: %v", err)
	}
}

func TestCommand_ViaFleet_NoCredsErrors(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	// No TESLA_FLEET_TOKEN env, no [fleet] config block.

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--via", "fleet", "--send"})
	if err == nil {
		t.Fatalf("expected error, got nil; output=%s", out.String())
	}
	if !strings.Contains(err.Error(), "Fleet API not configured") {
		t.Errorf("expected Fleet-not-configured error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Vehicle resolution
// ---------------------------------------------------------------------------

func TestCommand_VehicleNotFound(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})

	out, err := runCommandForTest(t, flags, []string{"honk_horn", "--vehicle", "Mystery"})
	if err == nil {
		t.Fatalf("expected error for unknown vehicle, got nil; output=%s", out.String())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "tesla sync") && !strings.Contains(err.Error(), "VIN") {
		t.Errorf("expected hint about sync or VIN, got: %v", err)
	}
}

func TestCommand_AmbiguousVehicle(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snow Globe", CommandSigning: "required"},
		{VIN: "SNOWMOBILEVIN02", DisplayName: "Snow Plow", CommandSigning: "required"},
	})

	out, err := runCommandForTest(t, flags, []string{"honk_horn", "--vehicle", "Snow"})
	if err == nil {
		t.Fatalf("expected ambiguity error, got nil; output=%s", out.String())
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Snow Globe") || !strings.Contains(err.Error(), "Snow Plow") {
		t.Errorf("expected both candidates listed, got: %v", err)
	}
}

func TestCommand_VehicleVinSuffixResolves(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})

	out, err := runCommandForTest(t, flags, []string{"honk_horn", "--vehicle", "VIN0001"})
	if err != nil {
		t.Fatalf("unexpected error for VIN suffix: %v; output=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "SNOWFLAKEVIN0001") {
		t.Errorf("expected resolved VIN in output, got: %s", out.String())
	}
}

// ---------------------------------------------------------------------------
// tesla-control binary missing
// ---------------------------------------------------------------------------

func TestCommand_Fleet_TeslaControlMissing(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	t.Setenv("TESLA_FLEET_TOKEN", "fleet-bearer-xyz")

	keyFile := filepath.Join(t.TempDir(), "fleet-private.pem")
	if err := os.WriteFile(keyFile, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	t.Setenv("TESLA_FLEET_KEY_FILE", keyFile)

	// PATH points at an empty dir; ~/go/bin is under a temp home so absent.
	t.Setenv("PATH", t.TempDir())

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send"})
	if err == nil {
		t.Fatalf("expected tesla-control-missing error, got nil; output=%s", out.String())
	}
	if !strings.Contains(err.Error(), "tesla-control") {
		t.Errorf("expected error to name tesla-control, got: %v", err)
	}
	if !strings.Contains(err.Error(), "go install") {
		t.Errorf("expected error to include the install recipe, got: %v", err)
	}
	if !errIsUsage(err) {
		t.Errorf("expected usage error (exit 2), got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Verify-mode short-circuit
// ---------------------------------------------------------------------------

func TestCommand_VerifyMode_ShortCircuits(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{})
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	// No tesla-control planted, no Fleet creds, no Hermes — none of that
	// matters because verify-mode short-circuits before any of it.
	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send", "--via", "fleet"})
	if err != nil {
		t.Fatalf("verify-mode should short-circuit cleanly, got: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), `"verify_noop"`) {
		t.Errorf("expected verify_noop sentinel, got: %s", out.String())
	}
}

// ---------------------------------------------------------------------------
// REST-friendly hint surface
// ---------------------------------------------------------------------------

func TestCommand_RESTFriendly_HintsAtLegacyCmd(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		// CommandSigning empty + "off" -> RESTFriendly.
		{VIN: "STELLAVIN00001", DisplayName: "Stella", CommandSigning: "off"},
	})

	out, err := runCommandForTest(t, flags, []string{"honk_horn", "--vehicle", "Stella", "--send"})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out.String())
	}
	body := out.String()
	if !strings.Contains(body, `"path": "rest"`) && !strings.Contains(body, `"path":"rest"`) {
		t.Errorf("expected path=rest for REST-friendly car, got: %s", body)
	}
	if !strings.Contains(body, "vehicles create_honk_horn") {
		t.Errorf("expected hint pointing at legacy REST command, got: %s", body)
	}
}

// ---------------------------------------------------------------------------
// BLE recipe surface
// ---------------------------------------------------------------------------

func TestCommand_BLE_PrintsRecipeAndExitsZero(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	// No Fleet, no Hermes — picker falls through to BLE on auto.

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send"})
	if err != nil {
		t.Fatalf("BLE recipe path should exit zero, got: %v\n%s", err, out.String())
	}
	body := out.String()
	if !strings.Contains(body, `"path": "ble"`) && !strings.Contains(body, `"path":"ble"`) {
		t.Errorf("expected path=ble, got: %s", body)
	}
	if !strings.Contains(body, "tesla-control -ble") {
		t.Errorf("expected BLE recipe in output, got: %s", body)
	}
	if !strings.Contains(body, "SNOWFLAKEVIN0001") {
		t.Errorf("expected VIN in recipe, got: %s", body)
	}
}

// ---------------------------------------------------------------------------
// SweepCommandTmp
// ---------------------------------------------------------------------------

func TestCommand_SweepCommandTmp_RemovesStaleFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", relayDirName, commandTmpDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	stale := filepath.Join(dir, "fleet-token-stale.txt")
	if err := os.WriteFile(stale, []byte("oops"), 0o600); err != nil {
		t.Fatalf("write stale: %v", err)
	}
	SweepCommandTmp()
	if _, err := os.Stat(stale); err == nil {
		t.Errorf("expected stale token file %q to be swept", stale)
	}
}

// ---------------------------------------------------------------------------
// Fleet token self-heal on tesla-control 401 (U1 characterization, U2 fix)
// ---------------------------------------------------------------------------

// seedFleetSelfHeal populates the [fleet] block of the test config (so
// commandFleetReady is true via config, not env, mirroring an agentcookie
// sink), plants a fake signing key + a fake tesla-control on PATH, and wires
// TESLA_FLEET_AUTH_URL at a local server that counts refresh_token grants and
// returns a freshly-minted access token. Returns a pointer to the live refresh
// counter so a test can assert how many times the fleet token was re-minted.
//
// expiry controls the stored [fleet] token_expiry: pass a future time to keep
// the proactive clock check (commandDispatchFleet) from firing, so a test
// isolates the reactive (401-driven) refresh path.
func seedFleetSelfHeal(t *testing.T, flags *rootFlags, expiry time.Time, refreshOK bool) *int {
	t.Helper()

	cfg, err := config.Load(flags.configPath)
	if err != nil {
		t.Fatalf("Load cfg: %v", err)
	}
	keyFile := filepath.Join(t.TempDir(), "fleet-private.pem")
	if err := os.WriteFile(keyFile, []byte("-----BEGIN EC PRIVATE KEY-----\nfake\n-----END EC PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	// clientID + refreshToken populate everything tryRefreshFleetToken needs;
	// stale access token is what tesla-control will reject with a 401.
	if err := cfg.SaveFleetTokens("fleet-cid", "fleet-csec", "stale-fleet-access", "fleet-refresh-tok", expiry, "keys.example.com", keyFile); err != nil {
		t.Fatalf("SaveFleetTokens: %v", err)
	}
	t.Setenv("TESLA_FLEET_KEY_FILE", keyFile)

	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "tesla-control"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("plant tesla-control: %v", err)
	}
	t.Setenv("PATH", binDir)

	refreshes := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "refresh_token" {
			http.Error(w, "wrong grant_type", 400)
			return
		}
		refreshes++
		if !refreshOK {
			// Simulate a dead refresh token: the grant fails, so the caller
			// gets no new access token and must surface the original 401.
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "fresh-fleet-access",
			"refresh_token": "fresh-fleet-refresh",
			"expires_in":    28800,
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("TESLA_FLEET_AUTH_URL", srv.URL)

	return &refreshes
}

// auth401Stub returns a runTeslaControlSubprocessFn replacement that emits a
// 401-shaped failure on the first N calls and success afterward, recording the
// call count via the returned pointer. N<0 means "always 401".
func auth401Stub(t *testing.T, failFirst int) (*int, func(ctx context.Context, bin string, args []string) (string, string, error)) {
	t.Helper()
	calls := 0
	fn := func(ctx context.Context, bin string, args []string) (string, string, error) {
		calls++
		if failFirst < 0 || calls <= failFirst {
			return "", "Error: request failed: 401 Unauthorized (token expired)\n",
				&exitErr{code: 1}
		}
		return "command succeeded\n", "", nil
	}
	return &calls, fn
}

// exitErr is a minimal error mimicking a non-zero tesla-control exit.
type exitErr struct{ code int }

func (e *exitErr) Error() string { return "exit status " + strconv.Itoa(e.code) }

// TestCommand_Fleet_StaleToken_401_SelfHeals asserts that when tesla-control
// fails with a 401 and the stored [fleet] token_expiry is still in the future
// (so the proactive clock check does NOT fire), the dispatch reactively
// re-mints the fleet token from the stored refresh token and retries once,
// succeeding without any source-side action. This is the core sink-autonomy
// behavior (plan U1 -> U2).
func TestCommand_Fleet_StaleToken_401_SelfHeals(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	refreshes := seedFleetSelfHeal(t, flags, time.Now().Add(time.Hour), true) // future: proactive check stays quiet

	calls, stub := auth401Stub(t, 1) // 401 once, then success on retry
	orig := runTeslaControlSubprocessFn
	t.Cleanup(func() { runTeslaControlSubprocessFn = orig })
	runTeslaControlSubprocessFn = stub

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send"})
	if err != nil {
		t.Fatalf("expected self-heal success, got error: %v\n%s", err, out.String())
	}
	if *refreshes != 1 {
		t.Errorf("expected exactly 1 reactive fleet refresh, got %d", *refreshes)
	}
	if *calls != 2 {
		t.Errorf("expected tesla-control invoked twice (401 then retry), got %d", *calls)
	}
	if !strings.Contains(out.String(), `"status": "ok"`) && !strings.Contains(out.String(), `"status":"ok"`) {
		t.Errorf("expected ok status after self-heal, got: %s", out.String())
	}
}

// TestCommand_Fleet_401_RetryBoundedToOnce: when the retry also 401s, the
// dispatch refreshes exactly once and does NOT loop. Caps blast radius (R2).
func TestCommand_Fleet_401_RetryBoundedToOnce(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	refreshes := seedFleetSelfHeal(t, flags, time.Now().Add(time.Hour), true)

	calls, stub := auth401Stub(t, -1) // always 401
	orig := runTeslaControlSubprocessFn
	t.Cleanup(func() { runTeslaControlSubprocessFn = orig })
	runTeslaControlSubprocessFn = stub

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send"})
	if err == nil {
		t.Fatalf("expected error when both attempts 401, got success: %s", out.String())
	}
	if *refreshes != 1 {
		t.Errorf("expected exactly 1 refresh (no loop), got %d", *refreshes)
	}
	if *calls != 2 {
		t.Errorf("expected exactly 2 tesla-control calls (original + one retry), got %d", *calls)
	}
}

// TestCommand_Fleet_401_RefreshFails_SurfacesOriginal: when the refresh grant
// itself fails (dead refresh token), no retry fires and the original 401 is
// surfaced with the existing fleet-login guidance preserved (R3).
func TestCommand_Fleet_401_RefreshFails_SurfacesOriginal(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	refreshes := seedFleetSelfHeal(t, flags, time.Now().Add(time.Hour), false) // refresh endpoint fails

	calls, stub := auth401Stub(t, -1)
	orig := runTeslaControlSubprocessFn
	t.Cleanup(func() { runTeslaControlSubprocessFn = orig })
	runTeslaControlSubprocessFn = stub

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send"})
	if err == nil {
		t.Fatalf("expected error when refresh fails, got success: %s", out.String())
	}
	if *refreshes != 1 {
		t.Errorf("expected exactly 1 refresh attempt, got %d", *refreshes)
	}
	if *calls != 1 {
		t.Errorf("expected exactly 1 tesla-control call (no retry without a new token), got %d", *calls)
	}
	if !strings.Contains(out.String(), "401") {
		t.Errorf("expected original 401 surfaced, got: %s", out.String())
	}
}

// TestCommand_Fleet_NonAuthError_NoRefresh: a non-auth failure (sleeping car)
// must NOT trigger a token refresh or retry (KD2).
func TestCommand_Fleet_NonAuthError_NoRefresh(t *testing.T) {
	flags, _ := commandTestSetup(t, []productEntry{
		{VIN: "SNOWFLAKEVIN0001", DisplayName: "Snowflake", CommandSigning: "required"},
	})
	refreshes := seedFleetSelfHeal(t, flags, time.Now().Add(time.Hour), true)

	calls := 0
	orig := runTeslaControlSubprocessFn
	t.Cleanup(func() { runTeslaControlSubprocessFn = orig })
	runTeslaControlSubprocessFn = func(ctx context.Context, bin string, args []string) (string, string, error) {
		calls++
		return "", "Error: vehicle is asleep; wake it first\n", &exitErr{code: 1}
	}

	out, err := runCommandForTest(t, flags, []string{"unlock", "--vehicle", "Snowflake", "--send"})
	if err == nil {
		t.Fatalf("expected error for sleeping vehicle, got success: %s", out.String())
	}
	if *refreshes != 0 {
		t.Errorf("expected 0 refreshes for a non-auth failure, got %d", *refreshes)
	}
	if calls != 1 {
		t.Errorf("expected exactly 1 tesla-control call (no retry), got %d", calls)
	}
}

// TestIsFleetAuthError covers the auth-vs-not classification table (KD2).
func TestIsFleetAuthError(t *testing.T) {
	someErr := &exitErr{code: 1}
	cases := []struct {
		name           string
		stdout, stderr string
		err            error
		want           bool
	}{
		{"nil error is never auth", "", "401 unauthorized", nil, false},
		{"401 in stderr", "", "request failed: 401 Unauthorized", someErr, true},
		{"unauthorized word", "", "Unauthorized", someErr, true},
		{"invalid_token", "", "oauth: invalid_token", someErr, true},
		{"token expired", "", "the token expired", someErr, true},
		{"mixed case 401", "", "HTTP 401 UNAUTHORIZED", someErr, true},
		{"timeout is not auth", "", "context deadline exceeded", someErr, false},
		{"sleeping car is not auth", "", "vehicle is asleep", someErr, false},
		{"offline is not auth", "", "vehicle offline", someErr, false},
		{"connection refused is not auth", "", "dial tcp: connection refused", someErr, false},
		{"generic failure is not auth", "", "command failed for unknown reason", someErr, false},
		{"401 substring loses to timeout negative", "", "401 but actually timed out", someErr, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isFleetAuthError(c.stdout, c.stderr, c.err); got != c.want {
				t.Errorf("isFleetAuthError(%q,%q,%v) = %v, want %v", c.stdout, c.stderr, c.err, got, c.want)
			}
		})
	}
}

// TestTryRefreshFleetToken_ConcurrentGuard exercises the serialization guard
// (R5): many goroutines racing a refresh must not panic or tear config.toml,
// and the final state is a single coherent rotated token.
func TestTryRefreshFleetToken_ConcurrentGuard(t *testing.T) {
	flags := commandTestFlags(t)
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.SaveFleetTokens("cid", "csec", "old-access", "old-refresh", time.Now().Add(-time.Hour), "keys.example.com", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "rotated-access",
			"refresh_token": "rotated-refresh",
			"expires_in":    28800,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("TESLA_FLEET_AUTH_URL", srv.URL)

	const n = 8
	done := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			teslaFleetRefreshGuard.Lock()
			_, _ = tryRefreshFleetToken(cfg)
			teslaFleetRefreshGuard.Unlock()
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}

	cfg2, _ := config.Load(flags.configPath)
	ft := cfg2.FleetTokens()
	if ft.AccessToken != "rotated-access" {
		t.Errorf("config not coherently rotated: AccessToken=%q", ft.AccessToken)
	}
	if ft.RefreshToken != "rotated-refresh" {
		t.Errorf("config not coherently rotated: RefreshToken=%q", ft.RefreshToken)
	}
}

// TestTryRefreshFleetToken_SaveFailureStillReturnsToken locks the return
// contract: a successful grant whose config persistence fails must still return
// the freshly-minted token (alongside the save error) so the caller can use it
// for the current request. Greptile flagged callers dropping this token.
func TestTryRefreshFleetToken_SaveFailureStillReturnsToken(t *testing.T) {
	// Point the config at a path whose parent is a regular file, so save()'s
	// MkdirAll fails and the write cannot land.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	cfg := &config.Config{
		Path:  filepath.Join(blocker, "config.toml"),
		Fleet: config.FleetConfig{ClientID: "cid", RefreshToken: "refresh-tok"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/v3/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "minted-but-unsaved",
			"refresh_token": "new-refresh",
			"expires_in":    28800,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("TESLA_FLEET_AUTH_URL", srv.URL)

	tok, err := tryRefreshFleetToken(cfg)
	if tok != "minted-but-unsaved" {
		t.Errorf("expected minted token returned despite save failure, got %q", tok)
	}
	if err == nil {
		t.Errorf("expected a non-nil save error alongside the token")
	}
}

// TestFleetTokenNeedsProactiveRefresh covers the skew-window proactive check
// (plan U3): refresh when expired, near-expiry, or unknown-expiry-with-refresh;
// skip when comfortably valid or when no refresh token exists.
func TestFleetTokenNeedsProactiveRefresh(t *testing.T) {
	skew := 60 * time.Second
	cases := []struct {
		name string
		ft   config.FleetConfig
		want bool
	}{
		{"expired with refresh token", config.FleetConfig{RefreshToken: "r", TokenExpiry: time.Now().Add(-time.Hour)}, true},
		{"near-expiry within skew", config.FleetConfig{RefreshToken: "r", TokenExpiry: time.Now().Add(30 * time.Second)}, true},
		{"comfortably valid beyond skew", config.FleetConfig{RefreshToken: "r", TokenExpiry: time.Now().Add(time.Hour)}, false},
		{"zero expiry with refresh token", config.FleetConfig{RefreshToken: "r"}, true},
		{"zero expiry without refresh token", config.FleetConfig{}, false},
		{"expired but no refresh token", config.FleetConfig{TokenExpiry: time.Now().Add(-time.Hour)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := fleetTokenNeedsProactiveRefresh(c.ft, skew); got != c.want {
				t.Errorf("fleetTokenNeedsProactiveRefresh = %v, want %v", got, c.want)
			}
		})
	}
}

// TestNewClient_ReadPathSelfHealsInBothModes guards that the read client always
// wires a 401 auto-refresh callback (plan U4): owner-api reads heal via the
// owner-api refresh closure, Fleet-routed reads heal via the Fleet closure.
// A future refactor that drops OnTokenExpired would make a sink's reads stop
// self-healing — this test fails loudly if that happens.
func TestNewClient_ReadPathSelfHealsInBothModes(t *testing.T) {
	t.Run("owner-api creds present routes owner-api and self-heals", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_TOKEN", "")
		t.Setenv("TESLA_PP_NO_AUTOREFRESH", "")
		flags := commandTestFlags(t)
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := cfg.SaveTokens("ownerapi", "", "owner-bearer", "owner-refresh", time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("SaveTokens: %v", err)
		}
		c, err := flags.newClient()
		if err != nil {
			t.Fatalf("newClient: %v", err)
		}
		if c.FleetMode {
			t.Errorf("expected owner-api read routing (FleetMode=false) when a valid owner token exists")
		}
		if c.OnTokenExpired == nil {
			t.Errorf("read client must wire a 401 self-heal callback (owner-api mode)")
		}
	})

	t.Run("fleet-only creds route fleet and self-heal", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_TOKEN", "")
		t.Setenv("TESLA_PP_NO_AUTOREFRESH", "")
		flags := commandTestFlags(t)
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		// Fleet token only, no usable owner-api credential: reads must route to
		// the Fleet API and still self-heal via the Fleet refresh closure.
		if err := cfg.SaveFleetTokens("cid", "csec", "fleet-bearer", "fleet-refresh", time.Now().Add(time.Hour), "keys.example.com", ""); err != nil {
			t.Fatalf("SaveFleetTokens: %v", err)
		}
		c, err := flags.newClient()
		if err != nil {
			t.Fatalf("newClient: %v", err)
		}
		if !c.FleetMode {
			t.Errorf("expected Fleet read routing (FleetMode=true) when only Fleet creds exist")
		}
		if c.OnTokenExpired == nil {
			t.Errorf("read client must wire a 401 self-heal callback (Fleet mode)")
		}
	})

	t.Run("opt-out disables the self-heal callback", func(t *testing.T) {
		t.Setenv("TESLA_FLEET_TOKEN", "")
		t.Setenv("TESLA_PP_NO_AUTOREFRESH", "1")
		flags := commandTestFlags(t)
		cfg, err := config.Load(flags.configPath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := cfg.SaveTokens("ownerapi", "", "owner-bearer", "owner-refresh", time.Now().Add(time.Hour)); err != nil {
			t.Fatalf("SaveTokens: %v", err)
		}
		c, err := flags.newClient()
		if err != nil {
			t.Fatalf("newClient: %v", err)
		}
		if c.OnTokenExpired != nil {
			t.Errorf("TESLA_PP_NO_AUTOREFRESH=1 must leave OnTokenExpired unset for explicit 401 handling")
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertArgPair verifies that the args slice contains -<name> <value> as
// adjacent entries.
func assertArgPair(t *testing.T, args []string, name, want string) {
	t.Helper()
	for i, a := range args {
		if a == name && i+1 < len(args) {
			if args[i+1] != want {
				t.Errorf("arg %s: got %q want %q", name, args[i+1], want)
			}
			return
		}
	}
	t.Errorf("arg %s not found in %v", name, args)
}

// readAll is a thin wrapper around the io.ReadAll seam tests use.
func readAll(r interface {
	Read(p []byte) (int, error)
}) ([]byte, error) {
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 512)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return buf, nil
			}
			return buf, err
		}
	}
}

// TestProductEntry_UnmarshalsHeterogeneousIDs guards the Wall Connector fix:
// /api/1/products returns an int id for vehicles and a non-numeric string id
// for energy devices (e.g. "STE20240625-00048"). The router only consumes
// VIN/display_name, but a typed mismatch on id aborts the whole unmarshal —
// which historically broke `tesla command` for any account owning a Wall
// Connector. json.RawMessage is the only stdlib type that swallows both
// shapes; json.Number rejects non-numeric strings via isValidNumber.
func TestProductEntry_UnmarshalsHeterogeneousIDs(t *testing.T) {
	payload := []byte(`{"response":[
		{"vin":"5YJ3000000000VIN1","display_name":"car","id":3744559116524749},
		{"display_name":"Wall Connector","id":"STE20240625-00048"}
	]}`)
	var wrapper struct {
		Response []productEntry `json:"response"`
	}
	if err := json.Unmarshal(payload, &wrapper); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(wrapper.Response) != 2 {
		t.Fatalf("got %d entries, want 2", len(wrapper.Response))
	}
	if wrapper.Response[0].VIN != "5YJ3000000000VIN1" {
		t.Errorf("vehicle VIN: got %q", wrapper.Response[0].VIN)
	}
	if wrapper.Response[1].DisplayName != "Wall Connector" {
		t.Errorf("wall connector display_name: got %q", wrapper.Response[1].DisplayName)
	}
}

// TestFetchProductsList_FiltersEnergyDevices guards the second half of the
// Wall Connector fix: even after the unmarshal succeeds, an entry with an
// empty VIN would slip into resolveCommandVehicle's exact-match loop, where
// strings.EqualFold("", "") returns true and silently routes commands at a
// Wall Connector. fetchProductsList must drop empty-VIN entries at the
// boundary so the downstream matching logic only ever sees vehicles.
func TestFetchProductsList_FiltersEnergyDevices(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	mux := http.NewServeMux()
	mux.HandleFunc("/api/1/products", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":[
			{"vin":"5YJ3000000000VIN1","display_name":"Snowflake","id":3744559116524749},
			{"display_name":"Wall Connector","id":"STE20240625-00048"}
		]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("TESLA_BASE_URL", srv.URL)

	flags := commandTestFlags(t)
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		t.Fatalf("Load cfg: %v", err)
	}
	if err := cfg.SaveTokens("ownerapi", "", "ios-app-bearer", "ios-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	got, err := fetchProductsList(context.Background(), flags, cfg)
	if err != nil {
		t.Fatalf("fetchProductsList: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 (Wall Connector should be filtered)", len(got))
	}
	if got[0].VIN != "5YJ3000000000VIN1" {
		t.Errorf("surviving entry VIN: got %q, want vehicle VIN", got[0].VIN)
	}
}

// Compile-time sanity: ensure config import isn't dropped if any test path
// stops referencing it (defensive: simplifies refactors that move test setup
// across files).
var _ = config.Load
var _ = strconv.Itoa

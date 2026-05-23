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

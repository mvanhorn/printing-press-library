// tesla command — unified router that picks the right transport per vehicle
// and per command, with explicit --via override and an opt-in --send flag.
//
// Defaults follow KD6 in 2026-05-22-001:
//   - REST-friendly vehicle: legacy owner-API REST (unchanged from v0.1)
//   - signed-cmd vehicle, owner-API command, Hermes relay running: Hermes
//   - signed-cmd vehicle, owner-API command, Fleet creds present: Fleet
//   - signed-cmd vehicle, VCSEC command: Fleet (Hermes cannot VCSEC)
//   - signed-cmd vehicle, wake_up: Fleet (Hermes wake_up bug)
//   - no internet path available: print the tesla-control -ble recipe
//
// KD5 governs side-effect handling: without --send the command prints intent
// and exits zero. With --send it actually fires.
//
// MCP annotation: the cobra parent carries `mcp:destructive=true` per OQ1 in
// the plan. Even though --send guards real fires, an MCP agent invoking the
// tool is signaling intent and the worst-case outcome (vehicle command) is
// destructive.
//
// Hand-coded; lives outside the generator's emit set.
package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// runTeslaControlSubprocessFn is the seam tests use to stub the Fleet path's
// `tesla-control` invocation. Production calls into runTeslaControlSubprocessReal,
// which forks a real subprocess against the token + key + VIN. Tests replace
// this var to assert the argv shape without planting a binary on PATH.
var runTeslaControlSubprocessFn = runTeslaControlSubprocessReal

// runHermesHTTPClientFn is the seam tests use to stub the Hermes path's local
// relay HTTP call. Production builds an *http.Client that trusts the relay's
// self-signed cert and POSTs the command. Tests replace this var with one
// pointed at an httptest.Server.
var runHermesHTTPClientFn = runHermesHTTPClientReal

// commandHermesPortEnv overrides the default :4443 the Hermes path targets.
// Tests use this to point at an httptest.Server's port.
const commandHermesPortEnv = "TESLA_PP_RELAY_PORT"

// commandFleetKeyFileEnv overrides the Fleet private key path stored in
// config.Fleet.PrivateKeyPath. Lets a user run from an alternate keyring
// without rewriting config.
const commandFleetKeyFileEnv = "TESLA_FLEET_KEY_FILE"

// commandTmpDirName is the subdirectory under ~/.config/tesla-pp-cli/ where
// short-lived token files are written for tesla-control. Created mode 0o700
// so the contained mode-0o600 token files are not group/world readable even
// if a future mode-leak bug widens the file mode.
const commandTmpDirName = "tmp"

// SweepCommandTmp removes any stale token files left under
// ~/.config/tesla-pp-cli/tmp/ from a previous crashed run. Safe to call from
// cli.Execute() at startup because the dir only ever holds short-lived files
// that the command path defers a per-call cleanup on. Returns nil on missing
// dir; non-fatal best-effort cleanup.
func SweepCommandTmp() {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}
	dir := filepath.Join(home, ".config", relayDirName, commandTmpDirName)
	_ = os.RemoveAll(dir)
}

// newCommandCmd returns the `tesla command <name>` parent. Sub-actions are
// not modeled as cobra subcommands because the command name is dynamic; the
// router accepts any name, classifies via classifyCommand, and dispatches.
func newCommandCmd(flags *rootFlags) *cobra.Command {
	var (
		vehicleArg string
		viaArg     string
		sendArg    bool
	)
	cmd := &cobra.Command{
		Use:   "command <name> [--vehicle NAME-OR-VIN] [--via=auto|fleet|hermes|ble] [--send] [extra-args...]",
		Short: "Send a Tesla command via the best-available signed path",
		Long: `Unified router for Tesla vehicle commands. Picks the cheapest available
internet path per command:

  - REST-friendly vehicles (pre-2021 S/X, pre-late-2021 3/Y) -> legacy REST
  - signed-cmd vehicles, owner-API command (charge, climate, honk, media):
    Hermes relay if running (free), Fleet API otherwise (paid)
  - signed-cmd vehicles, VCSEC (lock, unlock, trunk, sentry): Fleet API only
    (Hermes proxy does not support VCSEC)
  - signed-cmd vehicles, wake_up: Fleet API (Hermes proxy has a known wake_up
    bug)
  - no internet path available: prints the exact tesla-control -ble recipe

By default this command PRINTS the intent and exits zero ("would unlock
<vehicle> via Fleet API"). Pass --send to actually fire. This mirrors the
AGENTS.md agent-native side-effect rule and stops an MCP agent from
accidentally unlocking the car.

Use --via=auto|fleet|hermes|ble to override the picker. The router surfaces a
clear error if the requested path is unavailable (e.g. --via=hermes for unlock
errors out because Hermes cannot VCSEC).`,
		Example: `  tesla-pp-cli command unlock --vehicle Snowflake --send
  tesla-pp-cli command set_charge_limit --vehicle Snowflake --via=hermes --send -- 80
  tesla-pp-cli command honk_horn --vehicle Stella`,
		Annotations: map[string]string{
			// KD5/OQ1: worst-case is destructive even with --send guarding the
			// real fire. An MCP agent invoking the tool is signaling intent.
			"mcp:destructive": "true",
		},
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommand(cmd, flags, args[0], args[1:], vehicleArg, viaArg, sendArg)
		},
	}
	cmd.Flags().StringVar(&vehicleArg, "vehicle", "", "Vehicle name or VIN (substring match against products cache)")
	cmd.Flags().StringVar(&viaArg, "via", "auto", "Override the path picker: auto|fleet|hermes|ble")
	cmd.Flags().BoolVar(&sendArg, "send", false, "Actually fire the command (default: print intent and exit zero)")
	return cmd
}

// commandResolvedVehicle carries everything the router needs to know about
// the target vehicle to issue a command: VIN for tesla-control / Hermes URL
// path, display name for the user-visible reason string, and the
// REST-friendly-vs-signed-cmd classification that drives the path picker.
type commandResolvedVehicle struct {
	VIN          string
	DisplayName  string
	VehicleClass string // VehicleClassRESTFriendly | VehicleClassSignedCmd
}

func runCommand(cmd *cobra.Command, flags *rootFlags, name string, extraArgs []string, vehicleArg, viaArg string, send bool) error {
	if cliutil.IsVerifyEnv() {
		// Verify-mode: short-circuit; emit the resolved decision without
		// touching the network or any subprocess. Honors the AGENTS.md verify
		// contract.
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"verify_noop": true,
			"step":        "command",
			"command":     name,
			"via":         viaArg,
			"send":        send,
		}, flags)
	}
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"dry_run": true,
			"step":    "command",
			"command": name,
			"via":     viaArg,
			"send":    send,
		}, flags)
	}

	cfg, err := config.Load(flagsConfigPath(flags))
	if err != nil {
		return configErr(err)
	}

	// Resolve the target vehicle. Substring match across display_name and VIN
	// against the live products endpoint; ambiguous matches surface both
	// candidates so the user can disambiguate.
	resolved, err := resolveCommandVehicle(cmd.Context(), flags, cfg, vehicleArg)
	if err != nil {
		return err
	}

	// Pre-flight the picker inputs.
	cmdClass := classifyCommand(name)
	fleetReady := commandFleetReady(cfg)
	hermesRunning := commandHermesRunning()

	choice, perr := PickPath(cmdClass, resolved.VehicleClass, fleetReady, hermesRunning, viaArg)
	if perr != nil {
		return usageErr(perr)
	}

	// Default-print: surface intent and exit zero. The whole point of KD5.
	if !send {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"step":          "command",
			"command":       name,
			"vehicle":       resolved.DisplayName,
			"vin":           resolved.VIN,
			"vehicle_class": resolved.VehicleClass,
			"path":          choice.Path,
			"reason":        fmt.Sprintf("would %s %s via %s; pass --send to fire", name, resolved.DisplayName, choice.Path),
			"hint":          choice.Reason,
			"sent":          false,
		}, flags)
	}

	// --send: dispatch to the chosen transport.
	switch choice.Path {
	case PathFleet:
		return commandDispatchFleet(cmd, flags, cfg, resolved, name, extraArgs, choice)
	case PathHermes:
		return commandDispatchHermes(cmd, flags, cfg, resolved, name, extraArgs, choice)
	case PathBLE:
		return commandDispatchBLE(cmd, flags, resolved, name, extraArgs, choice)
	case PathREST:
		// REST-friendly cars: surface a pointer to the generated REST command.
		// The unified router doesn't re-implement every REST endpoint; users
		// of REST-friendly cars can call the legacy `vehicles command_*`
		// subcommands directly. This branch primarily exists for the path
		// picker matrix completeness; we surface a hint and exit zero.
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"step":    "command",
			"command": name,
			"vehicle": resolved.DisplayName,
			"vin":     resolved.VIN,
			"path":    PathREST,
			"reason":  choice.Reason,
			"hint":    fmt.Sprintf("REST-friendly vehicle: invoke the legacy REST command directly, e.g. `tesla vehicles create_%s --vehicle %s`", name, resolved.VIN),
			"sent":    false,
		}, flags)
	default:
		return fmt.Errorf("unknown path %q from picker", choice.Path)
	}
}

// commandFleetReady reports whether Fleet API credentials are present and
// usable. We treat a present access token (even if expired) as ready because
// the caller will auto-refresh on dispatch. Empty access token means the user
// must run fleet-login first.
func commandFleetReady(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	ft := cfg.FleetTokens()
	if os.Getenv("TESLA_FLEET_TOKEN") != "" {
		return true
	}
	return ft.AccessToken != ""
}

// commandHermesRunning reports whether the local Hermes relay subprocess is
// alive. Reads the relay state files written by `tesla relay start`. A test
// override path is provided via the TESLA_PP_RELAY_PORT env var: when that's
// set we treat the relay as running unconditionally (the test points at an
// httptest.Server on that port).
func commandHermesRunning() bool {
	if os.Getenv(commandHermesPortEnv) != "" {
		return true
	}
	paths, err := newRelayPaths()
	if err != nil {
		return false
	}
	_, _, alive := readRelayState(paths)
	return alive
}

// commandHermesPort returns the port the local Hermes relay listens on. The
// test override via TESLA_PP_RELAY_PORT wins; otherwise we read the port file
// the relay writes on startup; finally we fall back to relayDefaultPort.
func commandHermesPort() int {
	if v := os.Getenv(commandHermesPortEnv); v != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && p > 0 {
			return p
		}
	}
	paths, err := newRelayPaths()
	if err != nil {
		return relayDefaultPort
	}
	if portBytes, err := os.ReadFile(paths.Port); err == nil {
		if p, err := strconv.Atoi(strings.TrimSpace(string(portBytes))); err == nil && p > 0 {
			return p
		}
	}
	return relayDefaultPort
}

// resolveCommandVehicle walks the products endpoint (or a substring match
// against the in-memory list) to find the target. Matching precedence: exact
// VIN -> exact display_name -> VIN-suffix (last 6+ chars) -> display_name
// substring. Ambiguous substring matches surface BOTH candidates so the user
// can disambiguate.
func resolveCommandVehicle(ctx context.Context, flags *rootFlags, cfg *config.Config, want string) (*commandResolvedVehicle, error) {
	// In verify-mode tests we don't have a live products endpoint. Fall back
	// to treating the user-supplied --vehicle as a VIN-shaped opaque token
	// and assume signed-cmd. The path picker is the load-bearing logic
	// tested separately.
	products, fetchErr := fetchProductsList(ctx, flags, cfg)
	if fetchErr != nil && strings.TrimSpace(want) == "" {
		return nil, usageErr(fmt.Errorf("could not list vehicles to auto-pick: %w; pass --vehicle <name-or-VIN>", fetchErr))
	}
	if len(products) == 0 && strings.TrimSpace(want) == "" {
		return nil, usageErr(fmt.Errorf("no vehicles found and no --vehicle supplied; run `tesla sync` first or pass --vehicle <VIN>"))
	}

	want = strings.TrimSpace(want)
	if want == "" && len(products) == 1 {
		// Single-vehicle account: pick it.
		p := products[0]
		return &commandResolvedVehicle{
			VIN:          p.VIN,
			DisplayName:  firstNonEmpty(p.DisplayName, p.VIN),
			VehicleClass: classifyVehicleClass(p),
		}, nil
	}

	// Exact-match passes.
	for _, p := range products {
		if strings.EqualFold(p.VIN, want) {
			return &commandResolvedVehicle{
				VIN:          p.VIN,
				DisplayName:  firstNonEmpty(p.DisplayName, p.VIN),
				VehicleClass: classifyVehicleClass(p),
			}, nil
		}
		if p.DisplayName != "" && strings.EqualFold(p.DisplayName, want) {
			return &commandResolvedVehicle{
				VIN:          p.VIN,
				DisplayName:  p.DisplayName,
				VehicleClass: classifyVehicleClass(p),
			}, nil
		}
	}

	// VIN suffix (>=6 chars).
	if len(want) >= 6 {
		var hits []productEntry
		for _, p := range products {
			if strings.HasSuffix(strings.ToLower(p.VIN), strings.ToLower(want)) {
				hits = append(hits, p)
			}
		}
		if len(hits) == 1 {
			p := hits[0]
			return &commandResolvedVehicle{
				VIN:          p.VIN,
				DisplayName:  firstNonEmpty(p.DisplayName, p.VIN),
				VehicleClass: classifyVehicleClass(p),
			}, nil
		}
		if len(hits) > 1 {
			return nil, usageErr(fmt.Errorf("ambiguous --vehicle %q: %s", want, formatAmbiguousCandidates(hits)))
		}
	}

	// Substring on display_name.
	var nameHits []productEntry
	for _, p := range products {
		if p.DisplayName != "" && strings.Contains(strings.ToLower(p.DisplayName), strings.ToLower(want)) {
			nameHits = append(nameHits, p)
		}
	}
	if len(nameHits) == 1 {
		p := nameHits[0]
		return &commandResolvedVehicle{
			VIN:          p.VIN,
			DisplayName:  p.DisplayName,
			VehicleClass: classifyVehicleClass(p),
		}, nil
	}
	if len(nameHits) > 1 {
		return nil, usageErr(fmt.Errorf("ambiguous --vehicle %q: %s", want, formatAmbiguousCandidates(nameHits)))
	}

	// No match anywhere. If fetchProductsList failed earlier (e.g. 401, network
	// error, verify-mode), surface that as the real cause rather than the
	// generic "not found" message - the products list we matched against was
	// empty because the fetch died, not because the vehicle is absent.
	if fetchErr != nil {
		return nil, usageErr(fmt.Errorf("vehicle %q not resolvable: products fetch failed (%w); run `tesla auth status` to confirm credentials", want, fetchErr))
	}
	return nil, usageErr(fmt.Errorf("vehicle %q not found in your products list; run `tesla sync` first or pass the full VIN", want))
}

// productEntry is the minimal shape of /api/1/products we use for the router.
// Tesla returns a much richer object; we only care about VIN + display_name
// here. command_signing is a hint Tesla sometimes surfaces; absent we fall
// back to a year-based heuristic via tesla_reachability's classifier.
//
// NOTE: the id field is json.RawMessage because /api/1/products returns a
// heterogeneous array — vehicles use integer IDs (e.g. 3744559116524749) while
// energy devices such as Wall Connectors use non-numeric string IDs (e.g.
// "STE20240625-00048"). Typing the field as int64 causes json.Unmarshal to
// fail on the entire response the moment any non-vehicle product is present;
// json.Number is also insufficient because it rejects non-numeric strings via
// isValidNumber. json.RawMessage implements json.Unmarshaler and accepts any
// token without inspection, so the parse succeeds for every product shape.
// No call site currently reads this field; routing keys off VIN.
type productEntry struct {
	VIN              string          `json:"vin"`
	DisplayName      string          `json:"display_name"`
	CommandSigning   string          `json:"command_signing"`
	VehicleCommandID json.RawMessage `json:"id"`
}

// fetchProductsList calls /api/1/products and returns the typed entries.
// Returns (nil, nil) when the user has no Tesla auth token at all (commonly
// a fresh install before login); the caller treats that as "list empty" and
// surfaces a usage error pointing at --vehicle or tesla auth login.
func fetchProductsList(ctx context.Context, flags *rootFlags, cfg *config.Config) ([]productEntry, error) {
	// Build a client; even with no token we can still call the endpoint and
	// surface a 401 to the caller (the verify-mode + dry-run branches above
	// already short-circuited).
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	// client.Get does not take a context, so we dispatch on a goroutine and
	// honor ctx via select. On ctx cancel, the orphan goroutine still runs to
	// client.Timeout (default 30s) and writes its result to a buffered channel
	// the GC reclaims when no one reads it. This bounds the user-visible wait
	// to the caller's deadline while letting the underlying request complete
	// cleanly in the background, which avoids leaking sockets.
	type result struct {
		raw json.RawMessage
		err error
	}
	ch := make(chan result, 1)
	go func() {
		raw, gerr := c.Get("/api/1/products", nil)
		ch <- result{raw, gerr}
	}()
	var raw json.RawMessage
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		raw = r.raw
	}
	var env struct {
		Response []productEntry `json:"response"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parse products: %w", err)
	}
	// cfg is plumbed for future routing tweaks (e.g. consult cfg.Fleet.PublicKeyDomain).
	_ = cfg
	// Drop energy devices (Wall Connector, Powerwall) — they have no VIN and
	// can't receive vehicle commands. Leaving them in would let
	// strings.EqualFold("", "") match an empty --vehicle against an empty VIN
	// in resolveCommandVehicle's exact-match loop, silently routing the
	// command to a non-vehicle target.
	vehicles := env.Response[:0]
	for _, p := range env.Response {
		if strings.TrimSpace(p.VIN) == "" {
			continue
		}
		vehicles = append(vehicles, p)
	}
	return vehicles, nil
}

// classifyVehicleClass maps a productEntry into REST-friendly vs signed-cmd.
// command_signing is the Tesla-surfaced hint when present; otherwise we
// pessimistically assume signed-cmd (the modern default).
func classifyVehicleClass(p productEntry) string {
	switch strings.ToLower(strings.TrimSpace(p.CommandSigning)) {
	case "allowed", "off", "disabled":
		return VehicleClassRESTFriendly
	case "required", "signed", "enabled":
		return VehicleClassSignedCmd
	}
	// No signal: assume signed-cmd. Most vehicles in the field are signed-cmd
	// now; routing through Fleet/Hermes works the same for either class on a
	// command-by-command basis if the user passes --via.
	return VehicleClassSignedCmd
}

func formatAmbiguousCandidates(hits []productEntry) string {
	parts := make([]string, 0, len(hits))
	for _, p := range hits {
		label := p.DisplayName
		if label == "" {
			label = p.VIN
		}
		parts = append(parts, fmt.Sprintf("%s (VIN %s)", label, p.VIN))
	}
	return strings.Join(parts, ", ")
}

// commandDispatchFleet writes the Fleet user token to a short-lived mode-0o600
// file under ~/.config/tesla-pp-cli/tmp/, then execs `tesla-control` against
// the user's private key + VIN. The token file is removed in a defer; the dir
// is mode 0o700 so the file is not group/world readable even if a future
// mode-leak bug widens the file mode.
func commandDispatchFleet(cmd *cobra.Command, flags *rootFlags, cfg *config.Config, v *commandResolvedVehicle, name string, extra []string, choice PathChoice) error {
	if cfg == nil {
		return fmt.Errorf("nil config in Fleet dispatch")
	}
	ft := cfg.FleetTokens()
	token := firstNonEmpty(os.Getenv("TESLA_FLEET_TOKEN"), ft.AccessToken)
	if token == "" {
		return usageErr(fmt.Errorf("Fleet API not configured; run `tesla auth fleet-login`"))
	}

	// Auto-refresh on expired token, best-effort. If refresh fails we still
	// attempt the call; tesla-control surfaces a meaningful 401 error.
	if !ft.TokenExpiry.IsZero() && time.Now().After(ft.TokenExpiry) {
		if refreshed, rerr := tryRefreshFleetToken(cfg); rerr == nil && refreshed != "" {
			token = refreshed
		}
	}

	// Locate the user's private key. Env wins, then config, then a couple
	// of conventional locations.
	keyPath, kerr := resolveFleetKeyPath(cfg)
	if kerr != nil {
		return usageErr(kerr)
	}

	bin := detectTeslaControlBinary()
	if bin == "" {
		return usageErr(fmt.Errorf(
			"tesla-control not found on PATH or at ~/go/bin/%s; install via:\n"+
				"  go install github.com/teslamotors/vehicle-command/cmd/tesla-control@latest",
			teslaControlBinary,
		))
	}

	tokenFile, cleanup, terr := writeTokenFile(token)
	if terr != nil {
		return fmt.Errorf("write token file: %w", terr)
	}
	defer cleanup()

	args := []string{
		"-token-file", tokenFile,
		"-key-file", keyPath,
		"-vin", v.VIN,
		name,
	}
	args = append(args, extra...)

	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	stdout, stderr, runErr := runTeslaControlSubprocessFn(ctx, bin, args)
	result := map[string]any{
		"step":    "command",
		"command": name,
		"vehicle": v.DisplayName,
		"vin":     v.VIN,
		"path":    choice.Path,
		"sent":    true,
		"stdout":  strings.TrimSpace(stdout),
		"stderr":  strings.TrimSpace(stderr),
	}
	if runErr != nil {
		result["status"] = "error"
		result["error"] = runErr.Error()
		_ = printJSONFiltered(cmd.OutOrStdout(), result, flags)
		return fmt.Errorf("tesla-control %s failed: %w", name, runErr)
	}
	result["status"] = "ok"
	return printJSONFiltered(cmd.OutOrStdout(), result, flags)
}

// runTeslaControlSubprocessReal is the production path for the Fleet dispatch.
// Captures stdout/stderr separately so structured output can surface both.
func runTeslaControlSubprocessReal(ctx context.Context, bin string, args []string) (string, string, error) {
	c := exec.CommandContext(ctx, bin, args...)
	var so, se strings.Builder
	c.Stdout = &so
	c.Stderr = &se
	if err := c.Run(); err != nil {
		return so.String(), se.String(), err
	}
	return so.String(), se.String(), nil
}

// writeTokenFile materializes the Fleet bearer to a short-lived mode-0o600
// file inside ~/.config/tesla-pp-cli/tmp/ (mode 0o700 dir). Returns the
// absolute path and a cleanup func the caller defers. NOT the system temp
// dir: that path is process-listing visible and group/world readable on
// many distros.
func writeTokenFile(token string) (string, func(), error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", func() {}, fmt.Errorf("resolve home: %w", err)
	}
	dir := filepath.Join(home, ".config", relayDirName, commandTmpDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", func() {}, fmt.Errorf("create tmp dir: %w", err)
	}
	f, err := os.CreateTemp(dir, "fleet-token-*.txt")
	if err != nil {
		return "", func() {}, err
	}
	path := f.Name()
	// Chmod first, then write, so we never have a window where the bytes are
	// on disk under a wider mode than 0o600.
	if err := os.Chmod(path, 0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", func() {}, err
	}
	if _, err := io.WriteString(f, token); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}

// resolveFleetKeyPath returns the absolute path of the Fleet signing private
// key, preferring env TESLA_FLEET_KEY_FILE, then cfg.Fleet.PrivateKeyPath. No
// existence check beyond stat: we want tesla-control's own error message to
// surface when the key is unreadable for any other reason (mode mismatch
// etc.). Returns a usage error when neither source is set.
func resolveFleetKeyPath(cfg *config.Config) (string, error) {
	if v := strings.TrimSpace(os.Getenv(commandFleetKeyFileEnv)); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", fmt.Errorf("resolve %s=%q: %w", commandFleetKeyFileEnv, v, err)
		}
		if _, err := os.Stat(abs); err != nil {
			return "", fmt.Errorf("%s=%q not readable: %w", commandFleetKeyFileEnv, abs, err)
		}
		return abs, nil
	}
	if cfg != nil {
		ft := cfg.FleetTokens()
		if ft.PrivateKeyPath != "" {
			abs, err := filepath.Abs(ft.PrivateKeyPath)
			if err != nil {
				return "", fmt.Errorf("resolve config Fleet.PrivateKeyPath %q: %w", ft.PrivateKeyPath, err)
			}
			if _, err := os.Stat(abs); err != nil {
				return "", fmt.Errorf("Fleet.PrivateKeyPath %q not readable: %w", abs, err)
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("no Fleet signing key configured; set TESLA_FLEET_KEY_FILE or run `tesla auth fleet-template --gen-key` and store the path with `tesla auth fleet-register`")
}

// tryRefreshFleetToken attempts a silent refresh_token grant. Returns the new
// access token on success, "" on any failure. Errors are swallowed so the
// caller can fall through to dispatch with the stale token and let
// tesla-control surface a clear 401.
func tryRefreshFleetToken(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("nil cfg")
	}
	ft := cfg.FleetTokens()
	if ft.RefreshToken == "" {
		return "", fmt.Errorf("no Fleet refresh token stored")
	}
	clientID := firstNonEmpty(os.Getenv("TESLA_FLEET_CLIENT_ID"), ft.ClientID)
	if clientID == "" {
		return "", fmt.Errorf("no Fleet client_id stored")
	}
	tokenURL := fleetTokenURL
	if base := os.Getenv("TESLA_FLEET_AUTH_URL"); base != "" {
		tokenURL = base + "/oauth2/v3/token"
	}
	tok, err := fleetRefreshGrant(tokenURL, clientID, ft.RefreshToken)
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second).UTC()
	finalRefresh := tok.RefreshToken
	if strings.TrimSpace(finalRefresh) == "" {
		finalRefresh = ft.RefreshToken
	}
	if err := cfg.SaveFleetTokens("", "", tok.AccessToken, finalRefresh, expiresAt, "", ""); err != nil {
		return tok.AccessToken, err
	}
	return tok.AccessToken, nil
}

// commandDispatchHermes posts the command to the local Hermes relay's REST
// endpoint. The relay re-signs the protobuf for us; from this CLI's
// perspective it's just an HTTPS POST with the iOS-app bearer.
func commandDispatchHermes(cmd *cobra.Command, flags *rootFlags, cfg *config.Config, v *commandResolvedVehicle, name string, extra []string, choice PathChoice) error {
	// Bearer for the relay: the iOS-app token, NOT the Fleet user token.
	// Hermes proxies the existing owner-api session with re-signing applied.
	bearer := ""
	if cfg != nil {
		bearer = strings.TrimSpace(cfg.AuthHeader())
	}
	if bearer == "" {
		return usageErr(fmt.Errorf("no iOS-app bearer available for Hermes; run `tesla auth login` first"))
	}
	if !strings.HasPrefix(bearer, "Bearer ") {
		bearer = "Bearer " + bearer
	}

	port := commandHermesPort()
	endpoint := fmt.Sprintf("https://localhost:%d/api/1/vehicles/%s/command/%s", port, url.PathEscape(v.VIN), url.PathEscape(name))

	// Build a request body from extra args. The router accepts arbitrary
	// trailing key=value pairs, JSON-encoded. Empty body when no extras.
	body, berr := commandExtraBody(extra)
	if berr != nil {
		return usageErr(berr)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	status, respBody, herr := runHermesHTTPClientFn(ctx, endpoint, bearer, body)
	result := map[string]any{
		"step":     "command",
		"command":  name,
		"vehicle":  v.DisplayName,
		"vin":      v.VIN,
		"path":     choice.Path,
		"sent":     true,
		"endpoint": endpoint,
		"status":   status,
		"body":     strings.TrimSpace(respBody),
	}
	if herr != nil {
		result["error"] = herr.Error()
		_ = printJSONFiltered(cmd.OutOrStdout(), result, flags)
		return fmt.Errorf("hermes POST %s failed: %w", endpoint, herr)
	}
	if status < 200 || status >= 300 {
		_ = printJSONFiltered(cmd.OutOrStdout(), result, flags)
		return fmt.Errorf("hermes POST %s http %d", endpoint, status)
	}
	return printJSONFiltered(cmd.OutOrStdout(), result, flags)
}

// commandExtraBody encodes trailing positional args into a JSON object. The
// router accepts patterns like `set_charge_limit -- percent=80`; we walk the
// extras for `key=value` shapes and emit `{"percent":"80"}`. Empty when no
// k=v args. Bare values without an `=` are illegal at this layer so the
// router doesn't have to know the command's parameter names.
func commandExtraBody(extras []string) ([]byte, error) {
	if len(extras) == 0 {
		return nil, nil
	}
	obj := map[string]string{}
	for _, e := range extras {
		k, v, ok := strings.Cut(e, "=")
		if !ok || strings.TrimSpace(k) == "" {
			return nil, fmt.Errorf("invalid command arg %q: expected key=value", e)
		}
		obj[k] = v
	}
	return json.Marshal(obj)
}

// runHermesHTTPClientReal performs the HTTPS POST against the local Hermes
// relay. Trusts the relay's self-signed cert by loading it from
// ~/.config/tesla-pp-cli/relay-cert.pem. Falls back to InsecureSkipVerify
// only when the cert file isn't readable (e.g. the relay was started via a
// custom path and the env override is set).
func runHermesHTTPClientReal(ctx context.Context, endpoint, bearer string, body []byte) (int, string, error) {
	tlsCfg, terr := buildHermesTLSConfig()
	if terr != nil {
		return 0, "", terr
	}
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, reqBody)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Authorization", bearer)
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), nil
}

// buildHermesTLSConfig returns a tls.Config that trusts the relay's
// self-signed cert. Two cases are valid: cert file absent (intentional test
// override path or pre-first-relay-start) -> insecure-skip-verify against
// localhost; cert file present and parses cleanly -> pinned trust. A third
// case -- cert file present but unparseable -- is a relay config error and
// returns the error instead of silently degrading to insecure mode.
func buildHermesTLSConfig() (*tls.Config, error) {
	paths, err := newRelayPaths()
	if err != nil {
		return nil, err
	}
	pemBytes, rerr := os.ReadFile(paths.CertPEM)
	if rerr != nil {
		// Relay cert not on disk (test path or env override). Skip verify;
		// the endpoint is hardcoded to localhost.
		return &tls.Config{InsecureSkipVerify: true}, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		// Cert file exists but contains no usable PEM blocks. This is a
		// relay-side configuration error (corrupt/empty cert file), not a
		// missing-cert case; surface it instead of silently dropping verify.
		return nil, fmt.Errorf("relay cert at %s exists but contains no valid PEM blocks; remove the file and re-run `tesla relay start` to regenerate", paths.CertPEM)
	}
	return &tls.Config{RootCAs: pool}, nil
}

// commandDispatchBLE prints the exact tesla-control -ble recipe and exits
// zero. The CLI does not wrap BLE; it teaches.
func commandDispatchBLE(cmd *cobra.Command, flags *rootFlags, v *commandResolvedVehicle, name string, extra []string, choice PathChoice) error {
	keyHint := "$HOME/.tesla/" + v.VIN + "-private.pem"
	recipe := fmt.Sprintf(
		"tesla-control -ble -vin %s -key-file %s %s",
		v.VIN, keyHint, name,
	)
	if len(extra) > 0 {
		recipe += " " + strings.Join(extra, " ")
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"step":    "command",
		"command": name,
		"vehicle": v.DisplayName,
		"vin":     v.VIN,
		"path":    PathBLE,
		"sent":    false,
		"reason":  choice.Reason,
		"recipe":  recipe,
		"hint":    "BLE requires laptop within ~30ft of the vehicle. Run the printed command on the host with Bluetooth enabled.",
	}, flags)
}

// ensure we don't drop imports that are conditionally used.
var (
	_ = errors.New
)

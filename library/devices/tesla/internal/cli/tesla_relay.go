// tesla relay - manage the external `tesla-http-proxy` (Hermes/owner-api relay)
// subprocess. This is the daemon shape: a long-lived background process that
// outlives the CLI invocation, with PID file + log + TLS cert under
// ~/.config/tesla-pp-cli/. See U3 in 2026-05-22-001 plan.
//
// Hand-coded; out-of-tree from the generator.
package cli

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

const (
	relayDefaultPort        = 4443
	relayBinaryName         = "tesla-http-proxy"
	relayDirName            = "tesla-pp-cli"
	relayPIDFile            = "relay.pid"
	relayPortFile           = "relay.port"
	relayLogFile            = "relay.log"
	relayCertFile           = "relay-cert.pem"
	relayKeyFile            = "relay-key.pem"
	relayShutdownGrace      = 5 * time.Second
	relayStartupConfirm     = 2 * time.Second
	relaySubprocessMinAlive = 5 * time.Second
)

// runRelaySubprocessFn is the seam tests use to stub subprocess spawn. In
// production it forks a detached `tesla-http-proxy` and returns the PID. The
// signature returns the running PID, the absolute port (resolved from flags),
// and any error from the spawn itself (NOT from the subprocess's later exit).
var runRelaySubprocessFn = launchRelayDetached

// signalProcessFn is the seam tests use to stub signal delivery. Production
// dispatches the real syscall via os.Process.Signal.
var signalProcessFn = func(pid int, sig os.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

// processAliveFn checks whether a PID corresponds to a live process. POSIX
// path uses signal 0 (no-op signal with errno-based liveness reporting). The
// seam is exported so tests can simulate "PID file points at a dead process."
var processAliveFn = func(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

// relayPaths bundles the four state files the relay manages. Resolved once
// per command so test scopes that move $HOME pick up the new paths.
type relayPaths struct {
	Dir     string
	PIDFile string
	Port    string
	LogFile string
	CertPEM string
	KeyPEM  string
}

func newRelayPaths() (relayPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return relayPaths{}, fmt.Errorf("resolve home: %w", err)
	}
	dir := filepath.Join(home, ".config", relayDirName)
	return relayPaths{
		Dir:     dir,
		PIDFile: filepath.Join(dir, relayPIDFile),
		Port:    filepath.Join(dir, relayPortFile),
		LogFile: filepath.Join(dir, relayLogFile),
		CertPEM: filepath.Join(dir, relayCertFile),
		KeyPEM:  filepath.Join(dir, relayKeyFile),
	}, nil
}

func newRelayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relay",
		Short: "Manage the local Hermes/owner-api relay (tesla-http-proxy subprocess)",
		Long: `Start, stop, and inspect a local ` + "`tesla-http-proxy`" + ` daemon. The relay
re-signs Tesla mobile-app bearer requests so REST commands still reach 2021+
vehicles that require Vehicle Command Protocol. This is the "Hermes path"
(infotainment-domain only - lock/unlock/trunk are VCSEC, not supported here).

Subprocess lifecycle:
  start   - spawn tesla-http-proxy detached, write PID + log files
  stop    - SIGTERM the relay, wait, SIGKILL if needed
  status  - report running/stopped, PID, port, uptime, log tail
  doctor  - end-to-end probe; auto-starts a transient relay if not running

The relay survives across CLI invocations; the CLI does NOT auto-start it on
` + "`tesla command`" + ` dispatch. Start it explicitly before routing through Hermes.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRelayStartCmd(flags))
	cmd.AddCommand(newRelayStopCmd(flags))
	cmd.AddCommand(newRelayStatusCmd(flags))
	cmd.AddCommand(newRelayDoctorCmd(flags))
	return cmd
}

// ---------------------------------------------------------------------------
// start
// ---------------------------------------------------------------------------

func newRelayStartCmd(flags *rootFlags) *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the tesla-http-proxy relay subprocess (detached)",
		Long: `Launches ` + "`tesla-http-proxy -mode owner`" + ` as a detached background process.
Generates a self-signed TLS cert at ~/.config/tesla-pp-cli/relay-cert.pem
on first run; reuses thereafter. PID file at ~/.config/tesla-pp-cli/relay.pid;
log file at relay.log.

Idempotent: re-running while already alive prints "already running, PID X"
and exits zero.

On Windows the relay is not supported; the command prints a hint and exits
zero. Use ` + "`tesla command --via=fleet`" + ` for the Fleet API path instead.`,
		Example: `  tesla-pp-cli relay start
  tesla-pp-cli relay start --port 4443
  tesla-pp-cli relay start --json`,
		Annotations: map[string]string{"mcp:destructive": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"action":      "relay_start",
					"port":        port,
				}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"action":  "relay_start",
					"port":    port,
				}, flags)
			}
			return runRelayStart(cmd, flags, port)
		},
	}
	cmd.Flags().IntVar(&port, "port", relayDefaultPort, "Port the relay listens on (TLS)")
	return cmd
}

func runRelayStart(cmd *cobra.Command, flags *rootFlags, port int) error {
	if runtime.GOOS == "windows" {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"status":   "unsupported_platform",
			"platform": "windows",
			"hint":     "Hermes relay subprocess management not supported on Windows; use Fleet API via `tesla command --via=fleet`",
		}, flags)
	}

	paths, err := newRelayPaths()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		return fmt.Errorf("create relay dir: %w", err)
	}

	// Idempotency: if PID file points at a live process, do nothing.
	if pid, alivePort, alive := readRelayState(paths); alive {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"status":   "already_running",
			"pid":      pid,
			"port":     alivePort,
			"pid_file": paths.PIDFile,
			"log_file": paths.LogFile,
		}, flags)
	}

	// Locate tesla-http-proxy.
	bin, lookErr := findRelayBinary()
	if lookErr != nil {
		return usageErr(fmt.Errorf("%s not found on PATH or in ~/go/bin\n\n%s", relayBinaryName, relayInstallRecipe()))
	}

	// Generate TLS cert if missing.
	if err := ensureRelayCert(paths.CertPEM, paths.KeyPEM); err != nil {
		return fmt.Errorf("generate TLS cert: %w", err)
	}

	// Find the operator's signing private key.
	keyPath, kerr := locateRelayPrivateKey(flags)
	if kerr != nil {
		return usageErr(kerr)
	}

	// Verify the port isn't already taken so we can produce a friendly
	// "use --port" error instead of letting the subprocess crash silently.
	if err := portFree(port); err != nil {
		return usageErr(fmt.Errorf("port :%d in use: %w; pass --port to override", port, err))
	}

	pid, err := runRelaySubprocessFn(relayLaunchSpec{
		Binary:     bin,
		Port:       port,
		CertPEM:    paths.CertPEM,
		KeyPEM:     paths.KeyPEM,
		PrivateKey: keyPath,
		LogPath:    paths.LogFile,
	})
	if err != nil {
		return fmt.Errorf("launch %s: %w", relayBinaryName, err)
	}

	if err := writeRelayState(paths, pid, port); err != nil {
		// Best-effort: don't leak the subprocess if state write fails.
		_ = signalProcessFn(pid, syscall.SIGTERM)
		return fmt.Errorf("record relay state: %w", err)
	}

	// Give the subprocess a brief moment to crash on its own. If it exits
	// within the grace window we clean up and report failure with log tail.
	if err := waitForRelayLiveOrEarlyExit(pid, paths, cmd, flags); err != nil {
		return err
	}

	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"status":   "started",
		"pid":      pid,
		"port":     port,
		"pid_file": paths.PIDFile,
		"log_file": paths.LogFile,
		"hint":     "Use `tesla relay stop` to terminate, `tesla relay status` to inspect.",
	}, flags)
}

// waitForRelayLiveOrEarlyExit polls for up to relayStartupConfirm to confirm
// the process is still alive. If it died during the grace window the state
// files are reaped, the last log tail is bubbled up, and a non-zero error is
// returned.
func waitForRelayLiveOrEarlyExit(pid int, paths relayPaths, cmd *cobra.Command, flags *rootFlags) error {
	deadline := time.Now().Add(relayStartupConfirm)
	for time.Now().Before(deadline) {
		if !processAliveFn(pid) {
			// Subprocess exited during the confirmation window — likely a
			// config error. Clean up the dead state files and surface the
			// log tail so the user can see why.
			tail := readLogTail(paths.LogFile, 10)
			_ = os.Remove(paths.PIDFile)
			_ = os.Remove(paths.Port)
			return fmt.Errorf("%s exited shortly after launch; recent log:\n%s", relayBinaryName, tail)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// ---------------------------------------------------------------------------
// stop
// ---------------------------------------------------------------------------

func newRelayStopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stop",
		Short:       "Stop the running tesla-http-proxy relay (SIGTERM then SIGKILL)",
		Long:        `Reads the PID from ~/.config/tesla-pp-cli/relay.pid, sends SIGTERM, waits up to 5 seconds for clean exit, then SIGKILL. Idempotent: if the relay is not running, prints "not running" and exits zero.`,
		Annotations: map[string]string{"mcp:destructive": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"action":      "relay_stop",
				}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"action":  "relay_stop",
				}, flags)
			}
			return runRelayStop(cmd, flags)
		},
	}
	return cmd
}

func runRelayStop(cmd *cobra.Command, flags *rootFlags) error {
	paths, err := newRelayPaths()
	if err != nil {
		return err
	}
	pid, _, alive := readRelayState(paths)
	if !alive {
		// Tidy up any leftover state files even on the not-running path.
		_ = os.Remove(paths.PIDFile)
		_ = os.Remove(paths.Port)
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"status": "not_running",
		}, flags)
	}

	if err := signalProcessFn(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("SIGTERM pid=%d: %w", pid, err)
	}
	if waitForExit(pid, relayShutdownGrace) {
		_ = os.Remove(paths.PIDFile)
		_ = os.Remove(paths.Port)
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"status": "stopped",
			"pid":    pid,
			"signal": "SIGTERM",
		}, flags)
	}

	// Process ignored SIGTERM; escalate.
	if err := signalProcessFn(pid, syscall.SIGKILL); err != nil {
		return fmt.Errorf("SIGKILL pid=%d: %w", pid, err)
	}
	_ = waitForExit(pid, time.Second)
	_ = os.Remove(paths.PIDFile)
	_ = os.Remove(paths.Port)
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"status": "stopped",
		"pid":    pid,
		"signal": "SIGKILL",
		"hint":   "Process did not exit within " + relayShutdownGrace.String() + " of SIGTERM; escalated to SIGKILL.",
	}, flags)
}

func waitForExit(pid int, max time.Duration) bool {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if !processAliveFn(pid) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return !processAliveFn(pid)
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

func newRelayStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Report relay running/stopped, PID, port, uptime, log tail",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"action":      "relay_status",
				}, flags)
			}
			return runRelayStatus(cmd, flags)
		},
	}
	return cmd
}

func runRelayStatus(cmd *cobra.Command, flags *rootFlags) error {
	paths, err := newRelayPaths()
	if err != nil {
		return err
	}
	pid, port, alive := readRelayState(paths)
	report := map[string]any{
		"pid_file": paths.PIDFile,
		"log_file": paths.LogFile,
	}
	if !alive {
		report["status"] = "stopped"
		report["log_tail"] = readLogTail(paths.LogFile, 10)
		return printJSONFiltered(cmd.OutOrStdout(), report, flags)
	}
	uptime := time.Duration(0)
	if info, err := os.Stat(paths.PIDFile); err == nil {
		uptime = time.Since(info.ModTime()).Round(time.Second)
	}
	report["status"] = "running"
	report["pid"] = pid
	report["port"] = port
	report["uptime_seconds"] = int(uptime.Seconds())
	report["uptime"] = uptime.String()
	report["log_tail"] = readLogTail(paths.LogFile, 10)
	return printJSONFiltered(cmd.OutOrStdout(), report, flags)
}

// ---------------------------------------------------------------------------
// doctor
// ---------------------------------------------------------------------------

func newRelayDoctorCmd(flags *rootFlags) *cobra.Command {
	var testVIN string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "End-to-end relay probe: start if needed, fire synthetic vehicle_data, tear down if self-started",
		Long: `Verifies the relay can answer a synthetic vehicle_data request on the
local HTTPS endpoint. If the relay is not running, doctor starts a transient
instance and tears it down after the probe.

The probe never reaches Tesla's cloud - a 4xx from the relay's URL handler
still proves the local TLS endpoint is healthy. VCSEC warnings (lock/unlock/
trunk not supported via this path) are surfaced regardless of probe outcome.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"verify_noop": true,
					"action":      "relay_doctor",
				}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"action":  "relay_doctor",
				}, flags)
			}
			return runRelayDoctor(cmd, flags, testVIN)
		},
	}
	cmd.Flags().StringVar(&testVIN, "test-vin", "", "Override VIN used for the synthetic probe (default: first vehicle from products, else placeholder)")
	return cmd
}

func runRelayDoctor(cmd *cobra.Command, flags *rootFlags, testVIN string) error {
	if runtime.GOOS == "windows" {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"status":   "unsupported_platform",
			"platform": "windows",
			"hint":     "Hermes relay not supported on Windows; use `tesla command --via=fleet`",
		}, flags)
	}

	paths, err := newRelayPaths()
	if err != nil {
		return err
	}

	checks := []map[string]any{}
	addCheck := func(severity, name, detail string) {
		checks = append(checks, map[string]any{
			"severity": severity,
			"name":     name,
			"detail":   detail,
		})
	}

	// Always-on VCSEC warning per the known proxy limitation.
	addCheck("WARN", "vcsec_unsupported",
		"Hermes path does not support lock/unlock/trunk (VCSEC commands); use `tesla command --via=fleet` or `--via=ble` for those.")

	pid, port, alive := readRelayState(paths)
	selfStarted := false
	if !alive {
		addCheck("INFO", "relay_state", "Relay not running; doctor will start a transient instance and tear it down after the probe.")
		if err := runRelayStart(cmd, flags, relayDefaultPort); err != nil {
			addCheck("FAIL", "relay_start", err.Error())
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status": "FAIL",
				"checks": checks,
			}, flags)
		}
		pid, port, alive = readRelayState(paths)
		if !alive {
			addCheck("FAIL", "relay_start", "relay reported started but PID is not alive")
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status": "FAIL",
				"checks": checks,
			}, flags)
		}
		selfStarted = true
	} else {
		addCheck("OK", "relay_state", fmt.Sprintf("Relay already running at pid=%d port=%d", pid, port))
	}

	// Resolve a VIN to use for the probe. Fall back to a placeholder; the
	// relay will reject it but a connection + TLS handshake + HTTP-level
	// response still proves the local endpoint is healthy.
	vin := strings.TrimSpace(testVIN)
	if vin == "" {
		vin = resolveDoctorVIN(flags)
	}
	if vin == "" {
		vin = "5YJ_DOCTORPROBE_VIN"
	}

	endpoint := fmt.Sprintf("https://localhost:%d/api/1/vehicles/%s/vehicle_data", port, vin)
	status, perr := probeRelayEndpoint(cmd.Context(), endpoint)
	switch {
	case perr != nil:
		addCheck("FAIL", "synthetic_probe", fmt.Sprintf("could not reach %s: %v", endpoint, perr))
	case status >= 200 && status < 500:
		// Anything in 2xx-4xx range proves the relay is alive and answered.
		// 401/404 against a placeholder VIN is the expected healthy result.
		addCheck("OK", "synthetic_probe", fmt.Sprintf("relay responded HTTP %d for synthetic vehicle_data probe (4xx-against-placeholder is expected)", status))
	default:
		addCheck("WARN", "synthetic_probe", fmt.Sprintf("relay responded HTTP %d (unexpected; check log tail)", status))
	}

	// Teardown if we started it.
	if selfStarted {
		if err := runRelayStop(cmd, flags); err != nil {
			addCheck("WARN", "relay_teardown", fmt.Sprintf("could not stop transient relay pid=%d: %v", pid, err))
		} else {
			addCheck("INFO", "relay_teardown", "Transient relay stopped.")
		}
	}

	worst := worstSeverity(checks)
	report := map[string]any{
		"status":       worst,
		"checks":       checks,
		"self_started": selfStarted,
		"endpoint":     endpoint,
	}
	return printJSONFiltered(cmd.OutOrStdout(), report, flags)
}

func worstSeverity(checks []map[string]any) string {
	rank := map[string]int{"OK": 0, "INFO": 1, "WARN": 2, "FAIL": 3}
	worst := "OK"
	for _, c := range checks {
		s, _ := c["severity"].(string)
		if rank[s] > rank[worst] {
			worst = s
		}
	}
	return worst
}

// probeRelayEndpoint fires a single GET against the relay with TLS-skip-verify
// (the relay's self-signed cert isn't in any trust store, by design - users
// don't add a one-off dev cert to their system roots). Returns the HTTP
// status; an error means the transport failed entirely.
func probeRelayEndpoint(ctx context.Context, endpoint string) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, "GET", endpoint, nil)
	if err != nil {
		return 0, err
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // local self-signed cert
		},
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// resolveDoctorVIN tries to pick the first VIN from the user's Fleet creds; if
// nothing useful is configured the caller falls back to a placeholder. This
// is best-effort: we never block the doctor on a network call.
func resolveDoctorVIN(flags *rootFlags) string {
	cfg, err := config.Load(flagsConfigPath(flags))
	if err != nil || cfg == nil {
		return ""
	}
	// Avoid live network probing here; doctor is supposed to be fast.
	// A future enhancement could call /api/1/products and pick the first
	// VIN, but the placeholder path is the contract the test scenario
	// describes ("4xx but healthy connection").
	return ""
}

// ---------------------------------------------------------------------------
// helpers: binary lookup, TLS cert, PID/port state, log tail
// ---------------------------------------------------------------------------

// findRelayBinary searches PATH then ~/go/bin for `tesla-http-proxy`. Returns
// an error when neither location holds it. Callers wrap with the install
// recipe.
func findRelayBinary() (string, error) {
	if p, err := exec.LookPath(relayBinaryName); err == nil {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err == nil {
		candidate := filepath.Join(home, "go", "bin", relayBinaryName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s not found", relayBinaryName)
}

// relayInstallRecipe returns the verbatim recipe the doctor and start
// commands print when tesla-http-proxy is missing. Single source of truth.
func relayInstallRecipe() string {
	return `Install recipe:
  git clone https://github.com/lotharbach/tesla-command-proxy
  cd tesla-command-proxy
  go build -o "$(go env GOPATH)/bin/tesla-http-proxy" ./cmd/tesla-http-proxy

NOTE: This uses lotharbach's archived fork (April 2025) because it carries the
` + "`-mode owner`" + ` patch the upstream teslamotors/vehicle-command does NOT have.
A plain ` + "`go install`" + ` does not work due to replace directives in the fork's
go.mod; use ` + "`go build`" + ` from the cloned tree.`
}

// locateRelayPrivateKey returns the private signing key path for the relay,
// preferring config.Fleet.PrivateKeyPath, then env TESLA_FLEET_KEY_FILE, then
// ~/.tesla/snowflake-private.pem (the BLE-pair default), then errors with a
// remediation hint.
func locateRelayPrivateKey(flags *rootFlags) (string, error) {
	cfg, _ := config.Load(flagsConfigPath(flags))
	if cfg != nil && strings.TrimSpace(cfg.Fleet.PrivateKeyPath) != "" {
		p := cfg.Fleet.PrivateKeyPath
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if v := strings.TrimSpace(os.Getenv("TESLA_FLEET_KEY_FILE")); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v, nil
		}
	}
	home, err := os.UserHomeDir()
	if err == nil {
		candidate := filepath.Join(home, ".tesla", "snowflake-private.pem")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("no private signing key found; run `tesla auth fleet-template --gen-key` (Fleet) or `tesla auth ble-pair` (BLE) first to generate one")
}

// ensureRelayCert generates a self-signed P256 cert (CN=localhost, SAN
// localhost + 127.0.0.1, 1 year) at certPath + keyPath if either is missing.
// Key file is written 0600.
func ensureRelayCert(certPath, keyPath string) error {
	if fileExists(certPath) && fileExists(keyPath) {
		return nil
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate P256 key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("serial: %w", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "localhost", Organization: []string{"tesla-pp-cli relay"}},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("create cert: %w", err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		return err
	}
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// readRelayState reads PID + port files, then verifies the PID is alive.
// Returns (pid, port, alive). Missing or unreadable files -> (0,0,false).
func readRelayState(paths relayPaths) (int, int, bool) {
	pidBytes, err := os.ReadFile(paths.PIDFile)
	if err != nil {
		return 0, 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil || pid <= 0 {
		return 0, 0, false
	}
	port := 0
	if portBytes, err := os.ReadFile(paths.Port); err == nil {
		if p, err := strconv.Atoi(strings.TrimSpace(string(portBytes))); err == nil {
			port = p
		}
	}
	alive := processAliveFn(pid)
	return pid, port, alive
}

func writeRelayState(paths relayPaths, pid, port int) error {
	if err := os.WriteFile(paths.PIDFile, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(paths.Port, []byte(strconv.Itoa(port)+"\n"), 0o644); err != nil {
		return err
	}
	return nil
}

// readLogTail returns the last `lines` newline-delimited lines of the log
// file, joined by "\n". Returns "" on read error or empty file.
func readLogTail(path string, lines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(all) <= lines {
		return strings.Join(all, "\n")
	}
	return strings.Join(all[len(all)-lines:], "\n")
}

// portFree returns nil when the given port is available for binding on
// 127.0.0.1. A non-nil error indicates the port is taken (or otherwise
// unbindable, which is functionally the same thing for the user).
func portFree(port int) error {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	return l.Close()
}

// relayLaunchSpec is the shape the platform-specific launcher consumes. Keeps
// the cross-platform signature stable; the platform file adds Setsid/equiv.
type relayLaunchSpec struct {
	Binary     string
	Port       int
	CertPEM    string
	KeyPEM     string
	PrivateKey string
	LogPath    string
}

// relayLaunchArgs returns the canonical argv for tesla-http-proxy. Pinned in
// one place so tests and the launcher agree.
func relayLaunchArgs(spec relayLaunchSpec) []string {
	return []string{
		"-mode", "owner",
		"-cert", spec.CertPEM,
		"-key-file", spec.KeyPEM,
		"-tls-key", spec.PrivateKey,
		"-port", strconv.Itoa(spec.Port),
	}
}

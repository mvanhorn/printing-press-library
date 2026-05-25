// tesla auth ble-pair — guided BLE virtual-key enrollment for Tesla VCP cars.
//
// Wraps the upstream `tesla-control -ble add-key-request ...` flow plus a
// `session-info` confirmation pass so a first-time user can pair their
// laptop's EC P256 keypair with their car in one CLI command. The CLI does
// not manage Bluetooth itself; it relies on tesla-control and the host BLE
// stack. The patterns here follow tesla_auth_via_subprocess.go (foreground
// one-shot exec with context timeout and captured stdout/stderr).
//
// Hand-coded; lives outside the generator's emit set so it survives regens.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
)

const (
	teslaControlBinary = "tesla-control"
	teslaKeygenBinary  = "tesla-keygen"

	// blePairTimeout bounds a single tesla-control invocation. BLE handshake
	// plus NFC tap can legitimately take ~30s; we go 60s to leave headroom.
	blePairTimeout = 60 * time.Second
	// blePairPollMax bounds the session-info confirmation loop after the
	// add-key-request returns. Up to ~30s total.
	blePairPollMax     = 10
	blePairPollBackoff = 3 * time.Second
)

// detectTeslaControlBinary returns the absolute path of tesla-control if
// resolvable via PATH or at ~/go/bin/tesla-control. Empty string means absent.
func detectTeslaControlBinary() string {
	if p, err := exec.LookPath(teslaControlBinary); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		cand := filepath.Join(home, "go", "bin", teslaControlBinary)
		if info, err := os.Stat(cand); err == nil && !info.IsDir() {
			return cand
		}
	}
	return ""
}

// detectTeslaKeygenBinary mirrors detectTeslaControlBinary for tesla-keygen.
// Currently advisory: we surface it in the missing-dependency message so the
// user installs both binaries at once, since they ship from the same module.
func detectTeslaKeygenBinary() string {
	if p, err := exec.LookPath(teslaKeygenBinary); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		cand := filepath.Join(home, "go", "bin", teslaKeygenBinary)
		if info, err := os.Stat(cand); err == nil && !info.IsDir() {
			return cand
		}
	}
	return ""
}

func newBlePairCmd(flags *rootFlags) *cobra.Command {
	var vin, keyFile, role string
	cmd := &cobra.Command{
		Use:   "ble-pair",
		Short: "Enroll a virtual key on the car over BLE via tesla-control",
		Long: `Pair your laptop's EC P256 public key with a Tesla over BLE using
the upstream tesla-control binary (teslamotors/vehicle-command).

Prerequisites:
  - tesla-control on PATH (or at ~/go/bin/tesla-control)
  - A private key for this VIN, located at one of:
      --key-file <path>
      ~/.tesla/<vin>-private.pem
  - Laptop within ~30ft of the vehicle with Bluetooth enabled

Flow:
  1. tesla-control -ble -vin <vin> -key-file <priv> add-key-request <pub> <role> cloud_key
  2. Walk to the car and tap your physical key card (or use the in-app
     phone-key tap) when prompted.
  3. tesla-control -ble -vin <vin> -key-file <priv> session-info <pub> vcsec
     to confirm the key handle is enrolled.

Idempotent: re-running on an already-enrolled key reports the existing handle
and exits zero without re-prompting for an NFC tap.`,
		Annotations: map[string]string{
			// Mutates vehicle state by enrolling a key in the VCSEC keystore.
			"mcp:destructive": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBlePair(cmd, flags, vin, keyFile, role)
		},
	}
	cmd.Flags().StringVar(&vin, "vin", "", "Vehicle VIN (required)")
	cmd.Flags().StringVar(&keyFile, "key-file", "", "Path to the private key PEM (overrides ~/.tesla/<vin>-private.pem)")
	cmd.Flags().StringVar(&role, "role", "owner", "Role for the enrolled key (owner|driver)")
	_ = cmd.MarkFlagRequired("vin")
	return cmd
}

func runBlePair(cmd *cobra.Command, flags *rootFlags, vin, keyFile, role string) error {
	// Normalize + validate role first so the error path doesn't depend on
	// tesla-control being installed.
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "owner" && role != "driver" {
		return usageErr(fmt.Errorf("invalid --role %q: must be owner or driver", role))
	}
	if strings.TrimSpace(vin) == "" {
		return usageErr(errors.New("--vin is required"))
	}

	// Verify-mode short-circuit: print intent, no subprocess.
	if cliutil.IsVerifyEnv() {
		intent := map[string]any{
			"verify_noop": true,
			"step":        "ble-pair",
			"vin":         vin,
			"role":        role,
			"key_file":    keyFile,
		}
		return printJSONFiltered(cmd.OutOrStdout(), intent, flags)
	}

	// Detect tesla-control. If missing, surface both prerequisite binaries
	// in a single message so the user installs them together.
	bin := detectTeslaControlBinary()
	if bin == "" {
		return usageErr(fmt.Errorf(
			"tesla-control not found on PATH or at ~/go/bin/%s; install both:\n"+
				"  go install github.com/teslamotors/vehicle-command/cmd/tesla-control@latest\n"+
				"  go install github.com/teslamotors/vehicle-command/cmd/tesla-keygen@latest",
			teslaControlBinary,
		))
	}

	// Resolve the private key file. --key-file wins; otherwise look for
	// ~/.tesla/<vin>-private.pem.
	privPath, err := resolveBlePrivateKey(vin, keyFile)
	if err != nil {
		return err
	}
	// Derive the public-key path by convention: sibling .public.pem (matching
	// the fleet-template pattern). The upstream tesla-control add-key-request
	// takes a PEM-encoded SPKI public key as a positional arg path.
	pubPath := blePublicKeyPath(privPath)
	if _, statErr := os.Stat(pubPath); statErr != nil {
		return fmt.Errorf("public key not found at %s (expected sibling of %s); run `tesla auth fleet-template --gen-key` to create the keypair", pubPath, privPath)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), blePairTimeout)
	defer cancel()

	// Step 1: idempotency probe. session-info reports the handle if the key
	// is already enrolled. Non-fatal if it fails (car asleep / out of range).
	if handle, ok := blePairCheckEnrolled(ctx, bin, vin, privPath, pubPath); ok {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"status":   "already_enrolled",
			"handle":   handle,
			"vin":      vin,
			"role":     role,
			"key_file": redactPath(privPath),
		}, flags)
	}

	// Step 2: add-key-request. Prompts the car to expect an NFC tap.
	addArgs := []string{
		"-ble",
		"-vin", vin,
		"-key-file", privPath,
		"add-key-request", pubPath, role, "cloud_key",
	}
	addOut, addErrOut, addErr := runBleSubprocess(ctx, bin, addArgs)
	if addErr != nil {
		return fmt.Errorf("tesla-control add-key-request failed: %w\n%s", addErr, truncateOutput(strings.TrimSpace(addErrOut+"\n"+addOut), 400))
	}

	fmt.Fprintln(cmd.OutOrStderr(), "BLE handshake complete. Walk to the vehicle and tap your physical Tesla key card (or use the in-app phone-key tap) within ~30 seconds. The car's display will prompt to confirm adding the new key.")

	// Step 3: poll session-info for the new handle. Each retry is a fresh
	// subprocess invocation, since tesla-control returns once.
	var lastHandle string
	for i := 0; i < blePairPollMax; i++ {
		handle, ok := blePairCheckEnrolled(ctx, bin, vin, privPath, pubPath)
		if ok {
			lastHandle = handle
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("tesla-control session-info timed out before key was confirmed (%w)", ctx.Err())
		case <-time.After(blePairPollBackoff):
		}
	}
	if lastHandle == "" {
		return fmt.Errorf("tesla-control did not report the new key handle after %d polls; if you missed the NFC tap window, re-run `tesla auth ble-pair --vin %s`", blePairPollMax, vin)
	}

	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"status":   "enrolled",
		"handle":   lastHandle,
		"vin":      vin,
		"role":     role,
		"key_file": redactPath(privPath),
	}, flags)
}

// resolveBlePrivateKey returns the absolute path to the private key file.
// --key-file wins; otherwise it falls back to ~/.tesla/<vin>-private.pem.
// Returns a usage error if no file is resolvable on disk.
func resolveBlePrivateKey(vin, keyFile string) (string, error) {
	if strings.TrimSpace(keyFile) != "" {
		abs, err := filepath.Abs(keyFile)
		if err != nil {
			return "", usageErr(fmt.Errorf("resolve --key-file: %w", err))
		}
		if _, err := os.Stat(abs); err != nil {
			return "", usageErr(fmt.Errorf("--key-file %s not readable: %w", abs, err))
		}
		return abs, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("cannot resolve home dir to look up private key for VIN %s", vin)
	}
	cand := filepath.Join(home, ".tesla", vin+"-private.pem")
	if _, err := os.Stat(cand); err == nil {
		return cand, nil
	}
	return "", usageErr(fmt.Errorf("no private key found at %s; run `tesla auth fleet-template --gen-key` to create one, or pass --key-file <path>", cand))
}

// blePublicKeyPath returns the conventional sibling public-key path for a
// given private-key path. Matches the keygen convention: foo-private.pem
// pairs with foo-public.pem; otherwise we append .public.pem.
func blePublicKeyPath(privPath string) string {
	if strings.HasSuffix(privPath, "-private.pem") {
		return strings.TrimSuffix(privPath, "-private.pem") + "-public.pem"
	}
	if strings.HasSuffix(privPath, ".pem") {
		return strings.TrimSuffix(privPath, ".pem") + ".public.pem"
	}
	return privPath + ".public.pem"
}

// redactPath returns a short form of the absolute path suitable for logs:
// the basename only. Avoids leaking the user's home dir in machine output.
func redactPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Base(p)
}

// blePairCheckEnrolled invokes `tesla-control session-info` and parses the
// output for a key handle. Returns (handle, true) when the key is enrolled.
// Errors are swallowed: the caller treats any failure as not-yet-enrolled.
func blePairCheckEnrolled(ctx context.Context, bin, vin, privPath, pubPath string) (string, bool) {
	args := []string{
		"-ble",
		"-vin", vin,
		"-key-file", privPath,
		"session-info", pubPath, "vcsec",
	}
	out, _, err := runBleSubprocess(ctx, bin, args)
	if err != nil {
		return "", false
	}
	if h := parseBleSessionInfoHandle(out); h != "" {
		return h, true
	}
	return "", false
}

// runBleSubprocess is the foreground exec primitive used by every step. It
// mirrors runTeslaAuthBinary in tesla_auth_via_subprocess.go but captures
// stderr separately so callers can surface tesla-control's own error text.
//
// Exported as a var so tests can stub the subprocess without planting a fake
// binary on PATH for every scenario.
var runBleSubprocess = func(ctx context.Context, bin string, args []string) (stdout, stderr string, err error) {
	c := exec.CommandContext(ctx, bin, args...)
	var so, se bytes.Buffer
	c.Stdout = &so
	c.Stderr = &se
	if runErr := c.Run(); runErr != nil {
		return so.String(), se.String(), runErr
	}
	return so.String(), se.String(), nil
}

// parseBleSessionInfoHandle extracts the key handle from a tesla-control
// session-info output. Upstream output isn't fully stable across versions,
// so we accept a few common shapes: `handle: 7`, `key_handle = 7`,
// `Handle 7`, or a `"handle": 7` JSON-ish field. Returns "" if no handle
// pattern matches.
func parseBleSessionInfoHandle(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, re := range bleHandleREs {
		m := re.FindStringSubmatch(raw)
		if len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

var bleHandleREs = []*regexp.Regexp{
	regexp.MustCompile(`"handle"\s*:\s*(\d+)`),
	regexp.MustCompile(`(?i)(?:key_)?handle\s*[:=]\s*(\d+)\b`),
	regexp.MustCompile(`(?i)\bhandle\s+(\d+)\b`),
}

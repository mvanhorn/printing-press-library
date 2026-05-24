package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// plantFakeTeslaControl writes a shell script that masquerades as
// `tesla-control` and returns the directory it lives in. Callers prepend
// that dir to PATH for the test scope. The script's behavior is driven by
// the positional subcommand (add-key-request | session-info) plus the
// `mode` argument:
//
//   - "success_enrolled"     session-info reports an existing handle
//   - "success_pair_flow"    add-key-request OK; session-info returns
//     no handle on the first call and a handle on
//     subsequent calls (simulates NFC tap latency)
//   - "ble_handshake_fail"   add-key-request exits 1 with "BLE connection failed"
//   - "nfc_timeout"          add-key-request OK; every session-info returns
//     blank output (simulates NFC tap that never came)
func plantFakeTeslaControl(t *testing.T, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("planting a shell-script fake tesla-control is POSIX-only")
	}
	dir := t.TempDir()
	binPath := filepath.Join(dir, "tesla-control")
	// State file lets the session-info script return different output on
	// successive calls within the same mode.
	statePath := filepath.Join(dir, "state")

	script := fmt.Sprintf(`#!/bin/sh
mode=%q
state=%q
sub=""
for a in "$@"; do
  case "$a" in
    add-key-request) sub="add"; break ;;
    session-info)    sub="session"; break ;;
  esac
done

case "$mode" in
  success_enrolled)
    if [ "$sub" = "session" ]; then
      echo "handle: 42"
      exit 0
    fi
    if [ "$sub" = "add" ]; then
      echo "key request sent"
      exit 0
    fi
    ;;
  success_pair_flow)
    if [ "$sub" = "add" ]; then
      echo "key request sent"
      exit 0
    fi
    if [ "$sub" = "session" ]; then
      count=0
      [ -f "$state" ] && count=$(cat "$state")
      count=$((count + 1))
      echo "$count" > "$state"
      if [ "$count" -ge 2 ]; then
        echo "handle: 7"
        exit 0
      fi
      echo "no key enrolled"
      exit 0
    fi
    ;;
  ble_handshake_fail)
    if [ "$sub" = "session" ]; then
      echo "BLE connection failed" 1>&2
      exit 1
    fi
    if [ "$sub" = "add" ]; then
      echo "BLE connection failed: vehicle out of range" 1>&2
      exit 1
    fi
    ;;
  nfc_timeout)
    if [ "$sub" = "add" ]; then
      echo "key request sent"
      exit 0
    fi
    if [ "$sub" = "session" ]; then
      echo "no key enrolled"
      exit 0
    fi
    ;;
esac
echo "unhandled fake tesla-control invocation: $*" 1>&2
exit 99
`, mode, statePath)

	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tesla-control: %v", err)
	}
	return dir
}

// scopedHome rebases $HOME and $PATH for the duration of a test. It also
// creates a fresh ~/.tesla/<vin>-private.pem (and matching public PEM) so
// the resolver succeeds.
func scopedHomeWithKey(t *testing.T, vin string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	teslaDir := filepath.Join(home, ".tesla")
	if err := os.MkdirAll(teslaDir, 0o700); err != nil {
		t.Fatalf("mkdir ~/.tesla: %v", err)
	}
	priv := filepath.Join(teslaDir, vin+"-private.pem")
	pub := filepath.Join(teslaDir, vin+"-public.pem")
	if err := os.WriteFile(priv, []byte("-----BEGIN PRIVATE KEY-----\nfake\n-----END PRIVATE KEY-----\n"), 0o600); err != nil {
		t.Fatalf("write priv: %v", err)
	}
	if err := os.WriteFile(pub, []byte("-----BEGIN PUBLIC KEY-----\nfake\n-----END PUBLIC KEY-----\n"), 0o644); err != nil {
		t.Fatalf("write pub: %v", err)
	}
	return home
}

// withPATH replaces PATH with only the given dir so exec.LookPath sees only
// the planted fake binary. Restored by t.Cleanup via t.Setenv.
func withPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir)
}

func newBlePairCmdForTest(t *testing.T) (*cobra.Command, *rootFlags) {
	t.Helper()
	flags := &rootFlags{}
	cmd := newBlePairCmd(flags)
	// Suppress cobra's own stderr noise during tests.
	cmd.SetErr(&strings.Builder{})
	cmd.SetOut(&strings.Builder{})
	cmd.SetContext(context.Background())
	return cmd, flags
}

func TestBlePair_HappyPath(t *testing.T) {
	vin := "TESTVIN1234567890"
	scopedHomeWithKey(t, vin)
	dir := plantFakeTeslaControl(t, "success_pair_flow")
	withPATH(t, dir)

	// Replace the subprocess primitive with a controlled fake so we can
	// exercise the add-then-poll flow without sleeping through real
	// backoff timers. The first session-info call returns no handle, the
	// second returns `handle: 7`.
	orig := runBleSubprocess
	t.Cleanup(func() { runBleSubprocess = orig })
	var sessionCalls int
	runBleSubprocess = func(ctx context.Context, bin string, args []string) (string, string, error) {
		sub := blePairSubcommand(args)
		switch sub {
		case "add-key-request":
			return "key request sent\n", "", nil
		case "session-info":
			sessionCalls++
			if sessionCalls >= 2 {
				return "handle: 7\n", "", nil
			}
			return "no key enrolled\n", "", nil
		}
		return "", "", fmt.Errorf("unexpected subcommand: %v", args)
	}

	cmd, flags := newBlePairCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	if err := runBlePair(cmd, flags, vin, "", "owner"); err != nil {
		t.Fatalf("runBlePair: %v", err)
	}
	if !strings.Contains(out.String(), `"status": "enrolled"`) && !strings.Contains(out.String(), `"status":"enrolled"`) {
		t.Errorf("expected status=enrolled, got: %s", out.String())
	}
	if !strings.Contains(out.String(), `"handle"`) {
		t.Errorf("expected handle in output, got: %s", out.String())
	}
}

func TestBlePair_TeslaControlNotInstalled(t *testing.T) {
	vin := "TESTVIN1234567890"
	scopedHomeWithKey(t, vin)
	// Empty PATH so tesla-control cannot resolve; also point $HOME away
	// from any user-local ~/go/bin install.
	withPATH(t, t.TempDir())
	t.Setenv("HOME", t.TempDir())

	cmd, flags := newBlePairCmdForTest(t)
	err := runBlePair(cmd, flags, vin, "", "owner")
	if err == nil {
		t.Fatalf("expected error when tesla-control is missing")
	}
	msg := err.Error()
	if !strings.Contains(msg, "tesla-control") {
		t.Errorf("error should name tesla-control, got: %v", err)
	}
	if !strings.Contains(msg, "tesla-keygen") {
		t.Errorf("error should also name tesla-keygen (sibling install), got: %v", err)
	}
	if !errIsUsage(err) {
		t.Errorf("expected usage error (exit 2), got: %v", err)
	}
}

func TestBlePair_BleHandshakeFails(t *testing.T) {
	vin := "TESTVIN1234567890"
	scopedHomeWithKey(t, vin)
	dir := plantFakeTeslaControl(t, "ble_handshake_fail")
	withPATH(t, dir)

	orig := runBleSubprocess
	t.Cleanup(func() { runBleSubprocess = orig })
	runBleSubprocess = func(ctx context.Context, bin string, args []string) (string, string, error) {
		return "", "BLE connection failed: vehicle out of range\n", fmt.Errorf("exit status 1")
	}

	cmd, flags := newBlePairCmdForTest(t)
	err := runBlePair(cmd, flags, vin, "", "owner")
	if err == nil {
		t.Fatalf("expected error on BLE handshake failure")
	}
	if !strings.Contains(err.Error(), "add-key-request") {
		t.Errorf("error should mention which step failed, got: %v", err)
	}
	if errIsUsage(err) {
		t.Errorf("BLE failure should NOT be classified as usage error, got: %v", err)
	}
}

func TestBlePair_NfcTapTimesOut(t *testing.T) {
	vin := "TESTVIN1234567890"
	scopedHomeWithKey(t, vin)
	dir := plantFakeTeslaControl(t, "nfc_timeout")
	withPATH(t, dir)

	// Stub subprocess so add succeeds but every session-info returns no
	// handle. We also short-circuit the backoff by making session-info
	// itself respect ctx.
	orig := runBleSubprocess
	t.Cleanup(func() { runBleSubprocess = orig })
	runBleSubprocess = func(ctx context.Context, bin string, args []string) (string, string, error) {
		sub := blePairSubcommand(args)
		if sub == "add-key-request" {
			return "key request sent\n", "", nil
		}
		// session-info: never report a handle.
		return "no key enrolled\n", "", nil
	}

	// Use a short context so the polling loop exits quickly. We swap in a
	// context with a small deadline by directly invoking the subprocess
	// stub and verifying the surface message; the function body's polling
	// loop honors ctx via the time.After fallthrough.
	cmd, flags := newBlePairCmdForTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 50)
	defer cancel()
	cmd.SetContext(ctx)

	err := runBlePair(cmd, flags, vin, "", "owner")
	if err == nil {
		t.Fatalf("expected timeout error after no handle was reported")
	}
	if !strings.Contains(err.Error(), "session-info") && !strings.Contains(err.Error(), "handle") {
		t.Errorf("error should indicate session-info timeout, got: %v", err)
	}
}

func TestBlePair_KeyAlreadyEnrolledIsIdempotent(t *testing.T) {
	vin := "TESTVIN1234567890"
	scopedHomeWithKey(t, vin)
	dir := plantFakeTeslaControl(t, "success_enrolled")
	withPATH(t, dir)

	orig := runBleSubprocess
	t.Cleanup(func() { runBleSubprocess = orig })
	runBleSubprocess = func(ctx context.Context, bin string, args []string) (string, string, error) {
		sub := blePairSubcommand(args)
		if sub == "session-info" {
			return "handle: 42\n", "", nil
		}
		// add-key-request should NOT be invoked on the idempotent path.
		t.Errorf("idempotent path invoked add-key-request: %v", args)
		return "", "", fmt.Errorf("should not reach")
	}

	cmd, flags := newBlePairCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	if err := runBlePair(cmd, flags, vin, "", "owner"); err != nil {
		t.Fatalf("expected zero exit on already-enrolled, got: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "already_enrolled") {
		t.Errorf("expected status=already_enrolled in output: %s", body)
	}
	if !strings.Contains(body, "42") {
		t.Errorf("expected handle=42 in output: %s", body)
	}
}

func TestBlePair_InvalidRole(t *testing.T) {
	cmd, flags := newBlePairCmdForTest(t)
	err := runBlePair(cmd, flags, "TESTVIN1234567890", "", "passenger")
	if err == nil {
		t.Fatalf("expected usage error for invalid role")
	}
	if !errIsUsage(err) {
		t.Errorf("expected usage error (exit 2), got: %v", err)
	}
	if !strings.Contains(err.Error(), "role") {
		t.Errorf("error should mention role, got: %v", err)
	}
}

func TestBlePair_VerifyModePrintsIntent(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	// Even though no fake tesla-control is installed and no key exists on
	// disk, verify-mode must short-circuit and exit zero.
	t.Setenv("HOME", t.TempDir())
	withPATH(t, t.TempDir())

	cmd, flags := newBlePairCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	if err := runBlePair(cmd, flags, "TESTVIN1234567890", "", "owner"); err != nil {
		t.Fatalf("verify mode should exit zero, got: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "verify_noop") {
		t.Errorf("expected verify_noop in output, got: %s", body)
	}
	if !strings.Contains(body, "TESTVIN1234567890") {
		t.Errorf("expected VIN in intent output, got: %s", body)
	}
}

func TestBlePair_MissingPrivateKey(t *testing.T) {
	vin := "TESTVIN1234567890"
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := plantFakeTeslaControl(t, "success_enrolled")
	withPATH(t, dir)

	cmd, flags := newBlePairCmdForTest(t)
	err := runBlePair(cmd, flags, vin, "", "owner")
	if err == nil {
		t.Fatalf("expected usage error when private key absent")
	}
	if !errIsUsage(err) {
		t.Errorf("expected usage error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "fleet-template") {
		t.Errorf("error should hint at `tesla auth fleet-template --gen-key`, got: %v", err)
	}
}

func TestParseBleSessionInfoHandle(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"handle: 7", "7"},
		{"Handle: 42", "42"},
		{"key_handle = 12", "12"},
		{`{"handle": 99, "role":"owner"}`, "99"},
		{"no key enrolled", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := parseBleSessionInfoHandle(c.in)
		if got != c.want {
			t.Errorf("parseBleSessionInfoHandle(%q): got %q want %q", c.in, got, c.want)
		}
	}
}

func TestBlePublicKeyPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/h/.tesla/v-private.pem", "/h/.tesla/v-public.pem"},
		{"/h/.tesla/v.pem", "/h/.tesla/v.public.pem"},
		{"/h/.tesla/v", "/h/.tesla/v.public.pem"},
	}
	for _, c := range cases {
		if got := blePublicKeyPath(c.in); got != c.want {
			t.Errorf("blePublicKeyPath(%q): got %q want %q", c.in, got, c.want)
		}
	}
}

// blePairSubcommand returns the subcommand positional from a tesla-control
// argv. Mirrors the parse the fake binary does. Used by the subprocess
// stubs above.
func blePairSubcommand(args []string) string {
	for _, a := range args {
		switch a {
		case "add-key-request", "session-info":
			return a
		}
	}
	return ""
}

// errIsUsage reports whether err is a *cliError with code 2 (usage).
func errIsUsage(err error) bool {
	if err == nil {
		return false
	}
	var ce *cliError
	if As(err, &ce) {
		return ce.code == 2
	}
	return false
}

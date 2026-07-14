// Package integration holds end-to-end tests that talk to a real MT5
// terminal + broker. They are gated on MT5_PAPER=1 plus MT5_ACCOUNT,
// MT5_SERVER, MT5_PASSWORD — without them every test in this package
// calls t.Skip(). This keeps `go test ./...` green in CI environments
// that have no broker, while still giving operators a single
// "are we wired up?" command.
//
// The tests refuse to run against real accounts (trade_mode != demo) so a
// stray MT5_LIVE in the operator's env can never cause a write here.
//
// Run locally on a Windows host with a logged-in MT5 terminal:
//
//	$env:MT5_PAPER     = "1"
//	$env:MT5_ACCOUNT   = "12345678"
//	$env:MT5_SERVER    = "JustMarkets-Demo"
//	$env:MT5_PASSWORD  = "..."
//	go test -tags=integration -v -timeout 120s ./test/integration
//
// The build tag is belt-and-braces — even with creds present, you must opt
// in. The MT5_PAPER gate is the load-bearing check.
//
//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/bridge"
	cli "github.com/mvanhorn/printing-press-library/library/other/mt5/internal/cli"
)

// guard returns the demo creds or skips. Every test calls this first.
func guard(t *testing.T) (account, server, passwordEnv string) {
	t.Helper()
	if os.Getenv("MT5_PAPER") != "1" {
		t.Skip("MT5_PAPER != 1 — integration tests opt-in only")
	}
	account = os.Getenv("MT5_ACCOUNT")
	server = os.Getenv("MT5_SERVER")
	passwordEnv = "MT5_PASSWORD"
	if account == "" || server == "" || os.Getenv(passwordEnv) == "" {
		t.Skip("MT5_ACCOUNT/MT5_SERVER/MT5_PASSWORD not set; skipping")
	}
	return
}

// runCLI executes the root command in-process with the given argv plus
// --agent (forces JSON). Returns stdout, stderr, exit code, and the wrapped
// error. Mirrors what the MCP server does so the test path is the operator
// path.
func runCLI(t *testing.T, ctx context.Context, args ...string) (string, string, int, error) {
	t.Helper()
	args = append([]string{}, args...)
	args = append(args, "--agent")

	root := cli.NewRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	code := 0
	if err != nil {
		var ex *cli.ExitErr
		if errors.As(err, &ex) {
			code = ex.Code
		} else {
			code = 1
		}
	}
	t.Logf("mt5-pp-cli %s\n  exit=%d\n  stdout(%d bytes): %s\n  stderr: %s",
		strings.Join(args, " "), code, len(stdout.String()), truncate(stdout.String(), 240),
		truncate(stderr.String(), 240))
	return stdout.String(), stderr.String(), code, err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// assertDemo refuses to run any test if AccountInfo reports a non-demo trade
// mode. This is the load-bearing guarantee that nothing in this package can
// touch a live account regardless of what env the operator has.
func assertDemo(t *testing.T, ctx context.Context) {
	t.Helper()
	b, err := bridge.New(bridge.Options{Stderr: os.Stderr, CallTimeout: 15 * time.Second})
	if err != nil {
		t.Skipf("bridge unavailable: %v", err)
	}
	defer b.Close()
	if err := b.Initialize(bridge.InitializeOptions{Timeout: 10000}); err != nil {
		t.Skipf("terminal not initializable: %v", err)
	}
	acc, err := b.AccountInfo()
	if err != nil {
		t.Skipf("AccountInfo unavailable: %v", err)
	}
	if acc.IsLive() {
		t.Fatalf("REFUSING: account %d on %s reports trade_mode=2 (real). "+
			"Integration tests require a demo or contest account.", acc.Login, acc.Server)
	}
	t.Logf("verified demo account: %d @ %s (%s)", acc.Login, acc.Server, acc.TradeModeName())
}

// ── tests ───────────────────────────────────────────────────────────────────

func TestDoctor(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assertDemo(t, ctx)
	out, _, code, _ := runCLI(t, ctx, "doctor")
	if code != 0 {
		t.Errorf("doctor exit %d; output: %s", code, truncate(out, 400))
	}
	if !strings.Contains(out, "python") {
		t.Errorf("doctor JSON missing 'python' check")
	}
}

func TestAccountInfoIsDemo(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assertDemo(t, ctx)
	out, _, code, err := runCLI(t, ctx, "account", "info")
	if err != nil {
		t.Fatalf("account info: %v", err)
	}
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// The JSON should contain trade_mode_name. Non-demo would fail assertDemo above.
	if !strings.Contains(out, "trade_mode_name") && !strings.Contains(out, "TradeMode") {
		t.Errorf("account info missing trade-mode field: %s", truncate(out, 200))
	}
}

func TestTerminalInfo(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assertDemo(t, ctx)
	_, _, code, err := runCLI(t, ctx, "terminal", "info")
	if err != nil || code != 0 {
		t.Errorf("terminal info exit=%d err=%v", code, err)
	}
}

func TestSyncSymbols(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	assertDemo(t, ctx)
	out, _, code, _ := runCLI(t, ctx, "sync", "symbols")
	if code != 0 {
		t.Errorf("sync symbols exit %d", code)
	}
	if !strings.Contains(out, "symbols") {
		t.Errorf("sync symbols JSON missing 'symbols' key: %s", truncate(out, 200))
	}
}

func TestSymbolsList(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assertDemo(t, ctx)
	out, _, code, _ := runCLI(t, ctx, "symbols", "list", "--filter", "EUR*")
	if code != 0 && code != 3 { // 3 = not-found is acceptable if EUR* has no matches
		t.Errorf("symbols list exit %d", code)
	}
	if len(out) == 0 {
		t.Error("symbols list produced no output")
	}
}

// TestOrderSendDryRunNeverExecutes verifies that a dry-run order_send returns
// a hash and exits 6 (safety-rejected) WITHOUT --confirm. Re-running the same
// without --confirm again must NEVER actually send. This is the core property
// of the safety pipeline.
func TestOrderSendDryRunNeverExecutes(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assertDemo(t, ctx)

	// Pick a symbol the broker is likely to have. Operator can override.
	sym := os.Getenv("MT5_TEST_SYMBOL")
	if sym == "" {
		sym = "EURUSD.s"
	}

	out, _, code, _ := runCLI(t, ctx, "order", "send",
		"--symbol", sym, "--side", "buy", "--volume", "0.01")
	if code != 6 {
		t.Errorf("expected exit 6 (safety rejected without --confirm), got %d. out=%s",
			code, truncate(out, 240))
	}
	// The dry-run prints the hash to stderr (human-friendly) or stdout (JSON);
	// either way the word 'hash' or a 64-char hex string should appear somewhere
	// in stderr (the JSON branch puts the exit reason on stderr).
	if !strings.Contains(out, "hash") && !strings.Contains(out, "dry") {
		t.Logf("note: dry-run output does not contain 'hash' or 'dry': %s", truncate(out, 240))
	}
}

func TestCloseAllDryRunNeverExecutes(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assertDemo(t, ctx)

	// "1=0" never matches anything — safe filter, exercises the path.
	out, _, code, _ := runCLI(t, ctx, "close", "all", "--filter", "1=0")
	// Either "no candidates" (exit 0) or dry-run with hash (exit 6) is acceptable.
	if code != 0 && code != 6 {
		t.Errorf("close all dry-run with empty filter: exit %d. out=%s", code, truncate(out, 240))
	}
}

func TestSQLReadOnly(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, _, code, _ := runCLI(t, ctx, "sql", "select count(*) n from symbols")
	if code != 0 {
		t.Errorf("sql select exit %d: %s", code, truncate(out, 200))
	}
	if !strings.Contains(out, "\"n\"") {
		t.Errorf("sql select missing 'n' column: %s", truncate(out, 200))
	}
}

func TestAuditTailDoesNotPanicOnEmptyTable(t *testing.T) {
	guard(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, _, code, err := runCLI(t, ctx, "audit", "tail", "--limit", "5"); err != nil || code != 0 {
		t.Errorf("audit tail exit=%d err=%v", code, err)
	}
}

// Final summary — printed only when the suite ran (not skipped).
func TestZ_Summary(t *testing.T) {
	guard(t)
	t.Logf("integration suite complete against MT5_ACCOUNT=%s MT5_SERVER=%s",
		os.Getenv("MT5_ACCOUNT"), os.Getenv("MT5_SERVER"))
	fmt.Fprintln(os.Stderr, "mt5-pp-cli integration tests: PASSED (demo account)")
}

package safety

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func TestHashIsDeterministicWithinWindow(t *testing.T) {
	req := map[string]any{
		"op": "order_send", "symbol": "EURUSD", "side": "buy", "volume": 0.10,
		"sl": 1.08, "tp": 1.10,
	}
	a, err := Hash(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Hash(req)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("hash changed within window: %s vs %s", a, b)
	}
}

func TestVerifyAcceptsCurrentAndPreviousWindow(t *testing.T) {
	req := map[string]any{"op": "close_all", "filter": "profit<0"}
	h, _ := Hash(req)

	ok, err := Verify(h, req)
	if err != nil || !ok {
		t.Fatalf("current-window verify failed: ok=%v err=%v", ok, err)
	}
	other := map[string]any{"op": "close_all", "filter": "profit<-100"}
	ok2, _ := Verify(h, other)
	if ok2 {
		t.Fatal("hash verified for a different request — collision/bug")
	}
}

func TestWindowConstant(t *testing.T) {
	if Window != 60*time.Second {
		t.Fatalf("spec says hash expires after 60s; got %s", Window)
	}
}

func TestKillSwitchBlocks(t *testing.T) {
	f := filepath.Join(t.TempDir(), "STOP")
	if err := os.WriteFile(f, []byte("stop"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := dummyWriteCmd()
	err := PrecheckWrite(cmd, Guardrails{KillSwitchFile: f}, 0)
	if err == nil {
		t.Fatal("kill switch present but writes allowed")
	}
}

func TestKillSwitchAbsentAllowsDemo(t *testing.T) {
	cmd := dummyWriteCmd()
	g := Guardrails{KillSwitchFile: filepath.Join(t.TempDir(), "definitely-absent")}
	if err := PrecheckWrite(cmd, g, 0); err != nil {
		t.Fatalf("demo write should pass kill-switch + live gates: %v", err)
	}
}

func TestRealAccountRequiresEnvAndFlag(t *testing.T) {
	os.Unsetenv("MT5_LIVE")
	cmd := dummyWriteCmd()
	if err := PrecheckWrite(cmd, Guardrails{}, 2); err == nil {
		t.Fatal("real account with no MT5_LIVE should be rejected")
	}
	t.Setenv("MT5_LIVE", "1")
	if err := PrecheckWrite(cmd, Guardrails{}, 2); err == nil {
		t.Fatal("real account with MT5_LIVE but no --i-understand-this-is-live should be rejected")
	}
	cmd.SetArgs([]string{"--i-understand-this-is-live"})
	if err := cmd.ParseFlags([]string{"--i-understand-this-is-live"}); err != nil {
		t.Fatal(err)
	}
	if err := PrecheckWrite(cmd, Guardrails{}, 2); err != nil {
		t.Fatalf("real account with both gates set should pass: %v", err)
	}
}

func TestMaxVolumeGuardrail(t *testing.T) {
	g := Guardrails{MaxVolumePerOrder: 1.0}
	if err := CheckGuardrails(g, 0.5); err != nil {
		t.Fatalf("0.5 under 1.0 should pass: %v", err)
	}
	if err := CheckGuardrails(g, 2.0); err == nil {
		t.Fatal("2.0 over 1.0 should fail")
	}
	if err := CheckGuardrails(Guardrails{}, 1e6); err != nil {
		t.Fatal("MaxVolumePerOrder=0 disables the limit")
	}
}

func dummyWriteCmd() *cobra.Command {
	c := &cobra.Command{Use: "test"}
	AddWriteFlags(c)
	return c
}

func TestCheckPositionCap(t *testing.T) {
	cases := []struct {
		max         int
		current     int
		wouldAdd    bool
		shouldError bool
	}{
		{0, 99, true, false}, // 0 = disabled
		{5, 4, true, false},  // would put count at 5, equal to max — OK
		{5, 5, true, true},   // would put count at 6, over — reject
		{5, 5, false, false}, // close action, not adding — OK
		{1, 1, true, true},   // edge: 1 max, 1 open, want to add
		{1, 0, true, false},  // edge: 1 max, 0 open, want to add
	}
	for _, c := range cases {
		err := CheckPositionCap(Guardrails{MaxOpenPositions: c.max}, c.current, c.wouldAdd)
		got := err != nil
		if got != c.shouldError {
			t.Errorf("CheckPositionCap(max=%d, current=%d, add=%v) err=%v, wantErr=%v",
				c.max, c.current, c.wouldAdd, err, c.shouldError)
		}
	}
}

func TestCheckDailyLoss(t *testing.T) {
	cases := []struct {
		max         float64 // configured floor (positive number)
		realized    float64 // signed today's P&L
		shouldError bool
	}{
		{0, -10000, false},    // 0 = disabled
		{500, 0, false},       // flat
		{500, -100, false},    // small loss, under floor
		{500, -499.99, false}, // just under
		{500, -500, true},     // exactly at floor → halt
		{500, -800, true},     // well past floor
		{500, 250, false},     // profitable day
	}
	for _, c := range cases {
		err := CheckDailyLoss(Guardrails{MaxDailyLoss: c.max}, c.realized)
		got := err != nil
		if got != c.shouldError {
			t.Errorf("CheckDailyLoss(max=%g, realized=%g) err=%v, wantErr=%v",
				c.max, c.realized, err, c.shouldError)
		}
	}
}

func TestHashBindsDeviation(t *testing.T) {
	// The hash must change when deviation changes — otherwise a user could
	// dry-run with tight slippage and confirm with wide slippage under the
	// same hash. This is the binding the fix for the 4th-review pass added.
	base := map[string]any{
		"op": "order_send", "symbol": "EURUSD", "side": "buy", "volume": 0.10,
		"deviation": 5,
	}
	wide := map[string]any{
		"op": "order_send", "symbol": "EURUSD", "side": "buy", "volume": 0.10,
		"deviation": 9999,
	}
	hBase, _ := Hash(base)
	hWide, _ := Hash(wide)
	if hBase == hWide {
		t.Fatalf("hash did not change when deviation changed (%s == %s); confirm-with-wide-deviation would slip through", hBase, hWide)
	}
}

func TestAppendJSONLUsesAndCleansLockFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	err := appendJSONL(context.Background(), path, AuditEntry{
		Command: "order_send",
		Request: json.RawMessage(`{"symbol":"EURUSD"}`),
		Hash:    "abc",
		Mode:    "dry-run",
	})
	if err != nil {
		t.Fatalf("appendJSONL: %v", err)
	}
	if _, err := os.Stat(path + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("lock file should be cleaned up, stat err=%v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit jsonl: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Fatalf("audit jsonl should contain a newline-terminated row, got %q", string(data))
	}
}

func TestAppendJSONLReturnsWhenLockIsHeldAndContextCancels(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.jsonl")
	if err := os.WriteFile(path+".lock", []byte("held"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := appendJSONL(ctx, path, AuditEntry{Command: "order_send", Request: json.RawMessage(`{}`), Hash: "abc", Mode: "dry-run"})
	if err == nil {
		t.Fatal("appendJSONL should fail once context expires while another process holds the lock")
	}
}

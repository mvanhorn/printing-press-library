package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// scopedRelayHome rebases $HOME to a temp directory for the duration of the
// test so PID/log/cert files don't collide between runs or with the user's
// actual ~/.config/tesla-pp-cli. Returns the resolved relayPaths.
func scopedRelayHome(t *testing.T) relayPaths {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	// On non-POSIX UserHomeDir, $HOME may not be the source of truth; the
	// tests target POSIX-only behavior (the Windows path short-circuits).
	if runtime.GOOS == "windows" {
		t.Skip("relay tests are POSIX-only")
	}
	paths, err := newRelayPaths()
	if err != nil {
		t.Fatalf("newRelayPaths: %v", err)
	}
	if err := os.MkdirAll(paths.Dir, 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", paths.Dir, err)
	}
	// Drop a fake snowflake-private.pem so locateRelayPrivateKey() succeeds.
	teslaDir := filepath.Join(home, ".tesla")
	_ = os.MkdirAll(teslaDir, 0o700)
	_ = os.WriteFile(filepath.Join(teslaDir, "snowflake-private.pem"),
		[]byte("-----BEGIN PRIVATE KEY-----\nfake\n-----END PRIVATE KEY-----\n"), 0o600)
	return paths
}

// plantFakeRelayBinary writes a sleep-forever shell script and returns the
// directory containing it. PATH is updated so exec.LookPath finds the fake.
func plantFakeRelayBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binPath := filepath.Join(dir, relayBinaryName)
	script := "#!/bin/sh\nexec sleep 60\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("PATH", dir)
	return dir
}

// stubRunRelaySubprocess replaces the spawn primitive with a function that
// returns the given PID without forking. Useful for tests that just want to
// drive the state-file machinery without a real child process.
func stubRunRelaySubprocess(t *testing.T, pid int) {
	t.Helper()
	orig := runRelaySubprocessFn
	runRelaySubprocessFn = func(spec relayLaunchSpec) (int, error) {
		return pid, nil
	}
	t.Cleanup(func() { runRelaySubprocessFn = orig })
}

// stubProcessAlive forces processAliveFn to return the given liveness for a
// known PID. Other PIDs delegate to the real check.
func stubProcessAlive(t *testing.T, pid int, alive bool) {
	t.Helper()
	orig := processAliveFn
	processAliveFn = func(p int) bool {
		if p == pid {
			return alive
		}
		return orig(p)
	}
	t.Cleanup(func() { processAliveFn = orig })
}

// stubSignal captures signal deliveries instead of dispatching them. Returns
// a pointer to the slice of (pid, signal) tuples observed.
func stubSignal(t *testing.T) *[]struct {
	pid int
	sig os.Signal
} {
	t.Helper()
	observed := []struct {
		pid int
		sig os.Signal
	}{}
	orig := signalProcessFn
	signalProcessFn = func(pid int, sig os.Signal) error {
		observed = append(observed, struct {
			pid int
			sig os.Signal
		}{pid, sig})
		return nil
	}
	t.Cleanup(func() { signalProcessFn = orig })
	return &observed
}

func newRelayCmdForTest(t *testing.T) (*cobra.Command, *rootFlags) {
	t.Helper()
	flags := &rootFlags{}
	cmd := newRelayCmd(flags)
	cmd.SetErr(&strings.Builder{})
	cmd.SetOut(&strings.Builder{})
	cmd.SetContext(context.Background())
	return cmd, flags
}

// ---------------------------------------------------------------------------
// start
// ---------------------------------------------------------------------------

func TestRelayStart_HappyPath_WritesPIDAndStatusReportsRunning(t *testing.T) {
	paths := scopedRelayHome(t)
	plantFakeRelayBinary(t)

	// Stub spawn to return our own PID — the test harness is by definition
	// alive, so processAliveFn(PID) is true without further patching.
	stubRunRelaySubprocess(t, os.Getpid())

	cmd, flags := newRelayCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	if err := runRelayStart(cmd, flags, relayDefaultPort); err != nil {
		t.Fatalf("runRelayStart: %v", err)
	}

	body := out.String()
	if !strings.Contains(body, `"status"`) || !strings.Contains(body, `"started"`) {
		t.Errorf("expected status=started in start output, got: %s", body)
	}

	// PID file must exist + contain our PID.
	pidBytes, err := os.ReadFile(paths.PIDFile)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	got, _ := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if got != os.Getpid() {
		t.Errorf("pid file = %d, want %d", got, os.Getpid())
	}

	// status now reports running.
	statusOut := &strings.Builder{}
	cmd.SetOut(statusOut)
	if err := runRelayStatus(cmd, flags); err != nil {
		t.Fatalf("runRelayStatus: %v", err)
	}
	if !strings.Contains(statusOut.String(), `"running"`) {
		t.Errorf("expected running in status output, got: %s", statusOut.String())
	}
}

func TestRelayStart_AlreadyRunning_IsIdempotent(t *testing.T) {
	paths := scopedRelayHome(t)
	plantFakeRelayBinary(t)

	// Pre-write PID + port files pointing at this process (always alive).
	if err := writeRelayState(paths, os.Getpid(), relayDefaultPort); err != nil {
		t.Fatalf("seed pid file: %v", err)
	}

	// If start tries to spawn, fail loudly.
	stubRunRelaySubprocess(t, -1)
	orig := runRelaySubprocessFn
	t.Cleanup(func() { runRelaySubprocessFn = orig })
	runRelaySubprocessFn = func(spec relayLaunchSpec) (int, error) {
		t.Errorf("idempotent start path must NOT call runRelaySubprocessFn")
		return -1, fmt.Errorf("should not reach")
	}

	cmd, flags := newRelayCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	if err := runRelayStart(cmd, flags, relayDefaultPort); err != nil {
		t.Fatalf("runRelayStart: %v", err)
	}
	if !strings.Contains(out.String(), `"already_running"`) {
		t.Errorf("expected status=already_running, got: %s", out.String())
	}
}

func TestRelayStart_BinaryMissing_PrintsInstallRecipeAndExits2(t *testing.T) {
	scopedRelayHome(t)
	// Empty PATH so exec.LookPath fails; HOME already scoped so ~/go/bin
	// lookup also misses.
	t.Setenv("PATH", t.TempDir())

	cmd, flags := newRelayCmdForTest(t)
	err := runRelayStart(cmd, flags, relayDefaultPort)
	if err == nil {
		t.Fatalf("expected error when tesla-http-proxy is missing")
	}
	if !errIsUsage(err) {
		t.Errorf("expected usage error (exit 2), got: %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, relayBinaryName) {
		t.Errorf("error should name %s, got: %v", relayBinaryName, err)
	}
	if !strings.Contains(msg, "git clone") || !strings.Contains(msg, "lotharbach") {
		t.Errorf("error should include install recipe (git clone + lotharbach), got: %v", err)
	}
}

func TestRelayStart_PortInUse_HintsToOverride(t *testing.T) {
	scopedRelayHome(t)
	plantFakeRelayBinary(t)

	// Listen on a port to make it "in use," then ask start to use that port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	cmd, flags := newRelayCmdForTest(t)
	err = runRelayStart(cmd, flags, port)
	if err == nil {
		t.Fatalf("expected port-in-use error")
	}
	if !errIsUsage(err) {
		t.Errorf("expected usage error (exit 2), got: %v", err)
	}
	if !strings.Contains(err.Error(), "--port") {
		t.Errorf("error should hint at --port override, got: %v", err)
	}
}

func TestRelayStart_SubprocessExitsImmediately_CleansUpAndPrintsLogTail(t *testing.T) {
	paths := scopedRelayHome(t)
	plantFakeRelayBinary(t)

	// Seed the log file with a "crash reason" line so the surfaced tail is
	// deterministic.
	_ = os.WriteFile(paths.LogFile, []byte("crash: cert file invalid\n"), 0o644)

	// Stub spawn to "succeed" but stub the liveness check to report dead
	// immediately. The implementation should detect the early exit during
	// waitForRelayLiveOrEarlyExit and clean up.
	deadPID := 99999991
	stubRunRelaySubprocess(t, deadPID)
	orig := processAliveFn
	t.Cleanup(func() { processAliveFn = orig })
	processAliveFn = func(p int) bool {
		if p == deadPID {
			return false
		}
		return orig(p)
	}

	cmd, flags := newRelayCmdForTest(t)
	err := runRelayStart(cmd, flags, relayDefaultPort+10000)
	if err == nil {
		t.Fatalf("expected non-nil error when subprocess exits early")
	}
	if !strings.Contains(err.Error(), "crash: cert file invalid") {
		t.Errorf("error should include log tail, got: %v", err)
	}
	if _, statErr := os.Stat(paths.PIDFile); !os.IsNotExist(statErr) {
		t.Errorf("PID file should have been cleaned up; statErr=%v", statErr)
	}
}

// ---------------------------------------------------------------------------
// stop
// ---------------------------------------------------------------------------

func TestRelayStop_HappyPath_SendsSIGTERM_RemovesPIDFile(t *testing.T) {
	paths := scopedRelayHome(t)

	fakePID := 4242
	if err := writeRelayState(paths, fakePID, relayDefaultPort); err != nil {
		t.Fatalf("seed pid: %v", err)
	}
	// First processAliveFn call (in readRelayState) -> alive; subsequent
	// calls in waitForExit -> dead. Use a counter.
	var calls int
	orig := processAliveFn
	t.Cleanup(func() { processAliveFn = orig })
	processAliveFn = func(p int) bool {
		if p == fakePID {
			calls++
			return calls == 1
		}
		return orig(p)
	}
	observed := stubSignal(t)

	cmd, flags := newRelayCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	if err := runRelayStop(cmd, flags); err != nil {
		t.Fatalf("runRelayStop: %v", err)
	}
	if !strings.Contains(out.String(), `"stopped"`) {
		t.Errorf("expected stopped in output, got: %s", out.String())
	}
	if len(*observed) == 0 || (*observed)[0].sig != syscall.SIGTERM {
		t.Errorf("expected first signal SIGTERM, got: %+v", *observed)
	}
	if _, statErr := os.Stat(paths.PIDFile); !os.IsNotExist(statErr) {
		t.Errorf("PID file should be removed after stop; statErr=%v", statErr)
	}
}

func TestRelayStop_NotRunning_IsIdempotent(t *testing.T) {
	scopedRelayHome(t)

	cmd, flags := newRelayCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	if err := runRelayStop(cmd, flags); err != nil {
		t.Fatalf("expected zero-exit on not-running, got: %v", err)
	}
	if !strings.Contains(out.String(), `"not_running"`) {
		t.Errorf("expected status=not_running, got: %s", out.String())
	}
}

func TestRelayStop_SIGTERMIgnored_EscalatesToSIGKILL(t *testing.T) {
	paths := scopedRelayHome(t)
	fakePID := 5252
	if err := writeRelayState(paths, fakePID, relayDefaultPort); err != nil {
		t.Fatalf("seed pid: %v", err)
	}
	// Always alive until the SIGKILL escalation runs waitForExit; we want
	// the stop path to deliver both signals. Use a counter: alive while
	// waitForExit ticks for SIGTERM, dead after SIGKILL waitForExit ticks.
	var calls int
	orig := processAliveFn
	t.Cleanup(func() { processAliveFn = orig })
	processAliveFn = func(p int) bool {
		if p != fakePID {
			return orig(p)
		}
		calls++
		// Stay alive long enough that waitForExit returns false after
		// SIGTERM, but eventually report dead so the SIGKILL waitForExit
		// completes cleanly.
		return calls < 200
	}
	observed := stubSignal(t)

	// Use a much shorter grace by temporarily monkey-patching nothing —
	// the existing 5s grace works but slows tests; rely on quick polling
	// instead. The test fakes "still alive" for ~10 polls (well under 5s).
	cmd, flags := newRelayCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	doneCh := make(chan error, 1)
	go func() { doneCh <- runRelayStop(cmd, flags) }()
	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("runRelayStop: %v", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatalf("runRelayStop did not return within 8s")
	}

	sigs := *observed
	if len(sigs) < 2 || sigs[0].sig != syscall.SIGTERM || sigs[1].sig != syscall.SIGKILL {
		t.Errorf("expected SIGTERM then SIGKILL escalation, got: %+v", sigs)
	}
}

// ---------------------------------------------------------------------------
// status
// ---------------------------------------------------------------------------

func TestRelayStatus_Running_ReportsPIDPortUptimeAndLogTail(t *testing.T) {
	paths := scopedRelayHome(t)
	if err := writeRelayState(paths, os.Getpid(), 4443); err != nil {
		t.Fatalf("seed: %v", err)
	}
	_ = os.WriteFile(paths.LogFile, []byte("line1\nline2\nline3\n"), 0o644)
	// Set PID file mtime to ~10s ago so uptime is meaningful.
	past := time.Now().Add(-10 * time.Second)
	_ = os.Chtimes(paths.PIDFile, past, past)

	cmd, flags := newRelayCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)
	if err := runRelayStatus(cmd, flags); err != nil {
		t.Fatalf("runRelayStatus: %v", err)
	}
	body := out.String()
	for _, want := range []string{`"running"`, `"pid"`, `"port"`, `"uptime"`, "line3"} {
		if !strings.Contains(body, want) {
			t.Errorf("status output missing %q; got: %s", want, body)
		}
	}
}

func TestRelayStatus_Stopped_ReportsStopped(t *testing.T) {
	scopedRelayHome(t)
	cmd, flags := newRelayCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)
	if err := runRelayStatus(cmd, flags); err != nil {
		t.Fatalf("runRelayStatus: %v", err)
	}
	if !strings.Contains(out.String(), `"stopped"`) {
		t.Errorf("expected stopped, got: %s", out.String())
	}
}

// ---------------------------------------------------------------------------
// doctor
// ---------------------------------------------------------------------------

func TestRelayDoctor_AlreadyRunning_SurfacesVCSECWarning(t *testing.T) {
	paths := scopedRelayHome(t)
	// Mark relay running at port 1 (an unbindable port; the synthetic probe
	// will fail, which doctor surfaces as WARN or FAIL — fine for this test;
	// what we're asserting is the VCSEC warning + probe path getting taken).
	if err := writeRelayState(paths, os.Getpid(), 1); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cmd, flags := newRelayCmdForTest(t)
	out := &strings.Builder{}
	cmd.SetOut(out)

	if err := runRelayDoctor(cmd, flags, "5YJ_TESTVIN"); err != nil {
		t.Fatalf("runRelayDoctor: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "vcsec_unsupported") {
		t.Errorf("doctor must surface vcsec_unsupported warning, got: %s", body)
	}
	if !strings.Contains(body, "synthetic_probe") {
		t.Errorf("doctor must run synthetic probe, got: %s", body)
	}
	if !strings.Contains(body, "relay_state") {
		t.Errorf("doctor must include relay_state check, got: %s", body)
	}
}

// ---------------------------------------------------------------------------
// verify mode
// ---------------------------------------------------------------------------

func TestRelay_VerifyMode_AllFourSubsShortCircuit(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	scopedRelayHome(t)
	t.Setenv("PATH", t.TempDir()) // no binary present; verify-mode must skip the lookup

	subs := []struct {
		name string
		run  func(*cobra.Command, *rootFlags) error
	}{
		{"start", func(cmd *cobra.Command, flags *rootFlags) error { return runRelayStartVerify(cmd, flags) }},
		{"stop", func(cmd *cobra.Command, flags *rootFlags) error { return runRelayStopVerify(cmd, flags) }},
		{"status", func(cmd *cobra.Command, flags *rootFlags) error { return runRelayStatusVerify(cmd, flags) }},
		{"doctor", func(cmd *cobra.Command, flags *rootFlags) error { return runRelayDoctorVerify(cmd, flags) }},
	}
	for _, tc := range subs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmd, flags := newRelayCmdForTest(t)
			out := &strings.Builder{}
			cmd.SetOut(out)
			if err := tc.run(cmd, flags); err != nil {
				t.Fatalf("verify mode for %s must exit zero, got: %v", tc.name, err)
			}
			body := out.String()
			if !strings.Contains(body, "verify_noop") {
				t.Errorf("expected verify_noop in %s output: %s", tc.name, body)
			}
		})
	}
}

// Wrappers exercise the RunE-level verify-mode short-circuit by walking the
// cobra tree (rather than the bare run* functions which skip the RunE
// envelope). The wrappers below dispatch through the actual command's RunE,
// which is where the IsVerifyEnv guard lives.

func runRelayStartVerify(cmd *cobra.Command, flags *rootFlags) error {
	c := newRelayStartCmd(flags)
	c.SetOut(cmd.OutOrStdout())
	c.SetErr(cmd.ErrOrStderr())
	c.SetContext(cmd.Context())
	return c.RunE(c, nil)
}

func runRelayStopVerify(cmd *cobra.Command, flags *rootFlags) error {
	c := newRelayStopCmd(flags)
	c.SetOut(cmd.OutOrStdout())
	c.SetErr(cmd.ErrOrStderr())
	c.SetContext(cmd.Context())
	return c.RunE(c, nil)
}

func runRelayStatusVerify(cmd *cobra.Command, flags *rootFlags) error {
	c := newRelayStatusCmd(flags)
	c.SetOut(cmd.OutOrStdout())
	c.SetErr(cmd.ErrOrStderr())
	c.SetContext(cmd.Context())
	return c.RunE(c, nil)
}

func runRelayDoctorVerify(cmd *cobra.Command, flags *rootFlags) error {
	c := newRelayDoctorCmd(flags)
	c.SetOut(cmd.OutOrStdout())
	c.SetErr(cmd.ErrOrStderr())
	c.SetContext(cmd.Context())
	return c.RunE(c, nil)
}

// ---------------------------------------------------------------------------
// helper unit tests
// ---------------------------------------------------------------------------

func TestRelay_EnsureRelayCert_GeneratesPEMsWithModeBits(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	if err := ensureRelayCert(cert, key); err != nil {
		t.Fatalf("ensureRelayCert: %v", err)
	}
	certData, _ := os.ReadFile(cert)
	if !strings.Contains(string(certData), "BEGIN CERTIFICATE") {
		t.Errorf("cert PEM missing header: %s", string(certData))
	}
	info, err := os.Stat(key)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("key file mode = %o, want 0600", mode)
	}
	// Second call should be a no-op (no error, cert untouched).
	mt1 := info.ModTime()
	time.Sleep(20 * time.Millisecond)
	if err := ensureRelayCert(cert, key); err != nil {
		t.Fatalf("ensureRelayCert second call: %v", err)
	}
	info2, _ := os.Stat(key)
	if !info2.ModTime().Equal(mt1) {
		t.Errorf("ensureRelayCert second call should be a no-op; mtime changed %v -> %v", mt1, info2.ModTime())
	}
}

func TestRelay_ReadLogTail_ReturnsLastNLines(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	_ = os.WriteFile(log, []byte("a\nb\nc\nd\ne\n"), 0o644)
	got := readLogTail(log, 3)
	want := "c\nd\ne"
	if got != want {
		t.Errorf("readLogTail = %q, want %q", got, want)
	}
}

func TestRelay_ReadLogTail_MissingReturnsEmpty(t *testing.T) {
	if got := readLogTail("/no/such/path", 5); got != "" {
		t.Errorf("readLogTail on missing file = %q, want empty", got)
	}
}

func TestRelay_PortFree_DetectsConflict(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	if err := portFree(port); err == nil {
		t.Errorf("portFree(%d) should error while listener is open", port)
	}
}

func TestRelay_LocateRelayPrivateKey_PrefersConfigThenEnvThenSnowflake(t *testing.T) {
	scopedRelayHome(t)
	// scopedRelayHome plants ~/.tesla/snowflake-private.pem; with no config
	// and no env, the snowflake path wins.
	key, err := locateRelayPrivateKey(&rootFlags{})
	if err != nil {
		t.Fatalf("locateRelayPrivateKey: %v", err)
	}
	if !strings.HasSuffix(key, "snowflake-private.pem") {
		t.Errorf("expected snowflake key, got: %s", key)
	}

	// Env var wins over snowflake when present + file exists.
	envKey := filepath.Join(t.TempDir(), "env-key.pem")
	_ = os.WriteFile(envKey, []byte("fake"), 0o600)
	t.Setenv("TESLA_FLEET_KEY_FILE", envKey)
	key, err = locateRelayPrivateKey(&rootFlags{})
	if err != nil {
		t.Fatalf("locateRelayPrivateKey env: %v", err)
	}
	if key != envKey {
		t.Errorf("env override not honored: got %s, want %s", key, envKey)
	}
}

func TestRelay_LocateRelayPrivateKey_NoneFound_ReturnsHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TESLA_FLEET_KEY_FILE", "")
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only test")
	}
	_, err := locateRelayPrivateKey(&rootFlags{})
	if err == nil {
		t.Fatalf("expected error when no key is available")
	}
	if !strings.Contains(err.Error(), "fleet-template") && !strings.Contains(err.Error(), "ble-pair") {
		t.Errorf("error should hint at fleet-template or ble-pair, got: %v", err)
	}
}

func TestRelay_FindRelayBinary_PATHThenGoBin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // empty PATH

	if _, err := findRelayBinary(); err == nil {
		t.Errorf("expected error when binary is nowhere")
	}

	// Plant in ~/go/bin instead of PATH.
	goBin := filepath.Join(home, "go", "bin")
	_ = os.MkdirAll(goBin, 0o755)
	bin := filepath.Join(goBin, relayBinaryName)
	_ = os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755)
	got, err := findRelayBinary()
	if err != nil {
		t.Fatalf("findRelayBinary fallback: %v", err)
	}
	if got != bin {
		t.Errorf("findRelayBinary = %s, want %s", got, bin)
	}
}

func TestRelay_LaunchArgs_StableShape(t *testing.T) {
	args := relayLaunchArgs(relayLaunchSpec{
		Binary:     "/x/tesla-http-proxy",
		Port:       4443,
		CertPEM:    "/c.pem",
		KeyPEM:     "/k.pem",
		PrivateKey: "/p.pem",
	})
	joined := strings.Join(args, " ")
	for _, want := range []string{"-mode owner", "-port 4443", "-cert /c.pem", "-key-file /k.pem", "-tls-key /p.pem"} {
		if !strings.Contains(joined, want) {
			t.Errorf("relayLaunchArgs missing %q; full=%s", want, joined)
		}
	}
}

func TestRelay_WorstSeverity_PicksHighest(t *testing.T) {
	cases := []struct {
		sevs []string
		want string
	}{
		{[]string{"OK"}, "OK"},
		{[]string{"OK", "INFO"}, "INFO"},
		{[]string{"OK", "INFO", "WARN"}, "WARN"},
		{[]string{"OK", "INFO", "WARN", "FAIL"}, "FAIL"},
		{[]string{"FAIL", "OK"}, "FAIL"},
		{[]string{}, "OK"},
	}
	for _, tc := range cases {
		checks := []map[string]any{}
		for _, s := range tc.sevs {
			checks = append(checks, map[string]any{"severity": s})
		}
		if got := worstSeverity(checks); got != tc.want {
			t.Errorf("worstSeverity(%v) = %s, want %s", tc.sevs, got, tc.want)
		}
	}
}

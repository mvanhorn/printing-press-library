package mcpconformance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/learn/lookups"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsyStartRunTimeout bounds each etsy-pp-mcp invocation in the start test.
const etsyStartRunTimeout = 30 * time.Second

var (
	etsyMCPBinOnce sync.Once
	etsyMCPBinPath string
	etsyMCPBinErr  error
)

func etsyMCPBinary(t *testing.T) string {
	t.Helper()
	etsyMCPBinOnce.Do(func() {
		etsyDir, etsyErr := os.MkdirTemp("", "etsy-mcp-start-")
		if etsyErr != nil {
			etsyMCPBinErr = etsyErr
			return
		}
		etsyMCPBinPath = filepath.Join(etsyDir, "etsy-pp-mcp")
		etsyOut, etsyErr := exec.Command("go", "build", "-o", etsyMCPBinPath, "../../cmd/etsy-pp-mcp").CombinedOutput()
		if etsyErr != nil {
			etsyMCPBinErr = &etsyBuildError{string(etsyOut), etsyErr}
		}
	})
	if etsyMCPBinErr != nil {
		t.Fatal(etsyMCPBinErr)
	}
	return etsyMCPBinPath
}

// etsyRunBounded runs a binary and reports its exit code and combined output. A run that outlives
// the timeout is killed through the context and reported as timedOut rather than hanging the
// package: a wedged binary (a store open that never returns, a server that starts serving) must
// fail the caller in bounded time.
func etsyRunBounded(bin string, timeout time.Duration, args ...string) (code int, out string, timedOut bool, err error) {
	etsyCtx, etsyCancel := context.WithTimeout(context.Background(), timeout)
	defer etsyCancel()
	etsyCmd := exec.CommandContext(etsyCtx, bin, args...)
	etsyCmd.Env = os.Environ()
	etsyOut, etsyErr := etsyCmd.CombinedOutput()
	if etsyCtx.Err() != nil {
		return -1, string(etsyOut), true, nil
	}
	if etsyErr == nil {
		return 0, string(etsyOut), false, nil
	}
	if etsyExit, ok := etsyErr.(*exec.ExitError); ok {
		return etsyExit.ExitCode(), string(etsyOut), false, nil
	}
	return -1, string(etsyOut), false, etsyErr
}

// A hung child must come back as timed out well before the hang ends.
func TestEtsyRunBoundedKillsAHungBinary(t *testing.T) {
	etsyStart := time.Now()
	_, _, etsyTimedOut, etsyErr := etsyRunBounded("/bin/sleep", 300*time.Millisecond, "20")
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	if !etsyTimedOut {
		t.Fatalf("a 20s sleep was not reported as timed out")
	}
	if etsyElapsed := time.Since(etsyStart); etsyElapsed > 10*time.Second {
		t.Fatalf("hung binary was not killed promptly: %s", etsyElapsed)
	}
	if _, _, etsyTimedOut, _ = etsyRunBounded("/bin/echo", 5*time.Second, "ok"); etsyTimedOut {
		t.Fatalf("a fast binary was reported as timed out")
	}
}

type etsyBuildError struct {
	out string
	err error
}

func (e *etsyBuildError) Error() string { return "build etsy-pp-mcp: " + e.err.Error() + ": " + e.out }

func etsyStartSyncedCount(t *testing.T, kind string) int {
	t.Helper()
	etsyPath, etsyErr := cli.MCPStorePath()
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyDB, etsyErr := store.OpenReadOnly(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	defer etsyDB.Close()
	var etsyN int
	if etsyErr = etsyDB.DB().QueryRow(`SELECT count(*) FROM entity_lookups WHERE kind = ? AND source = 'synced'`, kind).Scan(&etsyN); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	return etsyN
}

// The real MCP binary must purge legacy buyer lookups only after flag parsing: help (and a
// failed parse) must never open the operator's store, while a parsed start must still purge.
func TestEtsyMCPStartPurgeRunsAfterFlagParse(t *testing.T) {
	etsyBin := etsyMCPBinary(t)
	etsyRestore, etsyErr := cliutil.SetHomeOverride("")
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	t.Cleanup(etsyRestore)
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir, cliutil.CacheDir)
	etsyPath, etsyErr := cli.MCPStorePath()
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	etsyDB, etsyErr := store.Open(etsyPath)
	if etsyErr != nil {
		t.Fatal(etsyErr)
	}
	for _, etsyKind := range []string{"receipts", "listings"} {
		if etsyErr = lookups.Upsert(etsyDB.DB(), etsyKind, "QZXBUYERNAME", "QZXPERSONAL", "synced"); etsyErr != nil {
			t.Fatal(etsyErr)
		}
	}
	if etsyErr = etsyDB.Close(); etsyErr != nil {
		t.Fatal(etsyErr)
	}
	run := func(args ...string) (int, string) {
		etsyCode, etsyOut, etsyTimedOut, etsyErr := etsyRunBounded(etsyBin, etsyStartRunTimeout, args...)
		if etsyTimedOut {
			t.Fatalf("%v timed out after %s: %q", args, etsyStartRunTimeout, etsyOut)
		}
		if etsyErr != nil {
			t.Fatal(etsyErr)
		}
		return etsyCode, etsyOut
	}

	if code, out := run("-h"); code != 0 || !strings.Contains(out, "-transport") {
		t.Fatalf("-h exit=%d out=%q", code, out)
	}
	if n := etsyStartSyncedCount(t, "receipts"); n != 1 {
		t.Errorf("purge-before-parse: -h opened the store and purged the synced receipts lookup (rows=%d, want 1)", n)
	}

	// Positive control: a parsed start (bad transport exits 2 after the purge) does purge, and
	// leaves the allowlisted kind alone, so the -h assertion above is not vacuous.
	if code, out := run("--transport", "bogus"); code != 2 || !strings.Contains(out, "unknown --transport") {
		t.Fatalf("bogus transport exit=%d out=%q", code, out)
	}
	if n := etsyStartSyncedCount(t, "receipts"); n != 0 {
		t.Errorf("parsed start did not purge the synced receipts lookup (rows=%d, want 0)", n)
	}
	if n := etsyStartSyncedCount(t, "listings"); n != 1 {
		t.Errorf("parsed start purged an allowlisted lookup (rows=%d, want 1)", n)
	}
}

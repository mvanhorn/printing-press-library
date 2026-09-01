package sequence

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTokenConsumptionIsAtomicAcrossProcesses is the evidence for the
// "authorization consumption is not atomic" finding. The review asserted that
// consumption relies on a process-local mutex; it actually runs inside an flock
// on <store>.lock, which is a kernel-level cross-process lock. This test races
// N real OS processes against one token and asserts exactly one wins.
//
// If consumption were mutex-only, multiple processes would each read the token,
// each see it present, and each issue physical KVM actions.
func TestTokenConsumptionIsAtomicAcrossProcesses(t *testing.T) {
	if os.Getenv("KVMCTL_TEST_CONSUME_TOKEN") != "" {
		// Child mode: attempt to consume the token and report the outcome.
		runTokenConsumerChild()
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}

	base := t.TempDir()
	storeDir := filepath.Join(base, "auth")
	if err := os.Mkdir(storeDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storePath := filepath.Join(storeDir, "auth.json")

	store := NewStore(storePath)
	auth := NewAuthorizer(store, time.Now)

	target := "https://kvm-atomic.local"
	plan := Plan{
		Target:      target,
		Actions:     []Action{{Type: "key", Value: "KeyA"}},
		MaxDuration: 5 * time.Second,
	}
	token, err := auth.Authorize(plan, target, true, 30*time.Second)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}

	const procs = 8
	results := make([]string, procs)
	var wg sync.WaitGroup
	// Start together so the processes genuinely contend.
	startAt := time.Now().Add(600 * time.Millisecond)

	for i := 0; i < procs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cmd := exec.Command(exe,
				"-test.run", "TestTokenConsumptionIsAtomicAcrossProcesses",
				"-test.count", "1",
			)
			cmd.Env = append(os.Environ(),
				"KVMCTL_TEST_CONSUME_TOKEN=1",
				"KVMCTL_TEST_STORE="+storePath,
				"KVMCTL_TEST_TOKEN="+token,
				"KVMCTL_TEST_TARGET="+target,
				"KVMCTL_TEST_START="+startAt.Format(time.RFC3339Nano),
			)
			out, _ := cmd.CombinedOutput()
			results[idx] = string(out)
		}(i)
	}
	wg.Wait()

	wins := 0
	for i, out := range results {
		switch {
		case strings.Contains(out, "CONSUMED_OK"):
			wins++
		case strings.Contains(out, "CONSUMED_DENIED"):
		default:
			t.Fatalf("child %d produced no verdict; output:\n%s", i, out)
		}
	}

	if wins != 1 {
		t.Fatalf("%d of %d processes consumed the same single-use token; consumption must be atomic across processes", wins, procs)
	}

	// The token must be gone from the store afterwards.
	remaining, err := store.read()
	if err != nil {
		t.Fatalf("store.read: %v", err)
	}
	if _, ok := remaining[token]; ok {
		t.Fatal("token still present after a successful consumption")
	}
}

// runTokenConsumerChild is the subprocess half of the test above.
func runTokenConsumerChild() {
	storePath := os.Getenv("KVMCTL_TEST_STORE")
	token := os.Getenv("KVMCTL_TEST_TOKEN")
	target := os.Getenv("KVMCTL_TEST_TARGET")

	if startStr := os.Getenv("KVMCTL_TEST_START"); startStr != "" {
		if start, err := time.Parse(time.RFC3339Nano, startStr); err == nil {
			time.Sleep(time.Until(start))
		}
	}

	store := NewStore(storePath)
	_, err := store.takeMatching(token, func(a authorization) error {
		if a.Target != target {
			return errTestTargetMismatch
		}
		return nil
	})
	if err != nil {
		os.Stdout.WriteString("CONSUMED_DENIED\n")
		return
	}
	os.Stdout.WriteString("CONSUMED_OK\n")
}

var errTestTargetMismatch = &testErr{"target mismatch"}

type testErr struct{ s string }

func (e *testErr) Error() string { return e.s }

// TestStoreWriteIsAtomic guards the durability half of consumption: a partially
// written store would either resurrect a consumed token or destroy live ones.
func TestStoreWriteIsAtomic(t *testing.T) {
	base := t.TempDir()
	storeDir := filepath.Join(base, "auth")
	if err := os.Mkdir(storeDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storePath := filepath.Join(storeDir, "auth.json")
	store := NewStore(storePath)
	auth := NewAuthorizer(store, time.Now)

	target := "https://kvm-durable.local"
	plan := Plan{
		Target:      target,
		Actions:     []Action{{Type: "key", Value: "KeyA"}},
		MaxDuration: 5 * time.Second,
	}
	for i := 0; i < 4; i++ {
		if _, err := auth.Authorize(plan, target, true, 30*time.Second); err != nil {
			t.Fatalf("Authorize %d: %v", i, err)
		}
	}

	// No stray temp files may remain — each write must have been renamed into place.
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".auth-") {
			t.Errorf("leftover temp file %q; a crash here would leave the store torn", e.Name())
		}
	}

	// The store must be valid JSON with all four tokens.
	raw, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var parsed map[string]authorization
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("store is not valid JSON: %v", err)
	}
	if len(parsed) != 4 {
		t.Fatalf("expected 4 authorizations, got %d", len(parsed))
	}

	info, err := os.Stat(storePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("authorization store mode %o is group/world accessible; it holds live tokens", info.Mode().Perm())
	}
}

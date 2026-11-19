package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/config"
)

// TestEtsyQuotaProcess is re-executed with an explicit temporary state path.
func TestEtsyQuotaProcess(t *testing.T) {
	mode := os.Getenv("ETSY_QUOTA_TEST_MODE")
	if mode == "" {
		t.Skip("re-exec helper")
	}
	path := os.Getenv("ETSY_QUOTA_TEST_PATH")
	if mode == "hold" {
		acquired, err := cliutil.TryWithFileLock(path, func() error {
			fmt.Println("ready")
			_, err := io.Copy(io.Discard, os.Stdin)
			return err
		})
		if !acquired || err != nil {
			t.Fatalf("hold: %v %v", acquired, err)
		}
		return
	}
	var q EtsyQuota
	if err := json.Unmarshal([]byte(os.Getenv("ETSY_QUOTA_TEST_RECORD")), &q); err != nil {
		t.Fatal(err)
	}
	if mode == "park" {
		etsyQuotaMergeHook = func() { fmt.Println("loaded"); _, _ = io.Copy(io.Discard, os.Stdin) }
	} else {
		fmt.Println("ready")
		_, _ = io.Copy(io.Discard, os.Stdin)
	}
	observeEtsyQuotaRecord(context.Background(), path, q, filepath.Base(path))
	if mode == "observe" {
		etsyQuotaState.Lock()
		cached := etsyQuotaState.quotas[filepath.Base(path)]
		etsyQuotaState.Unlock()
		if cached.RemainingToday != q.RemainingToday {
			t.Fatal("missing process fallback")
		}
	}
}

func quotaChild(t *testing.T, path, mode string, q EtsyQuota) (*exec.Cmd, io.WriteCloser) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEtsyQuotaProcess$", "-test.count=1")
	record, _ := json.Marshal(q)
	cmd.Env = append(os.Environ(), "ETSY_QUOTA_TEST_MODE="+mode, "ETSY_QUOTA_TEST_PATH="+path, "ETSY_QUOTA_TEST_RECORD="+string(record))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Register cleanup before waiting for readiness, including startup failures.
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Wait()
		}
	})
	scanner := bufio.NewScanner(stdout)
	expected := "ready"
	if mode == "park" {
		expected = "loaded"
	}
	if !scanner.Scan() || scanner.Text() != expected {
		t.Fatal("child failed to become ready")
	}
	return cmd, stdin
}

func TestEtsyQuotaHeldLockFallback(t *testing.T) {
	path := isolateEtsyQuota(t)
	holder, input := quotaChild(t, path, "hold", EtsyQuota{})
	defer func() {
		_ = input.Close()
		if err := holder.Wait(); err != nil {
			t.Error(err)
		}
	}()
	start := time.Now()
	ObserveEtsyQuota(quotaHeaders(), "testkey")
	elapsed := time.Since(start)
	if elapsed < 80*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Fatalf("bounded wait: %s", elapsed)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("busy lock wrote state: %v", err)
	}
	q, ok := LoadEtsyQuota("testkey")
	if !ok || q.RemainingToday != 4210 {
		t.Fatalf("fallback: %+v %v", q, ok)
	}
	// A second key must not evict the first key's process-only fallback.
	ObserveEtsyQuota(quotaHeaders(), "another-synthetic-key")
	if _, ok := LoadEtsyQuota("testkey"); !ok {
		t.Fatal("cache evicted fallback for another key")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()
	start = time.Now()
	acquired, _ := withEtsyQuotaLock(ctx, path, func() error { t.Fatal("held lock acquired"); return nil })
	if acquired || time.Since(start) > 60*time.Millisecond {
		t.Fatal("context deadline ignored")
	}
}

// etsy-ext: A parks after loading while holding flock; B must finish without writing.
func TestEtsyQuotaConcurrentOrderedObservers(t *testing.T) {
	path := isolateEtsyQuota(t)
	now := time.Now().UTC()
	a := EtsyQuota{ObservedAt: now, RemainingToday: 10}
	b := EtsyQuota{ObservedAt: now.Add(-time.Second), RemainingToday: 50}
	childA, inputA := quotaChild(t, path, "park", a)
	childB, inputB := quotaChild(t, path, "observe", b)
	start := time.Now()
	_ = inputB.Close()
	if err := childB.Wait(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("B exceeded lock bound")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("B wrote while A parked: %v", err)
	}
	_ = inputA.Close()
	if err := childA.Wait(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got EtsyQuota
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	a.RawObservedAt, a.RawRemaining = a.ObservedAt, a.RemainingToday
	if got != a {
		t.Fatalf("got %+v want A %+v", got, a)
	}
}

func TestEtsyQuotaKeyPrivacyAndScope(t *testing.T) {
	isolateEtsyQuota(t)
	key := "synthetic-private-keystring"
	path := quotaPath(key)
	ObserveEtsyQuota(quotaHeaders(), key+":first-secret")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "etsy-quota-cd0092d33bd0.json" {
		t.Fatalf("unexpected hash filename %s", filepath.Base(path))
	}
	if strings.Contains(path, key) || strings.Contains(string(data), key) || strings.Contains(string(data), "secret") {
		t.Fatal("credential leaked")
	}
	if _, ok := LoadEtsyQuota(key + ":second-secret"); !ok {
		t.Fatal("same key separated by secret")
	}
	if _, ok := LoadEtsyQuota("different-synthetic-key"); ok {
		t.Fatal("different keys joined")
	}
	c1 := New(&config.Config{EtsyApiKey: key, AccessToken: "first-token"}, time.Second, 0)
	c2 := New(&config.Config{EtsyApiKey: key, AccessToken: "second-token"}, time.Second, 0)
	if quotaPath(c1.Config.EtsyApiKey) != quotaPath(c2.Config.EtsyApiKey) {
		t.Fatal("tokens changed principal")
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("state dir mode: %v %v", info, err)
	}
}

func TestEtsyQuotaNoKeyAndNoLegacyMigration(t *testing.T) {
	path := isolateEtsyQuota(t)
	dir := filepath.Dir(path)
	ObserveEtsyQuota(quotaHeaders(), "")
	if _, ok := LoadEtsyQuota(""); ok {
		t.Fatal("empty key has quota")
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("empty key wrote files: %v", entries)
	}
	data, _ := json.Marshal(EtsyQuota{ObservedAt: time.Now().UTC(), RemainingToday: 100})
	if err := cliutil.AtomicWritePrivateFile(filepath.Join(dir, "etsy-quota.json"), data, 0600, 0700); err != nil {
		t.Fatal(err)
	}
	if _, ok := LoadEtsyQuota("testkey"); ok {
		t.Fatal("legacy quota migrated")
	}
}

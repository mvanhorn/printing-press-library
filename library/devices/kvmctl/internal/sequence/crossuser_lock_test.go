package sequence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestCanonicalTargetIdentityCollapsesAliases proves that different spellings of
// the same physical device resolve to one lock identity. This is the core of the
// "target labels bypass serialization" finding: if these diverge, two workflows
// naming the same KVM take different locks and interleave physical input.
func TestCanonicalTargetIdentityCollapsesAliases(t *testing.T) {
	same := [][]string{
		{"kvm1.local", "https://kvm1.local", "https://kvm1.local:443", "https://KVM1.local/", "https://kvm1.local/some/path"},
		{"http://kvm2.local", "http://kvm2.local:80", "http://KVM2.LOCAL/"},
		{"10.0.0.5:8080", "https://10.0.0.5:8080", "https://10.0.0.5:8080/api"},
	}
	for _, group := range same {
		want, err := canonicalTargetIdentity(group[0])
		if err != nil {
			t.Fatalf("canonicalTargetIdentity(%q): %v", group[0], err)
		}
		for _, alias := range group[1:] {
			got, err := canonicalTargetIdentity(alias)
			if err != nil {
				t.Fatalf("canonicalTargetIdentity(%q): %v", alias, err)
			}
			if got != want {
				t.Errorf("alias %q resolved to %q, want %q (same device must share a lock)", alias, got, want)
			}
		}
	}
}

// TestCanonicalTargetIdentitySeparatesDistinctDevices guards the opposite error:
// over-normalizing would serialize unrelated devices and deadlock a fleet.
func TestCanonicalTargetIdentitySeparatesDistinctDevices(t *testing.T) {
	distinct := []string{
		"https://kvm1.local",
		"https://kvm2.local",
		"https://kvm1.local:8443",
		"http://kvm1.local",
	}
	seen := map[string]string{}
	for _, target := range distinct {
		id, err := canonicalTargetIdentity(target)
		if err != nil {
			t.Fatalf("canonicalTargetIdentity(%q): %v", target, err)
		}
		if prev, ok := seen[id]; ok {
			t.Errorf("targets %q and %q both mapped to %q; distinct devices must not share a lock", prev, target, id)
		}
		seen[id] = target
	}
}

func TestCanonicalTargetIdentityRejectsEmpty(t *testing.T) {
	if _, err := canonicalTargetIdentity("   "); err == nil {
		t.Fatal("expected empty target to be rejected rather than silently mapped to a shared default lock")
	}
}

// TestSharedLockDirIsNotPrivilegeDependent is the regression test for the bug in
// the previous fix attempt: the directory was chosen conditionally (/var/lock if
// creatable, else /tmp), so root and non-root processes silently picked
// different directories and never contended on the same lock.
func TestSharedLockDirIsNotPrivilegeDependent(t *testing.T) {
	dir := t.TempDir()
	// t.TempDir() is 0700/0755; make it match the shared-lock contract.
	if err := os.Chmod(dir, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv(sharedLockDirEnv, dir)
	first, err := sharedLockDir()
	if err != nil {
		t.Fatalf("sharedLockDir: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := sharedLockDir()
		if err != nil {
			t.Fatalf("sharedLockDir (repeat %d): %v", i, err)
		}
		if again != first {
			t.Fatalf("sharedLockDir returned %q then %q; the path must be deterministic", first, again)
		}
	}
}

func TestSharedLockDirCreatesWorldWritableStickyDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "locks")
	t.Setenv(sharedLockDirEnv, dir)

	got, err := sharedLockDir()
	if err != nil {
		t.Fatalf("sharedLockDir: %v", err)
	}
	if got != dir {
		t.Fatalf("sharedLockDir = %q, want %q", got, dir)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode().Perm()&0o002 == 0 {
		t.Errorf("lock dir mode %o is not world-writable; another OS user could not create its lock file", info.Mode().Perm())
	}
	if info.Mode()&os.ModeSticky == 0 {
		t.Errorf("lock dir mode %o lacks the sticky bit; a non-owner could unlink another user's lock", info.Mode())
	}
}

// TestSharedLockDirRejectsPrivateDir is the regression test for "shared lock
// rejects other users": a 0700 directory silently prevents cross-user
// serialization, so we must fail loudly instead of proceeding unserialized.
func TestSharedLockDirRejectsPrivateDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv(sharedLockDirEnv, dir)

	if _, err := sharedLockDir(); err == nil {
		t.Fatal("expected a 0700 lock directory to be rejected; silently accepting it breaks cross-user serialization")
	}
}

func TestSharedLockDirRejectsRelativePath(t *testing.T) {
	t.Setenv(sharedLockDirEnv, "relative/locks")
	if _, err := sharedLockDir(); err == nil {
		t.Fatal("expected a relative lock dir to be rejected; it would resolve differently per working directory")
	}
}

// TestSharedLockFileIsOpenableByOtherUsers verifies the lock file mode. A 0600
// lock file is unopenable by a second OS user, which turns the shared lock into
// a hard failure for them rather than a serialization point.
func TestSharedLockFileIsOpenableByOtherUsers(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv(sharedLockDirEnv, dir)

	if err := withTargetLock("https://kvm-mode.local", func() error { return nil }); err != nil {
		t.Fatalf("withTargetLock: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one lock file, got %d", len(entries))
	}
	info, err := entries[0].Info()
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Mode().Perm()&0o006 == 0 {
		t.Errorf("lock file mode %o denies other users read/write; they cannot contend on this lock", info.Mode().Perm())
	}
}

// TestWithTargetLockSerializesAliases is the behavioral proof: concurrent
// goroutines using DIFFERENT spellings of the same device must not overlap
// inside the critical section.
func TestWithTargetLockSerializesAliases(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv(sharedLockDirEnv, dir)

	aliases := []string{
		"kvm-serial.local",
		"https://kvm-serial.local",
		"https://KVM-SERIAL.local:443/",
		"https://kvm-serial.local/redfish",
	}

	var mu sync.Mutex
	inside := 0
	maxInside := 0

	var wg sync.WaitGroup
	errs := make(chan error, len(aliases))
	for _, alias := range aliases {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			errs <- withTargetLock(target, func() error {
				mu.Lock()
				inside++
				if inside > maxInside {
					maxInside = inside
				}
				mu.Unlock()

				time.Sleep(30 * time.Millisecond)

				mu.Lock()
				inside--
				mu.Unlock()
				return nil
			})
		}(alias)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("withTargetLock: %v", err)
		}
	}
	if maxInside != 1 {
		t.Errorf("observed %d concurrent holders of the device lock; aliases of one device must serialize", maxInside)
	}
}

// TestExecuteAuthorizedPreservesTokenWhenLockFails is the regression test for
// "consuming the authorization before acquiring the lock". A lock-setup failure
// must leave the single-use token intact so the caller can retry, rather than
// forcing a fresh authorization for a workflow that never touched the device.
func TestExecuteAuthorizedPreservesTokenWhenLockFails(t *testing.T) {
	base := t.TempDir()
	// A 0700 directory is rejected by sharedLockDir, so lock acquisition fails
	// before any device action — exactly the case that used to burn the token.
	badDir := filepath.Join(base, "private")
	if err := os.Mkdir(badDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv(sharedLockDirEnv, badDir)

	// The authorization store must stay private (0700) — it holds one-time
	// tokens. Only the device lock directory is shared.
	storeDir := filepath.Join(base, "auth")
	if err := os.Mkdir(storeDir, 0o700); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	store := NewStore(filepath.Join(storeDir, "auth.json"))
	auth := NewAuthorizer(store, time.Now)
	exec := NewExecutor()

	target := "https://kvm-token.local"
	plan := Plan{
		Target:      target,
		Actions:     []Action{{Type: "key", Value: "KeyA"}},
		MaxDuration: 5 * time.Second,
	}
	token, err := auth.Authorize(plan, target, true, 30*time.Second)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}

	dev := &recordingDevice{}
	err = ExecuteAuthorized(context.Background(), auth, exec, dev, token, target, plan, nil)
	if err == nil {
		t.Fatal("expected execution to fail when the lock directory is unusable")
	}
	if len(dev.actions) != 0 {
		t.Fatalf("device received %d actions despite lock failure: %v", len(dev.actions), dev.actions)
	}

	// The token must still be present and usable.
	stored, err := store.read()
	if err != nil {
		t.Fatalf("store.read: %v", err)
	}
	if _, ok := stored[token]; !ok {
		t.Fatal("authorization was consumed even though the lock could not be acquired; the caller must re-authorize for no reason")
	}
}

// TestExecuteAuthorizedConsumesTokenExactlyOnce proves the token is still
// single-use on the success path — the fix must not weaken that guarantee.
func TestExecuteAuthorizedConsumesTokenExactlyOnce(t *testing.T) {
	base := t.TempDir()
	lockDir := filepath.Join(base, "locks")
	if err := os.Mkdir(lockDir, 0o777); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Chmod(lockDir, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv(sharedLockDirEnv, lockDir)

	// The authorization store must stay private (0700) — it holds one-time
	// tokens. Only the device lock directory is shared.
	storeDir := filepath.Join(base, "auth")
	if err := os.Mkdir(storeDir, 0o700); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	store := NewStore(filepath.Join(storeDir, "auth.json"))
	auth := NewAuthorizer(store, time.Now)
	exec := NewExecutor()

	target := "https://kvm-once.local"
	plan := Plan{
		Target:      target,
		Actions:     []Action{{Type: "key", Value: "KeyA"}},
		MaxDuration: 5 * time.Second,
	}
	token, err := auth.Authorize(plan, target, true, 30*time.Second)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}

	dev := &recordingDevice{}
	if err := ExecuteAuthorized(context.Background(), auth, exec, dev, token, target, plan, nil); err != nil {
		t.Fatalf("first execution: %v", err)
	}
	if len(dev.actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(dev.actions))
	}

	if err := ExecuteAuthorized(context.Background(), auth, exec, dev, token, target, plan, nil); err == nil {
		t.Fatal("expected the second use of a single-use token to be rejected")
	}
	if len(dev.actions) != 1 {
		t.Fatalf("replay issued extra physical actions: %d total", len(dev.actions))
	}
}

// recordingDevice captures the physical actions a plan would have issued so a
// test can assert that a failed run touched no hardware.
type recordingDevice struct {
	mu       sync.Mutex
	actions  []string
	onAction func()
}

func (d *recordingDevice) record(s string) error {
	if d.onAction != nil {
		d.onAction()
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.actions = append(d.actions, s)
	return nil
}

func (d *recordingDevice) Key(_ context.Context, key string) error { return d.record("key:" + key) }
func (d *recordingDevice) Chord(_ context.Context, keys string) error {
	return d.record("chord:" + keys)
}
func (d *recordingDevice) Text(_ context.Context, text string) error { return d.record("text:" + text) }
func (d *recordingDevice) HoldKey(_ context.Context, key string, _ time.Duration) error {
	return d.record("hold:" + key)
}
func (d *recordingDevice) MouseMove(_ context.Context, _, _ int) error { return d.record("mousemove") }
func (d *recordingDevice) MouseMovePct(_ context.Context, _, _ float64) error {
	return errors.New("unsupported")
}
func (d *recordingDevice) MouseClick(_ context.Context, button string, _ int) error {
	return d.record("click:" + button)
}
func (d *recordingDevice) MouseScroll(_ context.Context, _, _ int) error { return d.record("scroll") }
func (d *recordingDevice) ReleaseAll(_ context.Context) error            { return nil }

// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sessionSandbox(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "granola-pp-cli")
	path := filepath.Join(dir, "session.json")
	t.Setenv("GRANOLA_CLI_SESSION_PATH", path)
	return path
}

// ageRotationMarker backdates the marker past rotationStaleAfter, standing in
// for the process that created it having died. Presence alone no longer means
// interruption -- a fresh marker is a live rotation -- so every test about the
// interrupted state has to age it first.
func ageRotationMarker(t *testing.T) {
	t.Helper()
	old := time.Now().Add(-2 * rotationStaleAfter)
	if err := os.Chtimes(rotationMarkerPath(), old, old); err != nil {
		t.Fatalf("backdate marker: %v", err)
	}
}

func sampleSession() CLISession {
	return CLISession{
		AccessToken:  "access-token-value-aaaaaaaaaaaaaaaa",
		RefreshToken: "refresh-token-value-bbbb",
		ExpiresAt:    time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		ObtainedAt:   time.Now().UTC().Truncate(time.Second),
		AccountID:    "user_123",
		AccountEmail: "someone@example.com",
	}
}

func TestCLISessionRoundTrip(t *testing.T) {
	sessionSandbox(t)
	want := sampleSession()
	if err := SaveCLISession(want); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	got, err := LoadCLISession()
	if err != nil {
		t.Fatalf("LoadCLISession: %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken {
		t.Errorf("tokens did not round-trip")
	}
	if got.AccountEmail != want.AccountEmail || got.AccountID != want.AccountID {
		t.Errorf("account fields did not round-trip: %+v", got.AccountEmail)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("ExpiresAt: got %v want %v", got.ExpiresAt, want.ExpiresAt)
	}
}

func TestCLISessionAbsentReturnsSentinel(t *testing.T) {
	sessionSandbox(t)
	if _, err := LoadCLISession(); !errors.Is(err, ErrNoCLISession) {
		t.Fatalf("want ErrNoCLISession, got %v", err)
	}
	if HasCLISession() {
		t.Error("HasCLISession true with no session on disk")
	}
}

func TestCLISessionFilePermissions(t *testing.T) {
	path := sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("session file mode: got %#o want 0600", perm)
	}
	di, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("session dir mode: got %#o want 0700", perm)
	}
}

// The MkdirAll no-op is the real-world case: ~/.local/share/granola-pp-cli
// already exists at 0755 on every machine that has ever synced, because
// WriteSyncState created it that way. A fresh-tempdir test would never catch it.
func TestCLISessionTightensPreExistingLooseDir(t *testing.T) {
	path := sessionSandbox(t)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("pre-create dir: %v", err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("chmod 0755: %v", err)
	}
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	di, _ := os.Stat(dir)
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("pre-existing 0755 dir was not tightened: got %#o want 0700", perm)
	}
}

func TestCLISessionRefusesWorldReadableFile(t *testing.T) {
	path := sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	_, err := LoadCLISession()
	if !errors.Is(err, ErrSessionPermissions) {
		t.Fatalf("want ErrSessionPermissions for a 0644 session file, got %v", err)
	}
}

func TestCLISessionMalformedIsTypedNotMissing(t *testing.T) {
	path := sessionSandbox(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"access_token": "trunc`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadCLISession()
	if err == nil {
		t.Fatal("expected an error for a truncated session file")
	}
	if errors.Is(err, ErrNoCLISession) {
		t.Error("a corrupt session must not be reported as absent; that silently falls through to desktop probes")
	}
}

func TestCLISessionInterruptedRotationDetected(t *testing.T) {
	sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	if _, err := BeginRotation(); err != nil {
		t.Fatalf("BeginRotation: %v", err)
	}
	// Still running: the session on disk is the pre-rotation credential, which
	// is valid until the replacement lands. Other processes must keep working.
	if _, err := LoadCLISession(); err != nil {
		t.Fatalf("a live rotation must not break concurrent readers: %v", err)
	}
	// Process dies here: marker present, replacement never written, and enough
	// time passes that no live rotation could still own it.
	ageRotationMarker(t)
	_, err := LoadCLISession()
	if !errors.Is(err, ErrSessionInterrupted) {
		t.Fatalf("want ErrSessionInterrupted, got %v", err)
	}
	// A completed save clears it.
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession after rotation: %v", err)
	}
	if _, err := LoadCLISession(); err != nil {
		t.Fatalf("session should load once rotation completed: %v", err)
	}
}

func TestCLISessionAbortRotationClearsMarker(t *testing.T) {
	sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	h, err := BeginRotation()
	if err != nil {
		t.Fatalf("BeginRotation: %v", err)
	}
	h.Abort()
	if _, err := LoadCLISession(); err != nil {
		t.Fatalf("a refused refresh must not look like an interrupted one: %v", err)
	}
}

// Two CLI processes run concurrently in normal use, and each has its own
// in-process mutex. Only an exclusive marker stops both from spending the same
// single-use chain.
func TestCLISessionRotationIsExclusiveAcrossProcesses(t *testing.T) {
	sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	first, err := BeginRotation()
	if err != nil {
		t.Fatalf("first BeginRotation: %v", err)
	}
	if _, err := BeginRotation(); !errors.Is(err, ErrRotationInProgress) {
		t.Fatalf("second process acquired the marker; want ErrRotationInProgress, got %v", err)
	}
	// A loser must not be able to clear the winner's marker, or a crash before
	// the winner's durable write leaves a spent token undetectable. The nonce is
	// non-empty and different: a zero handle is short-circuited by the empty-nonce
	// guard, so it would pass even against an implementation that removed the
	// marker unconditionally once a nonce was present.
	loser := RotationHandle{nonce: "a-different-process-nonce"}
	loser.Abort()
	if _, err := os.Stat(rotationMarkerPath()); err != nil {
		t.Error("a non-owning abort removed the marker")
	}
	ageRotationMarker(t)
	if _, err := LoadCLISession(); !errors.Is(err, ErrSessionInterrupted) {
		t.Error("marker no longer signals an interrupted rotation")
	}
	first.Abort()
	if _, err := os.Stat(rotationMarkerPath()); err == nil {
		t.Error("the owning handle failed to clear its own marker")
	}
}

// The marker guards a crash window, so it must be on disk before the exchange
// it guards -- not merely written into a buffer that a crash discards.
func TestCLISessionRotationMarkerIsDurableBeforeReturn(t *testing.T) {
	sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	h, err := BeginRotation()
	if err != nil {
		t.Fatalf("BeginRotation: %v", err)
	}
	// Readable by an independent open the moment BeginRotation returns, with
	// the owning nonce intact.
	got, err := os.ReadFile(rotationMarkerPath())
	if err != nil {
		t.Fatalf("marker not readable after BeginRotation returned: %v", err)
	}
	if len(got) == 0 {
		t.Error("marker is empty; a non-owning abort could not be distinguished from an owning one")
	}
	ageRotationMarker(t)
	if _, err := LoadCLISession(); !errors.Is(err, ErrSessionInterrupted) {
		t.Errorf("want ErrSessionInterrupted once the marker outlives a live rotation, got %v", err)
	}
	h.Abort()
}

func TestCLISessionRedactsTokens(t *testing.T) {
	s := sampleSession()
	for _, rendered := range []string{s.String(), fmt.Sprintf("%v", s), fmt.Sprintf("%+v", s), fmt.Sprintf("%#v", s)} {
		if strings.Contains(rendered, s.AccessToken) {
			t.Errorf("access token leaked into %q", rendered)
		}
		if strings.Contains(rendered, s.RefreshToken) {
			t.Errorf("refresh token leaked into %q", rendered)
		}
	}
}

// A login completing while another process rotates must not delete that
// process's marker: a third process could then begin a concurrent exchange of
// the same single-use chain, which is the one outcome the marker prevents.
func TestSaveDoesNotClearAnotherProcessLiveMarker(t *testing.T) {
	sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	holder, err := BeginRotation()
	if err != nil {
		t.Fatalf("BeginRotation: %v", err)
	}
	// An unrelated `auth login` lands mid-rotation.
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession during rotation: %v", err)
	}
	if _, err := os.Stat(rotationMarkerPath()); err != nil {
		t.Fatal("a concurrent save removed a live rotation's marker; two processes can now exchange the same chain")
	}
	if _, err := BeginRotation(); !errors.Is(err, ErrRotationInProgress) {
		t.Error("exclusivity lost after a concurrent save")
	}
	holder.Abort()
	// A stale marker is different: nothing live owns it, and a save is the
	// recovery path, so it must be cleared or the user can never get out.
	if _, err := BeginRotation(); err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	ageRotationMarker(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	if _, err := os.Stat(rotationMarkerPath()); err == nil {
		t.Error("a stale marker survived a completed save; the user cannot recover")
	}
}

func TestCLISessionClear(t *testing.T) {
	path := sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	if err := ClearCLISession(); err != nil {
		t.Fatalf("ClearCLISession: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("session file still present after clear")
	}
	if _, err := LoadCLISession(); !errors.Is(err, ErrNoCLISession) {
		t.Errorf("want ErrNoCLISession after clear, got %v", err)
	}
	if err := ClearCLISession(); err != nil {
		t.Errorf("clearing an already-absent session should be a no-op: %v", err)
	}
}

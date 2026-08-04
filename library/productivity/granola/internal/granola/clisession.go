// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PATCH(cli-owned-workos-session): a session this CLI owns, obtained through
// Granola's device authorization grant and stored under the CLI's own data
// directory.
//
// Why the CLI needs its own chain rather than borrowing one. Granola moved the
// data encryption key into an entitlement-gated Keychain group, so the desktop
// app's live token in supabase.json.enc is unreadable, and the plaintext files
// it used to leave behind are stale fossils. The remaining tokens on the
// machine belong to other clients -- the desktop app and the browser -- and
// refreshing someone else's chain against the WorkOS endpoint consumes it and
// signs that client out. So the only safe durable answer is a chain that
// belongs to this CLI, which is what CLISession holds.

// ErrNoCLISession reports that no CLI-owned session is stored. It is a normal
// state, not a failure: callers fall through to the desktop-owned probes.
var ErrNoCLISession = errors.New("granola: no CLI-owned session")

// ErrSessionInterrupted reports that a rotation was in flight when the process
// last exited. The stored refresh token may already have been consumed
// upstream, so the session cannot be trusted and `auth login` is required.
var ErrSessionInterrupted = errors.New("granola: interrupted token rotation; run `granola-pp-cli auth login`")

// ErrSessionPermissions reports that the session file is readable by users
// other than its owner, or is not owned by the current user. Refused rather
// than used: a credential that another account can read is already suspect.
var ErrSessionPermissions = errors.New("granola: session file has unsafe permissions")

// CLISession is the on-disk record. Tokens never reach String/Format output.
type CLISession struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	ObtainedAt   time.Time `json:"obtained_at"`
	AccountID    string    `json:"account_id,omitempty"`
	AccountEmail string    `json:"account_email,omitempty"`
}

// String redacts both tokens. Without this a %v or %+v anywhere in the CLI --
// a debug line, an error wrap -- would print the credential.
func (s CLISession) String() string {
	return fmt.Sprintf("CLISession{account:%q obtained:%s access:[redacted %d bytes] refresh:[redacted %d bytes]}",
		s.AccountEmail, s.ObtainedAt.UTC().Format(time.RFC3339), len(s.AccessToken), len(s.RefreshToken))
}

// GoString makes %#v safe too.
func (s CLISession) GoString() string { return s.String() }

// CLISessionPath resolves the session file. It follows SyncStatePath's
// XDG-aware resolution so the session, the sync state, and the store stay
// co-located, and it honors an override for tests.
func CLISessionPath() string {
	if v := os.Getenv("GRANOLA_CLI_SESSION_PATH"); v != "" {
		return v
	}
	xdg := os.Getenv("XDG_DATA_HOME")
	if xdg == "" {
		home, _ := os.UserHomeDir()
		xdg = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdg, "granola-pp-cli", "session.json")
}

// rotationMarkerPath is written before a token exchange and removed after the
// replacement is durable. Its presence at load time means the process died
// mid-rotation.
func rotationMarkerPath() string { return CLISessionPath() + ".rotating" }

// LoadCLISession reads the stored session.
//
// Returns ErrNoCLISession when absent, ErrSessionInterrupted when a rotation
// marker survives, and ErrSessionPermissions when the file is group/world
// readable or owned by another user.
func LoadCLISession() (CLISession, error) {
	// The marker is checked before the file itself: an interrupted rotation
	// that also lost the session file is still an interrupted rotation, and
	// reporting it as "no session" would hide the one state the marker exists
	// to make visible.
	if _, err := os.Stat(rotationMarkerPath()); err == nil {
		return CLISession{}, ErrSessionInterrupted
	}
	return loadCLISessionUnmarked()
}

// loadCLISessionUnmarked reads the session without consulting the rotation
// marker. Only for callers already holding the refresh lock, where the marker
// is their own and would otherwise block them from reading the very session
// they are about to replace.
func loadCLISessionUnmarked() (CLISession, error) {
	path := CLISessionPath()
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return CLISession{}, ErrNoCLISession
		}
		return CLISession{}, fmt.Errorf("clisession: stat: %w", err)
	}
	if err := checkOwnerOnly(info, path); err != nil {
		return CLISession{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return CLISession{}, fmt.Errorf("clisession: read: %w", err)
	}
	var s CLISession
	if err := json.Unmarshal(data, &s); err != nil {
		// Deliberately a typed error, not "no session": a corrupt file is a
		// state the user should be told about rather than silently treated as
		// a fresh install that then falls through to the desktop probes.
		return CLISession{}, fmt.Errorf("clisession: malformed session file at %s: %w", path, err)
	}
	if s.AccessToken == "" && s.RefreshToken == "" {
		return CLISession{}, ErrNoCLISession
	}
	return s, nil
}

// SaveCLISession writes the session durably.
//
// Durability matters more here than for ordinary state. If a rotation ever
// does consume the presented refresh token upstream and the replacement never
// reaches disk, the session is unrecoverable -- there is no second copy. So the
// write is fsync'd, renamed, and the parent directory fsync'd, and only then is
// the rotation marker cleared.
func SaveCLISession(s CLISession) error {
	path := CLISessionPath()
	dir := filepath.Dir(path)
	if err := ensureOwnerOnlyDir(dir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("clisession: marshal: %w", err)
	}
	tmp := path + ".tmp"
	// 0600 on the temp file, because Rename preserves the source mode -- a
	// 0644 temp file would land as a 0644 credential.
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("clisession: create tmp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("clisession: write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("clisession: fsync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("clisession: close tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("clisession: rename: %w", err)
	}
	// Best-effort for the session itself: the rename already published the file,
	// so an unsyncable directory costs durability of the entry, not the data.
	_ = syncDir(dir)
	os.Remove(rotationMarkerPath())
	return nil
}

// ErrRotationInProgress reports that another process holds the rotation marker.
// The caller must not exchange: the chain is single-use, so a second exchange
// spends the token the holder is already rotating.
var ErrRotationInProgress = errors.New("granola: another process is rotating this session")

// RotationHandle is proof of exclusive rotation ownership. Only the holder can
// clear the marker it created.
type RotationHandle struct{ nonce string }

// BeginRotation claims exclusive rotation ownership across processes.
//
// An in-process mutex is not sufficient here. Two CLI invocations run
// concurrently in normal use -- an agent driving several commands, or
// auto-refresh firing from two shells -- and each has its own mutex. Without
// O_EXCL both would create the marker, both would exchange the same single-use
// token, and the loser would then delete the winner's marker on abort, so a
// crash before the winner's durable write would leave a consumed token on disk
// with nothing to detect it. That is precisely the state the marker exists for.
//
// The nonce makes aborts ownership-aware: a process may only clear a marker it
// wrote. A marker left by a dead process therefore survives, which is intended
// -- that session really is unrecoverable, and `auth login` clears it by writing
// a new session.
func BeginRotation() (RotationHandle, error) {
	if err := ensureOwnerOnlyDir(filepath.Dir(CLISessionPath())); err != nil {
		return RotationHandle{}, err
	}
	nonce := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	f, err := os.OpenFile(rotationMarkerPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return RotationHandle{}, ErrRotationInProgress
		}
		return RotationHandle{}, fmt.Errorf("clisession: begin rotation: %w", err)
	}
	if _, werr := f.WriteString(nonce); werr != nil {
		f.Close()
		os.Remove(rotationMarkerPath())
		return RotationHandle{}, fmt.Errorf("clisession: begin rotation: %w", werr)
	}
	// The marker is a crash-durability device, so it has to be durable itself
	// before the exchange it guards. Written but unsynced, a crash can lose it
	// after the remote token is already spent -- leaving the dead token on disk
	// looking usable, which is the exact state the marker exists to catch. Sync
	// the file, then the directory, so the entry survives too.
	if serr := f.Sync(); serr != nil {
		f.Close()
		os.Remove(rotationMarkerPath())
		return RotationHandle{}, fmt.Errorf("clisession: begin rotation: %w", serr)
	}
	if cerr := f.Close(); cerr != nil {
		os.Remove(rotationMarkerPath())
		return RotationHandle{}, fmt.Errorf("clisession: begin rotation: %w", cerr)
	}
	// Fatal here, unlike the session write. The marker's only job is to be
	// findable after a crash, and it is claimed immediately before an exchange
	// that spends a single-use token. If its directory entry cannot be made
	// durable, refusing to rotate is strictly better than spending the token
	// with no way to detect having lost the record of it.
	if derr := syncDir(filepath.Dir(rotationMarkerPath())); derr != nil {
		os.Remove(rotationMarkerPath())
		return RotationHandle{}, fmt.Errorf("clisession: begin rotation: durable marker unavailable: %w", derr)
	}
	return RotationHandle{nonce: nonce}, nil
}

// Abort clears the marker only when this handle still owns it, so a process
// that failed harmlessly cannot erase another's in-flight rotation.
func (h RotationHandle) Abort() {
	if h.nonce == "" {
		return
	}
	got, err := os.ReadFile(rotationMarkerPath())
	if err != nil || string(got) != h.nonce {
		return
	}
	os.Remove(rotationMarkerPath())
}

// ClearCLISession removes the session. Backs `auth logout`.
//
// Serialized against refreshes in this process, and safe against one in
// another: a concurrent persist that finds the session gone abandons its
// rotation rather than recreating the credential (see
// ErrSessionLoggedOutDuringRotation). Without both halves, a refresh completing
// during logout would silently restore the session the user just removed while
// logout reported success.
func ClearCLISession() error {
	refreshMu.Lock()
	defer refreshMu.Unlock()
	os.Remove(rotationMarkerPath())
	if err := os.Remove(CLISessionPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clisession: remove: %w", err)
	}
	return nil
}

// HasCLISession reports whether a usable session is stored, without returning
// the credential. Used by doctor and auth status.
func HasCLISession() bool {
	_, err := LoadCLISession()
	return err == nil
}

// ensureOwnerOnlyDir creates the directory 0700, and tightens it when it
// already exists more permissively.
//
// The tightening is the load-bearing half. MkdirAll is a no-op on an existing
// directory, so its mode argument never applies to one -- and this directory
// already exists at 0755 on every machine that has ever synced, because
// WriteSyncState created it that way.
func ensureOwnerOnlyDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("clisession: mkdir: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("clisession: stat dir: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("clisession: chmod dir: %w", err)
		}
	}
	return nil
}

package hfx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// StateDir returns the resolved state directory honoring (in priority order):
// 1. an explicit override (e.g., --state-dir flag value)
// 2. HF_CLI_STATE env var
// 3. ~/.local/state/hf-cli/ (XDG-style default)
//
// Always returns an absolute path. Does not create the directory; callers
// scaffold lazily on first write so first-run is silent (per seed).
func StateDir(override string) (string, error) {
	if override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolving --state-dir: %w", err)
		}
		return abs, nil
	}
	if env := os.Getenv("HF_CLI_STATE"); env != "" {
		abs, err := filepath.Abs(env)
		if err != nil {
			return "", fmt.Errorf("resolving HF_CLI_STATE: %w", err)
		}
		return abs, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", "hf-cli"), nil
}

// EnsureStateDir creates the state dir if needed. Idempotent.
// noWrite = true makes this a no-op (returns the path without creating).
func EnsureStateDir(override string, noWrite bool) (string, error) {
	dir, err := StateDir(override)
	if err != nil {
		return "", err
	}
	if noWrite {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating state dir %s: %w", dir, err)
	}
	return dir, nil
}

// AsOf returns the current time as an RFC3339 UTC timestamp. Every JSON
// response carries this so callers detect staleness across container clock
// skew (per seed).
func AsOf() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// fileMu guards in-process concurrent state writes; flock guards cross-process.
var fileMu sync.Mutex

// WriteJSONLocked writes v as pretty-printed JSON to path, atomically and
// guarded by an OS file lock so JARVIS cron + Rick interactive cannot corrupt
// each other's writes. The lock target is `<dir>/.lock`.
//
// noWrite = true makes this a no-op (silent skip).
func WriteJSONLocked(stateDir, path string, v any, noWrite bool) error {
	if noWrite {
		return nil
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}

	fileMu.Lock()
	defer fileMu.Unlock()

	lockPath := filepath.Join(stateDir, ".lock")
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring flock: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)

	tmp := path + ".tmp"
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

// ReadJSONLocked reads the JSON file at path into v. Returns os.ErrNotExist
// if the file does not exist (caller decides whether that's terminal or an
// empty-state init).
func ReadJSONLocked(stateDir, path string, v any) error {
	fileMu.Lock()
	defer fileMu.Unlock()

	lockPath := filepath.Join(stateDir, ".lock")
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		return os.ErrNotExist
	}
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	defer lf.Close()
	if err := syscall.Flock(int(lf.Fd()), syscall.LOCK_SH); err != nil {
		return fmt.Errorf("acquiring shared flock: %w", err)
	}
	defer syscall.Flock(int(lf.Fd()), syscall.LOCK_UN)

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

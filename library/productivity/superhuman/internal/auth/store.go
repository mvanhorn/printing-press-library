// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package auth implements durable token storage and refresh for the
// Superhuman CLI. The store persists multiple accounts in a single
// JSON file under $XDG_CONFIG_HOME/superhuman-pp-cli/tokens.json with
// mode 0600 and atomic rename-on-write semantics.
package auth

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// tokensFilename is the on-disk basename for the multi-account token store.
const tokensFilename = "tokens.json"

// legacyConfigFilename is the sibling TOML file that may contain a bare
// SuperhumanJwt from the pre-durable-auth era. We detect it (without parsing
// the entire file) so callers can surface a one-time upgrade hint.
const legacyConfigFilename = "config.toml"

// CurrentSchemaVersion is the on-disk schema version this package writes.
// Bump when the shape of PersistedTokens changes.
const CurrentSchemaVersion = 1

// ErrUnauthorized signals that the Superhuman backend rejected the
// Authorization header even after a refresh succeeded. This is account-level
// (revoked, banned, deleted) rather than token-level: no further refresh can
// recover it, so the user must re-attach Chrome or check `auth status`.
// Lives in this file (rather than a new errors.go) to keep the auth package
// surface small — store.go already owns package-level vars and the typed-error
// idiom is established here.
var ErrUnauthorized = errors.New("authentication rejected by superhuman backend; check 'auth status' or re-run 'auth login --chrome'")

// PersistedTokens is the top-level on-disk shape.
type PersistedTokens struct {
	Version     int                      `json:"version"`
	Accounts    map[string]AccountTokens `json:"accounts"`
	LastUpdated int64                    `json:"lastUpdated"`

	// LegacyConfigDetected is set by Load when a sibling config.toml contains
	// a non-empty `jwt = "..."` line. Not serialized — populated transiently
	// so callers (auth status, doctor, auth login) can prompt for upgrade.
	LegacyConfigDetected bool `json:"-"`
}

// AccountTokens holds every credential the CLI needs for a single Superhuman
// account. Field names mirror edwinhu/superhuman-cli so a power user can
// drop a tokens.json from one CLI into the other.
type AccountTokens struct {
	Type            string          `json:"type"`
	AccessToken     string          `json:"accessToken"`
	RefreshToken    string          `json:"refreshToken"`
	Expires         int64           `json:"expires"`
	UserID          string          `json:"userId"`
	UserPrefix      string          `json:"userPrefix"`
	UserExternalID  string          `json:"userExternalId"`
	DeviceID        string          `json:"deviceId"`
	SuperhumanToken SuperhumanToken `json:"superhumanToken"`
	LastUsedAt      int64           `json:"lastUsedAt,omitempty"`
}

// SuperhumanToken is the Firebase-issued JWT the CLI sends as Bearer to
// mail.superhuman.com/~backend. Expires is epoch ms.
type SuperhumanToken struct {
	Token   string `json:"token"`
	Expires int64  `json:"expires"`
}

// Store is the on-disk handle. Methods are safe to call serially; concurrent
// Save calls are coordinated by an in-process mutex, and the rename step is
// atomic on POSIX, so partial files can never be observed by a concurrent
// Load.
type Store struct {
	path string

	// mu serializes in-process Save calls so the tmp filename and rename
	// step don't collide between goroutines. Cross-process atomicity still
	// relies on rename(2) being atomic on the same filesystem.
	mu sync.Mutex
}

// NewStore returns a Store at the default per-user path.
//
// Resolution order:
//  1. $XDG_CONFIG_HOME/superhuman-pp-cli/tokens.json (if XDG_CONFIG_HOME set)
//  2. ~/.config/superhuman-pp-cli/tokens.json
//
// Resolving the path does NOT touch the filesystem — the parent dir is
// created lazily on first Save.
func NewStore() (*Store, error) {
	path, err := defaultTokensPath()
	if err != nil {
		return nil, fmt.Errorf("token store: %w", err)
	}
	return &Store{path: path}, nil
}

// NewStoreAt returns a Store at an explicit path. Tests use this to point
// at t.TempDir(); production callers use NewStore.
func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

// Path returns the resolved tokens.json path. Useful for doctor and
// diagnostic output.
func (s *Store) Path() string {
	return s.path
}

// Load returns the persisted state. If the tokens file is absent, Load
// returns a zero-account PersistedTokens (version 1) — absence is not an
// error. If a sibling config.toml has a non-empty `jwt = "..."` line,
// LegacyConfigDetected is set on the returned struct so callers can prompt
// for upgrade without re-parsing the legacy TOML themselves.
func (s *Store) Load() (*PersistedTokens, error) {
	p, err := s.loadFile()
	if err != nil {
		return nil, err
	}

	// Legacy detection is best-effort: a config.toml read error is logged
	// to the caller via the typed flag staying false. Surfacing the error
	// here would conflate "no legacy config" with "couldn't tell"; the
	// caller already has tokens, so we prefer to proceed.
	if detected, _ := legacyJWTPresent(s.path); detected {
		p.LegacyConfigDetected = true
	}
	return p, nil
}

// loadFile reads tokens.json without touching legacy-config detection. Kept
// separate so the legacy probe doesn't recurse into Load.
func (s *Store) loadFile() (*PersistedTokens, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &PersistedTokens{
				Version:  CurrentSchemaVersion,
				Accounts: map[string]AccountTokens{},
			}, nil
		}
		return nil, fmt.Errorf("token store: read %s: %w", s.path, err)
	}

	var p PersistedTokens
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("token store: parse %s: %w", s.path, err)
	}
	if p.Accounts == nil {
		p.Accounts = map[string]AccountTokens{}
	}
	if p.Version == 0 {
		// A schema-less or future-zero file is treated as version 1 on
		// read; we don't surface a typed migration error yet because v1
		// is the only version that has ever shipped.
		p.Version = CurrentSchemaVersion
	}
	return &p, nil
}

// Save atomically writes the state. The on-disk file is created with mode
// 0600 from the start, then chmod'd to 0600 after rename as defense in
// depth (umask differences across platforms have historically widened
// permissions on rename in some edge cases).
//
// The parent directory is created with 0700 if missing.
func (s *Store) Save(p *PersistedTokens) error {
	if p == nil {
		return fmt.Errorf("token store: save: nil PersistedTokens")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	parent := filepath.Dir(s.path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("token store: mkdir %s: %w", parent, err)
	}

	// Normalize before marshal so a caller passing a nil map gets a clean
	// `"accounts": {}` on disk instead of `"accounts": null`. We mutate a
	// shallow copy to keep this defensive normalization invisible to the
	// caller's pointer.
	out := *p
	if out.Accounts == nil {
		out.Accounts = map[string]AccountTokens{}
	}
	if out.Version == 0 {
		out.Version = CurrentSchemaVersion
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("token store: marshal: %w", err)
	}

	// Tmp file lives in the same directory so rename(2) stays a same-fs
	// metadata op. Using a unique suffix per call prevents goroutine A's
	// tmp file from being clobbered by goroutine B mid-flight; if rename
	// loses the race, the loser's bytes are simply discarded by Remove.
	tmpPath, err := writeTempFile(parent, filepath.Base(s.path), data)
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath) // no-op if rename already moved the file

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("token store: rename %s -> %s: %w", tmpPath, s.path, err)
	}

	// Defensive: re-assert 0600 after rename. Rename preserves mode on
	// POSIX, but being explicit costs one syscall and removes a whole
	// category of "why is my token file world-readable" support tickets.
	if err := os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("token store: chmod %s: %w", s.path, err)
	}
	return nil
}

// writeTempFile creates a unique tmp file next to the final path, writes the
// data with mode 0600, fsyncs, and closes it. Returned path is the tmp file
// that the caller is responsible for renaming or removing.
func writeTempFile(parent, finalBase string, data []byte) (string, error) {
	// os.CreateTemp picks a unique name and creates the file with 0600 on
	// POSIX (the implementation uses O_CREATE|O_EXCL with mode 0600). We
	// then OpenFile-style-write the bytes ourselves so we can fsync before
	// rename.
	pattern := finalBase + ".*.tmp"
	f, err := os.CreateTemp(parent, pattern)
	if err != nil {
		return "", fmt.Errorf("token store: create tmp in %s: %w", parent, err)
	}
	tmpPath := f.Name()

	// Belt-and-braces: CreateTemp uses 0600 on Unix, but make it explicit
	// so a future platform divergence can't silently widen the mode.
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("token store: chmod tmp %s: %w", tmpPath, err)
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("token store: write tmp %s: %w", tmpPath, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("token store: fsync tmp %s: %w", tmpPath, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("token store: close tmp %s: %w", tmpPath, err)
	}
	return tmpPath, nil
}

// Upsert loads, replaces the entry at email, bumps LastUpdated, and saves.
// Returns the persisted state. The empty email is rejected — keying on ""
// would shadow accounts and is almost certainly a caller bug.
func (s *Store) Upsert(email string, t AccountTokens) (*PersistedTokens, error) {
	if email == "" {
		return nil, fmt.Errorf("token store: upsert: empty email")
	}
	p, err := s.Load()
	if err != nil {
		return nil, err
	}
	if p.Accounts == nil {
		p.Accounts = map[string]AccountTokens{}
	}
	p.Accounts[email] = t
	p.LastUpdated = nowMillis()
	if err := s.Save(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get loads the store and returns the entry for email. ok is false if the
// account is not present; err is non-nil only on I/O or parse failures.
func (s *Store) Get(email string) (AccountTokens, bool, error) {
	p, err := s.Load()
	if err != nil {
		return AccountTokens{}, false, err
	}
	t, ok := p.Accounts[email]
	return t, ok, nil
}

// SetLegacyDetected lets callers mark that a legacy SuperhumanJwt was
// observed without re-parsing config.toml themselves. Intended for the
// `auth status` and `doctor` commands that already have a *Config in hand.
func (p *PersistedTokens) SetLegacyDetected(detected bool) {
	p.LegacyConfigDetected = detected
}

// defaultTokensPath resolves the on-disk location for tokens.json, honoring
// XDG_CONFIG_HOME first and falling back to ~/.config.
func defaultTokensPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "superhuman-pp-cli", tokensFilename), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "superhuman-pp-cli", tokensFilename), nil
}

// legacyJWTRegexp matches `jwt = "..."` lines in config.toml. It tolerates
// leading whitespace and either single or double quotes; the value must be
// non-empty for the line to count as a real legacy config.
var legacyJWTRegexp = regexp.MustCompile(`^\s*jwt\s*=\s*["']([^"']+)["']\s*$`)

// legacyJWTPresent reports whether a sibling config.toml contains a
// non-empty `jwt = "..."` line. Path resolution mirrors defaultTokensPath
// (the tokens file's parent dir is also the legacy config dir). The probe
// is a line-by-line scan, not a full TOML parse — for one well-known key
// that's both faster and avoids dragging the toml dependency into this
// package.
//
// Returns (false, nil) on most failure modes so the caller can proceed
// without surfacing transient I/O problems as legacy-detection noise.
func legacyJWTPresent(tokensPath string) (bool, error) {
	configPath := filepath.Join(filepath.Dir(tokensPath), legacyConfigFilename)
	f, err := os.Open(configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// 1 MiB line cap handles even pathological legacy configs.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if legacyJWTRegexp.MatchString(scanner.Text()) {
			return true, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// nowMillis is a small package-local indirection so tests can assert that
// LastUpdated is being set without coupling to a real clock. It currently
// just calls time.Now().UnixMilli, but isolating it here means a future
// `WithClock` option can be added without touching every call site.
var nowMillis = func() int64 {
	return time.Now().UnixMilli()
}

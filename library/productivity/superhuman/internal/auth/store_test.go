// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// TestStoreRoundTrip exercises Save -> Load fidelity for a representative
// pair of accounts with non-zero epoch timestamps. Table-driven so adding
// future account shapes is one struct literal.
func TestStoreRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   *PersistedTokens
	}{
		{
			name: "two accounts with full fields",
			in: &PersistedTokens{
				Version:     1,
				LastUpdated: 1_715_000_000_000,
				Accounts: map[string]AccountTokens{
					"user2@example.com": {
						Type:           "google",
						AccessToken:    "ya29.access-google",
						RefreshToken:   "1//rt-google",
						Expires:        1_715_003_600_000,
						UserID:         "google-uid",
						UserPrefix:     "ABCDEF",
						UserExternalID: "user_XYZABCDEFghi123",
						DeviceID:       "device-aaa",
						SuperhumanToken: SuperhumanToken{
							Token:   "eyJ.firebase.google",
							Expires: 1_715_003_700_000,
						},
						LastUsedAt: 1_715_002_000_000,
					},
					"user@example.com": {
						Type:           "microsoft",
						AccessToken:    "EwBwA.access-ms",
						RefreshToken:   "M.R3.ms-refresh",
						Expires:        1_715_005_600_000,
						UserID:         "ms-uid",
						UserPrefix:     "GHIJKL",
						UserExternalID: "user_MNOGHIJKLpqr456",
						DeviceID:       "device-bbb",
						SuperhumanToken: SuperhumanToken{
							Token:   "eyJ.firebase.microsoft",
							Expires: 1_715_005_700_000,
						},
						LastUsedAt: 1_715_004_000_000,
					},
				},
			},
		},
		{
			name: "single account with zero LastUsedAt (omitempty path)",
			in: &PersistedTokens{
				Version:     1,
				LastUpdated: 1_715_010_000_000,
				Accounts: map[string]AccountTokens{
					"only@example.com": {
						Type:    "google",
						Expires: 1_715_013_600_000,
						SuperhumanToken: SuperhumanToken{
							Token:   "eyJ.only",
							Expires: 1_715_013_700_000,
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "tokens.json")
			s := NewStoreAt(path)
			if err := s.Save(tc.in); err != nil {
				t.Fatalf("Save: %v", err)
			}
			got, err := s.Load()
			if err != nil {
				t.Fatalf("Load: %v", err)
			}

			if got.Version != tc.in.Version {
				t.Errorf("Version: got %d want %d", got.Version, tc.in.Version)
			}
			if got.LastUpdated != tc.in.LastUpdated {
				t.Errorf("LastUpdated: got %d want %d", got.LastUpdated, tc.in.LastUpdated)
			}
			if len(got.Accounts) != len(tc.in.Accounts) {
				t.Fatalf("Accounts len: got %d want %d", len(got.Accounts), len(tc.in.Accounts))
			}
			for email, want := range tc.in.Accounts {
				gotAcct, ok := got.Accounts[email]
				if !ok {
					t.Fatalf("missing account %q", email)
				}
				if gotAcct != want {
					t.Errorf("account %q: got %+v want %+v", email, gotAcct, want)
				}
			}
		})
	}
}

// TestStoreMultiAccountUpsertGet exercises the Upsert/Get convenience pair,
// which is the API the auth-login command will call. Two emails are added
// in separate Upsert calls; each must be independently retrievable.
func TestStoreMultiAccountUpsertGet(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tokens.json")
	s := NewStoreAt(path)

	a := AccountTokens{
		Type:         "google",
		RefreshToken: "rt-a",
		SuperhumanToken: SuperhumanToken{
			Token:   "tok-a",
			Expires: 100,
		},
	}
	b := AccountTokens{
		Type:         "microsoft",
		RefreshToken: "rt-b",
		SuperhumanToken: SuperhumanToken{
			Token:   "tok-b",
			Expires: 200,
		},
	}

	if _, err := s.Upsert("a@example.com", a); err != nil {
		t.Fatalf("Upsert a: %v", err)
	}
	if _, err := s.Upsert("b@example.com", b); err != nil {
		t.Fatalf("Upsert b: %v", err)
	}

	gotA, ok, err := s.Get("a@example.com")
	if err != nil {
		t.Fatalf("Get a: %v", err)
	}
	if !ok || gotA != a {
		t.Errorf("Get a: got=%+v ok=%v want=%+v", gotA, ok, a)
	}

	gotB, ok, err := s.Get("b@example.com")
	if err != nil {
		t.Fatalf("Get b: %v", err)
	}
	if !ok || gotB != b {
		t.Errorf("Get b: got=%+v ok=%v want=%+v", gotB, ok, b)
	}

	// Missing account: ok=false, no error.
	_, ok, err = s.Get("nobody@example.com")
	if err != nil {
		t.Errorf("Get missing: unexpected err %v", err)
	}
	if ok {
		t.Errorf("Get missing: ok=true, want false")
	}

	// Empty email is a caller bug.
	if _, err := s.Upsert("", a); err == nil {
		t.Errorf("Upsert empty email: want error, got nil")
	}
}

// TestStoreFileMode0600 verifies the post-Save file mode. This is the
// load-bearing security property of the package — a wider mode means
// other local users can lift the Firebase JWT.
func TestStoreFileMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits not meaningful on Windows")
	}
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tokens.json")
	s := NewStoreAt(path)
	if err := s.Save(&PersistedTokens{Version: 1, Accounts: map[string]AccountTokens{}}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("file mode: got %o want 0600", mode)
	}
}

// TestStoreParentDirCreated0700 verifies that a missing parent dir is
// auto-created with mode 0700. The token file lives in a sensitive dir;
// we don't want it leaking into a world-readable parent.
func TestStoreParentDirCreated0700(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits not meaningful on Windows")
	}
	t.Parallel()

	root := t.TempDir()
	// Nested under a fresh dir that doesn't exist yet.
	path := filepath.Join(root, "fresh-dir", "superhuman-pp-cli", "tokens.json")
	s := NewStoreAt(path)
	if err := s.Save(&PersistedTokens{Version: 1, Accounts: map[string]AccountTokens{}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The immediate parent must exist with 0700. The grandparent is owned
	// by t.TempDir() and may have wider perms — that's expected.
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("Stat parent: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("parent is not a dir")
	}
	if mode := info.Mode().Perm(); mode != 0o700 {
		t.Errorf("parent mode: got %o want 0700", mode)
	}
}

// TestStoreConcurrentSaveAtomicity launches 5 goroutines each calling Save
// with a distinct payload. After they all return, the final file MUST
// parse cleanly (never a partial write), and the parsed content must match
// exactly one of the writes.
func TestStoreConcurrentSaveAtomicity(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tokens.json")
	s := NewStoreAt(path)

	const N = 5
	payloads := make([]*PersistedTokens, N)
	for i := 0; i < N; i++ {
		payloads[i] = &PersistedTokens{
			Version:     1,
			LastUpdated: int64(1_715_000_000_000 + i),
			Accounts: map[string]AccountTokens{
				"writer@example.com": {
					Type:         "google",
					RefreshToken: "rt",
					SuperhumanToken: SuperhumanToken{
						Token:   "tok",
						Expires: int64(i + 1),
					},
				},
			},
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(p *PersistedTokens) {
			defer wg.Done()
			if err := s.Save(p); err != nil {
				errs <- err
			}
		}(payloads[i])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent Save: %v", err)
	}

	// Final file must parse cleanly (no partial write) and equal one of
	// the writes. Last-writer-wins is the contract, but we can't predict
	// scheduling, so we just verify "matches some writer" rather than
	// "matches writer N-1".
	got, err := s.Load()
	if err != nil {
		t.Fatalf("Load after concurrent Save: %v", err)
	}
	matched := false
	for _, p := range payloads {
		if got.LastUpdated == p.LastUpdated {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("final file did not match any writer; got LastUpdated=%d", got.LastUpdated)
	}

	// Tmp files must NOT linger — defer os.Remove + rename should leave
	// only tokens.json in the directory. A failure here means we leaked
	// scratch files into the user's config dir.
	parent := filepath.Dir(path)
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatalf("ReadDir parent: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("leftover tmp file in parent: %s", e.Name())
		}
	}
}

// TestStoreLegacyDetection covers the upgrade-prompt hook: when
// tokens.json is absent but config.toml carries a `jwt = "..."` line,
// Load returns an empty-but-valid PersistedTokens with the flag set. This
// is NOT an error path — callers downstream of Load decide whether to
// surface the upgrade hint.
func TestStoreLegacyDetection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tokensPath := filepath.Join(dir, "tokens.json")
	configPath := filepath.Join(dir, "config.toml")

	// Sibling config.toml with a bare JWT. Quote style is arbitrary; the
	// regex accepts both — exercise the double-quoted form here and the
	// single-quoted form in a subtest below.
	if err := os.WriteFile(configPath, []byte(`base_url = "https://mail.superhuman.com/~backend"
jwt = "eyJ.legacy.jwt"
`), 0o600); err != nil {
		t.Fatalf("seed config.toml: %v", err)
	}

	s := NewStoreAt(tokensPath)
	p, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !p.LegacyConfigDetected {
		t.Errorf("LegacyConfigDetected: got false, want true")
	}
	if len(p.Accounts) != 0 {
		t.Errorf("Accounts: got %d entries, want 0", len(p.Accounts))
	}
	if p.Version != CurrentSchemaVersion {
		t.Errorf("Version: got %d want %d", p.Version, CurrentSchemaVersion)
	}

	t.Run("empty jwt value does not trigger", func(t *testing.T) {
		dir := t.TempDir()
		tokensPath := filepath.Join(dir, "tokens.json")
		configPath := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(configPath, []byte(`jwt = ""`), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		s := NewStoreAt(tokensPath)
		p, err := s.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if p.LegacyConfigDetected {
			t.Errorf("empty jwt should not count as legacy")
		}
	})

	t.Run("single-quoted jwt also detected", func(t *testing.T) {
		dir := t.TempDir()
		tokensPath := filepath.Join(dir, "tokens.json")
		configPath := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(configPath, []byte(`jwt = 'eyJ.single.quoted'`), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		s := NewStoreAt(tokensPath)
		p, err := s.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !p.LegacyConfigDetected {
			t.Errorf("single-quoted jwt should be detected")
		}
	})

	t.Run("no config.toml means no flag", func(t *testing.T) {
		s := NewStoreAt(filepath.Join(t.TempDir(), "tokens.json"))
		p, err := s.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if p.LegacyConfigDetected {
			t.Errorf("flag must be false when config.toml absent")
		}
	})
}

// TestStorePermissionDeniedParent verifies that an unwritable parent
// produces an error wrapped with the "token store" prefix and mentioning
// the mkdir step, so callers can downstream-classify the failure.
func TestStorePermissionDeniedParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses POSIX permission checks")
	}
	t.Parallel()

	root := t.TempDir()
	// A read-only parent ensures MkdirAll for the sub-dir fails.
	readOnly := filepath.Join(root, "ro")
	if err := os.Mkdir(readOnly, 0o500); err != nil {
		t.Fatalf("Mkdir read-only: %v", err)
	}
	// Restore write so t.TempDir cleanup can remove it. Without this,
	// `go test` leaves dangling temp dirs that wear on CI disk.
	t.Cleanup(func() {
		_ = os.Chmod(readOnly, 0o700)
	})

	path := filepath.Join(readOnly, "sub", "tokens.json")
	s := NewStoreAt(path)
	err := s.Save(&PersistedTokens{Version: 1, Accounts: map[string]AccountTokens{}})
	if err == nil {
		t.Fatalf("Save: expected error, got nil")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "token store:") {
		t.Errorf("error prefix: got %q, want 'token store:' prefix", msg)
	}
	if !strings.Contains(msg, "mkdir") {
		t.Errorf("error message must mention mkdir; got %q", msg)
	}
	// The wrapped error must still satisfy errors.Is(fs.ErrPermission)
	// so callers can match without string-sniffing.
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("errors.Is(err, fs.ErrPermission) = false; want true; err=%v", err)
	}
}

// TestStoreLoadMissingFile verifies the documented "absent file ->
// empty PersistedTokens" contract. The auth-login command relies on this
// to bootstrap a fresh user without a special-case "first run" branch.
func TestStoreLoadMissingFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tokens.json")
	s := NewStoreAt(path)
	p, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if p == nil {
		t.Fatalf("Load: got nil PersistedTokens")
	}
	if p.Version != CurrentSchemaVersion {
		t.Errorf("Version: got %d want %d", p.Version, CurrentSchemaVersion)
	}
	if p.Accounts == nil {
		t.Errorf("Accounts: got nil, want empty map")
	}
	if len(p.Accounts) != 0 {
		t.Errorf("Accounts: got %d entries, want 0", len(p.Accounts))
	}
}

// TestStoreNewStorePathResolution verifies the XDG_CONFIG_HOME override
// and the home-dir fallback. Captures the path the production
// constructor will resolve to so a regression in default location is
// caught by the unit suite, not at runtime.
func TestStoreNewStorePathResolution(t *testing.T) {
	// Cannot run in parallel: mutates process env.
	t.Run("XDG_CONFIG_HOME takes precedence", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
		s, err := NewStore()
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		want := filepath.Join("/tmp/xdg-test", "superhuman-pp-cli", "tokens.json")
		if s.Path() != want {
			t.Errorf("Path: got %q want %q", s.Path(), want)
		}
	})

	t.Run("home dir fallback when XDG unset", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		s, err := NewStore()
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("UserHomeDir: %v", err)
		}
		want := filepath.Join(home, ".config", "superhuman-pp-cli", "tokens.json")
		if s.Path() != want {
			t.Errorf("Path: got %q want %q", s.Path(), want)
		}
	})
}

// TestStoreSaveNormalizesNilAccounts ensures a caller passing a
// PersistedTokens with a nil Accounts map gets `"accounts": {}` on disk,
// not `"accounts": null`. The latter would force every reader to handle
// both shapes; we centralize the normalization here.
func TestStoreSaveNormalizesNilAccounts(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "tokens.json")
	s := NewStoreAt(path)
	if err := s.Save(&PersistedTokens{Version: 1, Accounts: nil}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var probe struct {
		Accounts map[string]json.RawMessage `json:"accounts"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("Unmarshal probe: %v", err)
	}
	if probe.Accounts == nil {
		t.Errorf("accounts field serialized as null; want {}")
	}
}

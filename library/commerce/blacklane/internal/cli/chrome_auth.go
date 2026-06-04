// Copyright 2026 omarshahine. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Chrome auth import (not generator output).
// PATCH: `auth login --chrome` reads Blacklane's current Auth0 access token
// straight from Chrome's Local Storage LevelDB, so users don't have to dig
// through DevTools. It imports only the 24h access token (not the rotating
// refresh token) to avoid poisoning the browser's session. Pattern adapted from
// the published superhuman-pp-cli chrome reader.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf16"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// chromeDataDir returns the OS-specific Chrome user-data directory.
func chromeDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome"), nil
	case "linux":
		for _, name := range []string{"google-chrome", "chromium"} {
			p := filepath.Join(home, ".config", name)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		return "", fmt.Errorf("Chrome/Chromium config dir not found")
	case "windows":
		ad := os.Getenv("LOCALAPPDATA")
		if ad == "" {
			return "", fmt.Errorf("LOCALAPPDATA not set")
		}
		return filepath.Join(ad, "Google", "Chrome", "User Data"), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// snapshotLevelDB copies a Chrome LevelDB dir to a temp dir so it can be opened
// without contending with Chrome's writer lock. Caller deletes the snapshot.
func snapshotLevelDB(srcDir string) (string, error) {
	if _, err := os.Stat(srcDir); err != nil {
		return "", fmt.Errorf("local storage dir: %w", err)
	}
	dst, err := os.MkdirTemp("", "blacklane-pp-cli-leveldb-*")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		os.RemoveAll(dst)
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() == "LOCK" { // skip Chrome's lock file
			continue
		}
		s, err := os.Open(filepath.Join(srcDir, e.Name()))
		if err != nil {
			os.RemoveAll(dst)
			return "", err
		}
		d, err := os.Create(filepath.Join(dst, e.Name()))
		if err != nil {
			s.Close()
			os.RemoveAll(dst)
			return "", err
		}
		if _, err := io.Copy(d, s); err != nil {
			s.Close()
			d.Close()
			os.RemoveAll(dst)
			return "", err
		}
		s.Close()
		d.Close()
	}
	return dst, nil
}

func decodeUTF16LE(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	if len(b) == 0 {
		return ""
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return string(utf16.Decode(u16))
}

func printableRatio(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	good := 0
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' || (r >= 0x20 && r <= 0x7e) {
			good++
		}
	}
	return float64(good) / float64(len([]rune(s)))
}

// decodeChromeString tries both Chrome encodings and returns whichever looks
// more like real text (the type tag isn't reliable across Chrome versions).
func decodeChromeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	rest := b
	if b[0] == 0x00 || b[0] == 0x01 {
		rest = b[1:]
	}
	if len(rest) == 0 {
		return ""
	}
	one := string(rest)
	two := decodeUTF16LE(rest)
	if printableRatio(two) > printableRatio(one)+0.1 {
		return two
	}
	return one
}

// readBlacklaneLocalStorage returns every localStorage key/value for the
// Blacklane origin from a given Chrome profile dir.
func readBlacklaneLocalStorage(profileDir string) (map[string]string, error) {
	srcDir := filepath.Join(profileDir, "Local Storage", "leveldb")
	snap, err := snapshotLevelDB(srcDir)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(snap)

	db, err := leveldb.OpenFile(snap, &opt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("open leveldb: %w", err)
	}
	defer db.Close()

	prefix := []byte("_https://www.blacklane.com\x00\x01")
	out := make(map[string]string)
	iter := db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()
	for iter.Next() {
		keyName := decodeChromeString(bytes.TrimPrefix(iter.Key(), prefix))
		if keyName == "" {
			continue
		}
		v := iter.Value()
		valBytes := make([]byte, len(v))
		copy(valBytes, v)
		out[keyName] = decodeChromeString(valBytes)
	}
	if err := iter.Error(); err != nil {
		return out, fmt.Errorf("iterate leveldb: %w", err)
	}
	return out, nil
}

// candidateProfiles returns the profile dirs to search, preferring the named
// one (default "Default"), then any other Profile* dirs.
func candidateProfiles(dataDir, named string) []string {
	if named == "" {
		named = "Default"
	}
	dirs := []string{filepath.Join(dataDir, named)}
	if entries, err := os.ReadDir(dataDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			n := e.Name()
			if n == named {
				continue
			}
			if n == "Default" || strings.HasPrefix(n, "Profile ") {
				dirs = append(dirs, filepath.Join(dataDir, n))
			}
		}
	}
	return dirs
}

// auth0CacheTokens extracts the access token + expiry from an @@auth0spajs@@
// localStorage value (the Auth0 SPA token cache). We intentionally do NOT take
// the refresh token: Auth0 rotates refresh tokens, so reusing the browser's
// would poison its session (reuse-detection). The 24h access token is read-only
// w.r.t. the browser's auth family.
func auth0CacheTokens(val string) (accessToken string, expiresAt int64) {
	var c struct {
		Body struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		} `json:"body"`
		ExpiresAt int64 `json:"expiresAt"`
	}
	if json.Unmarshal([]byte(val), &c) != nil {
		return "", 0
	}
	exp := c.ExpiresAt
	if exp == 0 && c.Body.ExpiresIn > 0 {
		exp = c.Body.ExpiresIn // best effort; treated as absolute below if large
	}
	if exp == 0 {
		// Neither expiresAt nor expires_in present in the cache entry — assume the
		// standard 24h Auth0 SPA access-token lifetime so the imported token is
		// usable. The caller treats this small value as a relative duration and
		// converts it to an absolute expiry. Without this fallback, exp=0 would
		// stamp the token already-expired and a successful `auth login --chrome`
		// would immediately report "session expired".
		exp = 86400
	}
	return c.Body.AccessToken, exp
}

// importAccessFromChrome finds Blacklane's current Auth0 access token in Chrome's
// localStorage. Returns the token, its expiry (unix seconds), and the profile.
func importAccessFromChrome(profile string) (accessToken string, expiresAt int64, foundProfile string, err error) {
	dataDir, err := chromeDataDir()
	if err != nil {
		return "", 0, "", err
	}
	var lastErr error
	for _, pdir := range candidateProfiles(dataDir, profile) {
		kv, err := readBlacklaneLocalStorage(pdir)
		if err != nil {
			lastErr = err
			continue
		}
		for k, v := range kv {
			if strings.Contains(k, "@@auth0spajs@@") && strings.Contains(k, "openid") {
				if at, exp := auth0CacheTokens(v); at != "" {
					return at, exp, filepath.Base(pdir), nil
				}
			}
		}
	}
	if lastErr != nil {
		return "", 0, "", fmt.Errorf("could not read Chrome storage: %w (is Chrome installed? try the manual 'pbpaste | auth login' path)", lastErr)
	}
	return "", 0, "", fmt.Errorf("no Blacklane login found in Chrome — log in to blacklane.com in Chrome first, or use the manual 'pbpaste | auth login' path")
}

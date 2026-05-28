// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package auth's chrome_disk.go reads tokens directly from Chrome's on-disk
// LevelDB files, bypassing CDP entirely. This is the runtime-friendly path: it
// doesn't require Chrome's debug port to be open, doesn't require an MCP
// middleware, and doesn't require relaunching Chrome.
//
// On macOS, Chrome stores localStorage at:
//   ~/Library/Application Support/Google/Chrome/<Profile>/Local Storage/leveldb/
//
// LevelDB allows concurrent readers while Chrome holds the writer lock, but
// in practice we snapshot-copy the directory to a temp dir first to avoid any
// race with Chrome's compaction activity. The copy takes ~50ms even for large
// directories.
//
// Chrome's localStorage on-disk schema (best summarized by chromium source
// `content/browser/dom_storage/local_storage_impl.cc`):
//   - Metadata keys are prefixed with `META:` followed by the origin
//   - Value keys are prefixed with `_` then the origin then `\x00\x01` then
//     the UTF-16-LE-encoded key name
//   - Values are `\x01` (or `\x00` for empty) followed by the UTF-16-LE-encoded
//     value bytes
//
// We iterate every key, filter those matching the Superhuman origin, and
// decode the values. The JWT lives under a key whose name ends in `:token`
// or similar (Superhuman's exact naming is observed at extraction time, not
// pre-hardcoded — we identify the JWT by its `eyJ` prefix shape).

package auth

import (
	"bytes"
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

// ErrChromeNotInstalled is returned when no Chrome user-data-dir exists at any
// known path for the current OS. The caller should surface this with a hint
// pointing at the `auth set-token` fallback.
var ErrChromeNotInstalled = fmt.Errorf("chrome user-data-dir not found at expected path")

// ErrNoSuperhumanLogin is returned when Chrome is installed but no profile
// contains a Superhuman login (no localStorage entries for the origin).
var ErrNoSuperhumanLogin = fmt.Errorf("no Superhuman login found in any Chrome profile; log in at https://mail.superhuman.com/ first")

// ErrJWTNotFound is returned when Chrome's localStorage contains the
// Superhuman origin but no JWT-shaped value (starting with `eyJ`). This can
// happen if the user has logged out of Superhuman or if Chrome cleared the
// data.
var ErrJWTNotFound = fmt.Errorf("Superhuman origin present in Chrome but no JWT found; re-log into mail.superhuman.com")

// DiskExtractedTokens is the subset of ExtractedTokens we can pull from
// Chrome's on-disk localStorage. Fields the CDP-IIFE path retrieves from
// in-memory globals (refresh_token, accessToken, deviceId, etc.) may or may
// not be present here — Firebase Auth's persistence model varies by app.
// Callers should treat missing fields as "not on disk for this user".
type DiskExtractedTokens struct {
	Email          string `json:"email,omitempty"`
	IDToken        string `json:"id_token"`
	IDTokenExpires int64  `json:"id_token_expires,omitempty"` // epoch ms; 0 if not parsed
	RefreshToken   string `json:"refresh_token,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	Provider       string `json:"provider,omitempty"` // "google" or "microsoft"

	// Raw is the complete map of Superhuman localStorage keys -> decoded
	// string values. Useful for diagnostics and for callers that want to
	// extract fields we didn't pre-name.
	Raw map[string]string `json:"-"`

	// ProfilePath is the absolute path to the Chrome profile dir this came from.
	ProfilePath string `json:"-"`
}

// ChromeDataDirOverride is a test seam. When set, ChromeDataDir returns this
// path verbatim, bypassing the OS-specific resolution. Production code never
// writes this — only tests use it to inject a fixture profile so the disk-auth
// flow can be exercised without touching the user's real Chrome.
var ChromeDataDirOverride string

// ChromeDataDir returns the OS-specific path to the Chrome user-data-dir
// (the directory containing profile subdirectories like "Default", "Profile 1").
func ChromeDataDir() (string, error) {
	if ChromeDataDirOverride != "" {
		return ChromeDataDirOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome"), nil
	case "linux":
		// Try google-chrome first, then chromium
		for _, name := range []string{"google-chrome", "chromium"} {
			p := filepath.Join(home, ".config", name)
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		return "", ErrChromeNotInstalled
	case "windows":
		appData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			return "", fmt.Errorf("LOCALAPPDATA env var not set")
		}
		return filepath.Join(appData, "Google", "Chrome", "User Data"), nil
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// snapshotLevelDB copies a Chrome LevelDB directory to a temp dir so we can
// open it without contending with Chrome's writer lock. Returns the path to
// the snapshot, which the caller is responsible for deleting via os.RemoveAll.
func snapshotLevelDB(srcDir string) (string, error) {
	if _, err := os.Stat(srcDir); err != nil {
		return "", fmt.Errorf("snapshot: source dir: %w", err)
	}
	dst, err := os.MkdirTemp("", "superhuman-pp-cli-leveldb-*")
	if err != nil {
		return "", fmt.Errorf("snapshot: mkdir: %w", err)
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		os.RemoveAll(dst)
		return "", fmt.Errorf("snapshot: readdir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue // Chrome LevelDB has no subdirs
		}
		// Skip the LOCK file — copying it would just bring along Chrome's
		// lock state. goleveldb in read-only mode skips lock acquisition.
		if e.Name() == "LOCK" {
			continue
		}
		s, err := os.Open(filepath.Join(srcDir, e.Name()))
		if err != nil {
			os.RemoveAll(dst)
			return "", fmt.Errorf("snapshot: open %s: %w", e.Name(), err)
		}
		d, err := os.Create(filepath.Join(dst, e.Name()))
		if err != nil {
			s.Close()
			os.RemoveAll(dst)
			return "", fmt.Errorf("snapshot: create %s: %w", e.Name(), err)
		}
		if _, err := io.Copy(d, s); err != nil {
			s.Close()
			d.Close()
			os.RemoveAll(dst)
			return "", fmt.Errorf("snapshot: copy %s: %w", e.Name(), err)
		}
		s.Close()
		d.Close()
	}
	return dst, nil
}

// decodeOneByte returns the byte slice as a Latin-1 / UTF-8 string (one byte
// per char). Chrome stores values in this format when the original WebKit
// String fits in one-byte representation.
func decodeOneByte(b []byte) string {
	return string(b)
}

// decodeUTF16LE returns the byte slice as a UTF-16 LE string (two bytes per
// char). Chrome uses this when the original WebKit String has any
// non-Latin-1 codepoint.
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

// printableRatio returns the fraction of bytes in `s` that are printable
// ASCII (0x20..0x7e) plus common whitespace (\t, \n, \r). High ratio means the
// string looks like real text; low ratio means we likely decoded the wrong
// encoding.
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

// decodeChromeString tries both Chrome encodings (one-byte and UTF-16 LE) on
// the bytes after the type tag and returns whichever looks more like real
// text. This is empirically more robust than trusting the type tag, which
// (in our observation) doesn't reliably indicate the encoding in every
// Chrome version.
//
// Both encodings are also exposed as separate fields on the result so callers
// who know the expected encoding can pick directly.
func decodeChromeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// Strip leading type tag (0x00 or 0x01) if present.
	rest := b
	if b[0] == 0x00 || b[0] == 0x01 {
		rest = b[1:]
	}
	if len(rest) == 0 {
		return ""
	}
	one := decodeOneByte(rest)
	two := decodeUTF16LE(rest)
	if printableRatio(two) > printableRatio(one)+0.1 {
		return two
	}
	return one
}

// decodeChromeKey decodes the part of a LevelDB key that follows the
// `_<origin>\x00\x01` prefix.
func decodeChromeKey(b []byte) string {
	return decodeChromeString(b)
}

// decodeChromeStringBoth returns both candidate decodings so callers can
// search for specific markers (e.g., JWT prefixes) in either encoding without
// relying on the printable-ratio heuristic.
func decodeChromeStringBoth(b []byte) (oneByte, utf16LE string) {
	if len(b) == 0 {
		return "", ""
	}
	rest := b
	if b[0] == 0x00 || b[0] == 0x01 {
		rest = b[1:]
	}
	if len(rest) == 0 {
		return "", ""
	}
	return decodeOneByte(rest), decodeUTF16LE(rest)
}

// SuperhumanLocalStorage is the result of reading Chrome's localStorage LevelDB
// for the Superhuman origin. Values are stored under both Chrome encodings so
// callers can search for markers (JWT prefix etc.) in either space without
// guessing.
type SuperhumanLocalStorage struct {
	// KV is the heuristic best-decoded value per key (whichever encoding looked
	// more like real text). Use this for general inspection.
	KV map[string]string
	// OneByte and UTF16 are the raw decodings — same key set as KV, useful when
	// the heuristic gets it wrong.
	OneByte map[string]string
	UTF16   map[string]string
}

// ReadSuperhumanLocalStorage opens Chrome's localStorage LevelDB (after
// snapshotting) and returns every key/value pair belonging to the Superhuman
// origin. Cleanup of the snapshot is automatic on return.
func ReadSuperhumanLocalStorage(profileDir string) (map[string]string, error) {
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

	// Chrome localStorage key format on disk:
	//   `_<origin>\x00\x01<utf16-le-key>`
	// The first byte is the storage-area marker `_`, followed by the origin
	// string (UTF-8 ASCII), then NUL, then `\x01` (key separator), then the
	// key name as UTF-16 LE bytes.
	prefix := []byte("_https://mail.superhuman.com\x00\x01")

	out := make(map[string]string)
	iter := db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		// Strip the prefix to get the encoded key name (with leading type tag).
		keyBytes := bytes.TrimPrefix(k, prefix)
		keyName := decodeChromeKey(keyBytes)
		if keyName == "" {
			continue
		}
		// Make a copy of the value because iter.Value() reuses its buffer.
		valBytes := make([]byte, len(v))
		copy(valBytes, v)
		valStr := decodeChromeString(valBytes)
		out[keyName] = valStr
	}
	if err := iter.Error(); err != nil {
		return out, fmt.Errorf("iterate leveldb: %w", err)
	}
	if len(out) == 0 {
		return nil, ErrNoSuperhumanLogin
	}
	return out, nil
}

// ReadSuperhumanLocalStorageBoth is like ReadSuperhumanLocalStorage but
// returns both candidate encodings for every value, so callers can search
// for markers (JWT prefix, etc.) in either encoding without relying on the
// printable-ratio heuristic.
func ReadSuperhumanLocalStorageBoth(profileDir string) (*SuperhumanLocalStorage, error) {
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

	prefix := []byte("_https://mail.superhuman.com\x00\x01")
	result := &SuperhumanLocalStorage{
		KV:      make(map[string]string),
		OneByte: make(map[string]string),
		UTF16:   make(map[string]string),
	}
	iter := db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		keyBytes := bytes.TrimPrefix(k, prefix)
		keyName := decodeChromeKey(keyBytes)
		if keyName == "" {
			continue
		}
		valBytes := make([]byte, len(v))
		copy(valBytes, v)
		one, two := decodeChromeStringBoth(valBytes)
		result.OneByte[keyName] = one
		result.UTF16[keyName] = two
		result.KV[keyName] = decodeChromeString(valBytes)
	}
	if err := iter.Error(); err != nil {
		return result, fmt.Errorf("iterate leveldb: %w", err)
	}
	if len(result.KV) == 0 {
		return nil, ErrNoSuperhumanLogin
	}
	return result, nil
}

// findJWT scans both encoding spaces for a value starting with `eyJ` (the JWT
// prefix) and returns the first match. Superhuman may store the JWT in either
// the one-byte or UTF-16 encoding depending on the Firebase SDK version.
func findJWT(s *SuperhumanLocalStorage) (key, jwt, encoding string) {
	if s == nil {
		return "", "", ""
	}
	for k, v := range s.OneByte {
		if strings.HasPrefix(v, "eyJ") && len(v) > 100 {
			return k, v, "one-byte"
		}
	}
	for k, v := range s.UTF16 {
		if strings.HasPrefix(v, "eyJ") && len(v) > 100 {
			return k, v, "utf-16"
		}
	}
	return "", "", ""
}

// FindAccountEmails returns the set of emails whose localStorage keys exist
// for the Superhuman origin. Superhuman scopes per-account state with keys
// like `user@example.com:flags`, `user@example.com:seatId`, etc. — we extract
// the email prefix from any key matching `<email>:*` shape.
func FindAccountEmails(kv map[string]string) []string {
	seen := make(map[string]bool)
	for k := range kv {
		if i := strings.Index(k, ":"); i > 0 {
			candidate := k[:i]
			// Lightweight email validation: must contain @ and have a TLD-ish suffix.
			if strings.Contains(candidate, "@") && strings.Contains(candidate, ".") {
				seen[candidate] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	return out
}

// ExtractFromDisk is the top-level entry point: discover Chrome's data dir,
// pick a profile, read Superhuman's localStorage, find the JWT, and return a
// DiskExtractedTokens ready for the token store.
//
// If `profileName` is empty, defaults to "Default". For now, multi-profile
// support is deferred — the user's machine has only the Default profile.
//
// If `accountEmail` is empty and multiple accounts are detected, returns an
// error listing the emails so the caller can prompt.
func ExtractFromDisk(profileName, accountEmail string) (*DiskExtractedTokens, error) {
	if profileName == "" {
		profileName = "Default"
	}
	dataDir, err := ChromeDataDir()
	if err != nil {
		return nil, err
	}
	profileDir := filepath.Join(dataDir, profileName)
	if _, err := os.Stat(profileDir); err != nil {
		return nil, fmt.Errorf("profile %q not found at %s", profileName, profileDir)
	}

	s, err := ReadSuperhumanLocalStorageBoth(profileDir)
	if err != nil {
		return nil, err
	}
	kv := s.KV

	jwtKey, jwt, _ := findJWT(s)
	if jwt == "" {
		return nil, ErrJWTNotFound
	}

	// Identify the email this JWT belongs to: if jwtKey starts with `<email>:`,
	// that's the account. Otherwise fall back to active account discovery.
	email := accountEmail
	if email == "" {
		if i := strings.Index(jwtKey, ":"); i > 0 {
			candidate := jwtKey[:i]
			if strings.Contains(candidate, "@") {
				email = candidate
			}
		}
	}
	if email == "" {
		// Fall back to whichever email appears most in the kv keys.
		emails := FindAccountEmails(kv)
		if len(emails) == 1 {
			email = emails[0]
		} else if len(emails) > 1 {
			return nil, fmt.Errorf("multiple Superhuman accounts found (%v); specify --account <email>", emails)
		}
	}

	// Detect provider: Superhuman scopes provider as `<email>:provider` typically.
	provider := kv[email+":provider"]
	if provider == "" {
		provider = "google" // sensible default
	}

	// User ID and external id (best-effort — keys vary).
	userID := kv[email+":id"]
	if userID == "" {
		userID = kv[email+":seatId"]
	}

	return &DiskExtractedTokens{
		Email:       email,
		IDToken:     jwt,
		UserID:      userID,
		Provider:    provider,
		Raw:         kv,
		ProfilePath: profileDir,
	}, nil
}

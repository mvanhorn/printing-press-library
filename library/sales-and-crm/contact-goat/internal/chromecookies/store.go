// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package chromecookies

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CookieFile is the on-disk representation of a saved cookie set. The file is
// written with mode 0600 and stored under ~/.config/<appDir>/ so it inherits
// the dotfile directory's 0700 permissions. Cookie values are stored
// plaintext — the file permissions are the only thing protecting them.
type CookieFile struct {
	Service  string    `json:"service"`
	SavedAt  time.Time `json:"saved_at"`
	SourceOS string    `json:"source_os"`
	Cookies  []Cookie  `json:"cookies"`
}

// DefaultCookieFilePath returns the conventional location for the saved
// cookie file, e.g.
// ~/.config/contact-goat-pp-cli/cookies-happenstance.json.
func DefaultCookieFilePath(appDir, service string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", appDir, "cookies-"+service+".json"), nil
}

// WriteCookieFile persists cookies to disk with mode 0600. The parent
// directory is created with mode 0700. Returns the absolute path written.
func WriteCookieFile(path, service string, cookies []Cookie) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	cf := CookieFile{
		Service:  service,
		SavedAt:  time.Now().UTC(),
		SourceOS: "darwin",
		Cookies:  cookies,
	}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cookie file: %w", err)
	}
	// Write to a temp file + rename so we never leave a half-written
	// secrets file at the target path.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write cookie file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename cookie file: %w", err)
	}
	return nil
}

// ReadCookieFile loads a previously-saved cookie file from disk. Returns
// os.ErrNotExist when the file is missing so callers can distinguish "never
// logged in" from real I/O errors.
func ReadCookieFile(path string) (*CookieFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cf CookieFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse cookie file: %w", err)
	}
	return &cf, nil
}

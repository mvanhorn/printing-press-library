// Copyright 2026 Peter Yang and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"crypto/sha1"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/substack-notes/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

type browserLoginResult struct {
	Browser     string `json:"browser"`
	Profile     string `json:"profile,omitempty"`
	ConfigPath  string `json:"config_path"`
	CookieCount int    `json:"cookie_count"`
	Saved       bool   `json:"saved"`
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var browser string
	var profile string
	var cookieDB string
	cmd := &cobra.Command{
		Use:     "login",
		Short:   "Save auth from a local Substack browser session",
		Example: "  substack-notes-pp-cli auth login --browser chrome\n  substack-notes-pp-cli auth login --browser brave --profile Default",
		RunE: func(cmd *cobra.Command, args []string) error {
			header, count, err := discoverSubstackCookieHeader(browser, profile, cookieDB)
			if err != nil {
				return configErr(err)
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.AuthHeaderVal = ""
			if err := cfg.SaveCredential(header); err != nil {
				return configErr(fmt.Errorf("saving browser session: %w", err))
			}
			result := browserLoginResult{
				Browser:     normalizeBrowserName(browser),
				Profile:     profile,
				ConfigPath:  cfg.Path,
				CookieCount: count,
				Saved:       true,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if result.Profile == "" {
				result.Profile = "Default"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved Substack browser session from %s/%s to %s\n", result.Browser, result.Profile, cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&browser, "browser", "chrome", "Browser to read: chrome, brave, or arc")
	cmd.Flags().StringVar(&profile, "profile", "Default", "Browser profile name")
	cmd.Flags().StringVar(&cookieDB, "cookie-db", "", "Advanced: explicit Chromium Cookies SQLite path")
	_ = cmd.Flags().MarkHidden("cookie-db")
	return cmd
}

func discoverSubstackCookieHeader(browser, profile, explicitDB string) (string, int, error) {
	dbPath := explicitDB
	if strings.TrimSpace(dbPath) == "" {
		found, err := findBrowserCookieDB(browser, profile)
		if err != nil {
			return "", 0, err
		}
		dbPath = found
	}
	cookies, err := readSubstackCookiesFromDB(dbPath, normalizeBrowserName(browser))
	if err != nil {
		return "", 0, err
	}
	if len(cookies) == 0 {
		return "", 0, fmt.Errorf("no Substack cookies found in %s; sign in to Substack in that browser and retry", dbPath)
	}
	if !hasSessionCookie(cookies) {
		return "", 0, fmt.Errorf("found Substack cookies in %s, but no session-like cookie; sign in to Substack and retry", dbPath)
	}
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+cookies[name])
	}
	return strings.Join(parts, "; "), len(parts), nil
}

func findBrowserCookieDB(browser, profile string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("browser login currently supports macOS Chromium-family profiles; use auth set-token as the manual fallback")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(profile) == "" {
		profile = "Default"
	}
	var base string
	switch normalizeBrowserName(browser) {
	case "chrome":
		base = filepath.Join(home, "Library", "Application Support", "Google", "Chrome")
	case "brave":
		base = filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser")
	case "arc":
		base = filepath.Join(home, "Library", "Application Support", "Arc", "User Data")
	default:
		return "", fmt.Errorf("unsupported browser %q; supported: chrome, brave, arc", browser)
	}
	candidates := []string{
		filepath.Join(base, profile, "Network", "Cookies"),
		filepath.Join(base, profile, "Cookies"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not find Cookies database for %s profile %q", normalizeBrowserName(browser), profile)
}

func normalizeBrowserName(browser string) string {
	switch strings.ToLower(strings.TrimSpace(browser)) {
	case "", "chrome", "google-chrome", "google chrome":
		return "chrome"
	case "brave", "brave-browser", "brave browser":
		return "brave"
	case "arc":
		return "arc"
	default:
		return strings.ToLower(strings.TrimSpace(browser))
	}
}

func readSubstackCookiesFromDB(path, browser string) (map[string]string, error) {
	tmp, err := copyCookieDB(path)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp)
	db, err := sql.Open("sqlite", tmp+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT host_key, name, value, encrypted_value FROM cookies WHERE host_key LIKE '%substack.com%' ORDER BY host_key, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cookies := map[string]string{}
	for rows.Next() {
		var host, name, value string
		var encrypted []byte
		if err := rows.Scan(&host, &name, &value, &encrypted); err != nil {
			return nil, err
		}
		if !isSubstackCookieHost(host) || strings.TrimSpace(name) == "" {
			continue
		}
		if value == "" && len(encrypted) > 0 {
			if decrypted, err := decryptChromiumCookie(browser, encrypted); err == nil {
				value = decrypted
			}
		}
		if value == "" {
			continue
		}
		if _, exists := cookies[name]; !exists {
			cookies[name] = value
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cookies, nil
}

func copyCookieDB(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading browser cookie database: %w", err)
	}
	tmp, err := os.CreateTemp("", "substack-cookies-*.sqlite")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func isSubstackCookieHost(host string) bool {
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), ".")
	return host == "substack.com" || strings.HasSuffix(host, ".substack.com")
}

func hasSessionCookie(cookies map[string]string) bool {
	for name := range cookies {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "sid") || strings.Contains(lower, "session") || strings.Contains(lower, "auth") {
			return true
		}
	}
	return false
}

func decryptChromiumCookie(browser string, encrypted []byte) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("encrypted browser cookies are only supported on macOS")
	}
	if bytes.HasPrefix(encrypted, []byte("v20")) {
		return "", fmt.Errorf("Chromium v20 cookie encryption is not supported")
	}
	if bytes.HasPrefix(encrypted, []byte("v10")) || bytes.HasPrefix(encrypted, []byte("v11")) {
		encrypted = encrypted[3:]
	}
	password, err := chromiumSafeStoragePassword(browser)
	if err != nil {
		return "", err
	}
	key := pbkdf2.Key([]byte(password), []byte("saltysalt"), 1003, 16, sha1.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	if len(encrypted)%aes.BlockSize != 0 {
		return "", fmt.Errorf("encrypted cookie has invalid block size")
	}
	iv := bytes.Repeat([]byte(" "), aes.BlockSize)
	plain := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, encrypted)
	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func chromiumSafeStoragePassword(browser string) (string, error) {
	service := "Chrome Safe Storage"
	switch normalizeBrowserName(browser) {
	case "chrome":
		service = "Chrome Safe Storage"
	case "brave":
		service = "Brave Safe Storage"
	case "arc":
		service = "Arc Safe Storage"
	}
	cmd := exec.Command("security", "find-generic-password", "-w", "-s", service)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not read macOS Keychain item %q; use auth set-token as the manual fallback", service)
	}
	return strings.TrimSpace(string(out)), nil
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("invalid PKCS7 data")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize || pad > len(data) {
		return nil, fmt.Errorf("invalid PKCS7 padding")
	}
	for _, b := range data[len(data)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("invalid PKCS7 padding")
		}
	}
	return data[:len(data)-pad], nil
}

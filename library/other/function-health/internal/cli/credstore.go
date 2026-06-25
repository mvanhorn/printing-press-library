// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

// credstore.go resolves Function Health credentials from one of three places,
// in priority order:
//
//  1. Explicit flags (--email / --password) or environment variables
//     (FH_EMAIL / FH_PASSWORD) passed by the caller.
//  2. The macOS Keychain (service name "function-health"). The account name
//     is the email; the password is the value. Created once with:
//
//       security add-generic-password -s function-health -a you@example.com -w 'your-password'
//
//  3. A .env file (in the cwd, or `~/.config/function-health-pp-cli/.env`).
//     Keys read: FH_EMAIL, FH_PASSWORD. POSIX `KEY=value` syntax; lines
//     starting with `#` are ignored; values may be quoted or unquoted; no
//     variable interpolation.
//
// The flag-or-env path returns immediately so callers can override credstore
// without setting up keychain / .env entries.

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveCredentials returns email + password, drawing from explicit values,
// env vars, the macOS Keychain entry, then a .env file. Returns
// (email, password, source, error) — source describes where the values came
// from for audit / debug logging ("flag", "env", "keychain", "dotenv").
func resolveCredentials(flagEmail, flagPassword string) (email, password, source string, err error) {
	// 1. Explicit / env-var fast path
	email = flagEmail
	password = flagPassword
	if email == "" {
		email = os.Getenv("FH_EMAIL")
	}
	if password == "" {
		password = os.Getenv("FH_PASSWORD")
	}
	if email != "" && password != "" {
		// Distinguish flag vs env; flag wins when set.
		switch {
		case flagEmail != "" && flagPassword != "":
			return email, password, "flag", nil
		case os.Getenv("FH_EMAIL") != "" || os.Getenv("FH_PASSWORD") != "":
			return email, password, "env", nil
		default:
			return email, password, "flag", nil
		}
	}

	// 2. macOS Keychain
	if runtime.GOOS == "darwin" {
		ke, kp, kerr := loadFromKeychain("function-health")
		if kerr == nil && ke != "" && kp != "" {
			if email == "" {
				email = ke
			}
			if password == "" {
				password = kp
			}
			if email != "" && password != "" {
				return email, password, "keychain", nil
			}
		}
	}

	// 3. .env file
	dotEnvPaths := dotEnvCandidates()
	for _, p := range dotEnvPaths {
		dotEmail, dotPassword, dotErr := loadFromDotEnv(p)
		if dotErr != nil {
			continue
		}
		if email == "" {
			email = dotEmail
		}
		if password == "" {
			password = dotPassword
		}
		if email != "" && password != "" {
			return email, password, "dotenv:" + p, nil
		}
	}

	if email == "" && password == "" {
		return "", "", "", errors.New("no credentials found (try --email/--password, set FH_EMAIL/FH_PASSWORD, add a macOS Keychain entry under service 'function-health', or create ~/.config/function-health-pp-cli/.env)")
	}
	if email == "" {
		return "", "", "", errors.New("password found but no email; supply --email or set FH_EMAIL")
	}
	return "", "", "", errors.New("email found but no password; supply --password or set FH_PASSWORD")
}

// loadFromKeychain runs `security find-generic-password -s <service>` to read
// the account, then the same command with `-w` to read the password.
//
// macOS `security` exit codes:
//
//	0  found
//	44 (kSecItemNotFound) not found
func loadFromKeychain(service string) (string, string, error) {
	if _, err := exec.LookPath("security"); err != nil {
		return "", "", fmt.Errorf("`security` CLI not found: %w", err)
	}
	// Get account name from the entry metadata.
	out, err := exec.Command("security", "find-generic-password", "-s", service).CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("keychain lookup (service=%q): %w; output=%s", service, err, strings.TrimSpace(string(out)))
	}
	var account string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Lines look like:  "acct"<blob>="you@example.com"
		if strings.HasPrefix(line, `"acct"<blob>=`) {
			account = strings.TrimSuffix(strings.TrimPrefix(line, `"acct"<blob>=`), `"`)
			account = strings.TrimPrefix(account, `"`)
			break
		}
	}
	if account == "" {
		return "", "", fmt.Errorf("keychain entry for service %q has no account attribute", service)
	}
	pwOut, err := exec.Command("security", "find-generic-password", "-s", service, "-a", account, "-w").Output()
	if err != nil {
		return account, "", fmt.Errorf("keychain password fetch (service=%q account=%q): %w", service, account, err)
	}
	return account, strings.TrimRight(string(pwOut), "\r\n"), nil
}

// dotEnvCandidates returns the .env paths to try in priority order: cwd,
// config dir, home dir.
func dotEnvCandidates() []string {
	var out []string
	if wd, err := os.Getwd(); err == nil {
		out = append(out, filepath.Join(wd, ".env"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".config", "function-health-pp-cli", ".env"))
		out = append(out, filepath.Join(home, ".function-health-pp-cli.env"))
	}
	return out
}

// loadFromDotEnv reads FH_EMAIL and FH_PASSWORD from a .env file. POSIX
// `KEY=value` syntax; `#` comments; quoted (single or double) or unquoted
// values; no variable interpolation; no `export` keyword recognition (treat
// as a literal prefix if present).
func loadFromDotEnv(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	var email, password string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		if len(v) >= 2 {
			if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
				v = v[1 : len(v)-1]
			}
		}
		switch k {
		case "FH_EMAIL":
			email = v
		case "FH_PASSWORD":
			password = v
		}
	}
	return email, password, scanner.Err()
}

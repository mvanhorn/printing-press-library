// Copyright 2026 smoochy and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/synology/internal/cliutil"
)

// Credential file for this CLI, at ~/.claude/printing-press/<cli-name>/.env,
// one directory per printing-press CLI. DSM has no API key: a login is an
// account plus a password, so keeping them in a dotfile is what lets an expired
// session renew itself unattended without the password ever reaching a shell
// profile or a command line.
//
// Precedence, weakest first: config/credentials file, this .env file, real
// environment variables. applyDotenv runs after the config file is read and
// before the SYNOLOGY_* env overrides, and only fills fields that are still
// empty, so both neighbors keep winning where they used to.
//
// Only BaseURL and InsecureTLS are Config fields. The account, password, OTP
// code and device identifiers deliberately are NOT: dsmCredentialsFromEnv
// reads them through DotenvLookup at relogin time, which keeps the password out
// of Config, out of snapshotFileConfig, and out of anything save() writes back
// to disk.
//
// Values are never pushed into the process environment via os.Setenv: this CLI
// spawns subprocesses that inherit the env, and the password has no business
// riding along.

const dotenvPathEnv = "SYNOLOGY_ENV_FILE"

// dotenvCLIDir is this CLI's own directory under ~/.claude/printing-press. One
// directory per CLI keeps every printing-press CLI's credentials collected in
// one place without any two of them sharing a file: the names each CLI reads
// are its own, so a single flat file would collide as soon as two CLIs wanted
// the same key. Spelled out rather than derived from the module path, which a
// regeneration can rewrite.
const dotenvCLIDir = "synology-pp-cli"

func dotenvPath() string {
	if p := strings.TrimSpace(os.Getenv(dotenvPathEnv)); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "printing-press", dotenvCLIDir, ".env")
}

// parseDotenv reads KEY=VALUE lines. Comments, blank lines, an optional
// "export " prefix, and matched surrounding quotes are handled; anything else
// is passed through verbatim. Keys are lower-cased.
func parseDotenv(data []byte) map[string]string {
	out := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

// dotenvValues reads and parses the file. A missing file is not an error: this
// is an opportunistic source, not a required one. The file is small and is read
// at most a couple of times per invocation, so it is re-read rather than cached.
func dotenvValues() (map[string]string, string) {
	path := dotenvPath()
	if path == "" {
		return nil, ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", path, err)
		}
		return nil, path
	}
	return parseDotenv(data), path
}

// DotenvLookup returns the first non-empty value for the given keys, which are
// matched case-insensitively. It is the seam dsmCredentialsFromEnv uses so a
// DSM account and password can live in the .env file without becoming Config
// fields. Lookup is per key, so an account from the real environment and a
// password from the file still combine into one usable credential pair.
func DotenvLookup(keys ...string) string {
	values, path := dotenvValues()
	if len(values) == 0 {
		return ""
	}
	for _, key := range keys {
		if v := values[strings.ToLower(key)]; v != "" {
			warnDotenvPerms(path, values)
			return v
		}
	}
	return ""
}

// warnDotenvPerms reports loose permissions on a file that holds a DSM
// password. It reports rather than tightens: the file belongs to the operator,
// not to this CLI, so silently changing its mode behind their back would be the
// wrong call. A DSM password is not a scoped, revocable API token - it is
// usually the same login that administers the whole NAS - so the wording says
// password rather than credential.
func warnDotenvPerms(path string, values map[string]string) {
	if path == "" || values["synology_password"] == "" {
		return
	}
	if err := cliutil.VerifyCredsPerms(path); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s holds a DSM account password but %v\n", path, err)
	}
}

// applyDotenv fills the Config fields the .env file can supply. Credentials
// are not among them by design; see the package comment above.
func applyDotenv(cfg *Config) {
	values, _ := dotenvValues()
	if len(values) == 0 {
		return
	}
	// Only fill what is still empty; the config file already had its turn.
	if v := values["synology_base_url"]; v != "" && cfg.BaseURL == defaultBaseURL {
		cfg.BaseURL = v
	}
	switch strings.ToLower(values["synology_insecure_tls"]) {
	case "1", "true", "yes", "on":
		cfg.InsecureTLS = true
	}
}

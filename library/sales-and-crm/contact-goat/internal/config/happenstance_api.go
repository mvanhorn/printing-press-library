// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
//
// Sidecar for the Happenstance public REST API key. Lives outside the
// generator-owned config.go so a future regeneration doesn't clobber it,
// and so we don't extend the pre-existing technical debt of hand-editing
// the DO NOT EDIT file (the cookie-auth field is the sole exception we
// inherit, and we are not adding to it).
//
// The single TOML field is `happenstance_api_key`. The single env var
// is HAPPENSTANCE_API_KEY. Env wins over config when both are set.

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// HappenstanceAPI is decoded from the same config.toml file as Config but
// via a second Unmarshal pass. This keeps the new field out of the
// generator-owned Config struct without losing TOML round-trip.
type HappenstanceAPI struct {
	APIKey string `toml:"happenstance_api_key"`
}

// HappenstanceAPIKeyEnvVar is the canonical environment variable name.
// Matches the convention used by Happenstance's own client libraries.
const HappenstanceAPIKeyEnvVar = "HAPPENSTANCE_API_KEY"

// happenstanceAPIKeyValidPrefixes lists prefixes we recognize as valid
// keys. We warn but do not reject unknown prefixes so future surfaces
// (e.g. hpn_live_workspace_) don't break the loose check.
var happenstanceAPIKeyValidPrefixes = []string{
	"hpn_live_personal_",
	"hpn_live_",
}

// loadHappenstanceAPIFromFile decodes the config file at cfg.Path into a
// HappenstanceAPI. Missing file or empty path returns a zero-value struct
// and no error: the caller treats absence as "not configured".
func loadHappenstanceAPIFromFile(path string) (HappenstanceAPI, error) {
	var hp HappenstanceAPI
	if path == "" {
		return hp, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return hp, nil
		}
		return hp, fmt.Errorf("reading config %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, &hp); err != nil {
		return hp, fmt.Errorf("parsing happenstance_api_key from %s: %w", path, err)
	}
	return hp, nil
}

// LoadAPIKey returns the user's Happenstance public-API key, preferring the
// HAPPENSTANCE_API_KEY environment variable over the config file's
// happenstance_api_key field. Returns an empty string when neither is set;
// callers must check for empty before constructing a bearer client.
//
// The key is returned unmodified. Redaction is the rendering layer's job,
// not config's.
//
// If the key has an unrecognized prefix, a warning is written to stderr
// (the key is still returned). The check is loose so future Happenstance
// key surfaces don't trigger spurious failures.
func LoadAPIKey(cfg *Config) string {
	if v := os.Getenv(HappenstanceAPIKeyEnvVar); v != "" {
		warnIfUnknownPrefix(v, "env:"+HappenstanceAPIKeyEnvVar)
		return v
	}
	if cfg == nil {
		return ""
	}
	hp, err := loadHappenstanceAPIFromFile(cfg.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return ""
	}
	if hp.APIKey == "" {
		return ""
	}
	warnIfUnknownPrefix(hp.APIKey, "config:"+cfg.Path)
	return hp.APIKey
}

func warnIfUnknownPrefix(key, source string) {
	for _, p := range happenstanceAPIKeyValidPrefixes {
		if strings.HasPrefix(key, p) {
			return
		}
	}
	fmt.Fprintf(os.Stderr,
		"warning: HAPPENSTANCE_API_KEY (from %s) does not start with any recognized prefix (%s); the call may still succeed but verify the key at https://happenstance.ai\n",
		source, strings.Join(happenstanceAPIKeyValidPrefixes, ", "),
	)
}

// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Public API for paid-provider key persistence.
//
// Adapters call Resolve(envName) to get env-or-config; the auth set-key
// command calls SetKey(provider, value) to persist + Save.

package config

import (
	"fmt"
	"os"
	"sync"
)

// KnownProviders lists the paid-provider keys the CLI manages.
var KnownProviders = []string{"spoken", "taddy", "taddy_user_id", "openai", "deepgram", "elevenlabs"}

// providerEnvVar maps a provider slug to its canonical env var name.
var providerEnvVar = map[string]string{
	"spoken":        "SPOKEN_API_KEY",
	"taddy":         "TADDY_API_KEY",
	"taddy_user_id": "TADDY_USER_ID",
	"openai":        "OPENAI_API_KEY",
	"deepgram":      "DEEPGRAM_API_KEY",
	"elevenlabs":    "ELEVENLABS_API_KEY",
}

// Resolve returns the credential for an env-var name, checking env first then
// the persisted config file. Used by paid adapters so a user who ran
// `auth set-key --provider spoken --value pt_xxx` doesn't have to also
// export SPOKEN_API_KEY every shell session.
//
// Resolution order:
//  1. env var with the requested name
//  2. matching field in the loaded config file
//  3. empty string (caller surfaces KeyMissingError to the user)
//
// Loads config lazily on first call and caches for the process lifetime.
// Cache is acceptable here because adapters are process-singletons and
// config changes go into effect on the next CLI invocation (a new process).
func Resolve(envName string) string {
	if v := os.Getenv(envName); v != "" {
		return v
	}
	c := cachedConfig()
	if c == nil {
		return ""
	}
	switch envName {
	case "SPOKEN_API_KEY":
		return c.SpokenApiKey
	case "TADDY_API_KEY":
		return c.TaddyApiKey
	case "TADDY_USER_ID":
		return c.TaddyUserId
	case "OPENAI_API_KEY":
		return c.OpenaiApiKey
	case "DEEPGRAM_API_KEY":
		return c.DeepgramApiKey
	case "ELEVENLABS_API_KEY":
		return c.ElevenlabsApiKey
	}
	return ""
}

// Source returns whether the credential came from env, config, or is missing.
// Used by doctor to render a per-provider key_source row.
func Source(envName string) string {
	if os.Getenv(envName) != "" {
		return "env"
	}
	c := cachedConfig()
	if c == nil {
		return "missing"
	}
	var val string
	switch envName {
	case "SPOKEN_API_KEY":
		val = c.SpokenApiKey
	case "TADDY_API_KEY":
		val = c.TaddyApiKey
	case "TADDY_USER_ID":
		val = c.TaddyUserId
	case "OPENAI_API_KEY":
		val = c.OpenaiApiKey
	case "DEEPGRAM_API_KEY":
		val = c.DeepgramApiKey
	case "ELEVENLABS_API_KEY":
		val = c.ElevenlabsApiKey
	}
	if val != "" {
		return "config"
	}
	return "missing"
}

// EnvVarFor returns the canonical env var name for a provider slug.
// Returns the empty string for unknown providers.
func EnvVarFor(provider string) string { return providerEnvVar[provider] }

// SetKey persists a credential for a provider into the config file. Empty
// value clears the field (the unset path). Returns the path of the written
// config file so callers can show it to the user.
func SetKey(provider, value string) (path string, err error) {
	c, err := Load("")
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	switch provider {
	case "spoken":
		c.SpokenApiKey = value
	case "taddy":
		c.TaddyApiKey = value
	case "taddy_user_id":
		c.TaddyUserId = value
	case "openai":
		c.OpenaiApiKey = value
	case "deepgram":
		c.DeepgramApiKey = value
	case "elevenlabs":
		c.ElevenlabsApiKey = value
	default:
		return "", fmt.Errorf("unknown provider %q (valid: %v)", provider, KnownProviders)
	}
	if err := c.save(); err != nil {
		return "", err
	}
	invalidateCache()
	return c.Path, nil
}

// cachedConfig loads the config once per process. Returns nil if Load
// errors (the file is malformed); callers treat nil as "no config".
//
// The cache is read by every adapter's New() (via Resolve) and by every
// doctor render (via Source). `episode batch` dispatches up to 5 goroutines
// each calling the dispatch chain → adapter → Resolve → cachedConfig
// concurrently, so the check-then-set pattern must be mutex-protected to
// avoid a data race flagged by Greptile (P1) and the race detector.
var (
	cacheMu            sync.Mutex
	cachedConfigVal    *Config
	cachedConfigLoaded bool
)

func cachedConfig() *Config {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedConfigLoaded {
		return cachedConfigVal
	}
	c, err := Load("")
	if err != nil {
		cachedConfigLoaded = true
		return nil
	}
	cachedConfigVal = c
	cachedConfigLoaded = true
	return c
}

// invalidateCache resets the cached config. Used by SetKey so a same-process
// follow-up Resolve() picks up the just-written value (useful for tests
// and for unusual scenarios like in-process re-reading after a write).
func invalidateCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cachedConfigVal = nil
	cachedConfigLoaded = false
}

package config

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
)

// ComposedAPIKey composes Etsy's request credential without changing persisted config.
func (c *Config) ComposedAPIKey() string {
	key := strings.TrimSpace(c.EtsyApiKey)
	if key == "" || strings.Contains(key, ":") {
		return key
	}
	secret := strings.TrimSpace(cliutil.EnvOverride("ETSY_SHARED_SECRET"))
	if secret == "" {
		return key
	}
	return key + ":" + secret
}

// APIKeyHasSharedSecret reports whether the request credential includes a secret.
func (c *Config) APIKeyHasSharedSecret() bool {
	return strings.Contains(c.ComposedAPIKey(), ":")
}

// SharedSecretConfigured reports whether the shared secret environment is configured.
func (c *Config) SharedSecretConfigured() bool {
	return strings.TrimSpace(cliutil.EnvOverride("ETSY_SHARED_SECRET")) != ""
}

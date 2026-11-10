// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/cliutil"
	"github.com/pelletier/go-toml/v2"
)

// Named accounts share a directory, so keep each key in its private account
// file instead of overwriting the framework's directory-wide credentials file.
func (c *Config) SaveKlaviyoAccountCredential(token string) error {
	persisted := c.configForSave()
	persisted.clearCredentialFields()
	persisted.KlaviyoApiKey = token
	data, err := toml.Marshal(persisted)
	if err != nil {
		return fmt.Errorf("marshaling account config: %w", err)
	}
	return cliutil.AtomicWritePrivateFile(c.Path, data, 0o600, 0o700)
}

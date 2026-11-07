// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output. Lives beside the emitted config so
// regen-merge keeps it; the emitted Config struct stays untouched.

package config

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/pelletier/go-toml/v2"
)

// UniFiTarget is what the transport needs to reach a console: which console,
// which local gateway, and the key that gateway accepts.
type UniFiTarget struct {
	ConsoleID   string
	Gateway     string
	LocalAPIKey string
	// Transport is "", "auto", "cloud" or "local". It only matters when both a
	// console and a gateway are configured.
	Transport string
}

type unifiTargetFile struct {
	ConsoleID   string `toml:"console_id"`
	Gateway     string `toml:"gateway"`
	LocalAPIKey string `toml:"local_api_key"`
	Transport   string `toml:"transport"`
}

// UniFiTarget resolves env over config file. Flags are layered on top by the
// caller because config must stay importable without cobra.
func (c *Config) UniFiTarget() UniFiTarget {
	var target UniFiTarget
	if c != nil && strings.TrimSpace(c.Path) != "" {
		if opened, err := cliutil.ReadOpenedConfig(c.Path); err == nil {
			var parsed unifiTargetFile
			if toml.Unmarshal(opened.Data, &parsed) == nil {
				target.ConsoleID = strings.TrimSpace(parsed.ConsoleID)
				target.Transport = strings.TrimSpace(parsed.Transport)
				// A file other users can edit must not choose where a key is
				// sent. A console id is safe because the connector host is
				// fixed, but a gateway or local key from a loose file would let
				// another account point the local key at a host of its choosing.
				if opened.PermErr == nil {
					target.Gateway = strings.TrimSpace(parsed.Gateway)
					target.LocalAPIKey = strings.TrimSpace(parsed.LocalAPIKey)
				}
			}
		}
	}
	if v := strings.TrimSpace(cliutil.EnvOverride("UNIFI_CONSOLE_ID")); v != "" {
		target.ConsoleID = v
	}
	if v := strings.TrimSpace(cliutil.EnvOverride("UNIFI_GATEWAY")); v != "" {
		target.Gateway = v
	}
	if v := strings.TrimSpace(cliutil.EnvOverride("UNIFI_LOCAL_API_KEY")); v != "" {
		target.LocalAPIKey = v
	}
	if v := strings.TrimSpace(cliutil.EnvOverride("UNIFI_TRANSPORT")); v != "" {
		target.Transport = v
	}
	return target
}

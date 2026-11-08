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

// persistedUniFiTarget omits empty values so a save does not invent route settings.
type persistedUniFiTarget struct {
	ConsoleID   string `toml:"console_id,omitempty"`
	Gateway     string `toml:"gateway,omitempty"`
	LocalAPIKey string `toml:"local_api_key,omitempty"`
	Transport   string `toml:"transport,omitempty"`
}

// keepUniFiTarget carries the route settings across a config save. The emitted Config
// does not know them, so saving a credential would otherwise erase them and the next
// command could reach a different console. They go first because TOML would read
// top-level keys placed after a table as part of that table. Logging out drops the
// stored gateway key, since it is a credential and the route is not.
func keepUniFiTarget(path string, data []byte, loggingOut bool) []byte {
	opened, err := cliutil.ReadOpenedConfig(path)
	if err != nil {
		return data
	}
	var parsed persistedUniFiTarget
	if toml.Unmarshal(opened.Data, &parsed) != nil {
		return data
	}
	// UniFiTarget refuses these from a file others can edit. The save rewrites the file
	// at 0600, so carrying them over would make the next load trust what this one refused.
	if opened.PermErr != nil {
		parsed.Gateway, parsed.LocalAPIKey = "", ""
	}
	if loggingOut {
		parsed.LocalAPIKey = ""
	}
	if parsed == (persistedUniFiTarget{}) {
		return data
	}
	head, err := toml.Marshal(parsed)
	if err != nil {
		return data
	}
	return append(head, data...)
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
	// Earlier releases documented UNIFI_GATEWAY_HOST with the gateway's key in
	// UNIFI_API_KEY. Keep those installs local, or that key would go to the cloud API.
	// The key only moves when the legacy host is what chose the gateway.
	if v := strings.TrimSpace(cliutil.EnvOverride("UNIFI_GATEWAY_HOST")); v != "" && target.Gateway == "" {
		target.Gateway = v
		if target.LocalAPIKey == "" {
			target.LocalAPIKey = strings.TrimSpace(cliutil.EnvOverride("UNIFI_API_KEY"))
		}
	}
	return target
}

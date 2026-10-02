// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/sprocket/internal/config"
)

// scopedDefaultDBPath isolates implicit local state by configuration, club,
// and credential. Sprocket has no offline-safe account-identity endpoint, so a
// one-way credential digest is included to prevent two accounts that reuse the
// same config path and club from sharing records or sync cursors. Token rotation
// deliberately starts a fresh scope; users who need a stable advanced location
// can provide --db explicitly.
func scopedDefaultDBPath(name string, flags *rootFlags) (string, error) {
	return AccountScopedDBPath(name, configPathFromFlags(flags))
}

// AccountScopedDBPath is shared with the MCP server so its search and SQL
// tools read the same account's local data as the CLI. The config path may be
// empty to use the usual SPROCKET_CONFIG or default path resolution.
func AccountScopedDBPath(name, configPath string) (string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("resolving account-scoped database: %w", err)
	}
	scope, err := cacheScopeKey(cfg)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(defaultDBPath(name)), "accounts", scope, "data.db"), nil
}

func cacheScopeKeyForFlags(flags *rootFlags) (string, error) {
	cfg, err := config.Load(configPathFromFlags(flags))
	if err != nil {
		return "", fmt.Errorf("resolving account cache scope: %w", err)
	}
	return cacheScopeKey(cfg)
}

func configPathFromFlags(flags *rootFlags) string {
	if flags == nil {
		return ""
	}
	return flags.configPath
}

func cacheScopeKey(cfg *config.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("resolving account cache scope: nil config")
	}
	configPath, err := filepath.Abs(cfg.Path)
	if err != nil {
		return "", fmt.Errorf("resolving config path: %w", err)
	}
	identity := filepath.Clean(configPath) + "\x00" + normalizedScopeURL(cfg.BaseURL) + "\x00" + cfg.AuthHeader()
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])[:20], nil
}

func normalizedScopeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return strings.ToLower(strings.TrimRight(raw, "/"))
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String()
}

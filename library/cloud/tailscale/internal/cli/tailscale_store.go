// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// The local store is kept per credential. The default selector "-" names a
// different tailnet for each credential and synced rows carry no credential,
// so two credentials sharing one database would mix tailnets in search and
// overwrite rows that share an ID. The generator already keys the default
// database by a hash of the configured credential (data-<hash>.db); this file
// makes that key follow the credential a run actually sends, including OAuth
// clients whose token is only minted later, and gives the MCP server's direct
// store tools the same path.

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/config"
)

// tsStoreScopeCredential is the string the default database is keyed by. The
// generated key hashes the raw Authorization header, which is empty for an
// OAuth client at startup, so every OAuth client shared one database.
// tsCredentialFingerprint follows Config.AuthHeader precedence and
// fingerprints an OAuth client by its ID and secret. With no credential, the
// generated key stands.
func tsStoreScopeCredential(cfg *config.Config) string {
	if fp := tsCredentialFingerprint(cfg); fp != "" {
		return "tailscale-store\x00" + fp
	}
	return cfg.StoreScopeCredential()
}

// ScopedDBPath returns the default database for the default config's
// credential: the file the CLI reads and writes for that credential. The MCP
// server uses it for its direct search and SQL tools. It computes the path
// without touching the process-wide scope, so concurrent calls cannot see
// each other's credential.
func ScopedDBPath() string {
	credential := ""
	if cfg, err := config.Load(""); err == nil {
		credential = strings.TrimSpace(tsStoreScopeCredential(cfg))
	}
	dir, err := cliutil.DataDir()
	if err != nil {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "data.db"
		}
		dir = filepath.Join(home, ".local", "share", "tailscale-pp-cli")
	}
	if credential == "" {
		return filepath.Join(dir, "data.db")
	}
	sum := sha256.Sum256([]byte(credential))
	return filepath.Join(dir, "data-"+hex.EncodeToString(sum[:])[:defaultDBScopeHashLen]+".db")
}

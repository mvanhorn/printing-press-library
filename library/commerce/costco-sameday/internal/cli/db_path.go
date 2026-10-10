// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored credential-scoped DB path export for MCP + CLI parity.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/commerce/costco-sameday/internal/config"
)

// ResolveDataDBPath returns the credential-scoped SQLite path the CLI uses
// (data-<hash>.db when scoped credentials exist, else data.db).
func ResolveDataDBPath() string {
	configureDefaultDBScope("")
	if cfg, err := config.Load(""); err == nil {
		setDefaultDBScopeCredential(cfg.StoreScopeCredential())
	}
	return defaultDBPath("costco-sameday-pp-cli")
}

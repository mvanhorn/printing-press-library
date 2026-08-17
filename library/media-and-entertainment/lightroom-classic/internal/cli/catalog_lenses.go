// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: mirrors the top-level 'lenses' command under the hidden
// 'catalog' typed surface. Local-sqlite CLI — no HTTP client involved.
package cli

import (
	"github.com/spf13/cobra"
)

func newCatalogLensesCmd(flags *rootFlags) *cobra.Command {
	cmd := makeLensesCmd(flags)
	cmd.Example = "  lightroom-classic-pp-cli catalog lenses --json"
	return cmd
}

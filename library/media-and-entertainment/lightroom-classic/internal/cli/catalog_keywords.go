// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: mirrors the top-level 'keywords' command under the hidden
// 'catalog' typed surface. Local-sqlite CLI — no HTTP client involved.
package cli

import (
	"github.com/spf13/cobra"
)

func newCatalogKeywordsCmd(flags *rootFlags) *cobra.Command {
	cmd := makeKeywordsCmd(flags)
	cmd.Example = "  lightroom-classic-pp-cli catalog keywords --json"
	return cmd
}

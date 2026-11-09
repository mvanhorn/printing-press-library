// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelImagesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "images",
		Short:       "Upload images, one at a time or as an even grid",
		Example:     "  milanote-pp-cli images grid ./frames --board 1Ab2Cd3EfGh4iJ --cols 3 --number",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelImagesGridCmd(flags))
	addNovelCommandIfAbsent(cmd, newImagesAddCmd(flags))
	return cmd
}

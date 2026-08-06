// Copyright 2026 Eric Rash and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command parent: local cache health and maintenance.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelCacheCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "cache",
		Short:       "cache subcommands: status, reindex, index-fulltext",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelCacheStatusCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelCacheReindexCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelCacheIndexFulltextCmd(flags))
	return cmd
}

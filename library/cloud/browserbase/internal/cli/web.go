// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelWebCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "web",
		Short:   "web subcommands: history",
		Example: "  browserbase-pp-cli web history --since 7d --type fetch --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelWebHistoryCmd(flags))
	return cmd
}

// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelUsageCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "usage",
		Short:   "usage subcommands: trend",
		Example: "  browserbase-pp-cli usage trend --project 1fbe3566-db19-4010-9410-0ba94f0497ea --since 30d --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelUsageTrendCmd(flags))
	return cmd
}

// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command group. Stores swell/wind/tide threshold rules in
// the local SQLite store and evaluates them against a fresh forecast — the
// cron-able surf alert Surfline only offers as Premium-and-iOS.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelAlertCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "alert",
		Short:       "Define and evaluate cron-able surf-condition alert rules (add, list, run).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelAlertAddCmd(flags))
	cmd.AddCommand(newNovelAlertListCmd(flags))
	cmd.AddCommand(newNovelAlertRunCmd(flags))
	return cmd
}

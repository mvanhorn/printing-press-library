// Copyright 2026 Amandeep Khurana and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"time"

	"github.com/spf13/cobra"
)

func newNovelNextCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "next",
		Short: "See your soonest upcoming reservation and when it starts.",
		Long: "Use this command for the single soonest reservation.\n" +
			"Do NOT use this command for refund status across upcoming reservations; use 'refundable' instead.",
		Example:     "  spothero-pp-cli next --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "next")
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			list, err := c.ListAllReservations(5)
			if err != nil {
				return mapConsumerErr(err)
			}
			now := time.Now()
			up := upcoming(list, now)
			if len(up) == 0 {
				return flags.printJSON(cmd, map[string]any{"reservation": nil, "upcoming_count": 0})
			}
			report := map[string]any{"reservation": up[0], "upcoming_count": len(up)}
			if start, err := time.Parse(time.RFC3339, up[0].Starts); err == nil {
				report["starts_in_minutes"] = int(start.Sub(now).Minutes())
			}
			return flags.printJSON(cmd, report)
		},
	}
}

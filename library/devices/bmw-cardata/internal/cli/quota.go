// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source computed

func newNovelQuotaCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	cmd := &cobra.Command{
		Use:         "quota",
		Short:       "Show how many of your ~50 daily API calls you have spent and when the budget resets.",
		Long:        "Report today's BMW CarData API-call count against the ~50 calls/day cap. The limit resets at midnight UTC. Computed from the local call log written by live fetches.",
		Example:     "  bmw-cardata-pp-cli quota",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report today's API quota usage")
				return nil
			}
			const cap = 50
			day, used, err := func() (string, int, error) {
				db, err := openCardataStore(resolveDBPath(flagDB))
				if err != nil {
					return "", 0, err
				}
				if db == nil {
					return nowUTCDay(), 0, nil
				}
				defer db.Close()
				return cardataQuota(db)
			}()
			if err != nil {
				return configErr(fmt.Errorf("reading quota: %w", err))
			}
			remaining := cap - used
			if remaining < 0 {
				remaining = 0
			}
			resets := nextUTCMidnight()
			view := map[string]any{
				"day":           day,
				"used":          used,
				"cap":           cap,
				"remaining":     remaining,
				"resets_at_utc": resets.Format("2006-01-02T15:04:05Z"),
			}
			if used >= cap {
				view["note"] = "daily cap reached; commands will fail until " + resets.Format("15:04:05Z") + " UTC"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "API quota for %s (UTC)\n", day)
			fmt.Fprintf(cmd.OutOrStdout(), "  used:      %d / %d\n", used, cap)
			fmt.Fprintf(cmd.OutOrStdout(), "  remaining: %d\n", remaining)
			fmt.Fprintf(cmd.OutOrStdout(), "  resets:    %s UTC\n", resets.Format("15:04:05"))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

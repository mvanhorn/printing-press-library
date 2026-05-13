// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local engagement decay analysis command.
func newEngagementDecayCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagWindow int
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "engagement-decay",
		Short: "Rank contacts whose engagement frequency dropped.",
		Long:  `Compares engagement counts in the recent window against the preceding window, skipping contacts already outside the lookback boundary.`,
		Example: `  hubspot-pp-cli engagement-decay --agent
  hubspot-pp-cli engagement-decay --days 45 --window 14 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := computeEngagementDecay(cmd.Context(), db, flagDays, flagWindow, flagLimit)
			if err != nil {
				return err
			}
			result := engagementDecayResult{Rows: rows, Total: len(rows), AsOf: time.Now().Format(time.RFC3339)}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), result, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "contactId\temail\tprev_count\tcurr_count\tdelta")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n", r.ContactID, r.Email, r.PriorCount, r.RecentCount, r.Delta)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 30, "Lookback boundary in days")
	cmd.Flags().IntVar(&flagWindow, "window", 7, "Comparison window size in days")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max rows to return")
	return cmd
}

// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local recent intake analysis command.
func newRecentIntakeCmd(flags *rootFlags) *cobra.Command {
	var flagHours int
	var flagSource string
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "recent-intake",
		Short: "Show recently created contacts with source attribution.",
		Long:  `Reads local contacts created in the last N hours and emits original source, latest source, and UTM attribution fields.`,
		Example: `  hubspot-pp-cli recent-intake --agent
  hubspot-pp-cli recent-intake --hours 72 --source PAID_SEARCH --agent`,
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
			rows, err := computeRecentIntake(cmd.Context(), db, flagHours, flagSource, flagLimit)
			if err != nil {
				return err
			}
			result := recentIntakeResult{Rows: rows, Total: len(rows), AsOf: time.Now().Format(time.RFC3339)}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), result, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "contactId\temail\tcreatedAt\tsource\tutm_campaign")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.ContactID, r.Email, r.CreatedAt, r.Source, r.UtmCampaign)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&flagHours, "hours", 24, "Lookback window in hours")
	cmd.Flags().StringVar(&flagSource, "source", "", "Filter by hs_analytics_source")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Max rows to return")
	return cmd
}

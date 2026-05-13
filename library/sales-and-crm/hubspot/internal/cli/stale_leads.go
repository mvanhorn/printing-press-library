// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local stale lead analysis command.
func newStaleLeadsCmd(flags *rootFlags) *cobra.Command {
	var flagStage string
	var flagDays int
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "stale-leads",
		Short: "Surface contacts in a lifecycle stage untouched for N days.",
		Long:  `Joins the local contacts table with engagement tables and ranks contacts in the given lifecycle stage by days-since-last-engagement.`,
		Example: `  hubspot-pp-cli stale-leads --agent
  hubspot-pp-cli stale-leads --stage opportunity --days 21 --agent --select rows.contactId,rows.days_since_touch`,
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
			rows, err := computeStaleLeads(cmd.Context(), db, flagStage, flagDays, flagLimit)
			if err != nil {
				return err
			}
			result := staleLeadsResult{Rows: rows, Total: len(rows), AsOf: time.Now().Format(time.RFC3339)}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), result, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "contactId\temail\tstage\tdays_since_touch\tlast_engagement_type\tlast_engagement_at")
			for _, r := range rows {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n", r.ContactID, r.Email, r.LifecycleStage, r.DaysSinceTouch, r.LastEngagementType, r.LastEngagementAt)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagStage, "stage", "leadqualified", "HubSpot lifecycle stage internal name")
	cmd.Flags().IntVar(&flagDays, "days", 14, "Number of days since last engagement to count as stale")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max rows to return")
	return cmd
}

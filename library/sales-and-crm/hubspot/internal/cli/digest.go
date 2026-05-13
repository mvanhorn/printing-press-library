// Copyright 2026 simplepathmedia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// PATCH: Register a read-only local digest command composed from transcendence helpers.
func newDigestCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Emit a compact local CRM digest.",
		Long:  `Composes new contacts, advanced deals, closed-won deals, stalest leads, and recent intake from the local SQLite mirror without shelling out.`,
		Example: `  hubspot-pp-cli digest --agent
  hubspot-pp-cli digest --since 72h --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dur, err := parseLookbackDuration(flagSince)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since: %w", err))
			}
			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			asOf := time.Now()
			newContacts, err := computeRecentIntake(cmd.Context(), db, int(dur.Hours()), "", 5)
			if err != nil {
				return err
			}
			allDeals, err := computePipelineHealth(cmd.Context(), db, "", 0)
			if err != nil {
				return err
			}
			cutoff := asOf.Add(-dur)
			advanced := filterDealsSince(cmd.Context(), db, allDeals.Rows, cutoff, false, 5)
			closed := filterDealsSince(cmd.Context(), db, allDeals.Rows, cutoff, true, 5)
			stale, err := computeStaleLeads(cmd.Context(), db, "leadqualified", 14, 5)
			if err != nil {
				return err
			}
			recentIntake, err := computeRecentIntake(cmd.Context(), db, int(dur.Hours()), "", 5)
			if err != nil {
				return err
			}
			result := digestResult{
				Since:          flagSince,
				AsOf:           asOf.Format(time.RFC3339),
				NewContacts:    newContacts,
				DealsAdvanced:  advanced,
				DealsClosedWon: closed,
				StalestLeads:   stale,
				RecentIntake:   recentIntake,
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return marshalAndPrint(cmd.OutOrStdout(), result, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "section\tcount")
			fmt.Fprintf(w, "new_contacts\t%d\n", len(result.NewContacts))
			fmt.Fprintf(w, "deals_advanced\t%d\n", len(result.DealsAdvanced))
			fmt.Fprintf(w, "deals_closed_won\t%d\n", len(result.DealsClosedWon))
			fmt.Fprintf(w, "stalest_leads\t%d\n", len(result.StalestLeads))
			fmt.Fprintf(w, "recent_intake\t%d\n", len(result.RecentIntake))
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Lookback duration such as 24h, 7d, or 30m")
	return cmd
}

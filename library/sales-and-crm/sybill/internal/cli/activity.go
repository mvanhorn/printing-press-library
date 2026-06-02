// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: per-rep activity aggregation (local group-by).

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/internal/store"
	"github.com/spf13/cobra"
)

type ownerActivity struct {
	Owner           string  `json:"owner"`
	OpenDeals       int     `json:"openDeals"`
	ClosedDeals     int     `json:"closedDeals"`
	TotalOpenAmount float64 `json:"totalOpenAmount,omitempty"`
	CallsInWindow   int     `json:"callsInWindow"`
	DealsGoneDark   int     `json:"dealsGoneDark"`
}

func newNovelActivityCmd(flags *rootFlags) *cobra.Command {
	var by string
	var since string
	var darkDays int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "activity",
		Short: "Per-rep breakdown: calls in window, open deals, and deals gone dark.",
		Long: `Aggregate the local store by deal owner: how many open and closed deals each
rep owns, the value of their open pipeline, how many linked calls happened in
the window, and how many of their open deals have gone dark (no call in
--dark-days). Calls are attributed to a rep through the deals they own. Run
'sync' first.`,
		Example: strings.Trim(`
  # Activity per rep over the last 7 days
  sybill-pp-cli activity --by owner --since 7d

  # As JSON
  sybill-pp-cli activity --by owner --since 30d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if by != "" && by != "owner" {
				return fmt.Errorf("--by currently supports only \"owner\" (got %q)", by)
			}
			out := cmd.OutOrStdout()
			now := time.Now().UTC()
			cutoff, err := parseSince(since, now)
			if err != nil {
				return err
			}
			darkCutoff := now.Add(-time.Duration(darkDays) * 24 * time.Hour)

			if dbPath == "" {
				dbPath = defaultDBPath("sybill-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'sybill-pp-cli sync' first.", err)
			}
			defer db.Close()

			deals, err := loadRecords(db, "deals")
			if err != nil {
				return err
			}
			convs, err := loadRecords(db, "conversations")
			if err != nil {
				return err
			}
			if len(deals) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No deals in the local store. Run 'sybill-pp-cli sync' first.")
			}

			agg := map[string]*ownerActivity{}
			get := func(owner string) *ownerActivity {
				if owner == "" {
					owner = "(unassigned)"
				}
				a := agg[owner]
				if a == nil {
					a = &ownerActivity{Owner: owner}
					agg[owner] = a
				}
				return a
			}

			for _, d := range deals {
				a := get(dealOwner(d))
				if dealClosed(d) {
					a.ClosedDeals++
					continue
				}
				a.OpenDeals++
				if amt, ok := floatField(d, "amount"); ok {
					a.TotalOpenAmount += amt
				}
				last, _, hasCall := lastCallForDeal(d, convs)
				if !hasCall || last.Before(darkCutoff) {
					a.DealsGoneDark++
				}
				// Calls in the window for this deal -> attributed to its owner.
				for _, c := range convs {
					if !convMatchesDeal(c, d) {
						continue
					}
					if t, ok := convStart(c); ok && !t.Before(cutoff) {
						a.CallsInWindow++
					}
				}
			}

			results := make([]ownerActivity, 0, len(agg))
			for _, a := range agg {
				results = append(results, *a)
			}
			sort.SliceStable(results, func(i, j int) bool {
				if results[i].OpenDeals != results[j].OpenDeals {
					return results[i].OpenDeals > results[j].OpenDeals
				}
				return results[i].CallsInWindow > results[j].CallsInWindow
			})

			if novelMachineOutput(out, flags) {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(out, "No deal owners found in the local store.")
				return nil
			}
			fmt.Fprintf(out, "%-26s  %-5s  %-6s  %-6s  %s\n", "OWNER", "OPEN", "CALLS", "DARK", "OPEN VALUE")
			for _, r := range results {
				fmt.Fprintf(out, "%-26s  %-5d  %-6d  %-6d  %.0f\n",
					truncate(r.Owner, 26), r.OpenDeals, r.CallsInWindow, r.DealsGoneDark, r.TotalOpenAmount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "owner", "Group by dimension (owner)")
	cmd.Flags().StringVar(&since, "since", "7d", "Window for the call count: 7d, 48h, 30m, or an RFC3339 timestamp")
	cmd.Flags().IntVar(&darkDays, "dark-days", 14, "Days without a call before an open deal counts as gone dark")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: deals gone dark (local cross-entity join).

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/internal/store"
	"github.com/spf13/cobra"
)

type darkDeal struct {
	DealID        string  `json:"dealId"`
	Name          string  `json:"name"`
	AccountName   string  `json:"accountName,omitempty"`
	Stage         string  `json:"stage,omitempty"`
	Owner         string  `json:"owner,omitempty"`
	Amount        float64 `json:"amount,omitempty"`
	LastCall      string  `json:"lastCall,omitempty"`
	LastCallTitle string  `json:"lastCallTitle,omitempty"`
	DaysSince     int     `json:"daysSinceLastCall"`
	Reason        string  `json:"reason"`
}

func newNovelDealsDarkCmd(flags *rootFlags) *cobra.Command {
	var days int
	var includeUncovered bool
	var owner string
	var stage string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dark",
		Short: "List open deals with no call activity in the last N days, so nothing stalls silently.",
		Long: `List open (not closed) deals whose most recent linked call is older than
--days, using the local store to join deals to conversations — a query the
entity-by-entity API cannot answer directly.

A deal is "dark" when its newest linked conversation started more than --days
ago. Deals with no linked call at all are "uncovered"; include them with
--include-uncovered. Run 'sync' first so deals and conversations are local.`,
		Example: strings.Trim(`
  # Open deals with no call in the last 14 days
  sybill-pp-cli deals dark --days 14

  # Include open deals that have never had a call
  sybill-pp-cli deals dark --days 21 --include-uncovered

  # One rep's stalled pipeline, as JSON
  sybill-pp-cli deals dark --owner jane@acme.com --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if days <= 0 {
				return fmt.Errorf("--days must be a positive integer (got %d)", days)
			}
			out := cmd.OutOrStdout()
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

			now := time.Now().UTC()
			cutoff := now.Add(-time.Duration(days) * 24 * time.Hour)

			var results []darkDeal
			for _, d := range deals {
				if dealClosed(d) {
					continue
				}
				if owner != "" && !strings.Contains(strings.ToLower(dealOwner(d)), strings.ToLower(owner)) {
					continue
				}
				if stage != "" && !strings.EqualFold(dealStage(d), stage) {
					continue
				}

				last, title, hasCall := lastCallForDeal(d, convs)
				rec := darkDeal{
					DealID:      dealID(d),
					Name:        dealName(d),
					AccountName: dealAccount(d),
					Stage:       dealStage(d),
					Owner:       dealOwner(d),
				}
				if amt, ok := floatField(d, "amount"); ok {
					rec.Amount = amt
				}

				switch {
				case hasCall:
					if !last.Before(cutoff) {
						continue // active: last call within the window
					}
					rec.LastCall = last.Format(time.RFC3339)
					rec.LastCallTitle = title
					rec.DaysSince = daysAgo(last, now)
					rec.Reason = fmt.Sprintf("no call in %d days", rec.DaysSince)
				default:
					if !includeUncovered {
						continue // uncovered, but caller didn't ask for these
					}
					if la, ok := parseTime(firstStr(d, "lastActivityDate", "last_activity_date")); ok {
						rec.DaysSince = daysAgo(la, now)
						rec.Reason = fmt.Sprintf("no calls on record; last activity %d days ago", rec.DaysSince)
					} else {
						rec.DaysSince = -1
						rec.Reason = "no calls on record, no activity date"
					}
				}
				results = append(results, rec)
			}

			// Darkest first: largest gap on top; unknown (-1) sorts to the very top.
			sort.SliceStable(results, func(i, j int) bool {
				return darkRank(results[i].DaysSince) > darkRank(results[j].DaysSince)
			})

			if novelMachineOutput(out, flags) {
				return printJSONFiltered(out, results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "No open deals have gone dark in the last %d days.\n", days)
				return nil
			}
			fmt.Fprintf(out, "%-36s  %-22s  %-14s  %-5s  %s\n", "DEAL", "ACCOUNT", "STAGE", "DAYS", "REASON")
			for _, r := range results {
				dn := r.Name
				if dn == "" {
					dn = r.DealID
				}
				daysCol := "-"
				if r.DaysSince >= 0 {
					daysCol = fmt.Sprintf("%d", r.DaysSince)
				}
				fmt.Fprintf(out, "%-36s  %-22s  %-14s  %-5s  %s\n",
					truncate(dn, 36), truncate(r.AccountName, 22), truncate(r.Stage, 14), daysCol, r.Reason)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d dark deal(s).\n", len(results))
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "A deal is dark if its last call is older than this many days")
	cmd.Flags().BoolVar(&includeUncovered, "include-uncovered", false, "Also include open deals that have never had a call")
	cmd.Flags().StringVar(&owner, "owner", "", "Filter to deals whose owner name or email contains this substring")
	cmd.Flags().StringVar(&stage, "stage", "", "Filter to deals in this exact stage")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// darkRank orders gone-dark deals: unknown activity date (-1) is the most
// urgent, then larger day gaps.
func darkRank(daysSince int) int {
	if daysSince < 0 {
		return 1 << 30
	}
	return daysSince
}

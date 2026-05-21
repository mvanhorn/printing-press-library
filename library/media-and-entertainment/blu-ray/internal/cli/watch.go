package cli

// PATCH: Hand-built local price watchlist commands backed by SQLite.

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	"github.com/spf13/cobra"
)

type watchRow struct {
	ReleaseID   int     `json:"release_id"`
	Title       string  `json:"title,omitempty"`
	TargetPrice float64 `json:"target_price,omitempty"`
	LowSeen     float64 `json:"low_seen,omitempty"`
	CurrentLow  float64 `json:"current_low,omitempty"`
	AddedAt     string  `json:"added_at"`
	AlertedAt   string  `json:"alerted_at,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage a local Blu-ray.com price-drop watchlist.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags), newWatchListCmd(flags), newWatchRmCmd(flags), newWatchCheckCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var target float64
	cmd := &cobra.Command{
		Use:   "add <release-id>",
		Short: "Add a release id to the local price watchlist.",
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli watch add 9929
  blu-ray-pp-cli watch add 9929 --target-price 14.99
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("release-id must be numeric"))
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}
			if err := s.AddToWatchlist(cmd.Context(), id, target); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Watching release %d\n", id)
			return nil
		},
	}
	cmd.Flags().Float64Var(&target, "target-price", 0, "Alert when a deal is at or below this price. If unset, alerts fire on a new historical low instead.")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List watched releases with the latest observed low price.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli watch list --json
  blu-ray-pp-cli watch list --json --select release_id,target_price,low_seen
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows, err := loadWatchRows(cmd.Context())
			if err != nil {
				return err
			}
			if flags.asJSON || flags.selectFields != "" || flags.csv || flags.quiet || flags.plain {
				return flags.printJSON(cmd, rows)
			}
			var table [][]string
			for _, r := range rows {
				table = append(table, []string{strconv.Itoa(r.ReleaseID), r.Title, formatPrice(r.TargetPrice), formatPrice(r.CurrentLow), formatPrice(r.LowSeen), r.AddedAt})
			}
			return flags.printTable(cmd, []string{"ID", "TITLE", "TARGET", "CURRENT_LOW", "LOW_SEEN", "ADDED"}, table)
		},
	}
	return cmd
}

func newWatchRmCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <release-id>",
		Short: "Remove a release id from the local watchlist.",
		// PATCH: Add agent-copyable example for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli watch rm 9929
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("release-id must be numeric"))
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}
			if _, err := s.RemoveFromWatchlist(cmd.Context(), id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed release %d\n", id)
			return nil
		},
	}
	return cmd
}

func newWatchCheckCmd(flags *rootFlags) *cobra.Command {
	var retailer string
	var limit int
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Scan current Blu-ray.com deals and record prices for watched releases.",
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli watch check --agent
  blu-ray-pp-cli watch check --retailer 1 --limit 50
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}
			// PATCH: Snapshot both target and the previously-stored historical low
			// so we can fire alerts on a *new* historical low even when no
			// --target-price was set. Fixes Greptile P1 on PR #634 — without
			// this, releases added via `watch add <id>` (target=0) never alerted
			// because the prior code only checked `target > 0 && sale <= target`.
			type watchEntry struct {
				target    float64
				prevLow   float64
				hasPrevLow bool
			}
			watched := map[int]watchEntry{}
			watchRows, err := s.ListWatchlist(cmd.Context())
			if err != nil {
				return err
			}
			for _, row := range watchRows {
				e := watchEntry{target: row.TargetPrice}
				if row.LowSeen.Valid {
					e.prevLow = row.LowSeen.Float64
					e.hasPrevLow = true
				}
				watched[row.ReleaseID] = e
			}
			if len(watched) == 0 {
				// PATCH: Keep --json output parseable even for an empty watchlist.
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{"watchlist_empty": true, "checked": 0, "alerted": 0})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Add a release with `blu-ray-pp-cli watch add <id>`.")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, err := bluRayGet(c, "https://www.blu-ray.com/deals/", false)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			deals, err := parseDealsHTML(body)
			if err != nil {
				return err
			}
			if limit <= 0 || limit > len(deals) {
				limit = len(deals)
			}
			observed := nowText()
			var alerts []DealRow
			persistenceErrors := 0
			// PATCH: --retailer accepts only a numeric retailer id. Blu-ray.com's
			// deal rows don't expose retailer names — the prior name-style match
			// (EqualFold(Itoa(d.RetailerID), retailer)) silently dropped every row
			// when the user typed "amazon". Fixes Greptile P1 on PR #634.
			retailerID := 0
			if retailer != "" {
				parsed, parseErr := strconv.Atoi(retailer)
				if parseErr != nil || parsed <= 0 {
					return fmt.Errorf("--retailer must be a numeric retailer id (got %q); Blu-ray.com deal rows do not carry retailer names", retailer)
				}
				retailerID = parsed
			}
			for _, d := range deals[:limit] {
				if retailerID > 0 && d.RetailerID != retailerID {
					continue
				}
				entry, ok := watched[d.ReleaseID]
				if !ok {
					continue
				}
				if err := s.RecordPrice(cmd.Context(), store.PriceObservation{ReleaseID: d.ReleaseID, RetailerID: d.RetailerID, ObservedAt: observed, Price: d.SalePrice}); err != nil {
					return err
				}
				if err := s.UpdateWatchlistLow(cmd.Context(), d.ReleaseID, d.SalePrice); err != nil {
					persistenceErrors++
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to persist low_seen for release %d: %v\n", d.ReleaseID, err)
				}
				hitTarget := entry.target > 0 && d.SalePrice <= entry.target
				newLow := entry.hasPrevLow && d.SalePrice < entry.prevLow
				if hitTarget || newLow {
					alerts = append(alerts, d)
					if err := s.MarkWatchlistAlerted(cmd.Context(), d.ReleaseID, d.SalePrice); err != nil {
						persistenceErrors++
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to persist alerted_at for release %d: %v\n", d.ReleaseID, err)
					}
				}
				// PATCH: Update the in-memory snapshot so the next deal row for this
				// same release (e.g. another retailer) compares against the running
				// low for this scan, not the pre-scan baseline. Fixes Greptile P1
				// follow-up on PR #634 — without this, prevLow=$20 + rows at $18
				// then $19 would fire two new-low alerts; the $19 row should not.
				if !entry.hasPrevLow || d.SalePrice < entry.prevLow {
					entry.prevLow = d.SalePrice
					entry.hasPrevLow = true
					watched[d.ReleaseID] = entry
				}
			}
			if flags.asJSON || flags.selectFields != "" || flags.csv || flags.quiet || flags.plain {
				// PATCH: Always emit the same envelope so downstream consumers
				// (especially agents in --agent mode) get a stable shape. Empty
				// alerts becomes [] not null; persistence_errors is always
				// present (0 when none). Fixes Greptile P1 follow-up on PR #634
				// — prior code returned a bare array on the success path and an
				// object only on the error path, with a third null variant when
				// alerts was nil.
				if alerts == nil {
					alerts = []DealRow{}
				}
				return flags.printJSON(cmd, map[string]any{
					"alerts":             alerts,
					"persistence_errors": persistenceErrors,
				})
			}
			if len(alerts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No watched releases hit their target price or a new historical low.")
				if persistenceErrors > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Persistence errors: %d\n", persistenceErrors)
				}
				return nil
			}
			var table [][]string
			for _, a := range alerts {
				table = append(table, []string{strconv.Itoa(a.ReleaseID), a.Title, formatPrice(a.SalePrice), strconv.Itoa(a.RetailerID), a.DetailURL})
			}
			if err := flags.printTable(cmd, []string{"ID", "TITLE", "PRICE", "RETAILER", "URL"}, table); err != nil {
				return err
			}
			if persistenceErrors > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Persistence errors: %d\n", persistenceErrors)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&retailer, "retailer", "", "Only record deals from this numeric retailer id (Blu-ray.com deal rows do not expose retailer names).")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum deal rows to scan.")
	return cmd
}

func loadWatchRows(ctx context.Context) ([]watchRow, error) {
	s, err := store.OpenWithContext(ctx, defaultDBPath("blu-ray-pp-cli"))
	if err != nil {
		return nil, err
	}
	defer s.Close()
	if err := s.MigrateBluRayCatalog(); err != nil {
		return nil, err
	}
	rows, err := s.ListWatchlist(ctx)
	if err != nil {
		return nil, err
	}
	var out []watchRow
	for _, row := range rows {
		r := watchRow{
			ReleaseID:   row.ReleaseID,
			Title:       nullStringValue(row.Title),
			TargetPrice: row.TargetPrice,
			LowSeen:     nullFloatValue(row.LowSeen),
			CurrentLow:  nullFloatValue(row.CurrentLow),
			AddedAt:     row.AddedAt,
			AlertedAt:   nullStringValue(row.AlertedAt),
		}
		out = append(out, r)
	}
	return out, nil
}

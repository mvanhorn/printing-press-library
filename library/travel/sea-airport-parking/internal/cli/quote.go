// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored headline command. Not generated.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/parking"

	"github.com/spf13/cobra"
)

func newQuoteCmd(flags *rootFlags) *cobra.Command {
	var entry, exit, promo string

	cmd := &cobra.Command{
		Use:   "quote",
		Short: "Quote price and availability for a specific entry/exit date range",
		Long: "Quote the official SEA on-airport Reserved garage for an exact entry/exit\n" +
			"date range: total price, the nightly breakdown, any promo discount, and whether\n" +
			"a space is available. Every quote is persisted so 'history' and 'drift' can\n" +
			"show how the price moved over time.\n\n" +
			"Use this for a single known date range. To search flexible dates use 'sweep';\n" +
			"to render a per-day grid use 'calendar'.",
		Example: "  sea-airport-parking-pp-cli quote --entry 2026-08-15T11:00 --exit 2026-08-18T11:00",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would quote SEA Reserved parking for the given entry/exit range")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if entry == "" || exit == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--entry and --exit are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entryT, err := parseWhen(entry)
			if err != nil {
				return usageErr(err)
			}
			exitT, err := parseWhen(exit)
			if err != nil {
				return usageErr(err)
			}
			if err := validateRange(entryT, exitT); err != nil {
				return usageErr(err)
			}

			pc, err := newParkingClient(flags)
			if err != nil {
				return err
			}
			q, err := pc.Quote(ctx, entryT, exitT, promo)
			if err != nil {
				return fmt.Errorf("quoting parking: %w", err)
			}

			// Persist the snapshot (best-effort; a store failure must not
			// swallow the quote the user asked for).
			if perr := persistQuote(ctx, defaultDBPath(dbName), q); perr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not persist quote: %v\n", perr)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, q)
			}
			renderQuoteHuman(cmd, q)
			return nil
		},
	}
	cmd.Flags().StringVar(&entry, "entry", "", "Entry date/time, e.g. 2026-08-15T11:00 or 2026-08-15")
	cmd.Flags().StringVar(&exit, "exit", "", "Exit date/time, e.g. 2026-08-18T11:00 or 2026-08-18")
	cmd.Flags().StringVar(&promo, "promo", "", "Optional promo code to apply")
	return cmd
}

// renderQuoteHuman prints a single quote in the terminal format.
func renderQuoteHuman(cmd *cobra.Command, q *parking.Quote) {
	w := cmd.OutOrStdout()
	name := q.ProductName
	if name == "" {
		name = "Reserved Parking"
	}
	fmt.Fprintf(w, "%s  (%s → %s, %d night(s))\n", name, q.EntryStr, q.ExitStr, q.Nights)
	switch {
	case q.Invalid:
		fmt.Fprintf(w, "  invalid request: %s\n", q.Reason)
	case q.SoldOut:
		fmt.Fprintf(w, "  SOLD OUT: %s\n", q.Reason)
	case q.Available:
		fmt.Fprintf(w, "  available: $%.2f total", q.TotalPrice)
		if q.OriginalPrice > 0 && q.OriginalPrice != q.TotalPrice {
			fmt.Fprintf(w, " (was $%.2f)", q.OriginalPrice)
		}
		fmt.Fprintln(w)
		if len(q.NightlyPrices) > 0 {
			for _, n := range q.NightlyPrices {
				fmt.Fprintf(w, "    %s  $%.2f\n", n.Date, n.Price)
			}
		}
	default:
		fmt.Fprintf(w, "  unavailable: %s\n", q.Reason)
	}
}

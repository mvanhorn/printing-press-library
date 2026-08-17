// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: price/availability drift. Hand-authored.
// pp:data-source computed

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/store"

	"github.com/spf13/cobra"
)

type driftResult struct {
	Entry              string  `json:"entry"`
	Exit               string  `json:"exit"`
	Snapshots          int     `json:"snapshots"`
	FirstPrice         float64 `json:"first_price"`
	LatestPrice        float64 `json:"latest_price"`
	PriceDelta         float64 `json:"price_delta"`
	MinPrice           float64 `json:"min_price"`
	MaxPrice           float64 `json:"max_price"`
	FirstCaptured      string  `json:"first_captured"`
	LatestCaptured     string  `json:"latest_captured"`
	AvailabilityFlips  int     `json:"availability_flips"`
	CurrentlyAvailable bool    `json:"currently_available"`
	Note               string  `json:"note,omitempty"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var entry, exit, since string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Summarize how a range's price and availability moved across stored snapshots: first vs latest, min/max, sold-out flips.",
		Long: "Summarize the change in an exact range's price and availability across your\n" +
			"stored snapshots: first vs latest price, min/max, and how many times it flipped\n" +
			"between sold-out and available. Requires at least 2 snapshots.\n\n" +
			"For the full row-by-row list use 'history'.",
		Example: "  sea-airport-parking-pp-cli drift --entry 2026-08-15T11:00 --exit 2026-08-18T11:00 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if entry == "" || exit == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--entry and --exit are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entryKey, exitKey, err := rangeKeys(entry, exit)
			if err != nil {
				return usageErr(err)
			}
			var sinceT time.Time
			if since != "" {
				d, derr := cliutil.ParseDurationLoose(since)
				if derr != nil {
					return usageErr(fmt.Errorf("invalid --since %q (use e.g. 7d, 24h)", since))
				}
				sinceT = time.Now().Add(-d)
			}

			dbPath := defaultDBPath(dbName)
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local quote history at %s\nrun: sea-airport-parking-pp-cli quote --entry %s --exit %s (twice, over time)\n", dbPath, entry, exit)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}

			s, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()
			rows, err := s.ListQuotes(ctx, entryKey, exitKey, sinceT)
			if err != nil {
				return fmt.Errorf("reading snapshots: %w", err)
			}

			res := computeDrift(entryKey, exitKey, rows)
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			renderDriftHuman(cmd, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&entry, "entry", "", "Entry date/time of the range, e.g. 2026-08-15T11:00")
	cmd.Flags().StringVar(&exit, "exit", "", "Exit date/time of the range, e.g. 2026-08-18T11:00")
	cmd.Flags().StringVar(&since, "since", "", "Only snapshots captured within this window (e.g. 30d)")
	return cmd
}

// computeDrift summarizes a range's snapshots. rows arrive newest-first.
func computeDrift(entry, exit string, rows []store.QuoteRecord) driftResult {
	res := driftResult{Entry: entry, Exit: exit, Snapshots: len(rows)}
	if len(rows) == 0 {
		res.Note = "no snapshots recorded for this range; run 'quote' first"
		return res
	}
	latest := rows[0]
	first := rows[len(rows)-1]
	res.FirstPrice = first.TotalPrice
	res.LatestPrice = latest.TotalPrice
	res.PriceDelta = latest.TotalPrice - first.TotalPrice
	res.FirstCaptured = first.CapturedAt
	res.LatestCaptured = latest.CapturedAt
	res.CurrentlyAvailable = latest.Available

	res.MinPrice = rows[0].TotalPrice
	res.MaxPrice = rows[0].TotalPrice
	prevAvail := rows[len(rows)-1].Available
	// Iterate chronologically (oldest -> newest) for flip counting.
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if r.TotalPrice > 0 {
			if res.MinPrice == 0 || r.TotalPrice < res.MinPrice {
				res.MinPrice = r.TotalPrice
			}
			if r.TotalPrice > res.MaxPrice {
				res.MaxPrice = r.TotalPrice
			}
		}
		if i < len(rows)-1 && r.Available != prevAvail {
			res.AvailabilityFlips++
		}
		prevAvail = r.Available
	}
	if len(rows) < 2 {
		res.Note = "only one snapshot; capture another over time to see drift"
	}
	return res
}

func renderDriftHuman(cmd *cobra.Command, res driftResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Drift for %s → %s (%d snapshot(s)):\n", res.Entry, res.Exit, res.Snapshots)
	if res.Snapshots == 0 {
		fmt.Fprintln(w, "  "+res.Note)
		return
	}
	fmt.Fprintf(w, "  first:  $%.2f  (%s)\n", res.FirstPrice, res.FirstCaptured)
	fmt.Fprintf(w, "  latest: $%.2f  (%s)\n", res.LatestPrice, res.LatestCaptured)
	fmt.Fprintf(w, "  change: $%.2f   range: $%.2f–$%.2f\n", res.PriceDelta, res.MinPrice, res.MaxPrice)
	fmt.Fprintf(w, "  availability flips: %d   currently: %s\n", res.AvailabilityFlips, availableWord(res.CurrentlyAvailable))
	if res.Note != "" {
		fmt.Fprintln(w, "  "+res.Note)
	}
}

func availableWord(b bool) string {
	if b {
		return "available"
	}
	return "unavailable"
}

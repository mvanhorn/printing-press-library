// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/pricehist"
	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/store"
	"github.com/spf13/cobra"
)

// cycleRow is one product's half-price rhythm, derived entirely from the local
// mirror. Woolworths has no historical endpoint, so there is deliberately no
// live equivalent for this command: --data-source live is refused rather than
// quietly answered from a single "now" snapshot.
type cycleRow struct {
	Stockcode         string              `json:"stockcode"`
	Name              string              `json:"name,omitempty"`
	Brand             string              `json:"brand,omitempty"`
	Observations      int                 `json:"observations"`
	DaysOfHistory     float64             `json:"days_of_history"`
	Episodes          int                 `json:"episodes"`
	MedianGapDays     float64             `json:"median_gap_days"`
	MedianRunDays     float64             `json:"median_run_days"`
	DaysSinceLastEnd  float64             `json:"days_since_last_episode_ended"`
	Forecast          bool                `json:"forecast"`
	NextWindowStart   string              `json:"next_window_start,omitempty"`
	NextWindowEnd     string              `json:"next_window_end,omitempty"`
	NextWindowStartAt int64               `json:"next_window_start_at,omitempty"`
	NextWindowEndAt   int64               `json:"next_window_end_at,omitempty"`
	Confidence        string              `json:"confidence"`
	Note              string              `json:"note"`
	EpisodeList       []pricehist.Episode `json:"episode_list"`
}

func newNovelCycleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "cycle <term|stockcode>",
		Short: "Estimates when a product's next half-price window is likely to open, from its own recorded discount episodes.",
		Long: strings.Trim(`
Segments the local price_observation mirror into half-price EPISODES -
contiguous runs of observations where IsHalfPrice was true - and reports the
rhythm they describe: how many episodes, the median gap between episode starts,
the median run length, how long since the last one ended, and only then a
next-window estimate.

Confidence follows the episode count: fewer than 3 episodes is "low", and with
fewer than 2 no window is estimated at all, because a single sighting contains
no gap to project. Irregular gaps downgrade confidence further. A product with
no recorded half-price episode yields an empty result and a note saying so -
never a row of zeroes that could read as a measured rhythm.

This command is local-only. Woolworths exposes no historical endpoint, so
--data-source live is refused rather than answered from a single snapshot.
`, "\n"),
		Example: strings.Trim(`
  woolworths-pp-cli cycle 6073909 --agent
  woolworths-pp-cli cycle 'tim tam' --json
  woolworths-pp-cli cycle 36066 --db ./woolworths.db
`, "\n"),
		Annotations: map[string]string{
			// Empty is a valid answer, not an error: an unknown stockcode is indistinguishable from a tracked one with no recorded episodes yet.
			// Inventing an "empty means not found" rule would break honest-empty output.
			"pp:no-error-path-probe": "true",
			"mcp:read-only":          "true",
			"pp:data-source":         "local",
			"pp:happy-args":          "6073909",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cycle")
			}
			// Refuse --data-source live with the shared "no live equivalent"
			// message rather than silently degrading to a one-point history.
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			query := ""
			if len(args) > 0 {
				query = strings.TrimSpace(args[0])
			}
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<term|stockcode> is required"))
			}
			resolvedDB := ppwDBPath(dbPath)

			// Missing-mirror guard: this command reads SQLite and never writes,
			// so an absent mirror is an empty answer plus a way forward, never
			// a crash and never a fabricated cycle.
			if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: woolworths-pp-cli real-special <term> --db %s to start recording price history (sync does not build history on this API)\n", resolvedDB, resolvedDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]cycleRow, 0), flags)
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, resolvedDB)
			if err != nil {
				return fmt.Errorf("opening local mirror %s: %w", resolvedDB, err)
			}
			defer db.Close()
			if err := store.EnsureWoolworthsTables(ctx, db.DB()); err != nil {
				return err
			}

			// Drain the stockcode lookup fully before running any per-product
			// history query.
			codes, err := ppwLookupStockcodes(ctx, db.DB(), query, 5)
			if err != nil {
				return err
			}

			rows := make([]cycleRow, 0, len(codes))
			notes := make([]string, 0, 2)
			now := time.Now().Unix()

			for _, code := range codes {
				history, err := store.PriceHistory(ctx, db.DB(), code)
				if err != nil {
					return err
				}
				if len(history) == 0 {
					continue
				}
				points := ppwPoints(history)
				summary := pricehist.Summarize(points)
				analysis := pricehist.AnalyzeCycle(points, now)
				latest := history[len(history)-1]
				// A product with no recorded half-price episode has no cycle,
				// so it contributes a note rather than a row of zeroes that a
				// caller could mistake for a measured rhythm.
				if analysis.Episodes == 0 {
					notes = append(notes, fmt.Sprintf("note: %s (%s) has no recorded half-price episode across %d observation(s) spanning %.0f day(s); nothing to forecast from - keep running 'real-special' on it over several weeks to record episodes",
						code, latest.Name, summary.Count, summary.DaysOfHistory))
					continue
				}
				rows = append(rows, cycleRow{
					Stockcode:         code,
					Name:              latest.Name,
					Brand:             latest.Brand,
					Observations:      summary.Count,
					DaysOfHistory:     summary.DaysOfHistory,
					Episodes:          analysis.Episodes,
					MedianGapDays:     analysis.MedianGapDays,
					MedianRunDays:     analysis.MedianRunDays,
					DaysSinceLastEnd:  analysis.DaysSinceLastEnd,
					Forecast:          analysis.Forecast,
					NextWindowStart:   ppwUnixString(analysis.NextWindowStartAt),
					NextWindowEnd:     ppwUnixString(analysis.NextWindowEndAt),
					NextWindowStartAt: analysis.NextWindowStartAt,
					NextWindowEndAt:   analysis.NextWindowEndAt,
					Confidence:        analysis.Confidence,
					Note:              analysis.Note,
					EpisodeList:       analysis.EpisodeList,
				})
			}
			if len(codes) == 0 {
				notes = append(notes, fmt.Sprintf("note: nothing recorded locally for %q; run 'woolworths-pp-cli real-special %s' a few times to start a history", query, query))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, n := range notes {
					fmt.Fprintln(cmd.ErrOrStderr(), n)
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no locally recorded products matched %q - no cycle to report\n", query)
				for _, n := range notes {
					fmt.Fprintln(cmd.OutOrStdout(), n)
				}
				return nil
			}
			headers := []string{"STOCKCODE", "PRODUCT", "OBS", "DAYS", "EPISODES", "MED GAP", "MED RUN", "SINCE LAST", "CONFIDENCE", "NEXT WINDOW"}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				next := "-"
				if r.Forecast {
					next = fmt.Sprintf("%s .. %s",
						time.Unix(r.NextWindowStartAt, 0).UTC().Format("2006-01-02"),
						time.Unix(r.NextWindowEndAt, 0).UTC().Format("2006-01-02"))
				}
				table = append(table, []string{
					r.Stockcode,
					ppwTruncate(r.Name, 34),
					fmt.Sprintf("%d", r.Observations),
					fmt.Sprintf("%.0f", r.DaysOfHistory),
					fmt.Sprintf("%d", r.Episodes),
					fmt.Sprintf("%.1f", r.MedianGapDays),
					fmt.Sprintf("%.1f", r.MedianRunDays),
					fmt.Sprintf("%.1f", r.DaysSinceLastEnd),
					r.Confidence,
					next,
				})
			}
			if err := flags.printTable(cmd, headers, table); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout())
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", r.Stockcode, r.Note)
			}
			for _, n := range notes {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite mirror (default: the CLI data directory)")
	return cmd
}

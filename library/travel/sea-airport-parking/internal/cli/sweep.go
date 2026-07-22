// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: flexible-date sweep. Hand-authored.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/internal/parking"

	"github.com/spf13/cobra"
)

type sweepResult struct {
	Nights      int              `json:"nights"`
	ScannedDays int              `json:"scanned_days"`
	MaxScanDays int              `json:"max_scan_days"`
	Cheapest    *parking.Quote   `json:"cheapest,omitempty"`
	Available   []*parking.Quote `json:"available"`
	Note        string           `json:"note,omitempty"`
}

func newNovelSweepCmd(flags *rootFlags) *cobra.Command {
	var from, to, tod, promo string
	var nights, limit, maxScanDays int
	var any bool

	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Search a flexible date window for the cheapest or any-available Reserved stay.",
		Long: "Search a flexible entry-date window for the cheapest (or first available)\n" +
			"fixed-length Reserved stay. Fans out a quote for each candidate entry day and\n" +
			"returns the winner, persisting every quote.\n\n" +
			"Use this for a flexible date window. For a single known range use 'quote'; to\n" +
			"render the whole per-day grid use 'calendar'.",
		Example: "  sea-airport-parking-pp-cli sweep --from 2026-08-10 --to 2026-08-20 --nights 3 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sweep the entry-date window for the cheapest available Reserved stay")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if from == "" || to == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			fromT, err := parseWhen(from)
			if err != nil {
				return usageErr(err)
			}
			toT, err := parseWhen(to)
			if err != nil {
				return usageErr(err)
			}
			hour, minute, err := parseTimeOfDay(tod)
			if err != nil {
				return usageErr(err)
			}
			if toT.Before(fromT) {
				return usageErr(fmt.Errorf("--to must not be before --from"))
			}
			if cliutil.IsDogfoodEnv() && maxScanDays > 1 {
				maxScanDays = 1 // keep live-dogfood inside the per-command timeout
			}

			pc, err := newParkingClient(flags)
			if err != nil {
				return err
			}

			res := sweepResult{Nights: nights, MaxScanDays: maxScanDays, Available: []*parking.Quote{}}
			day := time.Date(fromT.Year(), fromT.Month(), fromT.Day(), hour, minute, 0, 0, time.Local)
			last := time.Date(toT.Year(), toT.Month(), toT.Day(), hour, minute, 0, 0, time.Local)
			scanCapHit := true
			for !day.After(last) && res.ScannedDays < maxScanDays {
				exitT := day.AddDate(0, 0, nights)
				q, qerr := pc.Quote(ctx, day, exitT, promo)
				if qerr != nil {
					return fmt.Errorf("sweeping %s: %w", day.Format("2006-01-02"), qerr)
				}
				res.ScannedDays++
				if perr := persistQuote(ctx, defaultDBPath(dbName), q); perr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not persist quote for %s: %v\n", day.Format("2006-01-02"), perr)
				}
				if q.Available {
					res.Available = append(res.Available, q)
					if any {
						scanCapHit = false
						break
					}
				}
				day = day.AddDate(0, 0, 1)
			}
			if day.After(last) {
				scanCapHit = false
			}

			sort.SliceStable(res.Available, func(i, j int) bool {
				return res.Available[i].TotalPrice < res.Available[j].TotalPrice
			})
			if len(res.Available) > 0 {
				res.Cheapest = res.Available[0]
			}
			totalAvailable := len(res.Available)
			if limit > 0 && totalAvailable > limit {
				res.Available = res.Available[:limit]
				res.Note = fmt.Sprintf("showing %d of %d open stays (cheapest first); raise --limit to see the rest", limit, totalAvailable)
			}
			if len(res.Available) == 0 {
				if scanCapHit {
					res.Note = fmt.Sprintf("scanned %d of the requested entry days without finding availability; raise --max-scan-days to widen the search", res.ScannedDays)
				} else {
					res.Note = fmt.Sprintf("no availability across %d entry day(s) for a %d-night stay", res.ScannedDays, nights)
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			renderSweepHuman(cmd, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Earliest entry date, e.g. 2026-08-10")
	cmd.Flags().StringVar(&to, "to", "", "Latest entry date, e.g. 2026-08-20")
	cmd.Flags().IntVar(&nights, "nights", 3, "Length of stay in nights")
	cmd.Flags().StringVar(&tod, "time", "11:00", "Entry/exit time of day (HH:MM)")
	cmd.Flags().BoolVar(&any, "any", false, "Return the first available stay instead of the cheapest")
	cmd.Flags().StringVar(&promo, "promo", "", "Optional promo code to apply")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum available candidates to return")
	cmd.Flags().IntVar(&maxScanDays, "max-scan-days", 45, "Maximum entry days to scan before returning")
	return cmd
}

func renderSweepHuman(cmd *cobra.Command, res sweepResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Swept %d entry day(s) for a %d-night stay.\n", res.ScannedDays, res.Nights)
	if res.Cheapest != nil {
		fmt.Fprintf(w, "Cheapest: %s → %s  $%.2f\n", res.Cheapest.EntryStr, res.Cheapest.ExitStr, res.Cheapest.TotalPrice)
	}
	for _, q := range res.Available {
		fmt.Fprintf(w, "  %s → %s  $%.2f\n", q.EntryStr, q.ExitStr, q.TotalPrice)
	}
	if res.Note != "" {
		fmt.Fprintf(w, "%s\n", res.Note)
	}
}

// parseTimeOfDay parses HH:MM into hour and minute.
func parseTimeOfDay(s string) (int, int, error) {
	if s == "" {
		return defaultEntryHour, 0, nil
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid --time %q (use HH:MM)", s)
	}
	return t.Hour(), t.Minute(), nil
}

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newKTMacrosCmd hosts transcendence commands grouped under "macros".
// Current children: macros gap.
func newKTMacrosCmd(flags *rootFlags) *cobra.Command {
	parent := &cobra.Command{
		Use:   "macros",
		Short: "Macro analytics across a window of diary days",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	parent.AddCommand(newKTMacrosGapCmd(flags))
	return parent
}

// macros gap [--days N] [--by meal] [--date X]
// Computes per-macro `target - actual` across a window. Optionally
// groups the gap by meal slot.
func newKTMacrosGapCmd(flags *rootFlags) *cobra.Command {
	var days int
	var byMeal bool
	var dateFlag string

	cmd := &cobra.Command{
		Use:   "gap",
		Short: "Show macro target gaps across N days, optionally by meal slot",
		Long: `For each macro, compute target - actual over the trailing window.

Pulls daily summary (for the energy target) and the diary day (for actual
macros). With --by-meal, breaks the actual macros out by meal slot so you
can see where the gap concentrates.

Useful when planning what to eat: an agent can read this output and pick
foods that close the protein gap within the remaining energy budget.`,
		Example: `  kaloricke-tabulky-pp-cli macros gap
  kaloricke-tabulky-pp-cli macros gap --days 7 --json
  kaloricke-tabulky-pp-cli macros gap --by-meal --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		// pp:client-call — calls /user/diary/<date>/get and /statistic/summary/<date>/get
		// via ktFetchDiaryDay / ktFetchSummaryDay (both wrap client.GetWithHeadersNoCache).
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, _, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				return err
			}

			startDDMM, err := parseFlexDate(dateFlag)
			if err != nil {
				return err
			}
			start, err := time.Parse("02.01.2006", startDDMM)
			if err != nil {
				return err
			}
			if days < 1 {
				days = 1
			}

			type windowResult struct {
				StartDate      string                  `json:"start_date"`
				EndDate        string                  `json:"end_date"`
				Days           int                     `json:"days"`
				EnergyKJ       float64                 `json:"actual_energy_kj"`
				EnergyTargetKJ float64                 `json:"target_energy_kj"`
				EnergyGapKJ    float64                 `json:"gap_energy_kj"`
				EntryCount     int                     `json:"entry_count"`
				BySlot         map[string]ktSlotMacros `json:"by_meal_slot,omitempty"`
			}

			result := windowResult{
				EndDate: start.Format("2006-01-02"),
				Days:    days,
				BySlot:  map[string]ktSlotMacros{},
			}
			result.StartDate = start.AddDate(0, 0, -(days - 1)).Format("2006-01-02")

			for offset := 0; offset < days; offset++ {
				d := start.AddDate(0, 0, -offset)
				dd := d.Format("02.01.2006")

				diary, err := ktFetchDiaryDay(c, dd)
				if err != nil {
					return fmt.Errorf("diary fetch for %s: %w", dd, err)
				}
				dayMacros := ktAggregateDay(diary, d.Format("2006-01-02"))
				result.EnergyKJ += dayMacros.EnergyKJ
				result.EntryCount += dayMacros.EntryCount
				if byMeal {
					for slotID, slot := range dayMacros.BySlot {
						agg := result.BySlot[slotID]
						agg.SlotID = slotID
						agg.SlotLabel = slot.SlotLabel
						agg.EnergyKJ += slot.EnergyKJ
						agg.ProteinG += slot.ProteinG
						agg.CarbG += slot.CarbG
						agg.FatG += slot.FatG
						agg.FiberG += slot.FiberG
						agg.EntryCount += slot.EntryCount
						result.BySlot[slotID] = agg
					}
				}

				summary, err := ktFetchSummaryDay(c, dd)
				if err != nil {
					return fmt.Errorf("summary fetch for %s: %w", dd, err)
				}
				if t, ok := ktParseCzechNum(summary.TodayEnergyTarget); ok {
					result.EnergyTargetKJ += t
				}
			}
			result.EnergyGapKJ = result.EnergyTargetKJ - result.EnergyKJ

			// Sort BySlot for stable JSON output
			if byMeal {
				sortedSlots := make([]string, 0, len(result.BySlot))
				for k := range result.BySlot {
					sortedSlots = append(sortedSlots, k)
				}
				sort.Strings(sortedSlots)
				sortedMap := map[string]ktSlotMacros{}
				for _, k := range sortedSlots {
					sortedMap[k] = result.BySlot[k]
				}
				result.BySlot = sortedMap
			} else {
				result.BySlot = nil
			}

			// Output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return ktEmit(cmd.OutOrStdout(), flags, result)
			}
			// Human view
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Macros gap for %s to %s (%d days)\n", result.StartDate, result.EndDate, result.Days)
			fmt.Fprintf(w, "  Actual energy:  %.0f kJ\n", result.EnergyKJ)
			fmt.Fprintf(w, "  Target energy:  %.0f kJ\n", result.EnergyTargetKJ)
			fmt.Fprintf(w, "  Gap:            %.0f kJ%s\n", result.EnergyGapKJ, ifMatch(result.EnergyGapKJ > 0, " (under target)", " (over target)"))
			fmt.Fprintf(w, "  Diary entries:  %d\n", result.EntryCount)
			if byMeal && len(result.BySlot) > 0 {
				fmt.Fprintln(w, "By meal slot:")
				for _, slotID := range []string{"1", "2", "3", "4", "5", "6"} {
					s, ok := result.BySlot[slotID]
					if !ok {
						continue
					}
					label := s.SlotLabel
					if len(label) > 0 {
						label = strings.ToUpper(label[:1]) + label[1:]
					}
					fmt.Fprintf(w, "  %s (%d entries): %.0f kJ, %.1fg protein, %.1fg carb, %.1fg fat, %.1fg fiber\n",
						label, s.EntryCount, s.EnergyKJ, s.ProteinG, s.CarbG, s.FatG, s.FiberG)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 1, "Number of trailing days to include")
	cmd.Flags().BoolVar(&byMeal, "by-meal", false, "Break out the gap by meal slot")
	cmd.Flags().StringVar(&dateFlag, "date", "today", "Anchor date (today, yesterday, -N, YYYY-MM-DD, or DD.MM.YYYY)")
	return cmd
}

func ifMatch(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

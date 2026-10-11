// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

type vacancyWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Days  int    `json:"days"`
}

type vacancyResult struct {
	Units   []int           `json:"units"`
	MinDays int             `json:"min_consecutive_days"`
	Windows []vacancyWindow `json:"windows"`
}

func newNovelVacancyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var units string
	var consecutive int
	var days int

	cmd := &cobra.Command{
		Use:   "vacancy",
		Short: "Find consecutive vacant days across multiple units simultaneously",
		Long: `Finds windows where all specified units are vacant at the same time.
Useful for scheduling maintenance, personal stays, or renovations that
need multiple adjacent units free simultaneously.`,
		Example: strings.Trim(`
  hostfully-pp-cli vacancy --units 3,4 --consecutive 2
  hostfully-pp-cli vacancy --units all --consecutive 1 --days 30
  hostfully-pp-cli vacancy --units 3,4,5 --consecutive 3 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "vacancy")
			}

			if dbPath == "" {
				dbPath = defaultDBPath("hostfully-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w (run 'hostfully-pp-cli sync' first)", err)
			}
			defer db.Close()

			// Parse units
			targetUnits, err := parseUnits(units)
			if err != nil {
				return err
			}

			reservations, err := loadReservations(db)
			if err != nil {
				return fmt.Errorf("loading reservations: %w", err)
			}

			today := time.Now().Truncate(24 * time.Hour)

			// Build occupancy map per unit
			type occupancy map[int]map[string]bool // unit -> date_str -> occupied
			occ := make(map[int]map[string]bool)
			for _, u := range targetUnits {
				occ[u] = make(map[string]bool)
			}

			for _, r := range reservations {
				if _, ok := occ[r.Unit]; !ok {
					continue
				}
				ciDate := r.CheckIn.Truncate(24 * time.Hour)
				coDate := r.CheckOut.Truncate(24 * time.Hour)
				// Mark occupied: checkin <= day < checkout
				for d := ciDate; d.Before(coDate); d = d.Add(24 * time.Hour) {
					occ[r.Unit][d.Format("2006-01-02")] = true
				}
			}

			// Find days where ALL target units are vacant
			var allVacantDays []time.Time
			for i := 0; i < days; i++ {
				day := today.Add(time.Duration(i) * 24 * time.Hour)
				dayStr := day.Format("2006-01-02")
				allFree := true
				for _, u := range targetUnits {
					if occ[u][dayStr] {
						allFree = false
						break
					}
				}
				if allFree {
					allVacantDays = append(allVacantDays, day)
				}
			}

			// Find consecutive windows
			var windows []vacancyWindow
			i := 0
			for i < len(allVacantDays) {
				start := allVacantDays[i]
				end := start
				j := i + 1
				for j < len(allVacantDays) {
					next := allVacantDays[j]
					if next.Sub(end) == 24*time.Hour {
						end = next
						j++
					} else {
						break
					}
				}
				windowDays := int(end.Sub(start)/(24*time.Hour)) + 1
				if windowDays >= consecutive {
					windows = append(windows, vacancyWindow{
						Start: start.Format("Mon Jan 2"),
						End:   end.Format("Mon Jan 2"),
						Days:  windowDays,
					})
				}
				i = j
			}

			result := vacancyResult{
				Units:   targetUnits,
				MinDays: consecutive,
				Windows: windows,
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			// Human output
			unitStrs := make([]string, len(targetUnits))
			for i, u := range targetUnits {
				unitStrs[i] = fmt.Sprintf("#%d", u)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "## Vacancy Windows — Units %s (%d+ consecutive days)\n\n",
				strings.Join(unitStrs, ", "), consecutive)

			if len(windows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No %d+ consecutive vacant days found in the next %d days.\n", consecutive, days)
				return nil
			}

			for _, w := range windows {
				if w.Days == 1 {
					fmt.Fprintf(cmd.OutOrStdout(), "  • %s (%d day)\n", w.Start, w.Days)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  • %s – %s (%d days)\n", w.Start, w.End, w.Days)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&units, "units", "all", "Comma-separated unit numbers (e.g., 3,4) or 'all'")
	cmd.Flags().IntVar(&consecutive, "consecutive", 2, "Minimum consecutive vacant days required")
	cmd.Flags().IntVar(&days, "days", 30, "Number of days to search ahead")

	return cmd
}

func parseUnits(input string) ([]int, error) {
	if input == "all" {
		return unitNumbers(), nil
	}
	parts := strings.Split(input, ",")
	var units []int
	seen := map[int]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		u, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid unit number: %s", p)
		}
		if u == 8 {
			return nil, fmt.Errorf("unit 8 does not exist in Hostfully")
		}
		if u < 1 || u > 10 {
			return nil, fmt.Errorf("unit number must be 1-10 (got %d)", u)
		}
		if !seen[u] {
			units = append(units, u)
			seen[u] = true
		}
	}
	sort.Ints(units)
	return units, nil
}

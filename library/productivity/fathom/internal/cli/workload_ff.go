package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

func newWorkloadCmd(flags *rootFlags) *cobra.Command {
	var team string
	var weeks int
	var threshold float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "workload",
		Short: "Team meeting load — see who's spending the most hours in meetings per week",
		Long: `Aggregate meeting hours per team member per week from the local store.
Identifies individuals who may be overloaded with meetings.

Run 'sync --full' first to populate the local store.`,
		Example: strings.Trim(`
  fathom-pp-cli workload
  fathom-pp-cli workload --team Engineering --weeks 4
  fathom-pp-cli workload --threshold 15 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fathom-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if weeks <= 0 {
				weeks = 4
			}
			cutoff, _ := parseSince(fmt.Sprintf("%dd", weeks*7))

			meetings, err := loadAllMeetings(cmd.Context(), db)
			if err != nil {
				return err
			}

			// person -> week -> minutes
			type personKey struct{ name, email string }
			personWeek := map[personKey]map[string]float64{}

			for _, m := range meetings {
				t, err := parseFlexTime(m.CreatedAt)
				if err != nil || (!cutoff.IsZero() && t.Before(cutoff)) {
					continue
				}
				dur := m.durationMinutes()
				if dur <= 0 {
					continue
				}
				week := isoWeek(t)

				for _, inv := range m.CalendarInvitees {
					if team != "" {
						// Skip meeting if RecordedBy is nil or team doesn't match.
						if m.RecordedBy == nil || !strings.EqualFold(m.RecordedBy.Team, team) {
							continue
						}
					}
					pk := personKey{name: inv.Name, email: inv.Email}
					if _, ok := personWeek[pk]; !ok {
						personWeek[pk] = map[string]float64{}
					}
					personWeek[pk][week] += dur
				}
			}

			type weekLoad struct {
				Week         string  `json:"week"`
				MinutesTotal float64 `json:"minutes"`
				HoursTotal   float64 `json:"hours"`
			}
			type personLoad struct {
				Name         string     `json:"name"`
				Email        string     `json:"email"`
				TotalHours   float64    `json:"total_hours"`
				WeeklyAvg    float64    `json:"weekly_avg_hours"`
				Overloaded   bool       `json:"overloaded"`
				Weeks        []weekLoad `json:"weeks"`
			}

			var results []personLoad
			for pk, weekMinutes := range personWeek {
				var wl []weekLoad
				var total float64
				for w, mins := range weekMinutes {
					wl = append(wl, weekLoad{Week: w, MinutesTotal: mins, HoursTotal: mins / 60.0})
					total += mins
				}
				sort.Slice(wl, func(i, j int) bool { return wl[i].Week < wl[j].Week })
				avg := (total / 60.0) / float64(weeks)
				overloaded := threshold > 0 && avg >= threshold
				results = append(results, personLoad{
					Name:       pk.name,
					Email:      pk.email,
					TotalHours: total / 60.0,
					WeeklyAvg:  avg,
					Overloaded: overloaded,
					Weeks:      wl,
				})
			}

			// Sort by weekly avg desc
			sort.Slice(results, func(i, j int) bool {
				return results[i].WeeklyAvg > results[j].WeeklyAvg
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			// Human output
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No meeting data found. Run 'fathom-pp-cli sync --full' to populate the store.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Meeting workload (last %d weeks)\n\n", weeks)
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s  %-10s  %-12s  %s\n", "Name", "Total hrs", "Avg/wk hrs", "Status")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 70))
			for _, r := range results {
				status := ""
				if r.Overloaded {
					status = "⚠ overloaded"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s  %-10.1f  %-12.1f  %s\n",
					truncate(r.Name, 29), r.TotalHours, r.WeeklyAvg, status)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&team, "team", "", "Filter by team name (matches recorded_by.team)")
	cmd.Flags().IntVar(&weeks, "weeks", 4, "Number of weeks to analyze")
	cmd.Flags().Float64Var(&threshold, "threshold", 15, "Weekly hour threshold for overloaded flag (0 to disable)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}


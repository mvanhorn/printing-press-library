// Hand-built (Phase 3): cross-calendar conflict detection.
// Pure local-data feature; reads from the synced events table.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/outlook-calendar/internal/store"

	"github.com/spf13/cobra"
)

func newConflictsCmd(flags *rootFlags) *cobra.Command {
	var from, to string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "conflicts",
		Short: "Find overlapping events across all your synced calendars in a date window",
		Long: `Conflicts performs a self-join on the local SQLite event store and emits
every overlapping event pair within the requested window. Pre-conditions:
data must have been synced via 'outlook-calendar-pp-cli sync' first.`,
		Example: strings.Trim(`
  outlook-calendar-pp-cli conflicts --from today --to +7d --json
  outlook-calendar-pp-cli conflicts --from 2026-05-10 --to 2026-05-17 --json --select pair_id,a.subject,b.subject,overlap_minutes
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			start, end, err := resolveWindow(from, to, 7)
			if err != nil {
				return usageErr(err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("outlook-calendar-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()

			events, err := loadEvents(cmd.Context(), db.DB(), start, end)
			if err != nil {
				return apiErr(err)
			}

			type pair struct {
				PairID         string       `json:"pair_id"`
				OverlapMinutes int          `json:"overlap_minutes"`
				A              conflictSide `json:"a"`
				B              conflictSide `json:"b"`
			}

			pairs := []pair{}
			for i := 0; i < len(events); i++ {
				si, err := parseGraphTime(events[i].Start)
				if err != nil {
					continue
				}
				ei, err := parseGraphTime(events[i].End)
				if err != nil {
					continue
				}
				for j := i + 1; j < len(events); j++ {
					sj, err := parseGraphTime(events[j].Start)
					if err != nil {
						continue
					}
					ej, err := parseGraphTime(events[j].End)
					if err != nil {
						continue
					}
					// Overlap: max(starts) < min(ends)
					overlapStart := si
					if sj.After(si) {
						overlapStart = sj
					}
					overlapEnd := ei
					if ej.Before(ei) {
						overlapEnd = ej
					}
					if !overlapStart.Before(overlapEnd) {
						continue
					}
					mins := int(overlapEnd.Sub(overlapStart).Minutes())
					if mins <= 0 {
						continue
					}
					pairs = append(pairs, pair{
						PairID:         events[i].ID + ":" + events[j].ID,
						OverlapMinutes: mins,
						A:              eventToSide(events[i]),
						B:              eventToSide(events[j]),
					})
				}
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				return printJSONFiltered(w, pairs, flags)
			}
			if len(pairs) == 0 {
				fmt.Fprintln(w, "No conflicts.")
				return nil
			}
			fmt.Fprintf(w, "%d conflict(s):\n", len(pairs))
			for _, p := range pairs {
				fmt.Fprintf(w, "  %3d min: %s   ⨯   %s\n", p.OverlapMinutes, p.A.Subject, p.B.Subject)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "today", "Start of the window (ISO 8601 / today / tomorrow / +Nd / -Nh)")
	cmd.Flags().StringVar(&to, "to", "+7d", "End of the window (ISO 8601 / +Nd / +Nh)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/outlook-calendar-pp-cli/data.db)")
	return cmd
}

type conflictSide struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Location string `json:"location"`
	ShowAs   string `json:"show_as"`
}

func eventToSide(ev graphEvent) conflictSide {
	return conflictSide{
		ID:       ev.ID,
		Subject:  ev.Subject,
		Start:    ev.Start.DateTime,
		End:      ev.End.DateTime,
		Location: ev.Location,
		ShowAs:   ev.ShowAs,
	}
}

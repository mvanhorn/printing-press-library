// Hand-authored novel command (not generated). Cycle time from the activity
// log: pairs each card's entry into a "from" list with its arrival at a "to"
// list and reports median + p90 of the elapsed duration.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

func newNovelCycletimeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var board string
	var fromList string
	var toList string
	cmd := &cobra.Command{
		Use:         "cycletime",
		Short:       "How long cards take from started to done, with median and p90 per list or label.",
		Long:        "Use this command to quantify where work stalls and set realistic SLAs. It pairs each card's list-enter and list-exit move events from the local activity log and differences the timestamps, which no Trello endpoint exposes.",
		Example:     "  trello-pp-cli cycletime --board Eng --agent\n  trello-pp-cli cycletime --from \"In Progress\" --to Done",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("trello-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'trello-pp-cli trello-sync' first.", err)
			}
			defer db.Close()

			actions, err := loadActions(db)
			if err != nil {
				return err
			}
			boards := nameLookup(db, []string{"boards"})

			// Collect, per card, the time it entered the "from" list and the
			// time it reached the "to" list. A move's listAfter is an "enter".
			type stamps struct{ from, to time.Time }
			perCard := map[string]*stamps{}
			matchList := func(name, want string) bool {
				if want == "" {
					return true
				}
				return strings.Contains(strings.ToLower(name), strings.ToLower(want))
			}
			// Default "to" semantics: a done-like list when --to is empty.
			isDoneName := func(n string) bool {
				l := strings.ToLower(n)
				return strings.Contains(l, "done") || strings.Contains(l, "complete") || strings.Contains(l, "shipped")
			}
			// Sort actions oldest-first so first-enter wins.
			sort.Slice(actions, func(i, j int) bool { return actions[i].Date < actions[j].Date })
			for _, a := range actions {
				if a.Type != "updateCard" || a.Data.ListAfter.Name == "" {
					continue
				}
				if board != "" && !strings.EqualFold(resolve(boards, a.Data.Board.ID), board) && !strings.EqualFold(a.Data.Board.ID, board) {
					continue
				}
				cid := a.Data.Card.ID
				if cid == "" {
					continue
				}
				t, ok := parseTrelloTime(a.Date)
				if !ok {
					continue
				}
				if perCard[cid] == nil {
					perCard[cid] = &stamps{}
				}
				st := perCard[cid]
				// entering the "from" list (or the very first list when --from empty)
				if matchList(a.Data.ListAfter.Name, fromList) && st.from.IsZero() {
					st.from = t
				}
				reachedTo := false
				if toList != "" {
					reachedTo = matchList(a.Data.ListAfter.Name, toList)
				} else {
					reachedTo = isDoneName(a.Data.ListAfter.Name)
				}
				if reachedTo && st.to.IsZero() {
					st.to = t
				}
			}

			durations := make([]float64, 0)
			for _, st := range perCard {
				if st.from.IsZero() || st.to.IsZero() || !st.to.After(st.from) {
					continue
				}
				durations = append(durations, st.to.Sub(st.from).Hours()/24)
			}
			sort.Float64s(durations)

			view := map[string]any{
				"board":       board,
				"from_list":   fromList,
				"to_list":     toList,
				"sample_size": len(durations),
				"median_days": percentile(durations, 0.5),
				"p90_days":    percentile(durations, 0.9),
				"mean_days":   mean(durations),
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(durations) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No completed transitions found. Sync actions with 'trello-pp-cli trello-sync' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Cycle time (n=%d cards):\n", len(durations))
			fmt.Fprintf(cmd.OutOrStdout(), "  median: %.1f days\n  p90:    %.1f days\n  mean:   %.1f days\n",
				percentile(durations, 0.5), percentile(durations, 0.9), mean(durations))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&board, "board", "", "Filter to a single board by name or id")
	cmd.Flags().StringVar(&fromList, "from", "", "Start list name substring (default: first list a card enters)")
	cmd.Flags().StringVar(&toList, "to", "", "End list name substring (default: any done/complete/shipped list)")
	return cmd
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p * float64(len(sorted)-1)
	lo := int(rank)
	if lo >= len(sorted)-1 {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return sorted[lo] + frac*(sorted[lo+1]-sorted[lo])
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

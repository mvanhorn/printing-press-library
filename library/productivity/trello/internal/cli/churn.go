// Hand-authored novel command (not generated). Churn / thrash: cards that move
// backward between lists, detected by sequencing each card's move actions and
// counting non-forward transitions over a window.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

type churnEntry struct {
	CardID  string `json:"card_id"`
	Name    string `json:"name"`
	Board   string `json:"board"`
	Bounces int    `json:"bounces"`
}

func newNovelChurnCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int
	var minBounces int
	cmd := &cobra.Command{
		Use:         "churn",
		Short:       "Cards that bounce backward between lists, revealing rework and unstable requirements.",
		Long:        "Use this command in a retro to find where work keeps getting kicked back. It sequences each card's move events from the local activity log and counts backward transitions, an ordered-event analysis impossible without full local history.",
		Example:     "  trello-pp-cli churn --weeks 4 --agent\n  trello-pp-cli churn --min-bounces 2",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if weeks <= 0 {
				weeks = 4
			}
			if minBounces <= 0 {
				minBounces = 1
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
			cardNames := map[string]string{}
			cardBoard := map[string]string{}
			if cards, err := loadCards(db); err == nil {
				for _, c := range cards {
					cardNames[c.ID] = c.Name
					cardBoard[c.ID] = c.IDBoard
				}
			}

			cutoff := nowUTC().AddDate(0, 0, -7*weeks)
			// Build ordered move sequences per card, with a position index
			// inferred from the order lists first appear (a proxy for board flow).
			sort.Slice(actions, func(i, j int) bool { return actions[i].Date < actions[j].Date })
			listOrder := map[string]int{}
			nextPos := 0
			posOf := func(name string) int {
				if _, ok := listOrder[name]; !ok {
					listOrder[name] = nextPos
					nextPos++
				}
				return listOrder[name]
			}
			bounces := map[string]int{}
			lastPos := map[string]int{}
			for _, a := range actions {
				if a.Type != "updateCard" || a.Data.ListAfter.Name == "" || a.Data.ListBefore.Name == "" {
					continue
				}
				t, ok := parseTrelloTime(a.Date)
				if !ok || t.Before(cutoff) {
					continue
				}
				cid := a.Data.Card.ID
				if cid == "" {
					continue
				}
				beforePos := posOf(a.Data.ListBefore.Name)
				afterPos := posOf(a.Data.ListAfter.Name)
				_ = lastPos
				if afterPos < beforePos {
					bounces[cid]++
				}
			}

			entries := make([]churnEntry, 0)
			for cid, b := range bounces {
				if b < minBounces {
					continue
				}
				name := cardNames[cid]
				if name == "" {
					name = cid
				}
				entries = append(entries, churnEntry{
					CardID: cid, Name: name, Board: resolve(boards, cardBoard[cid]), Bounces: b,
				})
			}
			sortByCountDesc(entries, func(e churnEntry) int { return e.Bounces })

			view := map[string]any{
				"weeks_window": weeks,
				"min_bounces":  minBounces,
				"churn_count":  len(entries),
				"cards":        entries,
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No churning cards found. Sync actions with 'trello-pp-cli trello-sync' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Churn (last %d weeks, min %d bounces):\n\n", weeks, minBounces)
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-40s %s\n", "BOUNCES", "CARD", "BOARD")
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-8d %-40s %s\n", e.Bounces, truncate(e.Name, 38), truncate(e.Board, 18))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&weeks, "weeks", 4, "Number of weeks to look back")
	cmd.Flags().IntVar(&minBounces, "min-bounces", 1, "Minimum backward moves to report a card")
	return cmd
}

var _ = strings.TrimSpace

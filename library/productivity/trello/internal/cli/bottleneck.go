// Hand-authored novel command (not generated). Bottleneck / WIP-aging: which
// list holds the most cards and how long they have aged in place, computed by
// joining current card->list state with the last move action per card.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

type bottleneckEntry struct {
	List       string  `json:"list"`
	Board      string  `json:"board"`
	CardCount  int     `json:"card_count"`
	AvgAgeDays float64 `json:"avg_age_days"`
	MaxAgeDays float64 `json:"max_age_days"`
	OverThresh int     `json:"over_threshold"`
}

func newNovelBottleneckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var board string
	var threshold string
	cmd := &cobra.Command{
		Use:         "bottleneck",
		Short:       "Which list is clogged now, by card count and how long cards have aged in place.",
		Long:        "Use this command in standup to name the exact stage starving throughput. It joins each open card's current list to the timestamp of the action that placed it there, then aggregates time-in-state per list, which is not a stored Trello field.",
		Example:     "  trello-pp-cli bottleneck --board Eng --agent\n  trello-pp-cli bottleneck --threshold 5d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			thresh := 5 * 24 * time.Hour
			if threshold != "" {
				if d, err := cliutil.ParseDurationLoose(threshold); err == nil {
					thresh = d
				} else {
					return usageErr(fmt.Errorf("invalid --threshold %q: %w", threshold, err))
				}
			}
			if dbPath == "" {
				dbPath = defaultDBPath("trello-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'trello-pp-cli sync' first.", err)
			}
			defer db.Close()

			cards, err := loadCards(db)
			if err != nil {
				return err
			}
			actions, err := loadActions(db)
			if err != nil {
				return err
			}
			boards := nameLookup(db, []string{"boards"})
			lists := nameLookup(db, []string{"lists", "boards_lists"})

			// Latest "entered list" timestamp per card.
			enteredAt := map[string]time.Time{}
			for _, a := range actions {
				if a.Type != "updateCard" || a.Data.ListAfter.Name == "" {
					continue
				}
				cid := a.Data.Card.ID
				t, ok := parseTrelloTime(a.Date)
				if cid == "" || !ok {
					continue
				}
				if prev, seen := enteredAt[cid]; !seen || t.After(prev) {
					enteredAt[cid] = t
				}
			}

			now := nowUTC()
			type agg struct {
				board  string
				count  int
				sumAge float64
				maxAge float64
				over   int
			}
			byList := map[string]*agg{}
			for _, c := range cards {
				if c.Closed {
					continue
				}
				if board != "" && !strings.EqualFold(resolve(boards, c.IDBoard), board) && !strings.EqualFold(c.IDBoard, board) {
					continue
				}
				listName := resolve(lists, c.IDList)
				if byList[listName] == nil {
					byList[listName] = &agg{board: resolve(boards, c.IDBoard)}
				}
				g := byList[listName]
				g.count++
				// age = time since entered list, falling back to dateLastActivity.
				var age time.Duration
				if t, ok := enteredAt[c.ID]; ok {
					age = now.Sub(t)
				} else if t, ok := parseTrelloTime(c.DateLastActivity); ok {
					age = now.Sub(t)
				}
				days := age.Hours() / 24
				g.sumAge += days
				if days > g.maxAge {
					g.maxAge = days
				}
				if age >= thresh {
					g.over++
				}
			}

			entries := make([]bottleneckEntry, 0, len(byList))
			for name, g := range byList {
				avg := 0.0
				if g.count > 0 {
					avg = g.sumAge / float64(g.count)
				}
				entries = append(entries, bottleneckEntry{
					List: name, Board: g.board, CardCount: g.count,
					AvgAgeDays: avg, MaxAgeDays: g.maxAge, OverThresh: g.over,
				})
			}
			// Rank by cards over the aging threshold, then by count.
			sort.Slice(entries, func(i, j int) bool {
				if entries[i].OverThresh != entries[j].OverThresh {
					return entries[i].OverThresh > entries[j].OverThresh
				}
				return entries[i].CardCount > entries[j].CardCount
			})

			view := map[string]any{
				"board":      board,
				"threshold":  threshold,
				"list_count": len(entries),
				"lists":      entries,
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open cards found. Run 'trello-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Bottlenecks (threshold %s):\n\n", threshold)
			fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-18s %-6s %-9s %s\n", "LIST", "BOARD", "CARDS", "AVG-AGE", "OVER")
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-18s %-6d %-9.1f %d\n", truncate(e.List, 22), truncate(e.Board, 16), e.CardCount, e.AvgAgeDays, e.OverThresh)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&board, "board", "", "Filter to a single board by name or id")
	cmd.Flags().StringVar(&threshold, "threshold", "5d", "Aging threshold (e.g. 3d, 1w)")
	return cmd
}

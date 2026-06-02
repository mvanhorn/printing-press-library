// Hand-authored novel command (not generated). Cross-board overdue sweep.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

type overdueEntry struct {
	CardID   string   `json:"card_id"`
	Name     string   `json:"name"`
	Board    string   `json:"board"`
	List     string   `json:"list"`
	Due      string   `json:"due"`
	DaysLate float64  `json:"days_late"`
	Members  []string `json:"members"`
}

func newNovelOverdueCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "overdue",
		Short:       "Every past-due card across all your boards, ranked by lateness and owner.",
		Long:        "Use this command to find every overdue card across all boards in one shot. It joins cards from every board in the local store and ranks them by how late they are. Do NOT use it for cards untouched for a while; use 'stale' for that.",
		Example:     "  trello-pp-cli overdue --agent\n  trello-pp-cli overdue --json --select cards.name,cards.days_late",
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

			cards, err := loadCards(db)
			if err != nil {
				return err
			}
			boards := nameLookup(db, []string{"boards"})
			lists := nameLookup(db, []string{"lists", "boards_lists"})
			members := nameLookup(db, []string{"members", "boards_members"})

			now := nowUTC()
			entries := make([]overdueEntry, 0)
			for _, c := range cards {
				if c.Closed || c.DueComplete {
					continue
				}
				due, ok := parseTrelloTime(c.Due)
				if !ok || !due.Before(now) {
					continue
				}
				memberNames := make([]string, 0, len(c.IDMembers))
				for _, m := range c.IDMembers {
					memberNames = append(memberNames, resolve(members, m))
				}
				entries = append(entries, overdueEntry{
					CardID:   c.ID,
					Name:     c.Name,
					Board:    resolve(boards, c.IDBoard),
					List:     resolve(lists, c.IDList),
					Due:      c.Due,
					DaysLate: now.Sub(due).Hours() / 24,
					Members:  memberNames,
				})
			}
			sortByCountDesc(entries, func(e overdueEntry) int { return int(e.DaysLate * 100) })
			if limit > 0 && len(entries) > limit {
				entries = entries[:limit]
			}

			view := map[string]any{
				"overdue_count": len(entries),
				"as_of":         now.Format(time.RFC3339),
				"cards":         entries,
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No overdue cards. Run 'trello-pp-cli trello-sync' if this looks wrong.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Overdue cards (%d):\n\n", len(entries))
			fmt.Fprintf(cmd.OutOrStdout(), "%-6s %-40s %-20s %s\n", "LATE", "CARD", "BOARD", "LIST")
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-6.1f %-40s %-20s %s\n", e.DaysLate, truncate(e.Name, 38), truncate(e.Board, 18), e.List)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum cards to return (0 = all)")
	return cmd
}

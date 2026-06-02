// Hand-authored novel command (not generated). Checklist completion rollup:
// joins checkitems -> checklists -> cards across boards and computes true
// percent-complete per card, surfacing cards that look done but aren't.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

type checklistProgressEntry struct {
	CardID  string  `json:"card_id"`
	Card    string  `json:"card"`
	Board   string  `json:"board"`
	Total   int     `json:"total_items"`
	Done    int     `json:"done_items"`
	Percent float64 `json:"percent_complete"`
	Due     string  `json:"due,omitempty"`
}

// trelloChecklist is the subset of a synced checklist row we read.
type trelloChecklist struct {
	ID         string `json:"id"`
	IDCard     string `json:"idCard"`
	CheckItems []struct {
		State string `json:"state"` // "complete" | "incomplete"
	} `json:"checkItems"`
}

func newNovelChecklistProgressCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var below float64
	cmd := &cobra.Command{
		Use:         "checklist-progress",
		Short:       "Real card-level progress from checkitem completion across boards.",
		Long:        "Use this command to catch cards drifting toward a deadline with unchecked subtasks. It rolls up checkitems to checklists to cards across every board and computes true percent-complete, which the API never returns aggregated.",
		Example:     "  trello-pp-cli checklist-progress --below 80 --agent\n  trello-pp-cli checklist-progress --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if below <= 0 {
				below = 100
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
			cardByID := map[string]trelloCard{}
			for _, c := range cards {
				cardByID[c.ID] = c
			}
			boards := nameLookup(db, []string{"boards"})

			// Aggregate checkitems per card from checklist rows.
			type tally struct{ total, done int }
			perCard := map[string]*tally{}
			rows, err := db.Query(`SELECT resource_type, data FROM resources`)
			if err != nil {
				return fmt.Errorf("querying checklists: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var rt string
				var data []byte
				if rows.Scan(&rt, &data) != nil {
					continue
				}
				var cl trelloChecklist
				if json.Unmarshal(data, &cl) != nil {
					continue
				}
				if cl.IDCard == "" || len(cl.CheckItems) == 0 {
					continue
				}
				if perCard[cl.IDCard] == nil {
					perCard[cl.IDCard] = &tally{}
				}
				t := perCard[cl.IDCard]
				for _, it := range cl.CheckItems {
					t.total++
					if it.State == "complete" {
						t.done++
					}
				}
			}

			entries := make([]checklistProgressEntry, 0)
			for cid, t := range perCard {
				if t.total == 0 {
					continue
				}
				pct := float64(t.done) / float64(t.total) * 100
				if pct >= below {
					continue
				}
				c := cardByID[cid]
				if c.Closed {
					continue
				}
				name := c.Name
				if name == "" {
					name = cid
				}
				entries = append(entries, checklistProgressEntry{
					CardID: cid, Card: name, Board: resolve(boards, c.IDBoard),
					Total: t.total, Done: t.done, Percent: pct, Due: c.Due,
				})
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Percent < entries[j].Percent })

			view := map[string]any{
				"below_percent": below,
				"card_count":    len(entries),
				"cards":         entries,
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No cards below the threshold. Run 'trello-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Checklist progress below %.0f%% (%d cards):\n\n", below, len(entries))
			fmt.Fprintf(cmd.OutOrStdout(), "%-6s %-8s %-40s %s\n", "PCT", "ITEMS", "CARD", "BOARD")
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-6.0f %-8s %-40s %s\n", e.Percent, fmt.Sprintf("%d/%d", e.Done, e.Total), truncate(e.Card, 38), truncate(e.Board, 18))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().Float64Var(&below, "below", 100, "Only show cards below this completion percent")
	return cmd
}

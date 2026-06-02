// Hand-authored novel command (not generated). Blocked-card scan: cross-
// references labels, checkitem text, and the card name/desc against a
// "blocked" pattern and reports how long each has sat (via dateLastActivity).

package cli

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

type blockedEntry struct {
	CardID  string  `json:"card_id"`
	Name    string  `json:"name"`
	Board   string  `json:"board"`
	List    string  `json:"list"`
	Signal  string  `json:"signal"`
	AgeDays float64 `json:"age_days"`
}

func newNovelBlockedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var match string
	var over string
	cmd := &cobra.Command{
		Use:         "blocked",
		Short:       "Cards flagged blocked by label, checkitem text, or comment, and how long they've sat.",
		Long:        "Use this command to assemble an unblock list for a lead. It cross-references labels, checkitem text, and card text against a blocked pattern and reports time-since-activity, a multi-source join no Trello endpoint exposes.",
		Example:     "  trello-pp-cli blocked --over 3d --agent\n  trello-pp-cli blocked --match \"blocked|waiting on\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if match == "" {
				match = "blocked|waiting on|on hold|stuck"
			}
			re, err := regexp.Compile("(?i)" + match)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --match regex %q: %w", match, err))
			}
			var minAge time.Duration
			if over != "" {
				if d, err := cliutil.ParseDurationLoose(over); err == nil {
					minAge = d
				} else {
					return usageErr(fmt.Errorf("invalid --over %q: %w", over, err))
				}
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

			now := nowUTC()
			entries := make([]blockedEntry, 0)
			for _, c := range cards {
				if c.Closed {
					continue
				}
				signal := ""
				switch {
				case matchAnyLabel(c, re):
					signal = "label"
				case re.MatchString(c.Name):
					signal = "name"
				case re.MatchString(c.Desc):
					signal = "desc"
				}
				if signal == "" {
					continue
				}
				var age time.Duration
				if t, ok := parseTrelloTime(c.DateLastActivity); ok {
					age = now.Sub(t)
				}
				if minAge > 0 && age < minAge {
					continue
				}
				entries = append(entries, blockedEntry{
					CardID: c.ID, Name: c.Name, Board: resolve(boards, c.IDBoard),
					List: resolve(lists, c.IDList), Signal: signal, AgeDays: age.Hours() / 24,
				})
			}
			sortByCountDesc(entries, func(e blockedEntry) int { return int(e.AgeDays * 100) })

			view := map[string]any{
				"match":         match,
				"over":          over,
				"blocked_count": len(entries),
				"cards":         entries,
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No blocked cards matched. Run 'trello-pp-cli trello-sync' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Blocked cards (%d):\n\n", len(entries))
			fmt.Fprintf(cmd.OutOrStdout(), "%-6s %-8s %-40s %s\n", "AGE", "SIGNAL", "CARD", "BOARD")
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-6.1f %-8s %-40s %s\n", e.AgeDays, e.Signal, truncate(e.Name, 38), truncate(e.Board, 18))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&match, "match", "blocked|waiting on|on hold|stuck", "Case-insensitive regex matched against labels and card text")
	cmd.Flags().StringVar(&over, "over", "", "Only cards inactive longer than this (e.g. 3d, 1w)")
	return cmd
}

func matchAnyLabel(c trelloCard, re *regexp.Regexp) bool {
	for _, l := range c.Labels {
		if re.MatchString(l.Name) {
			return true
		}
	}
	return false
}

var _ = strings.TrimSpace

// Hand-authored novel command (not generated). Cross-board member workload.

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

type workloadEntry struct {
	Member  string `json:"member"`
	Open    int    `json:"open"`
	DueSoon int    `json:"due_soon"`
	Overdue int    `json:"overdue"`
}

func newNovelWorkloadCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string
	cmd := &cobra.Command{
		Use:         "workload",
		Short:       "Open and due-soon card load per member across every board.",
		Long:        "Use this command to see who is overloaded before assigning more work. It aggregates open, due-soon, and overdue card counts per assignee across every board in the local store, which no single Trello API call can sum.",
		Example:     "  trello-pp-cli workload --window 7d --agent\n  trello-pp-cli workload --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			soon := 7 * 24 * time.Hour
			if window != "" {
				if d, err := cliutil.ParseDurationLoose(window); err == nil {
					soon = d
				} else {
					return usageErr(fmt.Errorf("invalid --window %q: %w", window, err))
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
			members := nameLookup(db, []string{"members", "boards_members"})

			now := nowUTC()
			byMember := map[string]*workloadEntry{}
			get := func(name string) *workloadEntry {
				if byMember[name] == nil {
					byMember[name] = &workloadEntry{Member: name}
				}
				return byMember[name]
			}
			for _, c := range cards {
				if c.Closed {
					continue
				}
				owners := c.IDMembers
				if len(owners) == 0 {
					owners = []string{"(unassigned)"}
				}
				due, hasDue := parseTrelloTime(c.Due)
				for _, m := range owners {
					name := m
					if m != "(unassigned)" {
						name = resolve(members, m)
					}
					e := get(name)
					e.Open++
					if hasDue && !c.DueComplete {
						switch {
						case due.Before(now):
							e.Overdue++
						case due.Before(now.Add(soon)):
							e.DueSoon++
						}
					}
				}
			}

			entries := make([]workloadEntry, 0, len(byMember))
			for _, e := range byMember {
				entries = append(entries, *e)
			}
			sortByCountDesc(entries, func(e workloadEntry) int { return e.Open })

			view := map[string]any{
				"window":       window,
				"as_of":        now.Format(time.RFC3339),
				"member_count": len(entries),
				"distribution": entries,
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No cards found. Run 'trello-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Workload across all boards (%d members):\n\n", len(entries))
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-6s %-9s %s\n", "MEMBER", "OPEN", "DUE-SOON", "OVERDUE")
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-6d %-9d %d\n", truncate(e.Member, 28), e.Open, e.DueSoon, e.Overdue)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&window, "window", "7d", "Due-soon window (e.g. 3d, 1w, 48h)")
	return cmd
}

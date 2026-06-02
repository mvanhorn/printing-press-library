// Hand-authored command (not generated). Trello-specific store hydration.
//
// The generic `sync` command can't enumerate a Trello account because Trello
// has no top-level bulk-list endpoints — boards live under /members/{id}/boards
// and cards under /boards/{id}/cards. This command does the correct traversal
// (member -> boards -> lists/cards/members/actions) and writes every entity into
// the local SQLite store so the cross-board analytics commands (overdue,
// workload, velocity, cycletime, bottleneck, blocked, churn, checklist-progress)
// have data to work on.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/trello/internal/store"
	"github.com/spf13/cobra"
)

func newTrelloSyncCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var filter string
	var actionsLimit int
	cmd := &cobra.Command{
		Use:   "trello-sync",
		Short: "Mirror your Trello boards, lists, cards, members, and activity into the local store.",
		Long: `Hydrate the local SQLite store for offline search and the cross-board
analytics commands. Traverses your account the way the Trello API requires:
member -> boards -> (lists, cards, members, recent actions). Run this before
overdue, workload, velocity, cycletime, bottleneck, blocked, churn, or
checklist-progress.`,
		Example:     "  trello-pp-cli trello-sync\n  trello-pp-cli trello-sync --filter open --actions-limit 200",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would mirror boards/lists/cards/members/actions into the local store")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("trello-pp-cli")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			// 1. Boards for the authenticated member.
			boardsRaw, err := c.Get(ctx, "/members/me/boards", map[string]string{
				"filter": filter,
				"fields": "name,desc,closed,dateLastActivity,idOrganization",
			})
			if err != nil {
				return apiErr(fmt.Errorf("fetching boards: %w", err))
			}
			var boards []map[string]any
			if err := json.Unmarshal(boardsRaw, &boards); err != nil {
				return fmt.Errorf("parsing boards: %w", err)
			}

			counts := map[string]int{}
			upsertEach := func(resourceType string, raw json.RawMessage) {
				var items []json.RawMessage
				if json.Unmarshal(raw, &items) != nil {
					return
				}
				if len(items) == 0 {
					return
				}
				stored, _, _ := db.UpsertBatch(resourceType, items)
				counts[resourceType] += stored
			}

			for _, b := range boards {
				id, _ := b["id"].(string)
				if id == "" {
					continue
				}
				if bj, err := json.Marshal(b); err == nil {
					if db.Upsert("boards", id, bj) == nil {
						counts["boards"]++
					}
				}

				// Cards for this board (with members, labels, due, list).
				if cardsRaw, err := c.Get(ctx, "/boards/"+id+"/cards", map[string]string{
					"fields":  "name,desc,closed,due,dueComplete,dateLastActivity,idList,idBoard,idMembers,idLabels,labels",
					"members": "true",
				}); err == nil {
					upsertEach("cards", cardsRaw)
				}

				// Lists for this board (for name resolution).
				if listsRaw, err := c.Get(ctx, "/boards/"+id+"/lists", map[string]string{
					"fields": "name,closed,idBoard,pos",
				}); err == nil {
					upsertEach("lists", listsRaw)
				}

				// Members of this board.
				if membersRaw, err := c.Get(ctx, "/boards/"+id+"/members", map[string]string{
					"fields": "fullName,username",
				}); err == nil {
					upsertEach("members", membersRaw)
				}

				// Checklists for this board (with checkItems) so
				// checklist-progress can roll up completion per card.
				if checklistsRaw, err := c.Get(ctx, "/boards/"+id+"/checklists", map[string]string{
					"fields":           "name,idCard",
					"checkItems":       "all",
					"checkItem_fields": "name,state",
				}); err == nil {
					upsertEach("checklists", checklistsRaw)
				}

				// Recent actions (activity log) for velocity/cycletime/bottleneck/churn.
				if actionsLimit > 0 {
					if actionsRaw, err := c.Get(ctx, "/boards/"+id+"/actions", map[string]string{
						"filter": "updateCard,createCard,updateCheckItemStateOnCard",
						"limit":  fmt.Sprintf("%d", actionsLimit),
					}); err == nil {
						upsertEach("actions", actionsRaw)
					}
				}
			}

			// Record sync state so framework commands (stale, search, doctor)
			// and sync-hints recognize the store as hydrated.
			for _, rt := range []string{"boards", "cards", "lists", "members", "actions", "checklists"} {
				_ = db.SaveSyncState(rt, "", counts[rt])
			}

			summary := map[string]any{
				"event":   "trello_sync_summary",
				"boards":  counts["boards"],
				"cards":   counts["cards"],
				"lists":   counts["lists"],
				"members": counts["members"],
				"actions": counts["actions"],
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(out, summary, flags)
			}
			fmt.Fprintf(out, "Synced %d boards, %d cards, %d lists, %d members, %d actions into the local store.\n",
				counts["boards"], counts["cards"], counts["lists"], counts["members"], counts["actions"])
			fmt.Fprintln(out, "Now try: trello-pp-cli overdue   |   trello-pp-cli workload")
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&filter, "filter", "open", "Board filter: open, closed, all")
	cmd.Flags().IntVar(&actionsLimit, "actions-limit", 100, "Recent actions to pull per board (0 to skip; needed for velocity/cycletime/churn)")
	return cmd
}

// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: GTD next actions. Reframes the toodledo-mcp gtd_next_actions
// tool as an offline read over the local SQLite mirror.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelNextActionsCmd(flags *rootFlags) *cobra.Command {
	var flagContext string
	var flagGoal string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "next-actions",
		Short: "GTD 'what should I do now?' — incomplete Next-Action tasks, offline",
		Long: `List your GTD Next Actions (status = Next Action, incomplete) from the local
mirror, sorted by priority then due date. Optionally scope to a context or goal.

Reads the local SQLite mirror only (no API calls). Run 'sync' first to populate it.`,
		Example: strings.Trim(`
  toodledo-pp-cli next-actions
  toodledo-pp-cli next-actions --context @work --agent
  toodledo-pp-cli next-actions --goal "Ship v1" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath = toodledoDBPath(dbPath)
			db, ok, err := openLocalMirror(cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				emitMissingMirror(cmd, flags, dbPath)
				return nil
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "tasks") {
				hintIfStale(cmd, db, "tasks", flags.maxAge)
			}

			folders := loadNameMap(db, "folders")
			contexts := loadNameMap(db, "contexts")
			goals := loadNameMap(db, "goals")

			where := []string{"COALESCE(completed,0)=0", "status=?"}
			argv := []any{statusNextAction}
			if flagContext != "" {
				id, found, _ := resolveRefID(db, "contexts", flagContext)
				if !found {
					return usageErr(fmt.Errorf("context %q not found. available: %s", flagContext, strings.Join(availableNames(db, "contexts"), ", ")))
				}
				where = append(where, "context=?")
				argv = append(argv, id)
			}
			if flagGoal != "" {
				id, found, _ := resolveRefID(db, "goals", flagGoal)
				if !found {
					return usageErr(fmt.Errorf("goal %q not found. available: %s", flagGoal, strings.Join(availableNames(db, "goals"), ", ")))
				}
				where = append(where, "goal=?")
				argv = append(argv, id)
			}

			query := `SELECT ` + taskSelectColumns + ` FROM "tasks" WHERE ` + strings.Join(where, " AND ") +
				` ORDER BY priority DESC, CASE WHEN duedate>0 THEN duedate ELSE 99999999999 END ASC`
			if limit > 0 {
				query += " LIMIT " + strconv.Itoa(limit)
			}
			rows, err := scanTaskRows(db, query, argv, folders, contexts, goals)
			if err != nil {
				return err
			}
			return renderTaskList(cmd, flags, "Next Actions", rows)
		},
	}
	cmd.Flags().StringVar(&flagContext, "context", "", "Filter to a context (name or id), e.g. @work")
	cmd.Flags().StringVar(&flagGoal, "goal", "", "Filter to a goal (name or id)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max tasks to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	return cmd
}

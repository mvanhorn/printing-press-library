// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: GTD weekly review. Five offline aggregations over the local
// mirror in one pass — transcends the toodledo-mcp gtd_review tool, which
// recomputed the same buckets from a live full fetch every call.

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type reviewResult struct {
	Inbox           []taskRow        `json:"inbox"`
	Overdue         []taskRow        `json:"overdue"`
	StalledProjects []stalledProject `json:"stalled_projects"`
	Waiting         []taskRow        `json:"waiting"`
	Someday         []taskRow        `json:"someday"`
}

// pp:data-source local
func newNovelReviewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Full GTD weekly review (inbox, overdue, stalled, waiting, someday) — offline",
		Long: `Run the entire GTD weekly review from the local mirror in one pass:
  - Inbox: incomplete tasks with no folder and no context (untriaged)
  - Overdue: incomplete tasks past their due date
  - Stalled Projects: folders with open tasks but no Next Action
  - Waiting For: tasks in the Waiting status
  - Someday/Maybe: tasks in the Someday status

Reads the local SQLite mirror only (no API calls). Run 'sync' first.`,
		Example: strings.Trim(`
  toodledo-pp-cli review
  toodledo-pp-cli review --agent --select overdue.title,overdue.duedate,stalled_projects.folder`, "\n"),
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
			openTasks, err := scanTaskRows(db,
				`SELECT `+taskSelectColumns+` FROM "tasks" WHERE COALESCE(completed,0)=0 `+
					`ORDER BY priority DESC, CASE WHEN duedate>0 THEN duedate ELSE 99999999999 END ASC`,
				nil, folders, contexts, goals)
			if err != nil {
				return err
			}
			todayStart := startOfTodayUnix()
			res := reviewResult{Inbox: []taskRow{}, Overdue: []taskRow{}, Waiting: []taskRow{}, Someday: []taskRow{}}
			for _, t := range openTasks {
				if t.Folder == 0 && t.Context == 0 {
					res.Inbox = append(res.Inbox, t)
				}
				if t.Due > 0 && t.Due < todayStart {
					res.Overdue = append(res.Overdue, t)
				}
				if t.Status == statusWaiting {
					res.Waiting = append(res.Waiting, t)
				}
				if t.Status == statusSomeday {
					res.Someday = append(res.Someday, t)
				}
			}
			res.StalledProjects = computeStalledProjects(openTasks, folders)

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, bold("GTD Weekly Review"))
			printReviewBucket(w, "Inbox (untriaged)", res.Inbox)
			printReviewBucket(w, "Overdue", res.Overdue)
			fmt.Fprintf(w, "\nStalled Projects (%d)\n", len(res.StalledProjects))
			if len(res.StalledProjects) == 0 {
				fmt.Fprintln(w, "  (none)")
			} else {
				for _, s := range res.StalledProjects {
					name := s.Folder
					if name == "" {
						name = fmt.Sprintf("folder:%d", s.FolderID)
					}
					fmt.Fprintf(w, "  - %s (%d open, no next action)\n", name, s.OpenTasks)
				}
			}
			printReviewBucket(w, "Waiting For", res.Waiting)
			printReviewBucket(w, "Someday/Maybe", res.Someday)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	return cmd
}

func printReviewBucket(w io.Writer, title string, rows []taskRow) {
	fmt.Fprintf(w, "\n%s (%d)\n", title, len(rows))
	if len(rows) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	for _, r := range rows {
		due := r.DueISO
		if due == "" {
			due = "-"
		}
		fmt.Fprintf(w, "  [%s] %s  (%s, due %s)\n", r.ID, truncate(r.Title, 60), priorityLabel(r.Priority), due)
	}
}

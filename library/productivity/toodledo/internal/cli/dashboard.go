// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: GTD dashboard. A one-screen multi-axis status board computed
// in a single offline pass over the local mirror. Preserves the toodledo-mcp
// gtd_dashboard tool; distinct from single-axis `analytics --group-by`.

package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"
)

type countRow struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type dashboardResult struct {
	TotalIncomplete int        `json:"total_incomplete"`
	Overdue         int        `json:"overdue"`
	DueToday        int        `json:"due_today"`
	Starred         int        `json:"starred"`
	ByStatus        []countRow `json:"by_status"`
	ByPriority      []countRow `json:"by_priority"`
	ByFolder        []countRow `json:"by_folder"`
	ByContext       []countRow `json:"by_context"`
}

// pp:data-source local
func newNovelDashboardCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "One-screen status board: counts by status, priority, folder, context (+overdue/today/starred)",
		Long: `A multi-axis status board over the local mirror: incomplete-task counts by
status, priority, folder, and context, plus overdue, due-today, and starred
totals — all computed in one offline pass.

For a single grouped count, use 'analytics --type tasks --group-by <field>'.`,
		Example:     "  toodledo-pp-cli dashboard --json",
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
			openTasks, err := scanTaskRows(db, `SELECT `+taskSelectColumns+` FROM "tasks" WHERE COALESCE(completed,0)=0`, nil, folders, contexts, nil)
			if err != nil {
				return err
			}

			todayStart := startOfTodayUnix()
			todayEnd := todayStart + 86400

			res := dashboardResult{TotalIncomplete: len(openTasks)}
			statusCounts := map[int]int{}
			prioCounts := map[int]int{}
			folderCounts := map[int]int{}
			contextCounts := map[int]int{}
			for _, t := range openTasks {
				statusCounts[t.Status]++
				prioCounts[t.Priority]++
				folderCounts[t.Folder]++
				contextCounts[t.Context]++
				if t.Star == 1 {
					res.Starred++
				}
				if t.Due > 0 && t.Due < todayStart {
					res.Overdue++
				}
				if t.Due >= todayStart && t.Due < todayEnd {
					res.DueToday++
				}
			}
			res.ByStatus = labeledCounts(statusCounts, statusLabel, false)
			res.ByPriority = labeledCounts(prioCounts, priorityLabel, false)
			res.ByFolder = topLabeledCounts(folderCounts, folders, "(no folder)", 15)
			res.ByContext = topLabeledCounts(contextCounts, contexts, "(no context)", 15)

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, bold("Task Dashboard"))
			fmt.Fprintf(w, "Total incomplete: %d   Overdue: %d   Due today: %d   Starred: %d\n",
				res.TotalIncomplete, res.Overdue, res.DueToday, res.Starred)
			printCountSection(w, "By Status", res.ByStatus)
			printCountSection(w, "By Priority", res.ByPriority)
			printCountSection(w, "By Folder", res.ByFolder)
			printCountSection(w, "By Context", res.ByContext)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	return cmd
}

// labeledCounts converts an int-keyed count map to sorted countRows using a
// label function. When byKey is false, rows are sorted by count descending.
func labeledCounts(counts map[int]int, label func(int) string, byKey bool) []countRow {
	out := make([]countRow, 0, len(counts))
	for k, c := range counts {
		out = append(out, countRow{Label: label(k), Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// topLabeledCounts converts an id-keyed count map to the top-N rows, resolving
// ids to names (id 0 -> noneLabel).
func topLabeledCounts(counts map[int]int, names map[int]string, noneLabel string, top int) []countRow {
	out := make([]countRow, 0, len(counts))
	for id, c := range counts {
		label := noneLabel
		if id != 0 {
			if n, ok := names[id]; ok && n != "" {
				label = n
			} else {
				label = fmt.Sprintf("id:%d", id)
			}
		}
		out = append(out, countRow{Label: label, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Label < out[j].Label
	})
	if top > 0 && len(out) > top {
		out = out[:top]
	}
	return out
}

func printCountSection(w io.Writer, title string, rows []countRow) {
	fmt.Fprintf(w, "\n%s\n", title)
	if len(rows) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	tw := newTabWriter(w)
	for _, r := range rows {
		fmt.Fprintf(tw, "  %s\t%d\n", r.Label, r.Count)
	}
	_ = tw.Flush()
}

// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: stalled projects. A folders-to-tasks anti-join over the local
// mirror that surfaces the GTD "project with no next action" failure mode.
// computeStalledProjects is shared with the weekly review command.

package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

type stalledProject struct {
	FolderID  int    `json:"folder_id"`
	Folder    string `json:"folder"`
	OpenTasks int    `json:"open_tasks"`
}

// computeStalledProjects returns folders that have at least one open task but
// zero Next Actions (status=1). Input is the full set of incomplete tasks.
func computeStalledProjects(openTasks []taskRow, folders map[int]string) []stalledProject {
	open := map[int]int{}
	hasNA := map[int]bool{}
	for _, t := range openTasks {
		if t.Folder == 0 {
			continue
		}
		open[t.Folder]++
		if t.Status == statusNextAction {
			hasNA[t.Folder] = true
		}
	}
	out := make([]stalledProject, 0)
	for fid, cnt := range open {
		if hasNA[fid] {
			continue
		}
		out = append(out, stalledProject{FolderID: fid, Folder: folders[fid], OpenTasks: cnt})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].OpenTasks != out[j].OpenTasks {
			return out[i].OpenTasks > out[j].OpenTasks
		}
		return out[i].Folder < out[j].Folder
	})
	return out
}

// pp:data-source local
func newNovelStalledProjectsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "stalled-projects",
		Short: "Projects (folders) with open tasks but no Next Action",
		Long: `Surface folders that have incomplete tasks but no task marked 'Next Action' —
the GTD failure mode where a project silently stalls because nothing actionable
is defined. Reads the local SQLite mirror only.`,
		Example:     "  toodledo-pp-cli stalled-projects --json",
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
			openTasks, err := scanTaskRows(db, `SELECT `+taskSelectColumns+` FROM "tasks" WHERE COALESCE(completed,0)=0`, nil, folders, nil, nil)
			if err != nil {
				return err
			}
			stalled := computeStalledProjects(openTasks, folders)
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), stalled, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Stalled Projects (%d)\n", len(stalled))
			if len(stalled) == 0 {
				fmt.Fprintln(w, "  (none — every project with open tasks has a Next Action)")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, bold("OPEN\tPROJECT"))
			for _, s := range stalled {
				name := s.Folder
				if name == "" {
					name = "folder:" + strconv.Itoa(s.FolderID)
				}
				fmt.Fprintf(tw, "%d\t%s\n", s.OpenTasks, name)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	return cmd
}

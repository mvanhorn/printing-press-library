// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature: goal progress rollup. Joins tasks to goals and walks the
// `contributes` self-reference to roll child goals into their parents — a
// recursive cross-entity query only the local mirror makes possible.

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/toodledo/internal/store"
	"github.com/spf13/cobra"
)

type goalRow struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	LevelLabel  string `json:"level_label"`
	Contributes int    `json:"contributes"`
	DirectOpen  int    `json:"direct_open"`
	DirectDone  int    `json:"direct_done"`
	RolledOpen  int    `json:"rolled_open"`
	RolledDone  int    `json:"rolled_done"`
}

type ocCount struct{ open, done int }

// pp:data-source local
func newNovelGoalProgressCmd(flags *rootFlags) *cobra.Command {
	var flagLevel string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "goal-progress",
		Short: "Per-goal task progress, rolled up the lifetime/long-term/short-term hierarchy",
		Long: `For each goal, count contributing tasks (open vs done) and roll child goals'
counts up into their parent via the 'contributes' link. Reads the local mirror
only. By default the mirror holds incomplete tasks; run 'sync --param comp=-1'
to include completed tasks in the done counts.`,
		Example: strings.Trim(`
  toodledo-pp-cli goal-progress
  toodledo-pp-cli goal-progress --level short --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			levelFilter := -1
			if strings.TrimSpace(flagLevel) != "" {
				v, ok := parseGoalLevel(flagLevel)
				if !ok {
					return usageErr(fmt.Errorf("invalid --level %q (use lifetime, long-term, short-term, or 0/1/2)", flagLevel))
				}
				levelFilter = v
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
			if !hintIfUnsynced(cmd, db, "goals") {
				hintIfStale(cmd, db, "goals", flags.maxAge)
			}

			goals := loadGoalRows(db)
			direct := loadGoalTaskCounts(db)
			children := map[int][]int{}
			byID := map[int]*goalRow{}
			for i := range goals {
				goals[i].DirectOpen = direct[goals[i].ID].open
				goals[i].DirectDone = direct[goals[i].ID].done
				byID[goals[i].ID] = &goals[i]
				if goals[i].Contributes != 0 {
					children[goals[i].Contributes] = append(children[goals[i].Contributes], goals[i].ID)
				}
			}
			var roll func(id int, seen map[int]bool) (int, int)
			roll = func(id int, seen map[int]bool) (int, int) {
				if seen[id] {
					return 0, 0
				}
				seen[id] = true
				g := byID[id]
				if g == nil {
					return 0, 0
				}
				o, d := g.DirectOpen, g.DirectDone
				for _, c := range children[id] {
					co, cd := roll(c, seen)
					o += co
					d += cd
				}
				g.RolledOpen, g.RolledDone = o, d
				return o, d
			}
			for _, g := range goals {
				roll(g.ID, map[int]bool{})
			}

			out := make([]goalRow, 0, len(goals))
			for _, g := range goals {
				if levelFilter >= 0 && g.Level != levelFilter {
					continue
				}
				out = append(out, g)
			}
			sort.Slice(out, func(i, j int) bool {
				if out[i].Level != out[j].Level {
					return out[i].Level < out[j].Level
				}
				if out[i].RolledOpen != out[j].RolledOpen {
					return out[i].RolledOpen > out[j].RolledOpen
				}
				return out[i].Name < out[j].Name
			})

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Goal Progress (%d)\n", len(out))
			if len(out) == 0 {
				fmt.Fprintln(w, "  (no goals — run 'sync' or 'goals add')")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, bold("LEVEL\tOPEN\tDONE\tGOAL"))
			for _, g := range out {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%s\n", goalLevelLabel(g.Level), g.RolledOpen, g.RolledDone, truncate(g.Name, 50))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagLevel, "level", "", "Filter by goal level: lifetime, long-term, short-term")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard cache location)")
	return cmd
}

func loadGoalRows(db *store.Store) []goalRow {
	out := []goalRow{}
	rows, err := db.DB().Query(`SELECT "id","name","level","contributes" FROM "goals"`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var idStr, name sql.NullString
		var level, contributes sql.NullInt64
		if err := rows.Scan(&idStr, &name, &level, &contributes); err != nil {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(idStr.String))
		if err != nil {
			continue
		}
		out = append(out, goalRow{
			ID: id, Name: name.String, Level: int(level.Int64),
			LevelLabel: goalLevelLabel(int(level.Int64)), Contributes: int(contributes.Int64),
		})
	}
	return out
}

func loadGoalTaskCounts(db *store.Store) map[int]ocCount {
	m := map[int]ocCount{}
	rows, err := db.DB().Query(`SELECT "goal", COALESCE("completed",0) FROM "tasks" WHERE COALESCE("goal",0)<>0`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var goal, completed sql.NullInt64
		if err := rows.Scan(&goal, &completed); err != nil {
			continue
		}
		g := int(goal.Int64)
		c := m[g]
		if completed.Int64 > 0 {
			c.done++
		} else {
			c.open++
		}
		m[g] = c
	}
	return m
}

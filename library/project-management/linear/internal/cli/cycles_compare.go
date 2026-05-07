package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// newCyclesCompareCmd implements the prior `cycles compare` transcendence
// feature that shipped deferred under v1's ship-with-gaps verdict. Diffs two
// cycles by number, name, or "current"/"previous" alias and reports
// completion %, scope added, scope cut, carryover, and per-state buckets.
func newCyclesCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var teamFilter string
	cmd := &cobra.Command{
		Use:   "compare <cycle-a> <cycle-b>",
		Short: "Compare two cycles side-by-side: completion %, scope added/cut, carryover",
		Long: `Compare two cycles by cycle number, cycle ID, or alias. Aliases:
  current   the active cycle (or the most-recent one with started status)
  previous  the cycle immediately before current
  N         a numeric cycle.number

Outputs scope_count, completed_scope_count, completion_pct, plus diff metrics:
  scope_added       issues whose cycle_id is B but not A — net additions
  scope_cut         issues whose cycle_id is A but not B — net removals
  carryover         issues present in both cycles (came from A into B)
  per_state         counts grouped by issue state in each cycle`,
		Example: `  linear-pp-cli cycles compare 42 43
  linear-pp-cli cycles compare current previous --json
  linear-pp-cli cycles compare current previous --team ENG --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nRun 'linear-pp-cli sync' first.", err)
			}
			defer db.Close()

			cycles, err := db.ListCycles("")
			if err != nil {
				return err
			}
			if len(cycles) == 0 {
				return fmt.Errorf("no cycles in local store; run 'linear-pp-cli sync' first")
			}

			cyA, err := resolveCycleArg(cycles, args[0])
			if err != nil {
				return fmt.Errorf("cycle-a: %w", err)
			}
			cyB, err := resolveCycleArg(cycles, args[1])
			if err != nil {
				return fmt.Errorf("cycle-b: %w", err)
			}

			issuesA, err := db.ListIssues(map[string]string{"cycle_id": cyA.ID}, 1000)
			if err != nil {
				return fmt.Errorf("listing issues for cycle A: %w", err)
			}
			issuesB, err := db.ListIssues(map[string]string{"cycle_id": cyB.ID}, 1000)
			if err != nil {
				return fmt.Errorf("listing issues for cycle B: %w", err)
			}

			// Filter by team if requested (resolves team key to ID via local store)
			if teamFilter != "" {
				teamID := teamFilter
				if resolved, ok := resolveTeamID(db, teamFilter); ok {
					teamID = resolved
				}
				issuesA = filterIssuesByTeam(issuesA, teamID)
				issuesB = filterIssuesByTeam(issuesB, teamID)
			}

			diff := diffCycleIssues(issuesA, issuesB)
			summary := map[string]any{
				"cycle_a": map[string]any{
					"id":             cyA.ID,
					"number":         cyA.Number,
					"name":           cyA.Name,
					"starts_at":      cyA.StartsAt,
					"ends_at":        cyA.EndsAt,
					"scope_count":    len(issuesA),
					"completed":      diff.AClosed,
					"completion_pct": pctOf(diff.AClosed, len(issuesA)),
					"per_state":      diff.AStates,
				},
				"cycle_b": map[string]any{
					"id":             cyB.ID,
					"number":         cyB.Number,
					"name":           cyB.Name,
					"starts_at":      cyB.StartsAt,
					"ends_at":        cyB.EndsAt,
					"scope_count":    len(issuesB),
					"completed":      diff.BClosed,
					"completion_pct": pctOf(diff.BClosed, len(issuesB)),
					"per_state":      diff.BStates,
				},
				"scope_added": diff.Added,
				"scope_cut":   diff.Cut,
				"carryover":   diff.Carryover,
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(summary)
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "                 \t%-20s\t%-20s\n", labelCycle(cyA), labelCycle(cyB))
			fmt.Fprintf(tw, "scope            \t%d\t%d\n", len(issuesA), len(issuesB))
			fmt.Fprintf(tw, "completed        \t%d\t%d\n", diff.AClosed, diff.BClosed)
			fmt.Fprintf(tw, "completion %%     \t%.1f%%\t%.1f%%\n", pctOf(diff.AClosed, len(issuesA)), pctOf(diff.BClosed, len(issuesB)))
			fmt.Fprintln(tw)
			fmt.Fprintf(tw, "scope_added (only in %s)\t%d\n", labelCycle(cyB), len(diff.Added))
			fmt.Fprintf(tw, "scope_cut (only in %s)\t%d\n", labelCycle(cyA), len(diff.Cut))
			fmt.Fprintf(tw, "carryover (in both)\t%d\n", len(diff.Carryover))
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&teamFilter, "team", "", "Filter both cycles to a team (key or UUID)")
	return cmd
}

type cycleRef struct {
	ID         string  `json:"id"`
	Number     float64 `json:"number"`
	Name       string  `json:"name"`
	StartsAt   string  `json:"startsAt"`
	EndsAt     string  `json:"endsAt"`
	IsActive   bool    `json:"isActive"`
	IsPast     bool    `json:"isPast"`
	IsFuture   bool    `json:"isFuture"`
	IsCurrent  bool    `json:"-"` // resolved
	IsPrevious bool    `json:"-"`
}

func resolveCycleArg(cycles []json.RawMessage, arg string) (*cycleRef, error) {
	all := make([]cycleRef, 0, len(cycles))
	for _, raw := range cycles {
		var c cycleRef
		if err := json.Unmarshal(raw, &c); err == nil && c.ID != "" {
			all = append(all, c)
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("no cycles parseable from local store")
	}

	// "current" = highest-numbered cycle (or the active one if isActive is set);
	// "previous" = second-highest. The local store may not surface isActive on
	// every Linear deployment, so we lean on cycle.number ordering as the
	// primary signal and use isActive only as a tiebreaker.
	switch strings.ToLower(arg) {
	case "current":
		sort.Slice(all, func(i, j int) bool { return all[i].Number > all[j].Number })
		for i := range all {
			if all[i].IsActive {
				return &all[i], nil
			}
		}
		return &all[0], nil
	case "previous", "prev", "last":
		sort.Slice(all, func(i, j int) bool { return all[i].Number > all[j].Number })
		if len(all) < 2 {
			return nil, fmt.Errorf("only %d cycle(s) in local store; cannot resolve 'previous'", len(all))
		}
		return &all[1], nil
	}

	// Numeric: match by cycle.number
	if n, err := strconv.ParseFloat(arg, 64); err == nil {
		for i := range all {
			if all[i].Number == n {
				return &all[i], nil
			}
		}
		return nil, fmt.Errorf("no cycle with number %s in local store", arg)
	}

	// UUID-like: match by ID
	for i := range all {
		if all[i].ID == arg {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("could not resolve cycle %q (try a number, 'current', 'previous', or UUID)", arg)
}

type cycleDiff struct {
	Added     []string // issue ids present in B but not A
	Cut       []string // issue ids present in A but not B
	Carryover []string // issue ids present in both
	AClosed   int
	BClosed   int
	AStates   map[string]int
	BStates   map[string]int
}

func diffCycleIssues(a, b []json.RawMessage) cycleDiff {
	type slim struct {
		ID    string `json:"id"`
		State struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"state"`
	}
	parse := func(rows []json.RawMessage) (ids map[string]bool, states map[string]int, closed int) {
		ids = map[string]bool{}
		states = map[string]int{}
		for _, raw := range rows {
			var s slim
			if err := json.Unmarshal(raw, &s); err == nil && s.ID != "" {
				ids[s.ID] = true
				if s.State.Name != "" {
					states[s.State.Name]++
				}
				if s.State.Type == "completed" {
					closed++
				}
			}
		}
		return
	}
	aIDs, aStates, aClosed := parse(a)
	bIDs, bStates, bClosed := parse(b)
	d := cycleDiff{AStates: aStates, BStates: bStates, AClosed: aClosed, BClosed: bClosed}
	for id := range bIDs {
		if !aIDs[id] {
			d.Added = append(d.Added, id)
		} else {
			d.Carryover = append(d.Carryover, id)
		}
	}
	for id := range aIDs {
		if !bIDs[id] {
			d.Cut = append(d.Cut, id)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Cut)
	sort.Strings(d.Carryover)
	return d
}

func filterIssuesByTeam(rows []json.RawMessage, teamID string) []json.RawMessage {
	if teamID == "" {
		return rows
	}
	out := rows[:0]
	for _, raw := range rows {
		var t struct {
			Team struct {
				ID string `json:"id"`
			} `json:"team"`
			TeamID string `json:"team_id"`
		}
		if err := json.Unmarshal(raw, &t); err == nil {
			if t.Team.ID == teamID || t.TeamID == teamID {
				out = append(out, raw)
			}
		}
	}
	return out
}

func pctOf(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func labelCycle(c *cycleRef) string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("Cycle %g", c.Number)
}

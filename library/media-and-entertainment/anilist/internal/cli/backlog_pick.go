// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"sort"
)

func newNovelBacklogPickCmd(flags *rootFlags) *cobra.Command {
	var maxEpisodes, maxRuntime int
	cmd := &cobra.Command{Use: "pick", Short: "Choose a realistically short anime from your Planning list using deterministic personal ranking.", Example: "--max-episodes 13 --max-runtime-minutes 30 --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if maxEpisodes <= 0 || maxRuntime <= 0 {
			return usageErr(fmt.Errorf("--max-episodes and --max-runtime-minutes must be positive"))
		}
		if flags.dryRun {
			return flags.printJSON(cmd, map[string]any{"max_episodes": maxEpisodes, "max_runtime_minutes": maxRuntime, "dry_run": true})
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		u, err := viewerID(cmd.Context(), c)
		if err != nil {
			return err
		}
		all, err := allListEntries(cmd.Context(), c, u, "PLANNING")
		if err != nil {
			return err
		}
		eligible := rankBacklogCandidates(all, maxEpisodes, maxRuntime)
		if len(eligible) == 0 {
			return notFoundErr(fmt.Errorf("no eligible PLANNING anime within the requested bounds"))
		}
		e := eligible[0]
		return flags.printJSON(cmd, map[string]any{"media_id": e.Media.ID, "title": e.Media.Title.UserPreferred, "episodes": e.Media.Episodes, "duration_minutes": e.Media.Duration, "priority": e.Priority, "score": e.Score, "ranking": "priority desc, score desc, episodes asc, duration asc, media_id asc"})
	}}
	cmd.Flags().IntVar(&maxEpisodes, "max-episodes", 0, "Required positive maximum episode count")
	cmd.Flags().IntVar(&maxRuntime, "max-runtime-minutes", 0, "Required positive maximum episode runtime in minutes")
	return cmd
}

func rankBacklogCandidates(entries []personalEntry, maxEpisodes, maxRuntime int) []personalEntry {
	eligible := make([]personalEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Status != "PLANNING" || entry.Media.Status == "FINISHED" || entry.Media.Episodes <= 0 || entry.Media.Duration <= 0 || entry.Media.Episodes > maxEpisodes || entry.Media.Duration > maxRuntime {
			continue
		}
		eligible = append(eligible, entry)
	}
	sort.Slice(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Media.Episodes != b.Media.Episodes {
			return a.Media.Episodes < b.Media.Episodes
		}
		if a.Media.Duration != b.Media.Duration {
			return a.Media.Duration < b.Media.Duration
		}
		return a.Media.ID < b.Media.ID
	})
	return eligible
}

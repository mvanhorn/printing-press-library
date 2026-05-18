// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add persona leaderboard analytics over synced clips.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type personaLeaderboardRow struct {
	PersonaID string `json:"persona_id"`
	Name      string `json:"name,omitempty"`
	Likes     int    `json:"likes"`
	Plays     int    `json:"plays"`
	Extends   int    `json:"extends"`
	Score     int    `json:"score"`
}

func newPersonaLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var by, since string
	var limit int
	cmd := &cobra.Command{
		Use:         "leaderboard",
		Short:       "Rank personas by likes, plays, or extends",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if by == "" {
				by = "likes"
			}
			if by != "likes" && by != "plays" && by != "extends" {
				return usageErr(fmt.Errorf("invalid --by %q: expected likes, plays, or extends", by))
			}
			var sinceTime time.Time
			if since != "" {
				t, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since: %w", err))
				}
				sinceTime = t
			}
			s, err := openExistingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []personaLeaderboardRow{}, flags)
			}
			defer s.Close()
			names := map[string]string{}
			personaRows, _ := s.DB().QueryContext(cmd.Context(), `SELECT id, data FROM resources WHERE resource_type='persona'`)
			if personaRows != nil {
				defer personaRows.Close()
				for personaRows.Next() {
					var id, raw string
					_ = personaRows.Scan(&id, &raw)
					obj := unmarshalObject(json.RawMessage(raw))
					names[id] = clipTitle(obj)
				}
			}
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT id, data FROM resources WHERE resource_type IN ('clip','clips')`)
			if err != nil {
				return fmt.Errorf("querying local clips: %w", err)
			}
			defer rows.Close()
			type storedClip struct {
				id  string
				obj map[string]any
			}
			var clips []storedClip
			stats := map[string]*personaLeaderboardRow{}
			childCounts := map[string]int{}
			for rows.Next() {
				var id, raw string
				if err := rows.Scan(&id, &raw); err != nil {
					return fmt.Errorf("scanning clip: %w", err)
				}
				obj := unmarshalObject(json.RawMessage(raw))
				if parent := clipParentID(obj); parent != "" {
					childCounts[parent]++
				}
				clips = append(clips, storedClip{id: id, obj: obj})
			}
			for _, clip := range clips {
				id := clip.id
				obj := clip.obj
				if t := clipCreatedAt(obj); !sinceTime.IsZero() && (t.IsZero() || t.Before(sinceTime)) {
					continue
				}
				pid := clipPersonaID(obj)
				if pid == "" {
					continue
				}
				r := stats[pid]
				if r == nil {
					r = &personaLeaderboardRow{PersonaID: pid, Name: names[pid]}
					stats[pid] = r
				}
				r.Likes += int(numberAtAny(obj, []string{"like_count"}, []string{"likes"}, []string{"metadata", "like_count"}))
				r.Plays += int(numberAtAny(obj, []string{"play_count"}, []string{"plays"}, []string{"metadata", "play_count"}))
				r.Extends += childCounts[id]
			}
			out := make([]personaLeaderboardRow, 0, len(stats))
			for _, r := range stats {
				switch by {
				case "likes":
					r.Score = r.Likes
				case "plays":
					r.Score = r.Plays
				case "extends":
					r.Score = r.Extends
				}
				out = append(out, *r)
			}
			sortLeaderboard(out)
			if limit <= 0 {
				limit = 20
			}
			if len(out) > limit {
				out = out[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "likes", "Rank by likes, plays, or extends")
	cmd.Flags().StringVar(&since, "since", "", "Only include clips since duration")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum rows to return")
	return cmd
}

func sortLeaderboard(rows []personaLeaderboardRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && rows[j].Score > rows[j-1].Score; j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add local generation sessionization.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type sessionRow struct {
	SessionStart     string   `json:"session_start"`
	SessionEnd       string   `json:"session_end"`
	GenerationsCount int      `json:"generations_count"`
	EstimatedCredits int      `json:"estimated_credits"`
	DistinctPersonas int      `json:"distinct_personas"`
	DistinctTags     []string `json:"distinct_tags"`
}

func newSessionsCmd(flags *rootFlags) *cobra.Command {
	var today bool
	var since string
	var limit int
	cmd := &cobra.Command{
		Use:         "sessions",
		Short:       "Group synced generations into 30-minute-gap sessions",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var sinceTime time.Time
			if today {
				now := time.Now()
				sinceTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			} else if since != "" {
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
				return printJSONFiltered(cmd.OutOrStdout(), []sessionRow{}, flags)
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT data FROM resources WHERE resource_type IN ('clip','clips')`)
			if err != nil {
				return fmt.Errorf("querying local clips: %w", err)
			}
			defer rows.Close()
			type clipPoint struct {
				at      time.Time
				persona string
				tags    []string
			}
			var points []clipPoint
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					return fmt.Errorf("scanning clip: %w", err)
				}
				obj := unmarshalObject(json.RawMessage(raw))
				at := clipCreatedAt(obj)
				if at.IsZero() || (!sinceTime.IsZero() && at.Before(sinceTime)) {
					continue
				}
				points = append(points, clipPoint{at: at, persona: clipPersonaID(obj), tags: clipTags(obj)})
			}
			sort.Slice(points, func(i, j int) bool { return points[i].at.Before(points[j].at) })
			var out []sessionRow
			var cur *sessionRow
			personas := map[string]struct{}{}
			tags := map[string]struct{}{}
			last := time.Time{}
			flush := func() {
				if cur == nil {
					return
				}
				cur.DistinctPersonas = len(personas)
				cur.DistinctTags = sortedKeys(tags)
				out = append(out, *cur)
			}
			for _, p := range points {
				if cur == nil || p.at.Sub(last) > 30*time.Minute {
					flush()
					personas = map[string]struct{}{}
					tags = map[string]struct{}{}
					cur = &sessionRow{SessionStart: p.at.UTC().Format(time.RFC3339)}
				}
				cur.SessionEnd = p.at.UTC().Format(time.RFC3339)
				cur.GenerationsCount++
				cur.EstimatedCredits += 10
				if p.persona != "" {
					personas[p.persona] = struct{}{}
				}
				for _, tag := range p.tags {
					tags[tag] = struct{}{}
				}
				last = p.at
			}
			flush()
			if limit > 0 && len(out) > limit {
				out = out[len(out)-limit:]
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&today, "today", false, "Only include today's clips")
	cmd.Flags().StringVar(&since, "since", "", "Only include clips since duration (e.g. 7d)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum sessions to return")
	return cmd
}

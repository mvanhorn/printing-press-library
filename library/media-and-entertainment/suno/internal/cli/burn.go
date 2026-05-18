// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add local credit burn analytics over synced clips.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

type burnRow struct {
	GroupValue       string `json:"group_value"`
	GenerationCount  int    `json:"generation_count"`
	EstimatedCredits int    `json:"estimated_credits"`
	FirstAt          string `json:"first_at,omitempty"`
	LastAt           string `json:"last_at,omitempty"`
}

func newBurnCmd(flags *rootFlags) *cobra.Command {
	var by, since string
	cmd := &cobra.Command{
		Use:         "burn",
		Short:       "Aggregate estimated generation credits from local clips",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if by == "" {
				by = "tag"
			}
			if by != "tag" && by != "persona" && by != "model" && by != "hour" {
				return usageErr(fmt.Errorf("invalid --by %q: expected tag, persona, model, or hour", by))
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
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'suno-pp-cli sync' first to populate the local store")
				return nil
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT data FROM resources WHERE resource_type IN ('clip','clips')`)
			if err != nil {
				return fmt.Errorf("querying local clips: %w", err)
			}
			defer rows.Close()
			agg := map[string]*burnRow{}
			var total int
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					return fmt.Errorf("scanning clip: %w", err)
				}
				var obj map[string]any
				if json.Unmarshal([]byte(raw), &obj) != nil {
					continue
				}
				created := clipCreatedAt(obj)
				if !sinceTime.IsZero() && (created.IsZero() || created.Before(sinceTime)) {
					continue
				}
				var groups []string
				switch by {
				case "tag":
					groups = clipTags(obj)
				case "persona":
					groups = []string{clipPersonaID(obj)}
				case "model":
					groups = []string{clipModel(obj)}
				case "hour":
					if created.IsZero() {
						groups = []string{"unknown"}
					} else {
						groups = []string{created.Format("15:00")}
					}
				}
				if len(groups) == 0 {
					groups = []string{"unknown"}
				}
				total++
				for _, g := range groups {
					if g == "" {
						g = "unknown"
					}
					r := agg[g]
					if r == nil {
						r = &burnRow{GroupValue: g}
						agg[g] = r
					}
					r.GenerationCount++
					r.EstimatedCredits += 10
					if !created.IsZero() {
						ts := created.UTC().Format(time.RFC3339)
						if r.FirstAt == "" || ts < r.FirstAt {
							r.FirstAt = ts
						}
						if r.LastAt == "" || ts > r.LastAt {
							r.LastAt = ts
						}
					}
				}
			}
			if total == 0 && len(agg) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Run 'suno-pp-cli sync' first to populate the local store")
				return nil
			}
			out := make([]burnRow, 0, len(agg))
			for _, r := range agg {
				out = append(out, *r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "tag", "Group by tag, persona, model, or hour")
	cmd.Flags().StringVar(&since, "since", "", "Only include clips since duration (e.g. 30d)")
	return cmd
}

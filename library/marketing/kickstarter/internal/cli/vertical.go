// Copyright 2026 usc-tk. Licensed under Apache-2.0. See LICENSE.

// vertical scores every locally-known project against a named vertical
// (ai-agents, frontier-ai, smb-saas, geopolitics, aus-tech, india-tech)
// using keyword overlap against name+blurb+tags, persists the scores
// into the vertical_match table, and emits the top-N rows.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type verticalRow struct {
	ProjectID int64   `json:"project_id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	URL       string  `json:"url"`
	Score     float64 `json:"score"`
	Pledged   float64 `json:"pledged,omitempty"`
	Goal      float64 `json:"goal,omitempty"`
	State     string  `json:"state,omitempty"`
}

func newVerticalCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var since string
	var limit int
	var jsonl bool

	cmd := &cobra.Command{
		Use:   "vertical <vertical-slug>",
		Short: "Score locally-known projects against a named vertical",
		Long: `Score every project in the local store against one of the curated
verticals using keyword overlap (case-insensitive substring match)
against name + blurb + tags. Scores are normalized to 0..1 (hits / keyword
count, capped at 1.0).

Persists each scored match into the vertical_match table so subsequent
SQL queries can join projects with their best-fit vertical without re-
scoring.

Supported verticals: ai-agents, frontier-ai, smb-saas, geopolitics,
aus-tech, india-tech.`,
		Example: `  kickstarter-pp-cli vertical ai-agents --score-threshold 0.3
  kickstarter-pp-cli vertical frontier-ai --since 30d --limit 100 --jsonl`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			vert := strings.ToLower(strings.TrimSpace(args[0]))
			if _, ok := verticalKeywords[vert]; !ok {
				return usageErr(fmt.Errorf("unknown vertical %q; available: %s", vert, strings.Join(knownVerticals(), ", ")))
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openStoreForRead(cmd.Context(), "kickstarter-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local data. Run `kickstarter-pp-cli sync` first"))
			}
			defer db.Close()
			if err := db.EnsureV2Schema(cmd.Context()); err != nil {
				return fmt.Errorf("ensuring v2 schema: %w", err)
			}

			cutoff := int64(0)
			if since != "" {
				secs, ok := parseSinceFlagSeconds(since)
				if !ok {
					return usageErr(fmt.Errorf("invalid --since value %q: expected like 7d, 30d, 24h, 1w", since))
				}
				cutoff = time.Now().Unix() - secs
			}

			projects, err := db.LoadDiscoverProjects(0)
			if err != nil {
				return fmt.Errorf("loading projects: %w", err)
			}
			rows := make([]verticalRow, 0, len(projects))
			now := time.Now()
			for _, p := range projects {
				if cutoff > 0 && p.LaunchedAt > 0 && p.LaunchedAt < cutoff {
					continue
				}
				hay := p.Name + " " + p.Blurb
				if len(p.Tags) > 0 {
					hay += " " + strings.Join(p.Tags, " ")
				}
				score := verticalScore(vert, hay)
				if score < threshold {
					continue
				}
				rows = append(rows, verticalRow{
					ProjectID: p.ID,
					Name:      p.Name,
					Slug:      p.Slug,
					URL:       p.URLs.Web.Project,
					Score:     score,
					Pledged:   p.Pledged,
					Goal:      p.Goal,
					State:     p.State,
				})
				// Persist — best effort, but log to stderr if it fails.
				if err := db.UpsertVerticalMatch(p.ID, vert, score, now); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: upserting vertical_match for %d: %v\n", p.ID, err)
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].Score > rows[j].Score
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if jsonl {
				enc := json.NewEncoder(cmd.OutOrStdout())
				for _, r := range rows {
					if err := enc.Encode(r); err != nil {
						return err
					}
				}
				return nil
			}
			env := map[string]any{
				"vertical": vert,
				"matches":  rows,
				"count":    len(rows),
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().Float64Var(&threshold, "score-threshold", 0.1, "Minimum normalized score (0..1) to include in the result set")
	cmd.Flags().StringVar(&since, "since", "", "Only score projects launched within this window (e.g. 7d, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results to emit after threshold filter")
	cmd.Flags().BoolVar(&jsonl, "jsonl", false, "Emit JSONL (one record per line)")
	return cmd
}

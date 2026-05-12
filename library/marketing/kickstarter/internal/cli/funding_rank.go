// Copyright 2026 usc-tk. Licensed under Apache-2.0. See LICENSE.

// funding-rank computes per-project funding velocity from the local
// project_snapshots table — *not* from a single live snapshot. It needs at
// least two snapshots within the window to compute a delta, so the
// command is only useful after at least two sync passes.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type fundingRankRow struct {
	ID                   int64   `json:"id"`
	Name                 string  `json:"name"`
	Slug                 string  `json:"slug"`
	URL                  string  `json:"url"`
	Category             string  `json:"category"`
	CategorySlug         string  `json:"category_slug"`
	State                string  `json:"state"`
	Pledged              float64 `json:"pledged"`
	BackersCount         int     `json:"backers_count"`
	LaunchedAt           int64   `json:"launched_at"`
	FundingVelocityPerHr float64 `json:"funding_velocity_per_hour"`
}

func newFundingRankCmd(flags *rootFlags) *cobra.Command {
	var window string
	var category string
	var limit int

	cmd := &cobra.Command{
		Use:   "funding-rank",
		Short: "Rank live projects by funding velocity computed from local snapshots",
		Long: `Compute funding velocity = (pledged_now - pledged_earlier) / hours_elapsed
using the two most recent project_snapshots within the window. Requires at
least two 'sync --since-last --diff' passes to have run within the window
so two distinct snapshots exist per project. Pure-local read; no network.`,
		Example: `  kickstarter-pp-cli funding-rank --window 24h
  kickstarter-pp-cli funding-rank --window 7d --category technology --limit 25 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			secs, ok := parseSinceFlagSeconds(window)
			if !ok {
				return usageErr(fmt.Errorf("invalid --window value %q: expected like 24h, 7d, 1w", window))
			}
			windowStart := time.Now().Add(-time.Duration(secs) * time.Second)

			db, err := openStoreForRead(cmd.Context(), "kickstarter-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("funding-rank requires at least two `kickstarter-pp-cli sync` snapshots within the window. Run sync, wait, run sync again, then retry."))
			}
			defer db.Close()
			if err := db.EnsureV2Schema(cmd.Context()); err != nil {
				return fmt.Errorf("ensuring v2 schema: %w", err)
			}

			snaps, err := db.SnapshotsSince(windowStart)
			if err != nil {
				return fmt.Errorf("loading snapshots: %w", err)
			}
			if len(snaps) < 2 {
				return notFoundErr(fmt.Errorf("funding-rank requires at least two `kickstarter-pp-cli sync` snapshots within the window. Run sync, wait, run sync again, then retry. (found %d snapshot rows in window)", len(snaps)))
			}
			byProject := map[int64][]int{}
			for i, s := range snaps {
				byProject[s.ProjectID] = append(byProject[s.ProjectID], i)
			}
			// No project has 2+ snapshots — agent-facing message rather
			// than empty success so Scout doesn't quietly skip past a
			// "no signal yet" state.
			hasDelta := false
			for _, idxs := range byProject {
				if len(idxs) >= 2 {
					hasDelta = true
					break
				}
			}
			if !hasDelta {
				return notFoundErr(fmt.Errorf("funding-rank requires at least two `kickstarter-pp-cli sync` snapshots PER PROJECT within the window. Run sync, wait, run sync again, then retry. (found %d snapshots across %d projects but no project has 2+ snapshots in window %q)", len(snaps), len(byProject), window))
			}

			// Load project metadata once. Catalog by ID for quick joins.
			projects, _ := db.LoadDiscoverProjects(0)
			meta := map[int64]int{}
			for i, p := range projects {
				meta[p.ID] = i
			}

			category = strings.ToLower(strings.TrimSpace(category))
			rows := make([]fundingRankRow, 0, len(byProject))
			for pid, idxs := range byProject {
				if len(idxs) < 2 {
					continue
				}
				a := snaps[idxs[0]]
				b := snaps[idxs[len(idxs)-1]]
				dh := float64(b.SnapshotAt-a.SnapshotAt) / 3600.0
				if dh <= 0 {
					continue
				}
				row := fundingRankRow{
					ID:                   pid,
					Pledged:              b.Pledged,
					BackersCount:         b.BackersCount,
					State:                b.State,
					FundingVelocityPerHr: (b.Pledged - a.Pledged) / dh,
				}
				if i, ok := meta[pid]; ok {
					p := projects[i]
					row.Name = p.Name
					row.Slug = p.Slug
					row.URL = p.URLs.Web.Project
					row.Category = p.Category.Name
					row.CategorySlug = p.Category.Slug
					row.LaunchedAt = p.LaunchedAt
				}
				if category != "" {
					slug := strings.ToLower(row.CategorySlug)
					name := strings.ToLower(row.Category)
					if !strings.Contains(slug, category) && !strings.Contains(name, category) {
						continue
					}
				}
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].FundingVelocityPerHr > rows[j].FundingVelocityPerHr
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			env := map[string]any{
				"projects":        rows,
				"window":          window,
				"category_filter": category,
				"count":           len(rows),
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "24h", "Window like 24h, 7d, 1w")
	cmd.Flags().StringVar(&category, "category", "", "Substring-match against project category slug or name")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results to emit")
	return cmd
}

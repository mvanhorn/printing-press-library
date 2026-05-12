// Copyright 2026 usc-tk. Licensed under Apache-2.0. See LICENSE.

// tech-radar walks Discover (category 16, sort=newest) page-by-page until
// it crosses the requested time window, then ranks the matching projects
// by funding velocity (pledged-per-hour-since-launch). When project
// snapshots exist for a project, the *delta* between the latest two
// snapshots wins over the launch-time approximation.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/kickstarter/internal/store"
)

type techRadarProject struct {
	ID                   int64   `json:"id"`
	Name                 string  `json:"name"`
	Blurb                string  `json:"blurb"`
	Slug                 string  `json:"slug"`
	URL                  string  `json:"url"`
	State                string  `json:"state"`
	Goal                 float64 `json:"goal"`
	Pledged              float64 `json:"pledged"`
	BackersCount         int     `json:"backers_count"`
	LaunchedAt           int64   `json:"launched_at"`
	Category             string  `json:"category"`
	CategorySlug         string  `json:"category_slug"`
	CreatorName          string  `json:"creator_name"`
	FundingVelocityPerHr float64 `json:"funding_velocity_per_hour"`
}

func newTechRadarCmd(flags *rootFlags) *cobra.Command {
	var subcategory string
	var days int
	var limit int

	cmd := &cobra.Command{
		Use:   "tech-radar",
		Short: "Newly launched Technology projects ranked by funding velocity",
		Long: `Walk Discover (category-id 16, sort=newest) page-by-page until launched_at
crosses --days, then rank by funding velocity = pledged / max(1, hours_since_launch).
When project_snapshots exist (populated by 'sync --since-last --diff'), the
delta between the latest two snapshots wins over the launch-time
approximation — a 2x more accurate signal of momentum.`,
		Example: `  kickstarter-pp-cli tech-radar --days 7
  kickstarter-pp-cli tech-radar --subcategory ai --limit 25 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()

			// Snapshot map keyed by project_id, holding the two most recent
			// snapshots — used to compute velocity-by-delta when available.
			snapMap := loadSnapshotMap(cmd.Context())

			projects, rawProjects, err := walkTechDiscover(cmd.Context(), c, cutoff, subcategory)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Write-through so local-only commands see what we observed.
			ingestProjectsIfPossible(cmd.Context(), rawProjects)
			for i := range projects {
				p := &projects[i]
				if snaps, ok := snapMap[p.ID]; ok && len(snaps) >= 2 {
					a, b := snaps[len(snaps)-2], snaps[len(snaps)-1]
					dh := float64(b.SnapshotAt-a.SnapshotAt) / 3600.0
					if dh > 0 {
						p.FundingVelocityPerHr = (b.Pledged - a.Pledged) / dh
						continue
					}
				}
				p.FundingVelocityPerHr = velocityFromLaunch(p.Pledged, p.LaunchedAt)
			}
			sort.Slice(projects, func(i, j int) bool {
				return projects[i].FundingVelocityPerHr > projects[j].FundingVelocityPerHr
			})
			if limit > 0 && len(projects) > limit {
				projects = projects[:limit]
			}
			env := map[string]any{
				"projects":           projects,
				"window_days":        days,
				"subcategory_filter": subcategory,
				"count":              len(projects),
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&subcategory, "subcategory", "", "Substring-match against project.category.slug (e.g. ai, hardware, software, robots)")
	cmd.Flags().IntVar(&days, "days", 7, "Time window in days for newly-launched projects")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results to emit")
	return cmd
}

// velocityFromLaunch returns the pledged-per-hour rate using the project's
// launched_at as the time origin. Clamps the divisor at 1 hour so a brand-
// new project doesn't divide by a near-zero and explode to infinity.
func velocityFromLaunch(pledged float64, launchedAt int64) float64 {
	if launchedAt <= 0 {
		return 0
	}
	hours := float64(time.Now().Unix()-launchedAt) / 3600.0
	if hours < 1 {
		hours = 1
	}
	return pledged / hours
}

// walkTechDiscover pages through /discover/advanced (category 16, newest)
// until either the page comes back empty or the oldest project on a page
// is older than the cutoff. Returns the projects in API order plus the
// raw DiscoverProject payloads for write-through caching; sorting is
// the caller's responsibility.
func walkTechDiscover(ctx context.Context, c *client.Client, cutoff int64, subcategoryFilter string) ([]techRadarProject, []store.DiscoverProject, error) {
	_ = ctx
	const maxPages = 10
	var out []techRadarProject
	var raw []store.DiscoverProject
	subcategoryFilter = strings.ToLower(strings.TrimSpace(subcategoryFilter))

	for page := 1; page <= maxPages; page++ {
		data, err := c.Get("/discover/advanced", map[string]string{
			"format":      "json",
			"category_id": "16",
			"sort":        "newest",
			"page":        fmt.Sprintf("%d", page),
		})
		if err != nil {
			return nil, nil, err
		}
		var resp struct {
			Projects []store.DiscoverProject `json:"projects"`
			HasMore  bool                    `json:"has_more"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, nil, fmt.Errorf("decoding discover page %d: %w", page, err)
		}
		if len(resp.Projects) == 0 {
			break
		}
		oldestOnPage := int64(0)
		for _, p := range resp.Projects {
			raw = append(raw, p)
			if oldestOnPage == 0 || (p.LaunchedAt > 0 && p.LaunchedAt < oldestOnPage) {
				oldestOnPage = p.LaunchedAt
			}
			if p.LaunchedAt > 0 && p.LaunchedAt < cutoff {
				continue
			}
			if subcategoryFilter != "" {
				slug := strings.ToLower(p.Category.Slug)
				name := strings.ToLower(p.Category.Name)
				if !strings.Contains(slug, subcategoryFilter) && !strings.Contains(name, subcategoryFilter) {
					continue
				}
			}
			out = append(out, techRadarProject{
				ID:           p.ID,
				Name:         p.Name,
				Blurb:        p.Blurb,
				Slug:         p.Slug,
				URL:          p.URLs.Web.Project,
				State:        p.State,
				Goal:         p.Goal,
				Pledged:      p.Pledged,
				BackersCount: p.BackersCount,
				LaunchedAt:   p.LaunchedAt,
				Category:     p.Category.Name,
				CategorySlug: p.Category.Slug,
				CreatorName:  p.Creator.Name,
			})
		}
		// Window guard: if every project on this page predates the
		// cutoff and we've already collected at least one item, stop.
		if oldestOnPage > 0 && oldestOnPage < cutoff {
			break
		}
		if !resp.HasMore {
			break
		}
	}
	return out, raw, nil
}

// loadSnapshotMap returns a per-project map of snapshots ordered by
// snapshot_at ASC. Used by tech-radar and funding-rank to compute
// delta-based funding velocity. Returns an empty map (not nil) if the
// store can't be opened so callers can branch on len() without nil-checks.
func loadSnapshotMap(ctx context.Context) map[int64][]store.ProjectSnapshot {
	out := map[int64][]store.ProjectSnapshot{}
	db, err := openStoreForRead(ctx, "kickstarter-pp-cli")
	if err != nil || db == nil {
		return out
	}
	defer db.Close()
	if err := db.EnsureV2Schema(ctx); err != nil {
		return out
	}
	// Pull everything; snapshots are small and per-project sequencing is
	// cheaper to do in Go than to push down into SQL with a window.
	snaps, err := db.SnapshotsSince(time.Unix(0, 0))
	if err != nil {
		return out
	}
	for _, s := range snaps {
		out[s.ProjectID] = append(out[s.ProjectID], s)
	}
	return out
}

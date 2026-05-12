// Copyright 2026 usc-tk. Licensed under Apache-2.0. See LICENSE.

// category is the parent cobra command for category-level rollups. Two
// children live here:
//
//   - `category list`     - enumerate distinct category slugs in the
//                           local store
//   - `category velocity` - aggregate launches and pledged-volume per
//                           category over a window
//
// Both are pure-local reads: no network. They depend on a previous
// `sync` having populated the discover resource type.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newCategoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "category",
		Short: "Category-level analytics over the local store",
		Long:  "Category-level rollups over locally-synced projects (no network).",
	}
	cmd.AddCommand(newCategoryListCmd(flags))
	cmd.AddCommand(newCategoryVelocityCmd(flags))
	return cmd
}

type categoryRow struct {
	Slug            string  `json:"slug"`
	Name            string  `json:"name,omitempty"`
	Launches        int     `json:"launches"`
	LaunchesPerDay  float64 `json:"launches_per_day"`
	TotalPledgedUSD float64 `json:"total_pledged_usd"`
}

func newCategoryListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List distinct category slugs in the local store",
		Example: `  kickstarter-pp-cli category list --json
  kickstarter-pp-cli category list --select slug,name --plain`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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
			projects, err := db.LoadDiscoverProjects(0)
			if err != nil {
				return fmt.Errorf("loading projects: %w", err)
			}
			seen := map[string]string{} // slug -> name
			for _, p := range projects {
				if p.Category.Slug == "" {
					continue
				}
				if _, ok := seen[p.Category.Slug]; !ok {
					seen[p.Category.Slug] = p.Category.Name
				}
			}
			slugs := make([]string, 0, len(seen))
			for k := range seen {
				slugs = append(slugs, k)
			}
			sort.Strings(slugs)
			out := make([]map[string]string, 0, len(slugs))
			for _, s := range slugs {
				out = append(out, map[string]string{"slug": s, "name": seen[s]})
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"categories": out,
				"count":      len(out),
			}, flags)
		},
	}
	return cmd
}

func newCategoryVelocityCmd(flags *rootFlags) *cobra.Command {
	var window string
	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Aggregate launches-per-day and total pledged per category over a window",
		Long: `Aggregate over locally-synced Discover projects:
  launches             = COUNT(*) where launched_at within --window
  launches_per_day     = launches / window_days
  total_pledged_usd    = SUM(pledged)   // currency is USD-equivalent in the Discover JSON

Sorted by launches DESC. Pure local read; no network.`,
		Example: `  kickstarter-pp-cli category velocity --window 7d
  kickstarter-pp-cli category velocity --window 30d --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			secs, ok := parseSinceFlagSeconds(window)
			if !ok {
				return usageErr(fmt.Errorf("invalid --window value %q: expected like 7d, 14d, 30d, 24h", window))
			}
			windowDays := float64(secs) / 86400.0
			if windowDays <= 0 {
				windowDays = 1
			}
			cutoff := time.Now().Unix() - secs

			db, err := openStoreForRead(cmd.Context(), "kickstarter-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local data. Run `kickstarter-pp-cli sync` first"))
			}
			defer db.Close()
			projects, err := db.LoadDiscoverProjects(0)
			if err != nil {
				return fmt.Errorf("loading projects: %w", err)
			}
			type agg struct {
				name    string
				count   int
				pledged float64
			}
			byCat := map[string]*agg{}
			for _, p := range projects {
				if p.LaunchedAt < cutoff {
					continue
				}
				slug := strings.TrimSpace(p.Category.Slug)
				if slug == "" {
					slug = "(unknown)"
				}
				a, ok := byCat[slug]
				if !ok {
					a = &agg{name: p.Category.Name}
					byCat[slug] = a
				}
				a.count++
				a.pledged += p.Pledged
			}
			rows := make([]categoryRow, 0, len(byCat))
			for slug, a := range byCat {
				rows = append(rows, categoryRow{
					Slug:            slug,
					Name:            a.name,
					Launches:        a.count,
					LaunchesPerDay:  float64(a.count) / windowDays,
					TotalPledgedUSD: a.pledged,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Launches > rows[j].Launches })
			env := map[string]any{
				"window_days": windowDays,
				"categories":  rows,
				"count":       len(rows),
			}
			return printJSONFiltered(cmd.OutOrStdout(), env, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "7d", "Time window (e.g. 7d, 14d, 30d)")
	return cmd
}

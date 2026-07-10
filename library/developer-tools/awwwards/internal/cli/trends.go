// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/store"
)

type trendBucket struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	PrevCount *int   `json:"prev_count,omitempty"`
	Delta     *int   `json:"delta,omitempty"`
}

type trendsResult struct {
	By            string        `json:"by"`
	Since         string        `json:"since"`
	Vs            string        `json:"vs,omitempty"`
	Buckets       []trendBucket `json:"buckets"`
	SitesInWindow int           `json:"sites_in_window"`
	Coverage      string        `json:"coverage,omitempty"`
	Note          string        `json:"note,omitempty"`
}

func newNovelTrendsCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var flagSince string
	var flagVs string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Quantify what's rising and falling in award-winning design: tag, color, and tech frequency over a time window",
		Long: strings.Trim(`
Trends computes frequency counts over the local design mirror: which tags,
palette color families, or technologies appear most among award entries in a
time window. With --vs it also counts the preceding window of the same length
and reports the delta, turning "is dark mode fading?" into actual numbers.

--by color requires palette data ('mirror --details'); coverage is reported so
you know how many sites the counts are based on. Font is not a supported axis:
awwwards.com attributes fonts only on filter pages, never per site.

Use this command for aggregate frequency of tags, colors, or tech over a time
window. Do NOT use this command to rank individual sites by jury score; use
'top' instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli trends --by tag --since 90d --json
  awwwards-pp-cli trends --by tech --since 90d --vs 90d --json
  awwwards-pp-cli trends --by color --since 180d
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--by=tag;--since=365d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate trend counts from the local design mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "trends"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			by := strings.ToLower(strings.TrimSpace(flagBy))
			if by == "" {
				by = "tag"
			}
			if by != "tag" && by != "color" && by != "tech" {
				return usageErr(fmt.Errorf("invalid --by %q: want tag, color, or tech (font is not attributable per-site on awwwards.com)", flagBy))
			}

			if limit < 0 {
				limit = 0
			}
			sinceDur, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w (use forms like 90d, 12w, 24h)", flagSince, err))
			}
			var vsDur time.Duration
			if flagVs != "" {
				vsDur, err = cliutil.ParseDurationLoose(flagVs)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --vs %q: %w", flagVs, err))
				}
			}

			if dbPath == "" {
				dbPath = defaultDBPath("awwwards-pp-cli")
			}
			db, done := requireMirror(cmd, flags, dbPath)
			if done {
				return nil
			}
			defer db.Close()

			now := time.Now()
			curFrom := now.Add(-sinceDur).Unix()
			cur, sitesInWindow, coverage, err := trendCounts(ctx, db, by, curFrom, now.Unix())
			if err != nil {
				return err
			}

			var prev map[string]int
			if flagVs != "" {
				prevFrom := now.Add(-sinceDur).Add(-vsDur).Unix()
				prev, _, _, err = trendCounts(ctx, db, by, prevFrom, curFrom)
				if err != nil {
					return err
				}
			}

			buckets := make([]trendBucket, 0, len(cur))
			for name, n := range cur {
				b := trendBucket{Name: name, Count: n}
				if prev != nil {
					p := prev[name]
					d := n - p
					b.PrevCount = &p
					b.Delta = &d
				}
				buckets = append(buckets, b)
			}
			sort.Slice(buckets, func(i, j int) bool {
				if buckets[i].Count != buckets[j].Count {
					return buckets[i].Count > buckets[j].Count
				}
				return buckets[i].Name < buckets[j].Name
			})
			if len(buckets) > limit {
				buckets = buckets[:limit]
			}

			res := trendsResult{By: by, Since: flagSince, Vs: flagVs, Buckets: buckets, SitesInWindow: sitesInWindow, Coverage: coverage}
			if sitesInWindow == 0 {
				res.Note = "no mirrored sites in this window; widen --since or run 'awwwards-pp-cli mirror --pages 10'"
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}

	cmd.Flags().StringVar(&flagBy, "by", "tag", "Axis to count: tag, color, or tech")
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Window length (e.g. 90d, 12w)")
	cmd.Flags().StringVar(&flagVs, "vs", "", "Compare against the preceding window of this length (e.g. 90d)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum buckets to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// trendCounts aggregates one window. Returns bucket counts, the number of
// sites in the window, and a coverage note for palette-based axes.
func trendCounts(ctx context.Context, db *store.Store, by string, fromUnix, toUnix int64) (map[string]int, int, string, error) {
	counts := map[string]int{}

	var sites int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM aw_sites WHERE created_at >= ? AND created_at < ?`, fromUnix, toUnix).Scan(&sites); err != nil {
		return nil, 0, "", fmt.Errorf("counting window: %w", err)
	}

	switch by {
	case "tag", "tech":
		rows, err := db.DB().QueryContext(ctx, `
			SELECT t.tag, COUNT(*) FROM aw_site_tags t
			JOIN aw_sites s ON s.slug = t.slug
			WHERE s.created_at >= ? AND s.created_at < ?
			GROUP BY t.tag`, fromUnix, toUnix)
		if err != nil {
			return nil, 0, "", fmt.Errorf("aggregating tags: %w", err)
		}
		for rows.Next() {
			var tag string
			var n int
			if err := rows.Scan(&tag, &n); err != nil {
				_ = rows.Close()
				return nil, 0, "", fmt.Errorf("scanning tag bucket: %w", err)
			}
			if by == "tech" && !awwwards.IsTech(tag) {
				continue
			}
			counts[tag] = n
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, 0, "", err
		}
		if err := rows.Close(); err != nil {
			return nil, 0, "", err
		}
		return counts, sites, "", nil

	case "color":
		rows, err := db.DB().QueryContext(ctx, `
			SELECT p.hex, COUNT(*) FROM aw_palette p
			JOIN aw_sites s ON s.slug = p.slug
			WHERE s.created_at >= ? AND s.created_at < ?
			GROUP BY p.hex`, fromUnix, toUnix)
		if err != nil {
			return nil, 0, "", fmt.Errorf("aggregating palette: %w", err)
		}
		type hexCount struct {
			hex string
			n   int
		}
		var hexRows []hexCount
		for rows.Next() {
			var hc hexCount
			if err := rows.Scan(&hc.hex, &hc.n); err != nil {
				_ = rows.Close()
				return nil, 0, "", fmt.Errorf("scanning palette bucket: %w", err)
			}
			hexRows = append(hexRows, hc)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, 0, "", err
		}
		if err := rows.Close(); err != nil {
			return nil, 0, "", err
		}

		for _, hc := range hexRows {
			rgb, err := awwwards.ParseHex(hc.hex)
			if err != nil {
				continue
			}
			counts[awwwards.HueFamily(rgb)] += hc.n
		}

		// Coverage: palette data exists only for detail-synced sites.
		var covered int
		if err := db.DB().QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT s.slug) FROM aw_sites s
			JOIN aw_palette p ON p.slug = s.slug
			WHERE s.created_at >= ? AND s.created_at < ?`, fromUnix, toUnix).Scan(&covered); err != nil {
			return nil, 0, "", fmt.Errorf("counting palette coverage: %w", err)
		}
		cov := fmt.Sprintf("%d of %d sites in window have palette data (run 'mirror --details' to raise coverage)", covered, sites)
		return counts, sites, cov, nil
	}
	return counts, sites, "", nil
}

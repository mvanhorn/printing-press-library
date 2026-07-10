// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
)

type paletteHit struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	MatchedHex   string   `json:"matched_hex"`
	Distance     float64  `json:"distance"`
	Palette      []string `json:"palette"`
	ScoreOverall *float64 `json:"score_overall,omitempty"`
	ThumbnailURL string   `json:"thumbnail_url"`
	DetailURL    string   `json:"detail_url"`
}

type paletteMatchResult struct {
	Target       string       `json:"target"`
	MaxDistance  float64      `json:"max_distance"`
	Items        []paletteHit `json:"items"`
	SitesScanned int          `json:"sites_scanned"`
	Note         string       `json:"note,omitempty"`
}

func newNovelPaletteMatchCmd(flags *rootFlags) *cobra.Command {
	var flagDistance float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "palette-match <hex>",
		Short: "Find award-winning sites whose palette contains a color near a target hex, ranked by RGB distance",
		Long: strings.Trim(`
Palette-match scans every extracted palette color in the local mirror and
returns the sites whose palette contains a color within --distance (Euclidean
RGB space, 0-441) of your target. The site's own color filter is exact-hex
over ~30 buckets; this is the fuzzy version for real brand colors.

Palette data comes from detail pages: run 'mirror --details' first.

Use this command to find sites whose palette contains a color NEAR a given
hex (fuzzy, local mirror). Do NOT use this command for exact server-side
color filtering; use 'find --color' instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli palette-match "#0F4C81" --distance 25 --json
  awwwards-pp-cli palette-match 1A1A1A --distance 40 --limit 5
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "hex=#111111;--distance=80"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan mirrored palettes for near-matches")
				return nil
			}
			if err := rejectLiveDataSource(flags, "palette-match"); err != nil {
				return err
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a target hex color is required (e.g. \"#0F4C81\")"))
			}
			if limit < 0 {
				limit = 0
			}
			target, err := awwwards.ParseHex(args[0])
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("awwwards-pp-cli")
			}
			db, done := requireMirror(cmd, flags, dbPath)
			if done {
				return nil
			}
			defer db.Close()

			// Single drained query: palette rows joined to site fields.
			rows, err := db.DB().QueryContext(ctx, `
				SELECT p.slug, p.hex, s.title, s.thumbnail, s.score_overall
				FROM aw_palette p JOIN aw_sites s ON s.slug = p.slug`)
			if err != nil {
				return fmt.Errorf("querying palettes: %w", err)
			}
			type prow struct {
				slug, hex, title, thumb string
				score                   sql.NullFloat64
			}
			var all []prow
			for rows.Next() {
				var r prow
				if err := rows.Scan(&r.slug, &r.hex, &r.title, &r.thumb, &r.score); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning palette row: %w", err)
				}
				all = append(all, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}

			// Best (closest) hex per site within the distance bound.
			best := map[string]*paletteHit{}
			palettes := map[string][]string{}
			scanned := map[string]bool{}
			for _, r := range all {
				scanned[r.slug] = true
				palettes[r.slug] = append(palettes[r.slug], r.hex)
				rgb, err := awwwards.ParseHex(r.hex)
				if err != nil {
					continue
				}
				d := awwwards.Distance(target, rgb)
				if d > flagDistance {
					continue
				}
				cur, ok := best[r.slug]
				if !ok || d < cur.Distance {
					h := &paletteHit{
						Slug: r.slug, Title: r.title, MatchedHex: r.hex, Distance: d,
						ThumbnailURL: awwwards.ThumbnailURL(r.thumb, ""),
						DetailURL:    "https://www.awwwards.com/sites/" + r.slug,
					}
					if r.score.Valid {
						f := r.score.Float64
						h.ScoreOverall = &f
					}
					best[r.slug] = h
				}
			}

			items := make([]paletteHit, 0, len(best))
			for slug, h := range best {
				h.Palette = palettes[slug]
				h.Distance = round2f(h.Distance)
				items = append(items, *h)
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].Distance != items[j].Distance {
					return items[i].Distance < items[j].Distance
				}
				return items[i].Slug < items[j].Slug
			})
			if len(items) > limit {
				items = items[:limit]
			}

			res := paletteMatchResult{
				Target:       strings.ToUpper(strings.TrimPrefix(args[0], "#")),
				MaxDistance:  flagDistance,
				Items:        items,
				SitesScanned: len(scanned),
			}
			res.Target = "#" + res.Target
			if len(items) == 0 {
				res.Note = fmt.Sprintf("no palette within distance %.0f across %d sites with palette data; raise --distance or run 'awwwards-pp-cli mirror --details' to add palettes", flagDistance, len(scanned))
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}

	cmd.Flags().Float64Var(&flagDistance, "distance", 25, "Maximum RGB distance (0-441; ~25 is 'visually close')")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum sites to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

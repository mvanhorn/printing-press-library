// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
)

type packSite struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	ScoreOverall *float64 `json:"score_overall,omitempty"`
	Award        string   `json:"award,omitempty"`
	Tags         []string `json:"tags"`
	Palette      []string `json:"palette,omitempty"`
	ThumbnailURL string   `json:"thumbnail_url"`
	DetailURL    string   `json:"detail_url"`
	ExternalURL  string   `json:"external_url,omitempty"`
}

type packCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type packBenchmarks struct {
	ScoredSites   int     `json:"scored_sites"`
	AvgOverall    float64 `json:"avg_overall,omitempty"`
	P90Overall    float64 `json:"p90_overall,omitempty"`
	AvgDesign     float64 `json:"avg_design,omitempty"`
	AvgUsability  float64 `json:"avg_usability,omitempty"`
	AvgCreativity float64 `json:"avg_creativity,omitempty"`
	AvgContent    float64 `json:"avg_content,omitempty"`
}

type contextPack struct {
	Query           map[string]any `json:"query"`
	Sites           []packSite     `json:"sites"`
	DominantColors  []packCount    `json:"dominant_colors"`
	TopHexes        []packCount    `json:"top_hexes"`
	RecurringTech   []packCount    `json:"recurring_tech"`
	CoOccurringTags []packCount    `json:"co_occurring_tags"`
	Benchmarks      packBenchmarks `json:"benchmarks"`
	Matched         int            `json:"matched"`
	Note            string         `json:"note,omitempty"`
}

// slugChunkSize keeps chunked IN-clause queries under SQLite's 999
// bind-parameter cap.
const slugChunkSize = 400

func newNovelContextPackCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string
	var flagTag []string
	var flagTech string
	var flagColor string
	var limitSites int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "context-pack",
		Short: "One-shot design briefing for a build: top-scoring reference sites, dominant palettes, recurring tech, and score benchmarks",
		Long: strings.Trim(`
Context-pack assembles everything an agent needs before designing: the
top-scoring reference sites matching your brief, the dominant palette color
families and exact hexes among them, the technologies that recur, co-occurring
style tags, and jury-score benchmarks (average and p90 per dimension) to aim
for. One JSON document, machine-ready.

Filters AND together over the local mirror; run 'mirror --pages 10 --details'
first for rich results (palettes and scores come from detail data).

Use this command to assemble a complete design-context pack for one design
brief. Do NOT use this command for a single aggregate metric; use 'trends'
instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli context-pack --category e-commerce --tag dark --agent
  awwwards-pp-cli context-pack --tech gsap --json
  awwwards-pp-cli context-pack --category portfolio --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--tag=clean"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would assemble a design context pack from the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "context-pack"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if limitSites < 0 {
				limitSites = 0
			}
			if dbPath == "" {
				dbPath = defaultDBPath("awwwards-pp-cli")
			}
			db, done := requireMirror(cmd, flags, dbPath)
			if done {
				return nil
			}
			defer db.Close()

			andTags := append([]string{}, flagTag...)
			if flagCategory != "" {
				andTags = append(andTags, flagCategory)
			}
			if flagTech != "" {
				andTags = append(andTags, flagTech)
			}

			var where []string
			var queryArgs []any
			for _, t := range andTags {
				where = append(where, `s.slug IN (SELECT slug FROM aw_site_tags WHERE tag COLLATE NOCASE IN (?, ?, ?))`)
				queryArgs = append(queryArgs, t, strings.ReplaceAll(t, "-", " "), strings.ReplaceAll(t, "-", "."))
			}
			if flagColor != "" {
				hex := strings.ToUpper(strings.TrimSpace(flagColor))
				if !strings.HasPrefix(hex, "#") {
					hex = "#" + hex
				}
				where = append(where, `s.slug IN (SELECT slug FROM aw_palette WHERE hex = ?)`)
				queryArgs = append(queryArgs, hex)
			}
			whereSQL := ""
			if len(where) > 0 {
				whereSQL = " WHERE " + strings.Join(where, " AND ")
			}

			// Pass 1: matched slugs + core fields, drained fully before follow-ups.
			type siteRow struct {
				slug, title, award, thumb, ext string
				overall                        sql.NullFloat64
				design, usab, creat, content   sql.NullFloat64
			}
			rows, err := db.DB().QueryContext(ctx, `
				SELECT s.slug, s.title, s.award, s.thumbnail, s.external_url,
				       s.score_overall, s.score_design, s.score_usability, s.score_creativity, s.score_content
				FROM aw_sites s`+whereSQL+`
				ORDER BY COALESCE(s.score_overall, 0) DESC, s.created_at DESC`, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying mirror: %w", err)
			}
			var matchedRows []siteRow
			for rows.Next() {
				var r siteRow
				if err := rows.Scan(&r.slug, &r.title, &r.award, &r.thumb, &r.ext, &r.overall, &r.design, &r.usab, &r.creat, &r.content); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning site: %w", err)
				}
				matchedRows = append(matchedRows, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}

			pack := contextPack{
				Query: map[string]any{
					"category": flagCategory, "tags": flagTag, "tech": flagTech, "color": flagColor,
				},
				Matched:         len(matchedRows),
				Sites:           []packSite{},
				DominantColors:  []packCount{},
				TopHexes:        []packCount{},
				RecurringTech:   []packCount{},
				CoOccurringTags: []packCount{},
			}
			if len(matchedRows) == 0 {
				pack.Note = "no matches; widen the mirror ('awwwards-pp-cli mirror --pages 10 --details') or relax filters"
				return printJSONFiltered(cmd.OutOrStdout(), pack, flags)
			}

			// Aggregations over all matched slugs (tags, palettes, benchmarks).
			querySet := map[string]bool{}
			for _, t := range andTags {
				querySet[strings.ToLower(t)] = true
				querySet[strings.ToLower(strings.ReplaceAll(t, "-", " "))] = true
				querySet[strings.ToLower(strings.ReplaceAll(t, "-", "."))] = true
			}
			tagCounts := map[string]int{}
			techCounts := map[string]int{}
			hexCounts := map[string]int{}
			hueCounts := map[string]int{}
			var overalls []float64
			var sums scores4
			var scored int

			slugSet := make([]string, 0, len(matchedRows))
			for _, r := range matchedRows {
				slugSet = append(slugSet, r.slug)
				if r.overall.Valid {
					overalls = append(overalls, r.overall.Float64)
					sums.design += r.design.Float64
					sums.usab += r.usab.Float64
					sums.creat += r.creat.Float64
					sums.content += r.content.Float64
					scored++
				}
			}

			// Tag co-occurrence across matched slugs (chunked IN query).
			for start := 0; start < len(slugSet); start += slugChunkSize {
				end := start + slugChunkSize
				if end > len(slugSet) {
					end = len(slugSet)
				}
				chunk := slugSet[start:end]
				ph := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
				cargs := make([]any, len(chunk))
				for i, s := range chunk {
					cargs[i] = s
				}
				trows, err := db.DB().QueryContext(ctx, `SELECT tag, COUNT(*) FROM aw_site_tags WHERE slug IN (`+ph+`) GROUP BY tag`, cargs...)
				if err != nil {
					return fmt.Errorf("aggregating tags: %w", err)
				}
				for trows.Next() {
					var tag string
					var n int
					if err := trows.Scan(&tag, &n); err != nil {
						_ = trows.Close()
						return fmt.Errorf("scanning tag: %w", err)
					}
					if awwwards.IsTech(tag) {
						techCounts[tag] += n
					} else if !querySet[strings.ToLower(tag)] {
						tagCounts[tag] += n
					}
				}
				if err := trows.Err(); err != nil {
					_ = trows.Close()
					return err
				}
				if err := trows.Close(); err != nil {
					return err
				}

				prows, err := db.DB().QueryContext(ctx, `SELECT hex, COUNT(*) FROM aw_palette WHERE slug IN (`+ph+`) GROUP BY hex`, cargs...)
				if err != nil {
					return fmt.Errorf("aggregating palettes: %w", err)
				}
				for prows.Next() {
					var hex string
					var n int
					if err := prows.Scan(&hex, &n); err != nil {
						_ = prows.Close()
						return fmt.Errorf("scanning hex: %w", err)
					}
					hexCounts[hex] += n
					if rgb, err := awwwards.ParseHex(hex); err == nil {
						hueCounts[awwwards.HueFamily(rgb)] += n
					}
				}
				if err := prows.Err(); err != nil {
					_ = prows.Close()
					return err
				}
				if err := prows.Close(); err != nil {
					return err
				}
			}

			// Top-N reference sites with their tags and palettes.
			topN := matchedRows
			if len(topN) > limitSites {
				topN = topN[:limitSites]
			}
			for _, r := range topN {
				ps := packSite{
					Slug: r.slug, Title: r.title, Award: r.award,
					ThumbnailURL: awwwards.ThumbnailURL(r.thumb, ""),
					DetailURL:    "https://www.awwwards.com/sites/" + r.slug,
					ExternalURL:  r.ext,
					Tags:         []string{},
				}
				if r.overall.Valid {
					f := r.overall.Float64
					ps.ScoreOverall = &f
				}
				siteTags, err := queryStrings(ctx, db, `SELECT tag FROM aw_site_tags WHERE slug = ?`, r.slug)
				if err != nil {
					return fmt.Errorf("reading tags for %q: %w", r.slug, err)
				}
				ps.Tags = append(ps.Tags, siteTags...)
				sitePalette, err := queryStrings(ctx, db, `SELECT hex FROM aw_palette WHERE slug = ? ORDER BY position`, r.slug)
				if err != nil {
					return fmt.Errorf("reading palette for %q: %w", r.slug, err)
				}
				ps.Palette = sitePalette
				pack.Sites = append(pack.Sites, ps)
			}

			pack.DominantColors = topCounts(hueCounts, 8)
			pack.TopHexes = topCounts(hexCounts, 10)
			pack.RecurringTech = topCounts(techCounts, 10)
			pack.CoOccurringTags = topCounts(tagCounts, 15)

			pack.Benchmarks.ScoredSites = scored
			if scored > 0 {
				sort.Float64s(overalls)
				var sum float64
				for _, v := range overalls {
					sum += v
				}
				pack.Benchmarks.AvgOverall = round2f(sum / float64(scored))
				pack.Benchmarks.P90Overall = round2f(overalls[int(math.Min(float64(len(overalls)-1), math.Ceil(0.9*float64(len(overalls)))-1))])
				pack.Benchmarks.AvgDesign = round2f(sums.design / float64(scored))
				pack.Benchmarks.AvgUsability = round2f(sums.usab / float64(scored))
				pack.Benchmarks.AvgCreativity = round2f(sums.creat / float64(scored))
				pack.Benchmarks.AvgContent = round2f(sums.content / float64(scored))
			} else {
				pack.Note = "no matched sites have jury scores yet; run 'awwwards-pp-cli mirror --details' to enrich benchmarks and palettes"
			}

			return printJSONFiltered(cmd.OutOrStdout(), pack, flags)
		},
	}

	cmd.Flags().StringVar(&flagCategory, "category", "", "Category tag to require (e.g. e-commerce, portfolio)")
	cmd.Flags().StringArrayVar(&flagTag, "tag", nil, "Style tag to require (repeatable, ANDed)")
	cmd.Flags().StringVar(&flagTech, "tech", "", "Technology tag to require (e.g. gsap, webflow)")
	cmd.Flags().StringVar(&flagColor, "color", "", "Exact palette hex to require (needs 'mirror --details')")
	cmd.Flags().IntVar(&limitSites, "limit-sites", 10, "Reference sites to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// scores4 accumulates dimension sums for benchmark averages.
type scores4 struct {
	design, usab, creat, content float64
}

func topCounts(m map[string]int, n int) []packCount {
	out := make([]packCount, 0, len(m))
	for k, v := range m {
		out = append(out, packCount{Name: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func round2f(f float64) float64 {
	return math.Round(f*100) / 100
}

// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
)

type findResult struct {
	Items       []cardView `json:"items"`
	Matched     int        `json:"matched"`
	MirrorTotal int        `json:"mirror_total"`
	Note        string     `json:"note,omitempty"`
}

func newFindCmd(flags *rootFlags) *cobra.Command {
	var tags []string
	var category string
	var tech string
	var text string
	var award string
	var color string
	var country string
	var font string
	var filterSeen string
	var minScore float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Multi-filter search across the local design mirror - the AND-intersection Awwwards cannot do",
		Long: strings.Trim(`
Find queries the local design mirror with any combination of filters. The
Awwwards site allows exactly ONE filter at a time; find intersects them all:
tags AND category AND tech AND color AND award AND text, in one query.

--tag/--category/--tech all match the merged card tag list (Awwwards mixes
styles, categories, and technologies in one tag vocabulary). --color matches
extracted palette hexes and requires detail data ('mirror --details').
--country and --font match sites first seen under that listing filter, so
mirror the filter first: 'mirror --filter france' or 'mirror --filter Aeonik'.

Use this command for filtered queries over mirrored data. Do NOT use it for
exact server-side single-filter browsing; use 'websites browse' instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli find --tag dark --category e-commerce --json
  awwwards-pp-cli find --tech gsap --min-score 7.5 --limit 10
  awwwards-pp-cli find --color "#0F4C81" --json
  awwwards-pp-cli find --text studio --award SOTD
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--tag=clean;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query the local design mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "find"); err != nil {
				return err
			}
			filterFlags := 0
			for _, v := range []string{country, font, filterSeen} {
				if v != "" {
					filterFlags++
				}
			}
			if filterFlags > 1 {
				return usageErr(fmt.Errorf("--country, --font, and --filter all match the same mirrored-filter column; use only one per query"))
			}
			if limit < 0 {
				limit = 0
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

			// Every tag-ish filter ANDs into the same tag-membership set.
			andTags := append([]string{}, tags...)
			if category != "" {
				andTags = append(andTags, category)
			}
			if tech != "" {
				andTags = append(andTags, tech)
			}

			var where []string
			var queryArgs []any
			for _, t := range andTags {
				where = append(where, `s.slug IN (SELECT slug FROM aw_site_tags WHERE tag COLLATE NOCASE IN (?, ?, ?))`)
				// Accept display labels ("Flat Design"), filter slugs ("flat-design"),
				// and dotted tech names via their slug form ("three-js" -> "Three.js").
				queryArgs = append(queryArgs, t, strings.ReplaceAll(t, "-", " "), strings.ReplaceAll(t, "-", "."))
			}
			if text != "" {
				where = append(where, `(s.title LIKE '%' || ? || '%' OR s.slug LIKE '%' || ? || '%')`)
				queryArgs = append(queryArgs, text, text)
			}
			if award != "" {
				where = append(where, `s.award LIKE '%' || ? || '%'`)
				queryArgs = append(queryArgs, award)
			}
			if color != "" {
				hex := strings.ToUpper(strings.TrimSpace(color))
				if !strings.HasPrefix(hex, "#") {
					hex = "#" + hex
				}
				where = append(where, `s.slug IN (SELECT slug FROM aw_palette WHERE hex = ?)`)
				queryArgs = append(queryArgs, hex)
			}
			if country != "" {
				where = append(where, `s.first_seen_filter = ? COLLATE NOCASE`)
				queryArgs = append(queryArgs, country)
			}
			if font != "" {
				where = append(where, `s.first_seen_filter = ? COLLATE NOCASE`)
				queryArgs = append(queryArgs, font)
			}
			if filterSeen != "" {
				where = append(where, `s.first_seen_filter = ? COLLATE NOCASE`)
				queryArgs = append(queryArgs, filterSeen)
			}
			if minScore > 0 {
				where = append(where, `s.score_overall >= ?`)
				queryArgs = append(queryArgs, minScore)
			}

			q := `SELECT s.slug, s.title, s.created_at, s.thumbnail, s.score_overall,
			             COALESCE((SELECT GROUP_CONCAT(tag, '||') FROM aw_site_tags t WHERE t.slug = s.slug), '')
			      FROM aw_sites s`
			if len(where) > 0 {
				q += " WHERE " + strings.Join(where, " AND ")
			}
			q += ` ORDER BY COALESCE(s.score_overall, 0) DESC, s.created_at DESC LIMIT ?`
			queryArgs = append(queryArgs, limit)

			rows, err := db.DB().QueryContext(ctx, q, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying mirror: %w", err)
			}
			items := make([]cardView, 0)
			for rows.Next() {
				var v cardView
				var thumb, tagsCat string
				var score sql.NullFloat64
				if err := rows.Scan(&v.Slug, &v.Title, &v.CreatedAt, &thumb, &score, &tagsCat); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning row: %w", err)
				}
				v.ThumbnailURL = awwwards.ThumbnailURL(thumb, "")
				v.DetailURL = "https://www.awwwards.com/sites/" + v.Slug
				if tagsCat != "" {
					v.Tags = strings.Split(tagsCat, "||")
				} else {
					v.Tags = []string{}
				}
				if score.Valid {
					f := score.Float64
					v.ScoreOverall = &f
				}
				items = append(items, v)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing rows: %w", err)
			}

			var total int
			if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM aw_sites`).Scan(&total); err != nil {
				return fmt.Errorf("counting mirror: %w", err)
			}

			res := findResult{Items: items, Matched: len(items), MirrorTotal: total}
			if len(items) == 0 {
				res.Note = fmt.Sprintf("no matches among %d mirrored sites; widen the mirror ('awwwards-pp-cli mirror --pages 10 --details') or relax filters", total)
			}
			if (color != "" || minScore > 0) && len(items) == 0 {
				res.Note += " (score/color filters need detail data: mirror --details)"
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}

	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag to require (repeatable, ANDed; matches labels or slugs)")
	cmd.Flags().StringVar(&category, "category", "", "Category tag to require (e.g. e-commerce, design-agencies)")
	cmd.Flags().StringVar(&tech, "tech", "", "Technology tag to require (e.g. gsap, webflow, three-js)")
	cmd.Flags().StringVar(&text, "text", "", "Substring match on title or slug")
	cmd.Flags().StringVar(&award, "award", "", "Award tier substring (SOTD, SOTM, Nominee, Honorable)")
	cmd.Flags().StringVar(&color, "color", "", "Exact palette hex to require (needs 'mirror --details')")
	cmd.Flags().StringVar(&country, "country", "", "Country filter the site was mirrored under (mirror --filter <country> first; exclusive with --font/--filter)")
	cmd.Flags().StringVar(&font, "font", "", "Font filter the site was mirrored under (mirror --filter <Font> first; exclusive with --country/--filter)")
	cmd.Flags().StringVar(&filterSeen, "filter", "", "Any listing filter the site was first mirrored under (exclusive with --country/--font)")
	cmd.Flags().Float64Var(&minScore, "min-score", 0, "Minimum overall jury score (needs 'mirror --details')")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

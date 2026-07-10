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

type studioSite struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Award        string   `json:"award,omitempty"`
	ScoreOverall *float64 `json:"score_overall,omitempty"`
	DetailURL    string   `json:"detail_url"`
}

type studioProfile struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	ProfileURL  string `json:"profile_url"`
	// Wins counts every credited mirrored entry, including nominees and
	// honorable mentions (see ByAward for the tier breakdown).
	Wins          int            `json:"wins"`
	ByAward       map[string]int `json:"by_award"`
	AvgOverall    float64        `json:"avg_overall,omitempty"`
	AvgDesign     float64        `json:"avg_design,omitempty"`
	AvgUsability  float64        `json:"avg_usability,omitempty"`
	AvgCreativity float64        `json:"avg_creativity,omitempty"`
	AvgContent    float64        `json:"avg_content,omitempty"`
	ScoredSites   int            `json:"scored_sites"`
	TopTags       []packCount    `json:"top_tags"`
	TopTech       []packCount    `json:"top_tech"`
	Sites         []studioSite   `json:"sites"`
	Note          string         `json:"note,omitempty"`
}

func newNovelStudioCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "studio <name>",
		Short: "Aggregated award profile of one agency or studio: wins by tier, average dimension scores, and dominant tags and tech",
		Long: strings.Trim(`
Studio aggregates one maker's footprint across the local mirror via detail-page
credits: how many mirrored entries credit them (all tiers, including nominees), their average jury
scores per dimension, and the tags and technologies that recur in their work.

Credits come from detail pages, so run 'mirror --details' first. The name
matches a profile username exactly or a display name case-insensitively.

Use this command for an aggregated award profile of one agency or studio.
Do NOT use this command for browsing or filtering the agency listing; use
'directory browse' instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli studio monolog --json
  awwwards-pp-cli studio "Huy Nguyen" --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "name=monolog"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate one studio's award profile from the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "studio"); err != nil {
				return err
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a studio/maker name or username is required"))
			}
			name := strings.TrimSpace(args[0])
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

			// Resolve identity: exact username first, then display name.
			var username, displayName string
			err := db.DB().QueryRowContext(ctx, `
				SELECT username, display_name FROM aw_credits
				WHERE username = ? COLLATE NOCASE OR display_name = ? COLLATE NOCASE
				ORDER BY username LIMIT 1`, name, name).Scan(&username, &displayName)
			if err == sql.ErrNoRows {
				// Fuzzy fallback on display name.
				escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(name)
				err = db.DB().QueryRowContext(ctx, `
					SELECT username, display_name FROM aw_credits
					WHERE display_name LIKE '%' || ? || '%' ESCAPE '\'
					ORDER BY username LIMIT 1`, escaped).Scan(&username, &displayName)
			}
			if err == sql.ErrNoRows {
				var creditCount int
				if cErr := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM aw_credits`).Scan(&creditCount); cErr != nil {
					return fmt.Errorf("counting credits: %w", cErr)
				}
				return notFoundErr(fmt.Errorf("no maker matching %q among %d mirrored credits; credits come from detail pages - run 'awwwards-pp-cli mirror --details' to widen coverage", name, creditCount))
			}
			if err != nil {
				return fmt.Errorf("resolving maker: %w", err)
			}

			// Drain the site list fully before aggregation follow-ups.
			rows, err := db.DB().QueryContext(ctx, `
				SELECT s.slug, s.title, s.award, s.score_overall, s.score_design, s.score_usability, s.score_creativity, s.score_content
				FROM aw_credits c JOIN aw_sites s ON s.slug = c.slug
				WHERE c.username = ?
				ORDER BY COALESCE(s.score_overall, 0) DESC, s.created_at DESC`, username)
			if err != nil {
				return fmt.Errorf("querying studio sites: %w", err)
			}
			type srow struct {
				slug, title, award   string
				overall, d, u, c, ct sql.NullFloat64
			}
			var srows []srow
			for rows.Next() {
				var r srow
				if err := rows.Scan(&r.slug, &r.title, &r.award, &r.overall, &r.d, &r.u, &r.c, &r.ct); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning studio site: %w", err)
				}
				srows = append(srows, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}

			p := studioProfile{
				Username:    username,
				DisplayName: displayName,
				ProfileURL:  "https://www.awwwards.com/" + username + "/",
				Wins:        len(srows),
				ByAward:     map[string]int{},
				TopTags:     []packCount{},
				TopTech:     []packCount{},
				Sites:       []studioSite{},
			}
			var sums scores4
			var sumOverall float64
			tagCounts := map[string]int{}
			techCounts := map[string]int{}
			for _, r := range srows {
				aw := r.award
				if aw == "" {
					aw = "unscored/unknown"
				}
				p.ByAward[aw]++
				ss := studioSite{Slug: r.slug, Title: r.title, Award: r.award, DetailURL: "https://www.awwwards.com/sites/" + r.slug}
				if r.overall.Valid {
					f := r.overall.Float64
					ss.ScoreOverall = &f
					sumOverall += f
					sums.design += r.d.Float64
					sums.usab += r.u.Float64
					sums.creat += r.c.Float64
					sums.content += r.ct.Float64
					p.ScoredSites++
				}
				if len(p.Sites) < limit {
					p.Sites = append(p.Sites, ss)
				}

				siteTags, err := queryStrings(ctx, db, `SELECT tag FROM aw_site_tags WHERE slug = ?`, r.slug)
				if err != nil {
					return fmt.Errorf("reading tags for %q: %w", r.slug, err)
				}
				for _, t := range siteTags {
					if awwwards.IsTech(t) {
						techCounts[t]++
					} else {
						tagCounts[t]++
					}
				}
			}

			if p.ScoredSites > 0 {
				n := float64(p.ScoredSites)
				p.AvgOverall = round2f(sumOverall / n)
				p.AvgDesign = round2f(sums.design / n)
				p.AvgUsability = round2f(sums.usab / n)
				p.AvgCreativity = round2f(sums.creat / n)
				p.AvgContent = round2f(sums.content / n)
			} else {
				p.Note = "no scored sites for this maker in the mirror; run 'awwwards-pp-cli mirror --details' to sync jury scores"
			}
			p.TopTags = topCounts(tagCounts, 10)
			p.TopTech = topCounts(techCounts, 10)

			return printJSONFiltered(cmd.OutOrStdout(), p, flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum sites to list in the profile")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

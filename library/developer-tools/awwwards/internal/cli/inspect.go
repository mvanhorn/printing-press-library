// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
)

func newInspectCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "inspect <slug>",
		Short: "Deep-dive one winner: jury scores by dimension, individual jury votes, color palette, tech stack, credits",
		Long: strings.Trim(`
Inspect fetches a site's detail page live, parses the full design profile
(overall + per-dimension jury scores, every juror's votes with country, the
extracted color palette, Technologies & Tools tags, and maker credits), stores
it in the local mirror, and prints it.

With --data-source local it reads the cached profile from the mirror without
touching the network.

Use this command for one site's full design profile. Do NOT use it for raw
page metadata; use 'sites get' instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli inspect monolog --json
  awwwards-pp-cli inspect monolog --data-source local --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "slug=monolog"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and parse one site detail page")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a site slug is required (find one via 'latest' or 'find')"))
			}
			slug := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("awwwards-pp-cli")
			}

			if flags.dataSource == "local" {
				d, err := readLocalDetail(ctx, cmd, flags, dbPath, slug)
				if err != nil {
					return err
				}
				if d == nil {
					return nil // requireMirror already reported
				}
				return printJSONFiltered(cmd.OutOrStdout(), d, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			d, err := fetchDetail(ctx, c, slug)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if d.Title == "" && len(d.Palette) == 0 && len(d.Jury) == 0 {
				return notFoundErr(fmt.Errorf("no design profile found at /sites/%s (unknown slug?)", slug))
			}

			// Persist best-effort so analytics commands benefit from every inspect.
			if db, dbErr := openMirror(ctx, dbPath); dbErr == nil {
				if upErr := db.UpsertAwDetail(ctx, d); upErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: mirror write failed: %v\n", upErr)
				}
				_ = db.Close()
			}

			return printJSONFiltered(cmd.OutOrStdout(), d, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// readLocalDetail loads a cached design profile from the mirror.
func readLocalDetail(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath, slug string) (*awwwards.Detail, error) {
	db, done := requireMirror(cmd, flags, dbPath)
	if done {
		return nil, nil
	}
	defer db.Close()

	d := awwwards.Detail{Slug: slug}
	var title, extURL, award string
	var so, sd, su, sc, sct sql.NullFloat64
	var detailSynced int64
	err := db.DB().QueryRowContext(ctx, `
		SELECT title, external_url, award, score_overall, score_design, score_usability, score_creativity, score_content, detail_synced_at
		FROM aw_sites WHERE slug = ?`, slug).
		Scan(&title, &extURL, &award, &so, &sd, &su, &sc, &sct, &detailSynced)
	if err == sql.ErrNoRows {
		return nil, notFoundErr(fmt.Errorf("%q is not in the local mirror; run 'awwwards-pp-cli inspect %s' (live) or 'mirror --details'", slug, slug))
	}
	if err != nil {
		return nil, fmt.Errorf("reading site: %w", err)
	}
	if detailSynced == 0 {
		return nil, notFoundErr(fmt.Errorf("%q is mirrored but has no cached detail; run 'awwwards-pp-cli inspect %s' (live) to fetch it", slug, slug))
	}
	d.Title, d.ExternalURL, d.Award = title, extURL, award
	d.Scores = awwwards.Scores{Overall: so.Float64, Design: sd.Float64, Usability: su.Float64, Creativity: sc.Float64, Content: sct.Float64}

	// Drain each result set fully before the next query (single SQLite conn).
	palette, err := queryStrings(ctx, db, `SELECT hex FROM aw_palette WHERE slug = ? ORDER BY position`, slug)
	if err != nil {
		return nil, fmt.Errorf("reading palette: %w", err)
	}
	d.Palette = palette

	jrows, err := db.DB().QueryContext(ctx, `SELECT juror, profile, country, design, usability, creativity, content, overall FROM aw_jury WHERE slug = ?`, slug)
	if err != nil {
		return nil, fmt.Errorf("reading jury: %w", err)
	}
	for jrows.Next() {
		var v awwwards.JuryVote
		var ds, us, cs, ct, ov sql.NullFloat64
		if err := jrows.Scan(&v.Name, &v.Profile, &v.Country, &ds, &us, &cs, &ct, &ov); err != nil {
			_ = jrows.Close()
			return nil, fmt.Errorf("scanning jury: %w", err)
		}
		v.Scores = []float64{ds.Float64, us.Float64, cs.Float64, ct.Float64, ov.Float64}
		d.Jury = append(d.Jury, v)
	}
	if err := jrows.Err(); err != nil {
		_ = jrows.Close()
		return nil, err
	}
	_ = jrows.Close()

	tags, err := queryStrings(ctx, db, `SELECT tag FROM aw_site_tags WHERE slug = ?`, slug)
	if err != nil {
		return nil, fmt.Errorf("reading tags: %w", err)
	}
	for _, t := range tags {
		d.Tags = append(d.Tags, awwwards.Tag{Slug: awwwards.TagFilterSlug(t), Label: t})
	}

	crows, err := db.DB().QueryContext(ctx, `SELECT username, display_name FROM aw_credits WHERE slug = ?`, slug)
	if err != nil {
		return nil, fmt.Errorf("reading credits: %w", err)
	}
	for crows.Next() {
		var c awwwards.Credit
		if err := crows.Scan(&c.Username, &c.DisplayName); err != nil {
			_ = crows.Close()
			return nil, fmt.Errorf("scanning credit: %w", err)
		}
		d.Credits = append(d.Credits, c)
	}
	if err := crows.Err(); err != nil {
		_ = crows.Close()
		return nil, err
	}
	_ = crows.Close()

	return &d, nil
}

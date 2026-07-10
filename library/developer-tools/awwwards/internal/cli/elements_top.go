// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
)

type elementHit struct {
	Rank         int      `json:"rank"`
	ElementTitle string   `json:"element_title"`
	Type         string   `json:"type"`
	ImageURL     string   `json:"image_url"`
	Tags         []string `json:"tags"`
	ParentSlug   string   `json:"parent_slug"`
	ParentTitle  string   `json:"parent_title"`
	ParentScore  float64  `json:"parent_score"`
	ScoreSource  string   `json:"score_source"` // "jury" (final award score) or "votes" (avg of captured votes; nominees/mentions)
	DetailURL    string   `json:"detail_url"`
}

type elementsTopResult struct {
	Type          string       `json:"type"`
	Dimension     string       `json:"dimension"`
	MinScore      float64      `json:"min_score"`
	Items         []elementHit `json:"items"`
	ElementsTotal int          `json:"elements_of_type_in_mirror"`
	Unjoined      int          `json:"elements_without_scored_parent"`
	Note          string       `json:"note,omitempty"`
}

func newNovelElementsTopCmd(flags *rootFlags) *cobra.Command {
	var flagDim string
	var flagMin float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "elements-top <type>",
		Short: "Section-level inspiration ranked by quality: heroes, footers, or 404 pages from sites whose jury score clears your bar",
		Long: strings.Trim(`
Elements-top joins mirrored section screenshots (/elements/) to their parent
site's jury scores and ranks them - "show me only the heroes from sites that
scored 8+ on design". The Awwwards elements wall is flat and unranked; the
ranking exists only in the local mirror.

Feed it first: 'mirror --elements hero,footer --details' (elements need their
parent sites mirrored WITH detail scores to rank).

Use this command to rank section screenshots by their parent site's jury
score from the local mirror. Do NOT use this command for a raw live listing
of a section type; use 'elements browse' instead.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli elements-top hero --dim design --min 8 --json
  awwwards-pp-cli elements-top footer --limit 5
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "type=hero;--dim=overall", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank mirrored elements by parent-site jury score")
				return nil
			}
			if err := rejectLiveDataSource(flags, "elements-top"); err != nil {
				return err
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an element type is required (hero, footer, 404_page, ...)"))
			}
			etype := strings.TrimSpace(args[0])
			col, err := scoreColumn(flagDim)
			if err != nil {
				return err
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

			// Official jury score when the award is final; average of captured
			// votes otherwise (nominees and honorable mentions render votes only).
			jcol := strings.TrimPrefix(col, "score_")
			rows, err := db.DB().QueryContext(ctx, `
				SELECT title, image, tags_json, parent_slug, parent_title, eff, src FROM (
					SELECT e.title AS title, e.image AS image, e.tags_json AS tags_json,
					       e.parent_slug AS parent_slug, s.title AS parent_title,
					       COALESCE(s.`+col+`, (SELECT AVG(j.`+jcol+`) FROM aw_jury j WHERE j.slug = s.slug)) AS eff,
					       CASE WHEN s.`+col+` IS NOT NULL THEN 'jury' ELSE 'votes' END AS src
					FROM aw_elements e
					JOIN aw_sites s ON s.slug = e.parent_slug
					WHERE e.etype = ?
				) WHERE eff IS NOT NULL AND eff >= ?
				ORDER BY eff DESC
				LIMIT ?`, etype, flagMin, limit)
			if err != nil {
				return fmt.Errorf("querying elements: %w", err)
			}
			items := make([]elementHit, 0)
			for rows.Next() {
				var h elementHit
				var image, tagsJSON string
				var score sql.NullFloat64
				if err := rows.Scan(&h.ElementTitle, &image, &tagsJSON, &h.ParentSlug, &h.ParentTitle, &score, &h.ScoreSource); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning element: %w", err)
				}
				h.Type = etype
				h.Rank = len(items) + 1
				h.ParentScore = round2f(score.Float64)
				h.ImageURL = awwwards.ThumbnailURL(image, "")
				h.DetailURL = "https://www.awwwards.com/sites/" + h.ParentSlug
				if err := json.Unmarshal([]byte(tagsJSON), &h.Tags); err != nil || h.Tags == nil {
					h.Tags = []string{}
				}
				items = append(items, h)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}

			var total, joined int
			if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM aw_elements WHERE etype = ?`, etype).Scan(&total); err != nil {
				return fmt.Errorf("counting elements: %w", err)
			}
			if err := db.DB().QueryRowContext(ctx, `
				SELECT COUNT(*) FROM aw_elements e JOIN aw_sites s ON s.slug = e.parent_slug
				WHERE e.etype = ? AND COALESCE(s.`+col+`, (SELECT AVG(j.`+jcol+`) FROM aw_jury j WHERE j.slug = s.slug)) IS NOT NULL`, etype).Scan(&joined); err != nil {
				return fmt.Errorf("counting joined elements: %w", err)
			}

			res := elementsTopResult{
				Type: etype, Dimension: strings.ToLower(flagDim), MinScore: flagMin,
				Items: items, ElementsTotal: total, Unjoined: total - joined,
			}
			if res.Dimension == "" {
				res.Dimension = "overall"
			}
			if total == 0 {
				res.Note = fmt.Sprintf("no %q elements mirrored yet; run 'awwwards-pp-cli mirror --elements %s' first", etype, etype)
			} else if len(items) == 0 {
				res.Note = fmt.Sprintf("%d %q elements mirrored but none have a scored parent clearing %.1f; run 'awwwards-pp-cli mirror --details' to score parents or lower --min", total, etype, flagMin)
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}

	cmd.Flags().StringVar(&flagDim, "dim", "overall", "Parent score dimension: design, usability, creativity, content, or overall")
	cmd.Flags().Float64Var(&flagMin, "min", 0, "Minimum parent-site score")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum elements to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

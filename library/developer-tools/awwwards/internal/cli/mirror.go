// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/cliutil"
)

// elementTypeRe keeps user-supplied element types path-safe (the segment is
// concatenated into the request URL).
var elementTypeRe = regexp.MustCompile(`^[a-z0-9_-]+$`)

type mirrorFailure struct {
	Target string `json:"target"`
	Error  string `json:"error"`
}

type mirrorSummary struct {
	PagesFetched     int             `json:"pages_fetched"`
	CardsMirrored    int             `json:"cards_mirrored"`
	DetailsSynced    int             `json:"details_synced"`
	ElementsMirrored int             `json:"elements_mirrored"`
	Filter           string          `json:"filter,omitempty"`
	FetchFailures    []mirrorFailure `json:"fetch_failures,omitempty"`
}

func newMirrorCmd(flags *rootFlags) *cobra.Command {
	var pages int
	var filter string
	var text string
	var details bool
	var maxDetails int
	var elementTypes string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Mirror award cards (and optionally detail scores and palettes) into the local design database",
		Long: strings.Trim(`
Mirror fetches awwwards.com listing pages, parses the embedded card data
(title, tags, dates, thumbnails), and persists it into typed local SQLite
tables. With --details it also fetches per-site detail pages to capture jury
scores, color palettes, credits, and tech tags.

The mirror feeds every analytics command: find, top, trends, context-pack,
palette-match, elements-top, and studio. Run it before those commands, and
re-run it periodically to keep the mirror current.

Use --filter to mirror a filtered listing (award tier, category, tag, country,
font, or URL-encoded hex color) - the filter is remembered per site so 'find'
can select on it later.
`, "\n"),
		Example: strings.Trim(`
  awwwards-pp-cli mirror --pages 5
  awwwards-pp-cli mirror --pages 3 --details --max-details 20
  awwwards-pp-cli mirror --filter sites_of_the_day --pages 2
  awwwards-pp-cli mirror --elements hero,footer
`, "\n"),
		Annotations: map[string]string{"pp:happy-args": "--pages=1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would mirror %d listing page(s) (filter=%q, details=%v, elements=%q)\n", pages, filter, details, elementTypes)
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if pages < 1 {
				pages = 1
			}
			if maxDetails < 0 {
				maxDetails = 0
			}
			if cliutil.IsDogfoodEnv() {
				if pages > 1 {
					pages = 1
				}
				if maxDetails > 2 {
					maxDetails = 2
				}
			}

			if dbPath == "" {
				dbPath = defaultDBPath("awwwards-pp-cli")
			}
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			sum := mirrorSummary{Filter: filter}

			for page := 1; page <= pages; page++ {
				cards, err := fetchListingCards(ctx, c, filter, page, text)
				if err != nil {
					sum.FetchFailures = append(sum.FetchFailures, mirrorFailure{
						Target: fmt.Sprintf("listing page %d", page), Error: err.Error(),
					})
					break
				}
				if len(cards) == 0 {
					break
				}
				if err := db.UpsertAwCards(ctx, cards, filter); err != nil {
					return fmt.Errorf("storing page %d: %w", page, err)
				}
				sum.PagesFetched++
				sum.CardsMirrored += len(cards)
			}

			if elementTypes != "" {
				types := strings.Split(elementTypes, ",")
				if cliutil.IsDogfoodEnv() && len(types) > 1 {
					types = types[:1]
				}
				for _, t := range types {
					t = strings.TrimSpace(t)
					if t == "" {
						continue
					}
					if !elementTypeRe.MatchString(t) {
						return usageErr(fmt.Errorf("invalid element type %q: want lowercase letters, digits, hyphens, or underscores (e.g. hero, footer, 404_page)", t))
					}
					raw, err := c.Get(ctx, "/elements/"+t+"/", nil)
					if err != nil {
						sum.FetchFailures = append(sum.FetchFailures, mirrorFailure{Target: "elements " + t, Error: err.Error()})
						continue
					}
					cards := awwwards.ParseCards(string(raw))
					if err := db.UpsertAwElements(ctx, cards, t); err != nil {
						return fmt.Errorf("storing elements %q: %w", t, err)
					}
					sum.ElementsMirrored += len(cards)
				}
			}

			if details {
				// Listing slugs first, then element parents (so elements-top can
				// rank against scored parents), sharing one --max-details budget.
				slugs, err := db.SlugsNeedingDetail(ctx, maxDetails)
				if err != nil {
					return err
				}
				if remaining := maxDetails - len(slugs); remaining > 0 {
					parents, err := db.ElementParentsNeedingDetail(ctx, remaining)
					if err != nil {
						return err
					}
					slugs = append(slugs, parents...)
				}
				for _, slug := range slugs {
					d, err := fetchDetail(ctx, c, slug)
					if err != nil {
						sum.FetchFailures = append(sum.FetchFailures, mirrorFailure{Target: "detail " + slug, Error: err.Error()})
						continue
					}
					// A 200 page that parses to nothing (interstitial, soft-404,
					// redesigned markup) must not mark the slug detail-synced.
					if d.Title == "" && len(d.Palette) == 0 && len(d.Jury) == 0 {
						sum.FetchFailures = append(sum.FetchFailures, mirrorFailure{Target: "detail " + slug, Error: "page parsed to empty detail (markup change or interstitial); not marked synced"})
						continue
					}
					if err := db.UpsertAwDetail(ctx, d); err != nil {
						return fmt.Errorf("storing detail %q: %w", slug, err)
					}
					sum.DetailsSynced++
				}
			}

			if len(sum.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d fetch(es) failed; mirror totals cover only the successful fetches\n", len(sum.FetchFailures))
			}
			if sum.FetchFailures == nil {
				sum.FetchFailures = []mirrorFailure{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(sum)
		},
	}

	cmd.Flags().IntVar(&pages, "pages", 3, "Listing pages to mirror (30 cards per page)")
	cmd.Flags().StringVar(&filter, "filter", "", "Listing filter segment: award tier, category, tag, country, font, or URL-encoded hex color")
	cmd.Flags().StringVar(&text, "text", "", "Free-text search to mirror results for")
	cmd.Flags().BoolVar(&details, "details", false, "Also fetch detail pages (scores, palettes, jury, credits) for mirrored sites lacking them")
	cmd.Flags().IntVar(&maxDetails, "max-details", 30, "Maximum detail pages to fetch per run")
	cmd.Flags().StringVar(&elementTypes, "elements", "", "Comma-separated element types to mirror (hero, footer, 404_page, ...)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the standard local store)")
	return cmd
}

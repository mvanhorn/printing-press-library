// `search` is the cross-city, FTS-aware wrapper around sapi search. It
// fans out across multiple sites in parallel via cliutil.FanoutRun, then
// applies the absorb-manifest's "negative keyword" smart-search filter on
// the merged result. The CL native search has no NOT-keyword support.
//
// This is intentionally distinct from the generated `postings` shortcut,
// which only proxies sapi for one site without --negate or fanout.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"

	"github.com/spf13/cobra"
)

// searchHit is the typed shape we emit from `search`. Mirrors craigslist.Listing
// but adds a Site tag so cross-city callers can group without a second call.
type searchHit struct {
	Site         string  `json:"site"`
	PostingID    int64   `json:"postingId"`
	UUID         string  `json:"uuid"`
	Title        string  `json:"title"`
	Price        int     `json:"price"`
	PriceDisplay string  `json:"priceDisplay,omitempty"`
	PostedAt     int64   `json:"postedAt,omitempty"`
	Subarea      string  `json:"subarea,omitempty"`
	Neighborhood string  `json:"neighborhood,omitempty"`
	Latitude     float64 `json:"lat,omitempty"`
	Longitude    float64 `json:"lng,omitempty"`
	URL          string  `json:"url,omitempty"`
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var site, sites, category, postal, sort, negate string
	var minPrice, maxPrice, distanceMi, page, limit int
	var hasPic, titleOnly bool
	var postedSince string

	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "Cross-city search with NOT-keyword filtering and posted-since",
		Long:        "Run a sapi search across one or many cities. Adds NOT-keyword filtering (--negate), posted-since cutoffs, and cross-city result merging that the official Craigslist search does not support.",
		Example:     "  craigslist-pp-cli search 'ipad' --site sfbay --max-price 100\n  craigslist-pp-cli search '1BR' --sites sfbay,nyc --category apa --negate furnished,sublet",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			targets := siteList(site, sites)
			if len(targets) == 0 {
				return fmt.Errorf("no site specified — pass --site or --sites")
			}
			cutoff, err := parseDuration(postedSince)
			if err != nil {
				return err
			}

			q := craigslist.SearchQuery{
				Query:          query,
				SearchPath:     category,
				Page:           page,
				MinPrice:       minPrice,
				MaxPrice:       maxPrice,
				HasPic:         hasPic,
				Postal:         postal,
				SearchDistance: distanceMi,
				TitleOnly:      titleOnly,
				Sort:           sort,
			}

			c := craigslist.New(1.0)
			results, errs := cliutil.FanoutRun[string, []searchHit](
				cmd.Context(),
				targets,
				func(s string) string { return s },
				func(ctx context.Context, s string) ([]searchHit, error) {
					sr, err := c.Search(ctx, s, q)
					if err != nil {
						return nil, err
					}
					hits := make([]searchHit, 0, len(sr.Items))
					for _, it := range sr.Items {
						hits = append(hits, listingToHit(it))
					}
					return hits, nil
				},
				cliutil.WithConcurrency(5),
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			merged := mergeFanoutHits(results)
			merged = applyNegate(merged, negate)
			if cutoff > 0 {
				merged = applyPostedSince(merged, cutoff, time.Now())
			}
			if limit > 0 && len(merged) > limit {
				merged = merged[:limit]
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(merged))
				for _, h := range merged {
					items = append(items, map[string]any{
						"site":      h.Site,
						"postingId": h.PostingID,
						"price":     h.Price,
						"title":     cliutil.CleanText(h.Title),
						"url":       h.URL,
					})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), merged, flags)
		},
	}
	cmd.Flags().StringVar(&site, "site", "sfbay", "Single Craigslist site (e.g. sfbay, nyc)")
	cmd.Flags().StringVar(&sites, "sites", "", "Comma-separated cross-city site list. Wins over --site when both set.")
	cmd.Flags().StringVar(&category, "category", "sss", "Category abbreviation (e.g. sss, apa, sof)")
	cmd.Flags().IntVar(&minPrice, "min-price", 0, "Minimum price filter")
	cmd.Flags().IntVar(&maxPrice, "max-price", 0, "Maximum price filter")
	cmd.Flags().BoolVar(&hasPic, "has-pic", false, "Require listings with a picture")
	cmd.Flags().StringVar(&postal, "postal", "", "ZIP / postal code to anchor distance filtering")
	cmd.Flags().IntVar(&distanceMi, "distance-mi", 0, "Distance in miles from --postal")
	cmd.Flags().BoolVar(&titleOnly, "title-only", false, "Match titles only")
	cmd.Flags().StringVar(&postedSince, "posted-since", "", "Drop listings older than this duration (e.g. 24h, 3d)")
	cmd.Flags().StringVar(&negate, "negate", "", "Comma-separated keywords to exclude (matches title/url, case-insensitive)")
	cmd.Flags().IntVar(&page, "page", 1, "1-indexed page number for sapi pagination")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap total cross-site results after merging (0 = no cap)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort order: date|rel|priceasc|pricedsc")
	return cmd
}

// siteList resolves the --site/--sites mutual-exclusion: --sites wins when set.
func siteList(site, sites string) []string {
	if s := strings.TrimSpace(sites); s != "" {
		parts := strings.Split(s, ",")
		out := out0(parts)
		return out
	}
	if s := strings.TrimSpace(site); s != "" {
		return []string{s}
	}
	return nil
}

func out0(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func listingToHit(l craigslist.Listing) searchHit {
	return searchHit{
		Site:         l.Site,
		PostingID:    l.PostingID,
		UUID:         l.UUID,
		Title:        l.Title,
		Price:        l.Price,
		PriceDisplay: l.PriceDisplay,
		PostedAt:     l.PostedAt,
		Subarea:      l.Subarea,
		Neighborhood: l.Neighborhood,
		Latitude:     l.Latitude,
		Longitude:    l.Longitude,
		URL:          l.CanonicalURL,
	}
}

// mergeFanoutHits flattens FanoutResult slices in source order.
func mergeFanoutHits(results []cliutil.FanoutResult[[]searchHit]) []searchHit {
	var out []searchHit
	for _, r := range results {
		out = append(out, r.Value...)
	}
	return out
}

// applyNegate drops hits whose title or URL contains any negate keyword
// (comma-separated, case-insensitive). Empty negate is a no-op.
func applyNegate(hits []searchHit, negate string) []searchHit {
	keywords := splitNegate(negate)
	if len(keywords) == 0 {
		return hits
	}
	out := make([]searchHit, 0, len(hits))
	for _, h := range hits {
		drop := false
		hay := strings.ToLower(h.Title + " " + h.URL)
		for _, k := range keywords {
			if strings.Contains(hay, k) {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, h)
		}
	}
	return out
}

// splitNegate parses a comma-separated negate flag into lowercase tokens.
// Exposed (lowercase, package-internal) so tests can exercise it directly.
func splitNegate(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// applyPostedSince drops hits with PostedAt older than (now - cutoff). Hits
// with an unset PostedAt are kept (we cannot prove they are stale).
func applyPostedSince(hits []searchHit, cutoff time.Duration, now time.Time) []searchHit {
	threshold := now.Add(-cutoff).Unix()
	out := make([]searchHit, 0, len(hits))
	for _, h := range hits {
		if h.PostedAt > 0 && h.PostedAt < threshold {
			continue
		}
		out = append(out, h)
	}
	return out
}

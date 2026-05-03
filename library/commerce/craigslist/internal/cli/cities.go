// `cities heat` ranks cities by fresh-listing volume per category over a
// window. Reads the per-day Craigslist sitemap for each known site (or the
// top-N by store presence) and counts URLs.

package cli

import (
	"context"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/craigslist/internal/source/craigslist"

	"github.com/spf13/cobra"
)

type cityHeat struct {
	Site  string `json:"site"`
	Count int    `json:"count"`
}

func newCitiesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cities",
		Short:       "Cross-city analytics over the local store and live sitemaps",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newCitiesHeatCmd(flags))
	return cmd
}

func newCitiesHeatCmd(flags *rootFlags) *cobra.Command {
	var category, since, sites string
	var top int
	cmd := &cobra.Command{
		Use:         "heat",
		Short:       "Rank sites by fresh-listing count for a category over a window",
		Long:        "Walk per-day sitemaps for the given category meta-cat across the supplied sites and rank by URL count.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseDuration(since)
			if err != nil {
				return err
			}
			if d <= 0 {
				d = 24 * time.Hour
			}
			meta := craigslist.MetaCategoryFromAbbr(category)
			c := craigslist.New(1.0)
			cities := splitCSV(sites)
			if len(cities) == 0 {
				cities = defaultTopCities()
			}
			results, errs := cliutil.FanoutRun[string, int](
				cmd.Context(),
				cities,
				func(s string) string { return s },
				func(ctx context.Context, s string) (int, error) {
					urls, err := c.FreshListingsWindow(ctx, s, meta, d)
					if err != nil {
						return 0, err
					}
					return len(urls), nil
				},
				cliutil.WithConcurrency(5),
			)
			cliutil.FanoutReportErrors(cmd.ErrOrStderr(), errs)

			heat := make([]cityHeat, 0, len(results))
			for _, r := range results {
				heat = append(heat, cityHeat{Site: r.Source, Count: r.Value})
			}
			sort.Slice(heat, func(i, j int) bool { return heat[i].Count > heat[j].Count })
			if top > 0 && top < len(heat) {
				heat = heat[:top]
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				items := make([]map[string]any, 0, len(heat))
				for _, h := range heat {
					items = append(items, map[string]any{"site": h.Site, "count": h.Count})
				}
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printJSONFiltered(cmd.OutOrStdout(), heat, flags)
		},
	}
	cmd.Flags().StringVar(&category, "category", "sss", "Category abbreviation (rolled up to its meta-cat)")
	cmd.Flags().StringVar(&since, "since", "24h", "Window size (e.g. 24h, 3d)")
	cmd.Flags().StringVar(&sites, "sites", "", "Comma-separated sites to probe (default: top US metros)")
	cmd.Flags().IntVar(&top, "top", 20, "Cap to the top N sites by count")
	return cmd
}

// defaultTopCities is a hand-curated short list of high-volume US metros so
// `cities heat` works without --sites. Not exhaustive — users with a populated
// store should pass --sites pulled from `areas list`.
func defaultTopCities() []string {
	return []string{
		"sfbay", "newyork", "losangeles", "chicago", "seattle",
		"boston", "washingtondc", "atlanta", "denver", "austin",
		"sandiego", "portland", "miami", "philadelphia", "dallas",
		"phoenix", "minneapolis", "orangecounty", "houston", "raleigh",
	}
}

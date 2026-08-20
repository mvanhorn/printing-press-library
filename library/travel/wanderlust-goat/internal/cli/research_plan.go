package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/regions"
)

// newResearchPlanCmd emits a typed JSON query plan agents execute in
// their own loop. No live API calls — no Google Places, no scraping.
// Pure metadata.
func newResearchPlanCmd(flags *rootFlags) *cobra.Command {
	var (
		anchor   string
		criteria string
		country  string
	)
	cmd := &cobra.Command{
		Use:   "research-plan [criteria]",
		Short: "Emit a typed JSON query plan for agent loops",
		Long: `research-plan returns a JSON document describing what an agent should research
for a given (anchor, criteria, country) tuple. No live calls — pure plan
output, country-aware, ordered by trust. Use this when the caller has its
own search tooling and just needs the right plan.`,
		Example: strings.Trim(`
  wanderlust-goat-pp-cli research-plan "vintage seafood" --anchor 'Tsukiji' --country JP --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			crit := criteria
			if crit == "" && len(args) > 0 {
				crit = strings.Join(args, " ")
			}
			if crit == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			cc := strings.ToUpper(strings.TrimSpace(country))
			if cc == "" && anchor != "" {
				if r, err := dispatch.ResolveAnchor(ctx, anchor); err == nil {
					cc = r.Country
				}
			}
			if cc == "" {
				cc = "*"
			}
			region := regions.Lookup(cc)

			plan := struct {
				Criteria       string         `json:"criteria"`
				Anchor         string         `json:"anchor,omitempty"`
				Country        string         `json:"country"`
				Region         regions.Region `json:"region"`
				QueryTemplates []QueryEntry   `json:"queries"`
				Notes          []string       `json:"notes"`
			}{
				Criteria: crit,
				Anchor:   anchor,
				Country:  cc,
				Region:   region,
			}
			plan.QueryTemplates = buildQueryEntries(crit, anchor, region)
			plan.Notes = buildPlanNotes(region)
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		},
	}
	cmd.Flags().StringVar(&anchor, "anchor", "", "anchor as <lat>,<lng> or address (used to derive country)")
	cmd.Flags().StringVar(&criteria, "criteria", "", "free-text criteria (positional also accepted)")
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 country code (overrides anchor-derived)")
	return cmd
}

// QueryEntry is one row in the agent's research plan.
type QueryEntry struct {
	Source   string `json:"source"`
	Locale   string `json:"locale"`
	Method   string `json:"method"` // "search-by-name", "google.<TLD> query", "subreddit search"
	Query    string `json:"query"`
	Priority int    `json:"priority"` // 1=highest
}

func buildQueryEntries(criteria, anchor string, region regions.Region) []QueryEntry {
	var out []QueryEntry
	priority := 1
	for _, slug := range region.LocalReviewSites {
		out = append(out, QueryEntry{
			Source:   slug,
			Locale:   region.PrimaryLanguage,
			Method:   "search-by-name",
			Query:    criteria + " " + anchor,
			Priority: priority,
		})
		priority++
	}
	if region.GoogleTLD != "" {
		out = append(out, QueryEntry{
			Source:   "google." + region.GoogleTLD,
			Locale:   region.PrimaryLanguage,
			Method:   "search-engine",
			Query:    fmt.Sprintf("%s %s -site:tripadvisor.* -site:yelp.com", criteria, anchor),
			Priority: priority,
		})
		priority++
	}
	for _, sub := range region.LocalForums {
		out = append(out, QueryEntry{
			Source:   "reddit",
			Locale:   "en",
			Method:   "subreddit-search",
			Query:    fmt.Sprintf("/r/%s %q", sub, criteria),
			Priority: priority,
		})
		priority++
	}
	out = append(out, QueryEntry{
		Source:   "wikipedia",
		Locale:   region.PrimaryLanguage,
		Method:   "geosearch+notability",
		Query:    fmt.Sprintf("places near %s with editorial coverage", anchor),
		Priority: priority,
	})
	return out
}

func buildPlanNotes(region regions.Region) []string {
	notes := []string{
		"Stage 1: seed via Google Places NearbySearch (filter business_status=OPERATIONAL).",
		"Stage 2: deep-research per candidate; in-country sources get +0.10 trust boost.",
		"Stage 3: closed-signal kill-gate — drop if any source confirms permanently closed.",
		"Walking radius is minutes × 4.5 km/h ÷ 1.3 tortuosity.",
	}
	if region.GoogleTLD == "" {
		notes = append(notes, "NOTE: Google search path is unavailable for this region (CN); use Baidu or skip the search-engine row.")
	}
	if len(region.LocalReviewSites) == 0 {
		notes = append(notes, "NOTE: no curated regional review sites for this country; rely on Wikipedia + Reddit + general Google search.")
	}
	return notes
}

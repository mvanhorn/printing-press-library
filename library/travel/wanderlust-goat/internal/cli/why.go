package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newWhyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "why <place-name-or-id>",
		Short: "Print every source that mentioned a place, the trust weight, country boost, walking time, criteria match, and the final goat-score breakdown.",
		Long: `Step-by-step audit of why a place appears (or doesn't) in a 'near' or 'goat'
result. Resolves the input by name (FTS match in goat_places) or by id, then
prints every source row referencing it, the trust contribution per source,
and the formula breakdown:

  goat_score = trust × (1 + country_match_boost) × intent_match × walking_decay`,
		Example: strings.Trim(`
  # By exact id
  wanderlust-goat-pp-cli why "wikipedia.ja:42519"

  # By name (FTS-style)
  wanderlust-goat-pp-cli why "Kohi Bibi" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()

			rows, err := store.DB().QueryContext(ctx, `
				SELECT id, source, intent, name, name_local, lat, lng, country, city_slug, trust, why_special
				FROM goat_places
				WHERE id = ? OR name = ? OR name_local = ?
				   OR id IN (SELECT id FROM goat_places_fts WHERE goat_places_fts MATCH ? LIMIT 10)`,
				query, query, query, query)
			if err != nil {
				return err
			}
			defer rows.Close()
			breakdown := whyReport{Query: query}
			for rows.Next() {
				var e whyEntry
				if err := rows.Scan(&e.ID, &e.Source, &e.Intent, &e.Name, &e.NameLocal, &e.Lat, &e.Lng, &e.Country, &e.CitySlug, &e.Trust, &e.WhySpecial); err != nil {
					return err
				}
				breakdown.Mentions = append(breakdown.Mentions, e)
			}
			if len(breakdown.Mentions) == 0 {
				breakdown.Note = "No matches in the local store. Run sync-city <slug> first, or pass --data-source live."
				_ = printJSONFiltered(cmd.OutOrStdout(), breakdown, flags)
				return notFoundErr(fmt.Errorf("no matches for %q", query))
			}
			breakdown.Note = "score formula: trust × (1 + country_boost_if_match) × intent_match × walking_decay"
			return printJSONFiltered(cmd.OutOrStdout(), breakdown, flags)
		},
	}
	return cmd
}

type whyReport struct {
	Query    string     `json:"query"`
	Mentions []whyEntry `json:"mentions"`
	Note     string     `json:"note,omitempty"`
}

type whyEntry struct {
	ID         string  `json:"id"`
	Source     string  `json:"source"`
	Intent     string  `json:"intent"`
	Name       string  `json:"name"`
	NameLocal  string  `json:"name_local,omitempty"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	Country    string  `json:"country,omitempty"`
	CitySlug   string  `json:"city_slug,omitempty"`
	Trust      float64 `json:"trust"`
	WhySpecial string  `json:"why_special,omitempty"`
}

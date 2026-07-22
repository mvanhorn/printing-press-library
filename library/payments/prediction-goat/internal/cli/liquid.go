// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// topicFTSQuery normalizes a free-text topic into an FTS5 MATCH expression.
// Hyphens, underscores, and quotes split into separate tokens so slug-style
// inputs (e.g. "kanye-west") behave the same as bare-term inputs ("kanye
// west"). Multi-token queries OR the terms so a row matching any term is
// a candidate; BM25 ranking then prefers rows matching more terms. AND-joining
// hyphenated queries was a recall bug: "kanye-west" returned strictly fewer
// hits than "kanye" because rows about "Donda" or "Graduation" did not
// contain "west".
func topicFTSQuery(s string) string {
	replacer := strings.NewReplacer("-", " ", "_", " ", "'", " ", `"`, " ")
	parts := strings.Fields(replacer.Replace(s))
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, `"`+part+`"`)
	}
	if len(quoted) == 0 {
		// Empty MATCH parameter triggers a SQLite FTS5 parse error rather
		// than returning zero rows. Return a sentinel that matches nothing
		// so callers see a clean empty result instead of an engine error
		// when the input was all separators (e.g. "---" or "-_-").
		return `""`
	}
	if len(quoted) == 1 {
		return quoted[0]
	}
	return strings.Join(quoted, " OR ")
}

// topicQueryTokens returns the lowercased word tokens of a topic query, used
// by force-include logic to recognize when an outcome named in the query
// appears in a candidate market title (e.g. "USA" in "Will USA win ...").
func topicQueryTokens(s string) []string {
	replacer := strings.NewReplacer("-", " ", "_", " ", "'", " ", `"`, " ", ",", " ")
	parts := strings.Fields(replacer.Replace(strings.ToLower(s)))
	tokens := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) > 1 {
			tokens = append(tokens, p)
		}
	}
	return tokens
}

func newLiquidCmd(flags *rootFlags) *cobra.Command {
	var minVolume float64
	var limit int
	var dbPath string
	var vf venueFlags
	cmd := &cobra.Command{
		Use:   "liquid",
		Short: "Markets above a volume floor across Polymarket and Kalshi",
		Example: `  prediction-goat-pp-cli liquid --min-volume 100000 --json
  prediction-goat-pp-cli liquid --min-volume 50000 --limit 25
  prediction-goat-pp-cli liquid --kalshi --min-volume 50000`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			venue, err := resolveVenue(vf)
			if err != nil {
				return err
			}
			items, err := runMarketScreen(cmd, "liquid", dbPath, venue, limit, minVolume, "", "")
			if err != nil {
				return err
			}
			outcome := refreshMarketScreenItems(cmd.Context(), nil, items)
			meta := buildFreshnessMeta(outcome, indexSyncedAtFromPath(cmd.Context(), dbPath))
			if renderErr := renderTrending(cmd, flags, trendingResult{Items: items, Meta: meta}); renderErr != nil {
				return renderErr
			}
			if len(items) == 0 {
				if hint := emptyStoreHint(cmd, dbPath, "liquid", venue); hint != nil {
					return hint
				}
			}
			return nil
		},
	}
	cmd.Flags().Float64Var(&minVolume, "min-volume", 10000, "Minimum 24h rolling volume (USD)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	addVenueFlags(cmd, &vf)
	return cmd
}

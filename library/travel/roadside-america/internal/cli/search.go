package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/roadside"
	"github.com/spf13/cobra"
)

var ftsTokenRe = regexp.MustCompile(`[A-Za-z0-9]+`)

// ftsMatchQuery builds a safe FTS5 MATCH string from free text by quoting each
// alphanumeric token (implicit AND). Returns "" when there is nothing to match,
// signalling the caller to use the substring fallback.
func ftsMatchQuery(q string) string {
	toks := ftsTokenRe.FindAllString(q, -1)
	parts := make([]string, 0, len(toks))
	for _, t := range toks {
		if len(t) < 2 {
			continue
		}
		parts = append(parts, `"`+t+`"`)
	}
	return strings.Join(parts, " ")
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var category string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the local cache of attractions (offline, full-text)",
		Long: strings.Trim(`
Search the local cache of attractions by name, city, or writeup text. This is an
offline, full-text search over data already fetched by 'state', 'near', or
'show' — it makes no network calls. Run those first to populate the cache.`, "\n"),
		Example: strings.Trim(`
  roadside-america-pp-cli search "alligator"
  roadside-america-pp-cli search "world's largest" --json
  roadside-america-pp-cli search dinosaur --category animals --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the local attraction cache")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			if limit <= 0 {
				limit = 25
			}
			query := strings.Join(args, " ")
			canonCat := ""
			if category != "" {
				c, ok := roadside.NormalizeCategory(category)
				if !ok {
					return usageErr(fmt.Errorf("unknown category %q; run 'category --list' to see options", category))
				}
				canonCat = c
			}

			ctx := cmd.Context()
			s, err := openRoadsideStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			var atts []roadside.Attraction
			if match := ftsMatchQuery(query); match != "" {
				if raws, serr := s.Search(match, limit*8); serr == nil {
					atts = decodeAttractions(raws)
				}
			}
			if len(atts) == 0 {
				// Substring fallback over the whole cache (handles short or
				// punctuation-heavy queries the FTS tokenizer drops).
				atts = substringSearch(s, query)
			}

			if canonCat != "" {
				filtered := make([]roadside.Attraction, 0, len(atts))
				for _, a := range atts {
					if roadside.MatchesCategory(a, canonCat) {
						filtered = append(filtered, a)
					}
				}
				atts = filtered
			}
			if len(atts) > limit {
				atts = atts[:limit]
			}

			view := attractionListView{
				Query:       map[string]any{"query": query},
				Attractions: atts,
			}
			if canonCat != "" {
				view.Query["category"] = canonCat
			}
			if len(atts) == 0 {
				view.Note = "No matches in the local cache. Run 'state <ST>' or 'near <place>' first to populate it."
			}
			return emitAttractions(cmd, flags, view)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum results to return")
	cmd.Flags().StringVar(&category, "category", "", "Filter to a local category (see 'category --list')")
	return cmd
}

// substringSearch is the fallback when FTS yields nothing: case-insensitive
// contains over name/city/state across the cache.
func substringSearch(s *storeHandle, query string) []roadside.Attraction {
	all, err := loadCachedAttractions(s, 0)
	if err != nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]roadside.Attraction, 0)
	for _, a := range all {
		hay := strings.ToLower(a.Name + " " + a.City + " " + a.State)
		if strings.Contains(hay, q) {
			out = append(out, a)
		}
	}
	return out
}

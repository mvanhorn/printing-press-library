package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/roadside"
	"github.com/spf13/cobra"
)

func newStateCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var category string
	cmd := &cobra.Command{
		Use:   "state <ST>",
		Short: "List offbeat attractions in a US state or Canadian province",
		Long: strings.Trim(`
List offbeat attractions in a US state or Canadian province by two-letter code
(e.g. TX, CA, ON). Results are fetched live from RoadsideAmerica.com and cached
locally; pass --data-source local to read only the local cache, or --category to
filter by a local superlative bucket (see 'category --list').`, "\n"),
		Example: strings.Trim(`
  roadside-america-pp-cli state TX --limit 10
  roadside-america-pp-cli state CA --category biggest --json
  roadside-america-pp-cli state ON --agent --select name,city,source_url`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list attractions for the given state")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a two-letter state/province code is required (e.g. TX)"))
			}
			code := strings.ToUpper(strings.TrimSpace(args[0]))
			if !roadside.ValidState(code) {
				return usageErr(fmt.Errorf("unknown state/province %q; expected one of: %s", args[0], roadside.StateCodes()))
			}
			canonCat := ""
			if category != "" {
				c, ok := roadside.NormalizeCategory(category)
				if !ok {
					return usageErr(fmt.Errorf("unknown category %q; run 'category --list' to see options", category))
				}
				canonCat = c
			}
			if limit < 0 {
				limit = 0
			}

			ctx := cmd.Context()
			s, err := openRoadsideStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()

			var atts []roadside.Attraction
			note := ""
			if flags.dataSource == "local" {
				atts, err = stateFromCache(s, code)
				if err != nil {
					return err
				}
				if len(atts) == 0 {
					note = fmt.Sprintf("No cached attractions for %s. Run without --data-source local to fetch from RoadsideAmerica.com.", code)
				}
			} else {
				c, ferr := flags.newClient()
				if ferr != nil {
					return ferr
				}
				atts, err = fetchStateAttractions(ctx, c, roadside.NormalizeState(code))
				if err != nil {
					if flags.dataSource == "auto" {
						if cached, cerr := stateFromCache(s, code); cerr == nil && len(cached) > 0 {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: live fetch failed (%v); serving %d cached results\n", err, len(cached))
							atts = cached
							note = "served from local cache (live fetch failed)"
						} else {
							return classifyFetchErr(err)
						}
					} else {
						return classifyFetchErr(err)
					}
				} else {
					cacheAttractions(s, atts)
				}
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
			if limit > 0 && len(atts) > limit {
				atts = atts[:limit]
			}

			view := attractionListView{
				Query:       map[string]any{"state": code, "state_name": roadside.StateName(code)},
				Attractions: atts,
				Note:        note,
			}
			if canonCat != "" {
				view.Query["category"] = canonCat
			}
			return emitAttractions(cmd, flags, view)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum attractions to return (0 for all)")
	cmd.Flags().StringVar(&category, "category", "", "Filter to a local category (see 'category --list')")
	return cmd
}

// stateFromCache returns cached attractions whose state matches code.
func stateFromCache(s *storeHandle, code string) ([]roadside.Attraction, error) {
	all, err := loadCachedAttractions(s, 0)
	if err != nil {
		return nil, err
	}
	out := make([]roadside.Attraction, 0)
	for _, a := range all {
		if strings.EqualFold(a.State, code) {
			out = append(out, a)
		}
	}
	return out, nil
}

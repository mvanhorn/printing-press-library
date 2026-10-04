// discover.go — hand-written Slice A novel command (top-level).
// pp:data-source live — RAWG computes the filter facets server-side.
// Standalone hand-authored file: generate --force preserves it (regen-merge).

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type discoverMeta struct {
	Source  string            `json:"source"`
	Count   int               `json:"count"`
	Filters map[string]string `json:"filters,omitempty"`
}

type discoverView struct {
	Meta    discoverMeta `json:"meta"`
	Results []gameRow    `json:"results"`
}

func newDiscoverCmd(flags *rootFlags) *cobra.Command {
	var fGenres, fTags, fPlatforms, fStores, fDevelopers, fPublishers, fDates, fOrdering, fMetacritic, fSearch string
	var limit int
	var withDemos bool

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover games by RAWG filters (genre, platform, dates, rating, ...)",
		Long: `Discover games by combining RAWG /games query filters. Values mirror the
RAWG API: comma-separated ids or slugs for --genres/--tags/--platforms/
--stores/--developers/--publishers, a YYYY-MM-DD,YYYY-MM-DD range for
--dates, a range like 80,100 for --metacritic, and -added/-rating/
-released/-name for --ordering.`,
		Example: strings.Trim(`
  game-goat-pp-cli discover --genres action --limit 5
  game-goat-pp-cli discover --platforms 4 --metacritic 80,100 --limit 10 --json
  game-goat-pp-cli discover --tags singleplayer --ordering -released --limit 10
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--genres=action;--limit=5;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "discover")
			}
			if len(args) > 0 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --<filter> <value>", "discover takes filter flags, not positional arguments")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if limit < 1 {
				return usageErrWithJSON(cmd, flags, cmd.CommandPath()+" --limit <n>", "--limit must be at least 1")
			}
			bindings := []struct{ value, param string }{
				{fGenres, "genres"},
				{fTags, "tags"},
				{fPlatforms, "platforms"},
				{fStores, "stores"},
				{fDevelopers, "developers"},
				{fPublishers, "publishers"},
				{fDates, "dates"},
				{fOrdering, "ordering"},
				{fMetacritic, "metacritic"},
				{fSearch, "search"},
			}
			params := map[string]string{"page_size": strconv.Itoa(limit)}
			filters := make(map[string]string, len(bindings))
			for _, b := range bindings {
				if strings.TrimSpace(b.value) == "" {
					continue
				}
				params[b.param] = b.value
				filters[b.param] = b.value
			}
			if len(filters) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: no filters set — returning RAWG's default /games ordering; narrow with --genres, --platforms, --dates, ...")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			games, source, err := fetchGamesResults(cmd.Context(), cmd, c, flags, "live", params)
			if err != nil {
				return err
			}
			if source == "" {
				source = "live"
			}
			rows := make([]gameRow, 0, len(games))
			for _, g := range games {
				rows = append(rows, toGameRow(g))
			}
			if withDemos {
				failed, aerr := annotateRowsWithSteamDemos(cmd, c, rows)
				if aerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam demo annotation unavailable: %v\n", aerr)
				} else if failed > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: steam demo annotation: %d of %d Steam store-link lookups failed; demo state unknown for those rows\n", failed, len(rows))
				}
			}
			view := discoverView{
				Meta:    discoverMeta{Source: source, Count: len(rows), Filters: filters},
				Results: rows,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No games matched those filters.")
				return nil
			}
			items := gameTableRows(rows)
			if withDemos {
				// The auto table switches to a card layout above 8 fields, so
				// swap the least informative column for the demo state to keep
				// a real demo column.
				for i := range items {
					delete(items[i], "platforms")
					items[i]["demo"] = steamDemoCell(rows[i].HasDemo)
				}
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}

	cmd.Flags().StringVar(&fGenres, "genres", "", "Filter by genre ids or slugs, e.g. 4,51 or action,indie")
	cmd.Flags().StringVar(&fTags, "tags", "", "Filter by tag ids or slugs, e.g. 31,7 or singleplayer,multiplayer")
	cmd.Flags().StringVar(&fPlatforms, "platforms", "", "Filter by platform ids, e.g. 4,5 (4=PC, 1=Xbox, 187=PlayStation 5)")
	cmd.Flags().StringVar(&fStores, "stores", "", "Filter by store ids or slugs, e.g. 5,6 or steam,gog")
	cmd.Flags().StringVar(&fDevelopers, "developers", "", "Filter by developer ids or slugs, e.g. 1612,18893 or valve-software")
	cmd.Flags().StringVar(&fPublishers, "publishers", "", "Filter by publisher ids or slugs, e.g. 354,20987 or electronic-arts")
	cmd.Flags().StringVar(&fDates, "dates", "", "Filter by release date range YYYY-MM-DD,YYYY-MM-DD")
	cmd.Flags().StringVar(&fOrdering, "ordering", "", "Sort order: -added, -rating, -released, -metacritic, or name")
	cmd.Flags().StringVar(&fMetacritic, "metacritic", "", "Filter by Metacritic score range, e.g. 80,100")
	cmd.Flags().StringVar(&fSearch, "search", "", "Also require a search-text match, e.g. \"Elden Ring\"")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of games to return (RAWG caps pages at 40)")
	cmd.Flags().BoolVar(&withDemos, "with-demos", false, withDemosHelp)
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newDiscoverCmd(flags))
	})
}

package cli

// PATCH: Hand-built offline FTS5 catalog search command.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var format, year, country string
	var limit int
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Search the locally synced Blu-ray.com catalog using SQLite FTS5.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli search 'fight club' --format 4k --json
  blu-ray-pp-cli search 'criterion' --year 2024 --limit 10 --json --select id,title,slug
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}

			stats, err := s.CatalogStats(cmd.Context())
			if err != nil {
				return err
			}
			if stats.TotalRows == 0 {
				// PATCH: Keep --json parseable when the local catalog exists but has no releases.
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{
						"catalog_empty": true,
						"hint":          "run `blu-ray-pp-cli sync` to populate the local catalog",
						"results":       []any{},
					})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Local catalog is empty. Run `blu-ray-pp-cli sync` first.")
				return nil
			}

			query := strings.Join(args, " ")
			searchExpr := ftsQuery(query)
			if year != "" {
				y, err := strconv.Atoi(year)
				if err != nil {
					return usageErr(fmt.Errorf("--year must be YYYY"))
				}
				_ = y
			}
			if limit <= 0 {
				limit = 25
			}
			catalogRows, err := s.SearchCatalog(cmd.Context(), store.CatalogSearchOpts{
				Query:   searchExpr,
				Format:  format,
				Year:    year,
				Country: country,
				Limit:   limit,
			})
			if err != nil {
				return err
			}
			var out []catalogRelease
			for _, row := range catalogRows {
				r := catalogRelease{
					ID:      row.ID,
					Kind:    row.Kind,
					Title:   row.TitleNormalized,
					Slug:    row.Slug,
					Year:    row.YearHint,
					Country: row.Country,
				}
				r.URL = releaseURL(r.Kind, r.Slug, r.ID)
				out = append(out, r)
			}
			if flags.asJSON || flags.selectFields != "" || flags.csv || flags.quiet || flags.plain {
				return flags.printJSON(cmd, out)
			}
			var table [][]string
			for _, r := range out {
				table = append(table, []string{strconv.Itoa(r.ID), r.Kind, r.Title, strconv.Itoa(r.Year), r.Country, r.URL})
			}
			return flags.printTable(cmd, []string{"ID", "KIND", "TITLE", "YEAR", "COUNTRY", "URL"}, table)
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "Format filter: 4k, bluray, 3d, dvd, or digital.")
	cmd.Flags().StringVar(&year, "year", "", "Release year filter (YYYY).")
	cmd.Flags().StringVar(&country, "country", "", "Country filter, when catalog rows include country data (e.g. US).")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of search results to return.")
	return cmd
}

func ftsQuery(q string) string {
	// PATCH: Strip FTS5 syntax chars before adding prefix markers. Hyphens
	// are NOT stripped — they are split on instead, mirroring titleFromSlug
	// which replaces hyphens with spaces before indexing. Stripping turned
	// "spider-man" into "spiderman*", which never matched the indexed tokens
	// ["spider", "man"] and produced zero results for every hyphenated query.
	// Fixes Greptile P1 on PR #634.
	re := regexp.MustCompile(`[:()"^+*]`)
	var parts []string
	for _, tok := range strings.Fields(strings.ReplaceAll(q, "-", " ")) {
		tok = re.ReplaceAllString(tok, "")
		if tok == "" {
			continue
		}
		parts = append(parts, tok+"*")
	}
	return strings.Join(parts, " ")
}

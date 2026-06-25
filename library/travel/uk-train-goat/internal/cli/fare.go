// uk-train-goat hand-authored: experimental fare lookup via NR website scrape.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/farescrape"

	"github.com/spf13/cobra"
)

func newFareCmd(flags *rootFlags) *cobra.Command {
	var date string
	cmd := &cobra.Command{
		Use:   "fare <from-crs> <to-crs>",
		Short: "Best-effort fare lookup A->B (experimental)",
		Long: `Look up an A->B fare via nationalrail.co.uk scrape. Marked EXPERIMENTAL:
the journey planner is JS-rendered, so plain HTTP fetches do not return numeric
fares. The command returns the search URL plus a clear note rather than
fabricating a price.

For booking, use the National Rail website directly. Fare data via this command
is best-effort and may break when the upstream layout changes.`,
		Example: strings.Trim(`
  uk-train-goat-pp-cli fare PAD RDG
  uk-train-goat-pp-cli fare PAD RDG --date 2026-06-01
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("fare requires <from-crs> <to-crs>; got %d args", len(args)))
			}
			from := strings.ToUpper(strings.TrimSpace(args[0]))
			to := strings.ToUpper(strings.TrimSpace(args[1]))
			scraper := farescrape.NewNRWebScraper()

			// pp:client-call — fetches https://www.nationalrail.co.uk/journey-planner/.
			result, err := scraper.Lookup(from, to, date)
			if err != nil {
				return apiErr(fmt.Errorf("fare lookup (experimental): %w", err))
			}
			data, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Target date (YYYY-MM-DD); defaults to today")
	return cmd
}

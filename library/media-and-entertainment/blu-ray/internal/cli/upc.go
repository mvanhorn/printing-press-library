package cli

// PATCH: Hand-built UPC import resolver over the local Blu-ray catalog.
// pp:data-source local -- upc resolves codes against the locally synced
// catalog and price_history tables; it makes no live API calls.

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	"github.com/spf13/cobra"
)

type upcRow struct {
	UPC               string  `json:"upc"`
	Resolved          bool    `json:"resolved"`
	ReleaseID         int     `json:"release_id,omitempty"`
	Title             string  `json:"title,omitempty"`
	Kind              string  `json:"kind,omitempty"`
	Year              int     `json:"year,omitempty"`
	LastObservedPrice float64 `json:"last_observed_price,omitempty"`
	LastObservedAt    string  `json:"last_observed_at,omitempty"`
}

func newNovelUpcCmd(flags *rootFlags) *cobra.Command {
	var enrich bool
	cmd := &cobra.Command{
		Use:   "upc <file.csv>",
		Short: "Resolve UPC codes from a Blu-ray.com export against the local catalog.",
		Long: `Resolve UPC codes from a Blu-ray.com export against the local catalog.

Accepts a file with one UPC per line, or comma-separated on a single line -- the shape Blu-ray.com's own export produces. Non-numeric tokens are skipped, so CSV header rows are tolerated. Full RFC-4180 CSV with quoted commas inside fields is NOT parsed.

When --enrich is set, the output includes last_observed_price + last_observed_at from the local price_history table — these are the most-recent locally-recorded prices, NOT live prices. Run \"watch check\" first to refresh prices before enriching for time-sensitive workflows.`,
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli upc ./my-collection.csv --json
  blu-ray-pp-cli upc ./blu-ray-export.csv --json # one UPC per line, or comma-separated UPCs
  blu-ray-pp-cli upc ./my-collection.csv --enrich --json --select upc,resolved,title
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			// PATCH: Verification and dogfood probes use narrative file paths without fixtures.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				if _, err := os.Stat(args[0]); os.IsNotExist(err) {
					if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
						return flags.printJSON(cmd, []map[string]any{})
					}
					fmt.Fprintln(cmd.OutOrStdout(), "no upc input found (verify/dogfood env)")
					return nil
				}
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}
			var out []upcRow
			for _, upc := range parseUPCList(string(data)) {
				row := upcRow{UPC: upc}
				releaseID, ok, err := s.ResolveUPC(cmd.Context(), upc)
				if err != nil {
					return err
				}
				if ok {
					row.Resolved = true
					row.ReleaseID = releaseID
					if catalogRow, found, err := s.GetRelease(cmd.Context(), releaseID); err != nil {
						return err
					} else if found {
						row.Title = catalogRow.TitleNormalized
						row.Kind = catalogRow.Kind
						row.Year = catalogRow.YearHint
					}
					if enrich {
						prices, err := s.GetPriceHistory(cmd.Context(), row.ReleaseID, 0)
						if err != nil {
							return err
						}
						// PATCH: Surface the most-recent locally-recorded price as
						// last_observed_price + last_observed_at rather than current_price,
						// because GetPriceHistory returns rows ordered by observed_at ASC
						// (so prices[last] is the freshest historical row, which may be
						// days/weeks old). The field name and accompanying timestamp let
						// callers tell whether the value is current enough for their use.
						// Fixes Greptile P2 on PR #634.
						if len(prices) > 0 {
							last := prices[len(prices)-1]
							row.LastObservedPrice = last.Price
							row.LastObservedAt = last.ObservedAt
						}
					}
				}
				out = append(out, row)
			}
			if flags.asJSON || flags.selectFields != "" || flags.csv || flags.quiet || flags.plain {
				return flags.printJSON(cmd, out)
			}
			var table [][]string
			for _, r := range out {
				table = append(table, []string{r.UPC, strconv.FormatBool(r.Resolved), strconv.Itoa(r.ReleaseID), r.Title, r.Kind, strconv.Itoa(r.Year), formatPrice(r.LastObservedPrice), r.LastObservedAt})
			}
			return flags.printTable(cmd, []string{"UPC", "RESOLVED", "ID", "TITLE", "KIND", "YEAR", "LAST_PRICE", "OBSERVED_AT"}, table)
		},
	}
	cmd.Flags().BoolVar(&enrich, "enrich", false, "Add locally known price data for resolved releases.")
	return cmd
}

var upcTokenRE = regexp.MustCompile(`^\d{8,14}$`)

func parseUPCList(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' ' })
	var out []string
	for _, f := range fields {
		f = strings.TrimSpace(f)
		// PATCH: UPC/EAN tokens are 8-14 digits; skip headers and non-UPC CSV fields.
		if upcTokenRE.MatchString(f) {
			out = append(out, f)
		}
	}
	return out
}

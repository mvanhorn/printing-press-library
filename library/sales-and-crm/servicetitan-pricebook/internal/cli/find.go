package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newFindCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var minScore float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "find <description>",
		Short: "Natural-language part finder over the synced pricebook",
		Long: "Runs a forgiving ranked search over every synced material, equipment,\n" +
			"and service for a plain-language description — 'describe the part, I\n" +
			"don't know the code'. Each SKU is scored on code, display name,\n" +
			"description, and vendor part; the best field wins. Returns the fields\n" +
			"a tech needs to pick a part. Results below --min-score are dropped;\n" +
			"a query that matches nothing exits non-zero (grep-style). Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli find "1 hp submersible pump motor"
  servicetitan-pricebook-pp-cli find "30k grain softener" --limit 5 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			db, err := openPricebookStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			results, err := pricebook.Find(db, query, minScore, limit)
			if err != nil {
				return err
			}
			if results == nil {
				results = []pricebook.FindResult{}
			}
			// grep-style: a query that matches nothing above the relevance
			// floor is a non-zero exit with an actionable message, not a
			// silent empty list of junk.
			if len(results) == 0 {
				return fmt.Errorf("no parts matched %q at or above --min-score %.2f; try different terms or lower --min-score", query, minScore)
			}

			table := make([][]string, 0, len(results))
			for _, r := range results {
				table = append(table, []string{
					f2(r.Score), string(r.Kind), r.Code, r.DisplayName,
					f2(r.Price), r.VendorPart, strconv.FormatBool(r.Active), r.MatchedOn,
				})
			}
			return pbOutput(cmd, flags, results,
				[]string{"SCORE", "KIND", "CODE", "NAME", "PRICE", "VENDOR PART", "ACTIVE", "MATCHED ON"},
				table)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum results to return")
	cmd.Flags().Float64Var(&minScore, "min-score", 0.4, "Minimum relevance score (0-1); results below this are dropped, and a query that matches nothing exits non-zero")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}

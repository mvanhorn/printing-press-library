package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newDedupeCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var minScore float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dedupe",
		Short: "Cluster near-duplicate materials and equipment",
		Long: "Fuzzy-matches active materials and equipment against each other on\n" +
			"normalized code, display name, description, and vendor part, then\n" +
			"clusters near-duplicates so excess pricebook growth can be collapsed.\n" +
			"An exact vendor-part match alone links two SKUs. The ServiceTitan API\n" +
			"has no 'find SKUs like this' query. Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli dedupe
  servicetitan-pricebook-pp-cli dedupe --kind material --min-score 0.9 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openPricebookStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			clusters, err := pricebook.Dedupe(db, pricebook.SKUKind(kind), minScore)
			if err != nil {
				return err
			}
			clusters = capRows(clusters, limit)
			if clusters == nil {
				clusters = []pricebook.DuplicateCluster{}
			}

			// Table output flattens to one row per member with a cluster
			// index so duplicates within a cluster sit visually together.
			table := make([][]string, 0)
			for ci, c := range clusters {
				for _, m := range c.Members {
					table = append(table, []string{
						strconv.Itoa(ci + 1), f2(c.Score), string(m.Kind),
						m.Code, m.DisplayName, m.VendorPart,
					})
				}
			}
			return pbOutput(cmd, flags, clusters,
				[]string{"CLUSTER", "SCORE", "KIND", "CODE", "NAME", "VENDOR PART"}, table)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Limit to one SKU kind: material or equipment")
	cmd.Flags().Float64Var(&minScore, "min-score", 0.85, "Minimum pairwise similarity (0-1) to link two SKUs into a cluster")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum clusters to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}

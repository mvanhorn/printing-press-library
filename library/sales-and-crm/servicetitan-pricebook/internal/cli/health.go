package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/pricebook"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var tolerance float64
	var minScore float64
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "One-shot rollup of every pricebook audit count",
		Long: "Aggregates markup drift, vendor-part gaps, warranty issues, orphan\n" +
			"SKUs, weak-copy issues, duplicate clusters, and cost-drift counts from\n" +
			"the local store into one compact rollup sized for agent priming. Run\n" +
			"this first in a pricebook session to see what needs attention. It also\n" +
			"snapshots cost history. Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-pricebook-pp-cli health
  servicetitan-pricebook-pp-cli health --tolerance 5 --json
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

			rep, err := pricebook.Health(db, tolerance, minScore, since)
			if err != nil {
				return err
			}

			table := [][]string{
				{"materials", strconv.Itoa(rep.Materials)},
				{"equipment", strconv.Itoa(rep.Equipment)},
				{"services", strconv.Itoa(rep.Services)},
				{"categories", strconv.Itoa(rep.Categories)},
				{"markup_tiers", strconv.Itoa(rep.MarkupTiers)},
				{"markup_drift", strconv.Itoa(rep.MarkupDrift)},
				{"vendor_part_gaps", strconv.Itoa(rep.VendorPartGaps)},
				{"warranty_issues", strconv.Itoa(rep.WarrantyIssues)},
				{"orphan_skus", strconv.Itoa(rep.OrphanSKUs)},
				{"copy_issues", strconv.Itoa(rep.CopyIssues)},
				{"duplicate_clusters", strconv.Itoa(rep.DuplicateClusters)},
				{"cost_drift_skus", strconv.Itoa(rep.CostDriftSKUs)},
				{"cost_history_rows", strconv.Itoa(rep.CostHistoryRows)},
			}
			return pbOutput(cmd, flags, rep, []string{"METRIC", "COUNT"}, table)
		},
	}
	cmd.Flags().Float64Var(&tolerance, "tolerance", 5.0, "Markup-drift tolerance (percentage points) for the markup_drift count")
	cmd.Flags().Float64Var(&minScore, "min-score", 0.85, "Minimum similarity (0-1) for the duplicate_clusters count")
	cmd.Flags().StringVar(&since, "since", "", "Baseline date for the cost_drift_skus count (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-pricebook-pp-cli/data.db)")
	return cmd
}

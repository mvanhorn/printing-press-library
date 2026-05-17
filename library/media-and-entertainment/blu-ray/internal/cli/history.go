package cli

// PATCH: Hand-built local price-history command with inline spark plot.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	"github.com/spf13/cobra"
)

type historyRow struct {
	ReleaseID  int     `json:"release_id"`
	RetailerID int     `json:"retailer_id"`
	ObservedAt string  `json:"observed_at"`
	Price      float64 `json:"price"`
}

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	var retailer string
	var plot bool
	cmd := &cobra.Command{
		Use:   "history <release-id>",
		Short: "Show locally recorded price history for a release.",
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli history 9929 --json
  blu-ray-pp-cli history 9929 --retailer 1 --plot
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("release-id must be numeric"))
			}
			// PATCH: --retailer accepts only a numeric retailer id. The prior
			// implementation passed the raw string to GetPriceHistory, which
			// silently dropped the filter when strconv.Atoi failed — so the
			// bundled example `--retailer amazon` returned every retailer's
			// history without warning. Fixes Greptile P1 on PR #634.
			retailerID := 0
			if retailer != "" {
				parsed, parseErr := strconv.Atoi(retailer)
				if parseErr != nil || parsed <= 0 {
					return fmt.Errorf("--retailer must be a numeric retailer id (got %q); Blu-ray.com price-history rows are keyed by numeric retailer id", retailer)
				}
				retailerID = parsed
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}
			observations, err := s.GetPriceHistory(cmd.Context(), id, retailerID)
			if err != nil {
				return err
			}
			var out []historyRow
			for _, row := range observations {
				out = append(out, historyRow(row))
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No price history for release %d. Add it with `watch add %d` and run `watch check`.\n", id, id)
				return nil
			}
			if flags.asJSON || flags.selectFields != "" || flags.csv || flags.quiet || flags.plain {
				return flags.printJSON(cmd, out)
			}
			if plot {
				fmt.Fprintln(cmd.OutOrStdout(), sparkPlot(out))
			}
			var table [][]string
			for _, r := range out {
				table = append(table, []string{strconv.Itoa(r.ReleaseID), strconv.Itoa(r.RetailerID), r.ObservedAt, formatPrice(r.Price)})
			}
			return flags.printTable(cmd, []string{"ID", "RETAILER", "OBSERVED", "PRICE"}, table)
		},
	}
	cmd.Flags().StringVar(&retailer, "retailer", "", "Numeric retailer id to filter price observations (Blu-ray.com price-history rows are keyed by retailer id, not name).")
	cmd.Flags().BoolVar(&plot, "plot", false, "Render a 60-character ASCII spark plot before the table.")
	return cmd
}

func sparkPlot(rows []historyRow) string {
	if len(rows) == 0 {
		return ""
	}
	const width = 60
	blocks := []rune("▁▂▃▄▅▆▇█")
	min, max := rows[0].Price, rows[0].Price
	for _, r := range rows {
		if r.Price < min {
			min = r.Price
		}
		if r.Price > max {
			max = r.Price
		}
	}
	var b strings.Builder
	// PATCH: Use one cell per point for small samples, and bucket averages for larger ones.
	plotWidth := width
	if len(rows) < plotWidth {
		plotWidth = len(rows)
	}
	for i := 0; i < plotWidth; i++ {
		bucketStart := i * len(rows) / plotWidth
		bucketEnd := (i + 1) * len(rows) / plotWidth
		var sum float64
		for _, r := range rows[bucketStart:bucketEnd] {
			sum += r.Price
		}
		price := sum / float64(bucketEnd-bucketStart)
		level := 0
		if max > min {
			level = int((price - min) / (max - min) * float64(len(blocks)-1))
		}
		b.WriteRune(blocks[level])
	}
	return b.String()
}

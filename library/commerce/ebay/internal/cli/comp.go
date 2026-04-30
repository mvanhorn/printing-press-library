package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	srcebay "github.com/mvanhorn/printing-press-library/library/commerce/ebay/internal/source/ebay"

	"github.com/spf13/cobra"
)

func newCompCmd(flags *rootFlags) *cobra.Command {
	var (
		days         int
		condition    string
		minPrice     float64
		maxPrice     float64
		category     string
		trim         bool
		dedupe       bool
		includeBO    bool
		includeItems bool
	)
	cmd := &cobra.Command{
		Use:   "comp <query>",
		Short: "Sold-comp intelligence: average sale price for a query over a time window",
		Long: `Analyze sold-completed eBay listings for a query and report mean, median, percentiles,
range, and outlier-trimmed stats. Default window is 90 days (the maximum eBay surfaces).

Smart matching can collapse near-duplicate variants and trim 1.5*IQR outliers.

Examples:
  ebay-pp-cli comp "Cooper Flagg gold /50 Topps Chrome"
  ebay-pp-cli comp "Rolex Submariner 116610LN" --condition used --trim
  ebay-pp-cli comp "PSA 10 Pikachu illustrator" --json --select mean,median,sample_size`,
		Example: `  ebay-pp-cli comp "Cooper Flagg gold /50 Topps Chrome" --trim
  ebay-pp-cli comp "Rolex Submariner" --condition used --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			src := srcebay.New(c)
			items, err := src.FetchSold(context.Background(), srcebay.SoldOptions{
				Query:      query,
				Category:   category,
				MinPrice:   minPrice,
				MaxPrice:   maxPrice,
				Condition:  condition,
				WindowDays: days,
			})
			if err != nil {
				return fmt.Errorf("fetching sold listings: %w", err)
			}
			if dedupe {
				items = srcebay.DedupeVariants(items)
			}
			stats := srcebay.AnalyzeComps(query, items, days, trim)
			if includeItems {
				stats.Items = items
			}
			data, err := json.Marshal(stats)
			if err != nil {
				return err
			}
			if !flags.asJSON && !flags.agent && flags.selectFields == "" {
				return printCompHuman(cmd, stats)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 90, "Sold-listings window in days (max 90)")
	cmd.Flags().StringVar(&condition, "condition", "", "Filter to a condition keyword: new, used, refurb, new-other")
	cmd.Flags().Float64Var(&minPrice, "min-price", 0, "Minimum sold price filter")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum sold price filter")
	cmd.Flags().StringVar(&category, "category", "", "eBay category id filter (numeric)")
	cmd.Flags().BoolVar(&trim, "trim", false, "Trim outliers using 1.5*IQR rule")
	cmd.Flags().BoolVar(&dedupe, "dedupe-variants", false, "Collapse near-duplicate titles to one exemplar each")
	cmd.Flags().BoolVar(&includeBO, "include-best-offers", false, "Include 'Best Offer Accepted' listings (parsed from caption)")
	cmd.Flags().BoolVar(&includeItems, "include-items", false, "Include the full sold-listings array in the JSON output")
	return cmd
}

func printCompHuman(cmd *cobra.Command, s srcebay.CompStats) error {
	w := cmd.OutOrStdout()
	if s.SampleSize == 0 {
		fmt.Fprintf(w, "No sold listings found for %q in the last %d days.\n", s.Query, s.WindowDays)
		return nil
	}
	fmt.Fprintf(w, "Comps for %q (%d days, n=%d", s.Query, s.WindowDays, s.SampleSize)
	if s.OutliersTrim > 0 {
		fmt.Fprintf(w, ", %d trimmed", s.OutliersTrim)
	}
	fmt.Fprintln(w, ")")
	fmt.Fprintf(w, "  Mean:   $%.2f\n", s.Mean)
	fmt.Fprintf(w, "  Median: $%.2f\n", s.Median)
	fmt.Fprintf(w, "  P25:    $%.2f\n", s.P25)
	fmt.Fprintf(w, "  P75:    $%.2f\n", s.P75)
	fmt.Fprintf(w, "  Min:    $%.2f\n", s.Min)
	fmt.Fprintf(w, "  Max:    $%.2f\n", s.Max)
	if s.StdDev > 0 {
		fmt.Fprintf(w, "  StdDev: $%.2f\n", s.StdDev)
	}
	if !s.FirstSold.IsZero() {
		fmt.Fprintf(w, "  Range:  %s -> %s\n", s.FirstSold.Format("2006-01-02"), s.LastSold.Format("2006-01-02"))
	}
	return nil
}

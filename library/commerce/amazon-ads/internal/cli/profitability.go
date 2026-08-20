package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/spf13/cobra"
)

func newBreakEvenACOSCmd(flags *rootFlags) *cobra.Command {
	var asin string
	var price float64
	var cogs float64
	var feePercent float64
	var currentACOSPercent float64
	var cogsPath string

	cmd := &cobra.Command{
		Use:   "break-even-acos",
		Short: "Calculate maximum ACOS before a product loses money",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := resolveProductCostForCommand(cogsPath, asin, price, cogs)
			if err != nil {
				return usageErr(err)
			}
			breakEven, err := adsanalytics.BreakEvenACOS(item.SellingPrice, item.COGS, feePercent)
			if err != nil {
				return usageErr(err)
			}
			out := map[string]any{
				"asin":                    asin,
				"name":                    item.Name,
				"price":                   item.SellingPrice,
				"cogs":                    item.COGS,
				"fee_percent":             feePercent,
				"break_even_acos":         breakEven,
				"break_even_acos_percent": breakEven * 100,
			}
			if currentACOSPercent > 0 {
				current := currentACOSPercent / 100
				out["current_acos"] = current
				out["current_acos_percent"] = currentACOSPercent
				out["profitable_at_current_acos"] = current <= breakEven
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&asin, "asin", "", "ASIN to look up in the COGS file")
	cmd.Flags().Float64Var(&price, "price", 0, "Selling price override")
	cmd.Flags().Float64Var(&cogs, "cogs", 0, "Cost of goods override")
	cmd.Flags().Float64Var(&feePercent, "fees", 30, "Estimated Amazon fees as a percentage of selling price")
	cmd.Flags().Float64Var(&currentACOSPercent, "current-acos", 0, "Current ACOS percentage for comparison")
	cmd.Flags().StringVar(&cogsPath, "cogs-file", "", "Path to COGS TOML file")
	return cmd
}

func newTrueProfitCmd(flags *rootFlags) *cobra.Command {
	var asin string
	var price float64
	var cogs float64
	var feePercent float64
	var adSpend float64
	var cogsPath string

	cmd := &cobra.Command{
		Use:   "true-profit",
		Short: "Calculate per-product profit after COGS, estimated fees, and ad spend",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			item, err := resolveProductCostForCommand(cogsPath, asin, price, cogs)
			if err != nil {
				return usageErr(err)
			}
			profit, err := adsanalytics.TrueProfit(item.SellingPrice, item.COGS, feePercent, adSpend)
			if err != nil {
				return usageErr(err)
			}
			fees := item.SellingPrice * (feePercent / 100)
			out := map[string]any{
				"asin":          asin,
				"name":          item.Name,
				"price":         item.SellingPrice,
				"cogs":          item.COGS,
				"fee_percent":   feePercent,
				"fees":          fees,
				"ad_spend":      adSpend,
				"profit":        profit,
				"profit_margin": profit / item.SellingPrice,
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&asin, "asin", "", "ASIN to look up in the COGS file")
	cmd.Flags().Float64Var(&price, "price", 0, "Selling price override")
	cmd.Flags().Float64Var(&cogs, "cogs", 0, "Cost of goods override")
	cmd.Flags().Float64Var(&feePercent, "fees", 30, "Estimated Amazon fees as a percentage of selling price")
	cmd.Flags().Float64Var(&adSpend, "ad-spend", 0, "Ad spend allocated to one sold unit")
	cmd.Flags().StringVar(&cogsPath, "cogs-file", "", "Path to COGS TOML file")
	return cmd
}

func newACOSVsTACOSCmd(flags *rootFlags) *cobra.Command {
	var adSpend float64
	var adRevenue float64
	var totalRevenue float64
	var reportPath string
	var sellerStorePath string
	var asin string

	cmd := &cobra.Command{
		Use:   "acos-vs-tacos",
		Short: "Calculate ACOS and TACOS from ad performance and optional seller-store revenue",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var sellerValidation *adsanalytics.SellerStoreValidation
			if reportPath != "" {
				rows, err := adsanalytics.LoadPerformanceReport(reportPath)
				if err != nil {
					return err
				}
				adSpend, adRevenue = summarizeAdRevenue(rows, asin)
				startDate, endDate := performanceDateRange(rows)
				if sellerStorePath != "" || totalRevenue == 0 {
					validation, err := adsanalytics.ValidateSellerStore(sellerStorePath, "", flags.adsProfileID, startDate, endDate)
					if err != nil {
						return err
					}
					sellerValidation = &validation
				}
			} else if totalRevenue == 0 {
				validation, err := adsanalytics.ValidateSellerStore(sellerStorePath, "", flags.adsProfileID, "", "")
				if err != nil {
					return err
				}
				sellerValidation = &validation
			}
			acos, err := adsanalytics.ACOS(adSpend, adRevenue)
			if err != nil {
				return usageErr(err)
			}
			out := map[string]any{
				"ad_spend":     adSpend,
				"ad_revenue":   adRevenue,
				"acos":         acos,
				"acos_percent": acos * 100,
			}
			if reportPath != "" {
				out["report"] = reportPath
			}
			if sellerValidation != nil {
				out["seller_store_validation"] = *sellerValidation
			}
			if asin != "" {
				out["asin"] = asin
			}
			if totalRevenue == 0 {
				sellerRevenue, err := adsanalytics.LoadSellerRevenue(sellerStorePath, asin)
				if err != nil {
					return err
				}
				if sellerRevenue.Revenue > 0 {
					totalRevenue = sellerRevenue.Revenue
				}
				out["seller_revenue"] = sellerRevenue
			}
			if totalRevenue > 0 {
				tacos, err := adsanalytics.TACOS(adSpend, totalRevenue)
				if err != nil {
					return usageErr(err)
				}
				out["total_revenue"] = totalRevenue
				out["organic_revenue"] = totalRevenue - adRevenue
				out["tacos"] = tacos
				out["tacos_percent"] = tacos * 100
			} else {
				out["note"] = "TACOS requires total revenue from amazon-seller store data or --total-revenue."
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().Float64Var(&adSpend, "ad-spend", 0, "Advertising spend")
	cmd.Flags().Float64Var(&adRevenue, "ad-revenue", 0, "Revenue attributed to advertising")
	cmd.Flags().Float64Var(&totalRevenue, "total-revenue", 0, "Total revenue including organic sales")
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a campaign/product performance CSV or JSON export")
	cmd.Flags().StringVar(&sellerStorePath, "seller-store", "", "Path to amazon-seller-pp-cli store.db")
	cmd.Flags().StringVar(&asin, "asin", "", "Limit report and seller-store revenue to one ASIN when available")
	return cmd
}

func performanceDateRange(rows []adsanalytics.PerformanceRow) (string, string) {
	start := ""
	end := ""
	for _, row := range rows {
		if row.Date == "" {
			continue
		}
		if start == "" || row.Date < start {
			start = row.Date
		}
		if end == "" || row.Date > end {
			end = row.Date
		}
	}
	return start, end
}

func summarizeAdRevenue(rows []adsanalytics.PerformanceRow, asin string) (float64, float64) {
	spend := 0.0
	sales := 0.0
	for _, row := range rows {
		if asin != "" && row.ASIN != asin {
			continue
		}
		spend += row.Spend
		sales += row.Sales
	}
	return spend, sales
}

func resolveProductCostForCommand(cogsPath, asin string, price, cogs float64) (adsanalytics.ProductCost, error) {
	items := map[string]adsanalytics.ProductCost{}
	if asin != "" && (price <= 0 || cogs <= 0) {
		loaded, err := adsanalytics.LoadCOGS(cogsPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return adsanalytics.ProductCost{}, err
		}
		if err == nil {
			items = loaded
		}
	}
	return adsanalytics.ResolveProductCost(items, asin, price, cogs)
}

func printCommandJSON(cmd *cobra.Command, flags *rootFlags, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}
	if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		} else if flags.compact {
			data = compactFields(data)
		}
		return printOutput(cmd.OutOrStdout(), data, true)
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}
	return printOutput(cmd.OutOrStdout(), pretty, true)
}

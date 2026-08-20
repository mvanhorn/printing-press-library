package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/spf13/cobra"
)

func newPortfolioDashboardCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var reportKind string
	var allowPartial bool

	cmd := &cobra.Command{
		Use:   "portfolio-dashboard",
		Short: "Summarize spend, sales, ACOS, CPC, CTR, and CVR from a performance report",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, schemaReport, err := loadPerformanceRowsForCommand(cmd, reportPath, reportLoadOptions{ReportKind: reportKind, AllowPartial: allowPartial, Command: "portfolio-dashboard"})
			if err != nil {
				return err
			}
			summary := adsanalytics.PortfolioDashboard(rows)
			out := map[string]any{
				"report":  reportPath,
				"summary": summary,
			}
			if schemaReport.Kind != "" {
				out["report_kind"] = schemaReport.Kind
				out["detected_candidates"] = schemaReport.Validation.Candidates
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a campaign/product performance CSV or JSON export")
	cmd.Flags().StringVar(&reportKind, "report-kind", "", "Explicit report schema kind (see reports recipe portfolio-dashboard)")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Allow missing schema columns with a warning")
	return cmd
}

func newCampaignComparisonCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var reportKind string
	var allowPartial bool

	cmd := &cobra.Command{
		Use:   "campaign-comparison",
		Short: "Compare campaigns by spend, sales, ACOS, CPC, CTR, and CVR",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, schemaReport, err := loadPerformanceRowsForCommand(cmd, reportPath, reportLoadOptions{ReportKind: reportKind, AllowPartial: allowPartial, Command: "campaign-comparison"})
			if err != nil {
				return err
			}
			campaigns := adsanalytics.CampaignComparison(rows)
			out := map[string]any{
				"report":    reportPath,
				"campaigns": campaigns,
				"count":     len(campaigns),
			}
			if schemaReport.Kind != "" {
				out["report_kind"] = schemaReport.Kind
				out["detected_candidates"] = schemaReport.Validation.Candidates
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a campaign performance CSV or JSON export")
	cmd.Flags().StringVar(&reportKind, "report-kind", "", "Explicit report schema kind (see reports recipe campaign-comparison)")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Allow missing schema columns with a warning")
	return cmd
}

func newProductAdProfitabilityCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var cogsPath string
	var feePercent float64
	var reportKind string
	var allowPartial bool

	cmd := &cobra.Command{
		Use:   "product-ad-profitability",
		Short: "Estimate per-ASIN profit from ad performance, COGS, fees, and spend",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, schemaReport, err := loadPerformanceRowsForCommand(cmd, reportPath, reportLoadOptions{ReportKind: reportKind, AllowPartial: allowPartial, Command: "product-ad-profitability"})
			if err != nil {
				return err
			}
			costs, err := loadOptionalCOGS(cogsPath)
			if err != nil {
				return err
			}
			products := adsanalytics.ProductAdProfitability(rows, costs, feePercent)
			out := map[string]any{
				"report":       reportPath,
				"fee_percent":  feePercent,
				"products":     products,
				"count":        len(products),
				"missing_cogs": missingCOGS(rows, costs),
			}
			if schemaReport.Kind != "" {
				out["report_kind"] = schemaReport.Kind
				out["detected_candidates"] = schemaReport.Validation.Candidates
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a product performance CSV or JSON export")
	cmd.Flags().StringVar(&cogsPath, "cogs-file", "", "Path to COGS TOML file")
	cmd.Flags().Float64Var(&feePercent, "fees", 30, "Estimated Amazon fees as a percentage of sales")
	cmd.Flags().StringVar(&reportKind, "report-kind", "", "Explicit report schema kind (see reports recipe product-ad-profitability)")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Allow missing schema columns with a warning")
	return cmd
}

func newPlacementAnalysisCmd(flags *rootFlags) *cobra.Command {
	var reportPath string

	cmd := &cobra.Command{
		Use:   "placement-analysis",
		Short: "Compare performance by placement from an Amazon Ads report",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadPerformanceRows(reportPath)
			if err != nil {
				return err
			}
			placements := adsanalytics.PlacementAnalysis(rows)
			out := map[string]any{
				"report":     reportPath,
				"placements": placements,
				"count":      len(placements),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a placement performance CSV or JSON export")
	return cmd
}

func newCompetitorASINMiningCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var ownASIN string

	cmd := &cobra.Command{
		Use:   "competitor-asin-mining",
		Short: "Find competitor ASIN product targets that convert or waste spend",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadPerformanceRows(reportPath)
			if err != nil {
				return err
			}
			findings := adsanalytics.CompetitorASINMining(rows, ownASIN)
			out := map[string]any{
				"report":   reportPath,
				"asin":     ownASIN,
				"findings": findings,
				"count":    len(findings),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a product-targeting performance CSV or JSON export")
	cmd.Flags().StringVar(&ownASIN, "asin", "", "Own ASIN to exclude from competitor findings")
	return cmd
}

func newSeasonalPlannerCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var budgetMultiplier float64

	cmd := &cobra.Command{
		Use:   "seasonal-planner",
		Short: "Summarize monthly performance and budget recommendations",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadPerformanceRows(reportPath)
			if err != nil {
				return err
			}
			plans := adsanalytics.SeasonalPlanner(rows, budgetMultiplier)
			out := map[string]any{
				"report":            reportPath,
				"budget_multiplier": budgetMultiplier,
				"periods":           plans,
				"count":             len(plans),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a dated campaign/product performance CSV or JSON export")
	cmd.Flags().Float64Var(&budgetMultiplier, "budget-multiplier", 1.25, "Multiplier for high-performing seasonal periods")
	return cmd
}

func newDaypartingAnalysisCmd(flags *rootFlags) *cobra.Command {
	var reportPath string

	cmd := &cobra.Command{
		Use:   "dayparting-analysis",
		Short: "Analyze hourly performance by day of week from a report with hour/date columns",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadPerformanceRows(reportPath)
			if err != nil {
				return err
			}
			cells := adsanalytics.DaypartingAnalysis(rows)
			out := map[string]any{
				"report": reportPath,
				"cells":  cells,
				"count":  len(cells),
			}
			if len(cells) == 0 {
				out["note"] = "No hourly rows found. Use an Amazon Ads report that includes hour/hourOfDay or timestamped date rows."
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to an hourly campaign/performance CSV or JSON export")
	return cmd
}

func newBudgetPacingCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var threshold float64
	var earlyHour int

	cmd := &cobra.Command{
		Use:   "budget-pacing",
		Short: "Find campaigns that reach a budget-spend threshold early in the day",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			rows, err := loadPerformanceRows(reportPath)
			if err != nil {
				return err
			}
			findings := adsanalytics.BudgetPacing(rows, threshold, earlyHour)
			out := map[string]any{
				"report":     reportPath,
				"threshold":  threshold,
				"early_hour": earlyHour,
				"findings":   findings,
				"count":      len(findings),
			}
			if len(findings) == 0 {
				out["note"] = "No early budget pacing found, or the report lacks hourly spend and daily budget columns."
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to an hourly campaign performance CSV or JSON export")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.90, "Budget spend threshold as a fraction, e.g. 0.90")
	cmd.Flags().IntVar(&earlyHour, "early-hour", 18, "Latest hour considered early budget exhaustion")
	return cmd
}

func loadPerformanceRows(reportPath string) ([]adsanalytics.PerformanceRow, error) {
	if reportPath == "" {
		return nil, usageErr(fmt.Errorf("--report is required"))
	}
	return adsanalytics.LoadPerformanceReport(reportPath)
}

func loadPerformanceRowsForCommand(cmd *cobra.Command, reportPath string, opts reportLoadOptions) ([]adsanalytics.PerformanceRow, adsanalytics.NormalizedSchemaReport, error) {
	if reportPath == "" {
		return nil, adsanalytics.NormalizedSchemaReport{}, usageErr(fmt.Errorf("--report is required"))
	}
	if opts.ReportKind == "" && !opts.AllowPartial {
		rows, err := adsanalytics.LoadPerformanceReport(reportPath)
		return rows, adsanalytics.NormalizedSchemaReport{}, err
	}
	rows, report, err := loadSchemaPerformanceReport(cmd, reportPath, opts)
	return rows, report, err
}

func loadOptionalCOGS(cogsPath string) (map[string]adsanalytics.ProductCost, error) {
	if cogsPath == "" && adsanalytics.DefaultCOGSPath() == "" {
		return map[string]adsanalytics.ProductCost{}, nil
	}
	costs, err := adsanalytics.LoadCOGS(cogsPath)
	if err != nil {
		return map[string]adsanalytics.ProductCost{}, nil
	}
	return costs, nil
}

func missingCOGS(rows []adsanalytics.PerformanceRow, costs map[string]adsanalytics.ProductCost) []string {
	seen := map[string]struct{}{}
	var missing []string
	for _, row := range rows {
		if row.ASIN == "" {
			continue
		}
		if _, ok := costs[row.ASIN]; ok {
			continue
		}
		if _, ok := seen[row.ASIN]; ok {
			continue
		}
		seen[row.ASIN] = struct{}{}
		missing = append(missing, row.ASIN)
	}
	return missing
}

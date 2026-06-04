package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/spf13/cobra"
)

func newSearchTermMiningCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var promoteThreshold int
	var negateThreshold float64
	var targetACOS float64

	cmd := &cobra.Command{
		Use:   "search-term-mining",
		Short: "Find search terms to promote or negate from a search-term report",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadSearchTermReport(reportPath)
			if err != nil {
				return err
			}
			recs := adsanalytics.SearchTermMining(rows, promoteThreshold, negateThreshold, targetACOS)
			out := map[string]any{
				"report":          reportPath,
				"recommendations": recs,
				"count":           len(recs),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a Search Term Report CSV or JSON export")
	cmd.Flags().IntVar(&promoteThreshold, "promote-threshold", 3, "Minimum conversions before suggesting exact-match promotion")
	cmd.Flags().Float64Var(&negateThreshold, "negate-threshold", 10, "Spend threshold for zero-conversion negative keyword candidates")
	cmd.Flags().Float64Var(&targetACOS, "target-acos", 25, "Target ACOS percentage for promotion candidates")
	return cmd
}

func newWastedSpendCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var threshold float64

	cmd := &cobra.Command{
		Use:   "wasted-spend",
		Short: "List zero-conversion search terms over a spend threshold",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadSearchTermReport(reportPath)
			if err != nil {
				return err
			}
			recs := adsanalytics.WastedSpend(rows, threshold)
			total := 0.0
			for _, rec := range recs {
				total += rec.Spend
			}
			out := map[string]any{
				"report":          reportPath,
				"threshold":       threshold,
				"wasted_spend":    total,
				"recommendations": recs,
				"count":           len(recs),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a Search Term Report CSV or JSON export")
	cmd.Flags().Float64Var(&threshold, "threshold", 10, "Minimum spend for wasted-spend candidates")
	return cmd
}

func newNegativeKeywordGeneratorCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var threshold float64

	cmd := &cobra.Command{
		Use:   "negative-keyword-generator",
		Short: "Generate negative exact keyword candidates from zero-conversion search terms",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadSearchTermReport(reportPath)
			if err != nil {
				return err
			}
			recs := adsanalytics.WastedSpend(rows, threshold)
			terms := make([]string, 0, len(recs))
			for _, rec := range recs {
				terms = append(terms, rec.SearchTerm)
			}
			out := map[string]any{
				"report":               reportPath,
				"threshold":            threshold,
				"negative_exact_terms": terms,
				"recommendations":      recs,
				"count":                len(recs),
				"dry_run":              true,
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a Search Term Report CSV or JSON export")
	cmd.Flags().Float64Var(&threshold, "threshold", 10, "Minimum spend for negative keyword candidates")
	return cmd
}

func newKeywordCannibalizationCmd(flags *rootFlags) *cobra.Command {
	var reportPath string

	cmd := &cobra.Command{
		Use:   "keyword-cannibalization",
		Short: "Find search terms competing across multiple campaigns",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadSearchTermReport(reportPath)
			if err != nil {
				return err
			}
			findings := adsanalytics.KeywordCannibalization(rows)
			out := map[string]any{
				"report":   reportPath,
				"findings": findings,
				"count":    len(findings),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a Search Term Report CSV or JSON export")
	return cmd
}

func newNewKeywordOpportunitiesCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var minConversions int
	var targetACOS float64

	cmd := &cobra.Command{
		Use:   "new-keyword-opportunities",
		Short: "Find converting broad/auto search terms missing exact-match coverage",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadSearchTermReport(reportPath)
			if err != nil {
				return err
			}
			opportunities := adsanalytics.NewKeywordOpportunities(rows, minConversions, targetACOS)
			out := map[string]any{
				"report":        reportPath,
				"target_acos":   targetACOS,
				"opportunities": opportunities,
				"count":         len(opportunities),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a Search Term Report CSV or JSON export")
	cmd.Flags().IntVar(&minConversions, "min-conversions", 3, "Minimum conversions before recommending a new exact keyword")
	cmd.Flags().Float64Var(&targetACOS, "target-acos", 25, "Maximum ACOS percentage for new keyword candidates")
	return cmd
}

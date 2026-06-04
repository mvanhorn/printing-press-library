package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/spf13/cobra"
)

func newReportsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reports",
		Short: "Report schemas, recipes, detection, and normalization help",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReportsRecipeCmd(flags))
	cmd.AddCommand(newReportsDetectCmd(flags))
	return cmd
}

func newReportsRecipeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe <command>",
		Short: "Show the report export recipe for an analytics command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := adsanalytics.LoadReportRegistry()
			if err != nil {
				return err
			}
			recipe, ok := registry.Recipe(args[0])
			if !ok {
				return usageErr(fmt.Errorf("no report recipe for %q", args[0]))
			}
			if flags.asJSON {
				schemas := make([]adsanalytics.ReportSchema, 0, len(recipe.AcceptedKinds))
				for _, kind := range recipe.AcceptedKinds {
					if schema, ok := registry.Schema(kind); ok {
						schemas = append(schemas, schema)
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"recipe":  recipe,
					"schemas": schemas,
				}, flags)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "command:\t%s\n", recipe.Command)
			fmt.Fprintf(tw, "purpose:\t%s\n", recipe.Description)
			fmt.Fprintf(tw, "accepted kinds:\t%s\n", strings.Join(recipe.AcceptedKinds, ", "))
			for _, kind := range recipe.AcceptedKinds {
				schema, ok := registry.Schema(kind)
				if !ok {
					continue
				}
				fmt.Fprintf(tw, "\nreport-kind:\t%s\n", schema.Kind)
				fmt.Fprintf(tw, "ad_product:\t%s\n", schema.AdProduct)
				fmt.Fprintf(tw, "entity_level:\t%s\n", schema.EntityLevel)
				fmt.Fprintf(tw, "time_unit:\t%s\n", schema.TimeUnit)
				fmt.Fprintf(tw, "attribution:\t%s\n", schema.AttributionWindow)
				fmt.Fprintf(tw, "required:\t%s\n", strings.Join(schema.RequiredColumns, ", "))
				if len(schema.ApplyCapableColumns) > 0 {
					fmt.Fprintf(tw, "for --apply:\t+ %s\n", strings.Join(schema.ApplyCapableColumns, ", "))
				} else {
					fmt.Fprintf(tw, "for --apply:\tnot mutation-capable\n")
				}
				fmt.Fprintf(tw, "formats:\tconsole CSV/TSV export, or Ads API JSON/GZIP report artifact\n")
				fmt.Fprintf(tw, "export path:\t%s\n", schema.ExportPath)
				fmt.Fprintf(tw, "sample header:\t%s\n", schema.SampleHeader)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newReportsDetectCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	cmd := &cobra.Command{
		Use:   "detect",
		Short: "Detect report schema candidates for a CSV/TSV/JSON/GZIP report",
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			candidates, err := adsanalytics.DetectReportKind(reportPath, nil)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"report":     reportPath,
					"candidates": candidates,
				}, flags)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "kind\tconfidence\tentity\ttime_unit\tmissing\n")
			for _, candidate := range candidates {
				fmt.Fprintf(tw, "%s\t%.2f\t%s\t%s\t%s\n", candidate.Kind, candidate.Confidence, candidate.EntityLevel, candidate.TimeUnit, strings.Join(candidate.Missing, ","))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a console CSV/TSV or Ads API JSON/GZIP report")
	return cmd
}

type reportLoadOptions struct {
	ReportKind   string
	AllowPartial bool
	Command      string
}

func loadSchemaPerformanceReport(cmd *cobra.Command, reportPath string, opts reportLoadOptions) ([]adsanalytics.PerformanceRow, adsanalytics.NormalizedSchemaReport, error) {
	accepted, err := acceptedReportKinds(opts.Command)
	if err != nil {
		return nil, adsanalytics.NormalizedSchemaReport{}, err
	}
	report, err := adsanalytics.NormalizeSchemaReport(reportPath, opts.ReportKind, accepted, opts.AllowPartial)
	if err != nil {
		return nil, adsanalytics.NormalizedSchemaReport{}, err
	}
	warnPartialReport(cmd, report)
	return adsanalytics.PerformanceRowsFromCanonical(report.Rows), report, nil
}

func loadSchemaSearchTermReport(cmd *cobra.Command, reportPath string, opts reportLoadOptions) ([]adsanalytics.SearchTermPerformance, adsanalytics.NormalizedSchemaReport, error) {
	accepted, err := acceptedReportKinds(opts.Command)
	if err != nil {
		return nil, adsanalytics.NormalizedSchemaReport{}, err
	}
	report, err := adsanalytics.NormalizeSchemaReport(reportPath, opts.ReportKind, accepted, opts.AllowPartial)
	if err != nil {
		return nil, adsanalytics.NormalizedSchemaReport{}, err
	}
	warnPartialReport(cmd, report)
	return adsanalytics.SearchTermRowsFromCanonical(report.Rows), report, nil
}

func loadSchemaKeywordReport(cmd *cobra.Command, reportPath string, opts reportLoadOptions) ([]adsanalytics.KeywordPerformance, adsanalytics.NormalizedSchemaReport, error) {
	accepted, err := acceptedReportKinds(opts.Command)
	if err != nil {
		return nil, adsanalytics.NormalizedSchemaReport{}, err
	}
	report, err := adsanalytics.NormalizeSchemaReport(reportPath, opts.ReportKind, accepted, opts.AllowPartial)
	if err != nil {
		return nil, adsanalytics.NormalizedSchemaReport{}, err
	}
	warnPartialReport(cmd, report)
	return adsanalytics.KeywordRowsFromCanonical(report.Rows), report, nil
}

func acceptedReportKinds(command string) ([]string, error) {
	if strings.TrimSpace(command) == "" {
		return nil, nil
	}
	registry, err := adsanalytics.LoadReportRegistry()
	if err != nil {
		return nil, err
	}
	recipe, ok := registry.Recipe(command)
	if !ok {
		return nil, nil
	}
	return recipe.AcceptedKinds, nil
}

func warnPartialReport(cmd *cobra.Command, report adsanalytics.NormalizedSchemaReport) {
	if !report.Validation.Partial {
		return
	}
	for _, warning := range report.Validation.Warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
	}
}

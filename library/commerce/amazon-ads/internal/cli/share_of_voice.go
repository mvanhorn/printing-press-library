package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/spf13/cobra"
)

func newShareOfVoiceCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var asin string
	var keywordsCSV string
	var threshold float64

	cmd := &cobra.Command{
		Use:   "share-of-voice",
		Short: "Summarize impression share by ASIN and keyword from a share-of-voice report",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadShareOfVoiceReport(reportPath)
			if err != nil {
				return err
			}
			keywords := splitCSV(keywordsCSV)
			findings := adsanalytics.ShareOfVoice(rows, asin, keywords, threshold)
			out := map[string]any{
				"report":    reportPath,
				"asin":      asin,
				"keywords":  keywords,
				"threshold": threshold,
				"findings":  findings,
				"count":     len(findings),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a share-of-voice CSV or JSON export")
	cmd.Flags().StringVar(&asin, "asin", "", "ASIN to filter")
	cmd.Flags().StringVar(&keywordsCSV, "keywords", "", "Comma-separated keywords to filter")
	cmd.Flags().Float64Var(&threshold, "low-share-threshold", 0.10, "Low share threshold as a fraction, e.g. 0.10")
	return cmd
}

func splitCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

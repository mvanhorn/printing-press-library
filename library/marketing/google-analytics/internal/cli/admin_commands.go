package cli

import (
	"context"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-analytics/internal/ga4"
	"github.com/spf13/cobra"
)

func newPropertiesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "properties", Short: "List accessible GA4 account summaries/properties", RunE: func(cmd *cobra.Command, args []string) error {
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.AccountSummaries(context.Background())
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, tableProperties(raw))
	}}
}
func newPropertyCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "property", Short: "Get GA4 Admin property metadata", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.Property(context.Background(), p)
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
}
func newStreamsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{Use: "streams", Short: "List data streams for a GA4 property", RunE: func(cmd *cobra.Command, args []string) error {
		p, err := requireProperty(flags)
		if err != nil {
			return err
		}
		cl, _, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, _, err := cl.DataStreams(context.Background(), p)
		if err != nil {
			return err
		}
		return output(cmd, flags, raw, "")
	}}
}
func tableProperties(raw ga4.AccountSummariesResponse) string {
	rows := []map[string]any{}
	for _, summary := range raw.AccountSummaries {
		for _, prop := range summary.PropertySummaries {
			rows = append(rows, map[string]any{
				"account":       summary.Account,
				"account_name":  summary.DisplayName,
				"property":      cleanProperty(prop.Property),
				"property_name": prop.DisplayName,
			})
		}
	}
	return table(rows)
}

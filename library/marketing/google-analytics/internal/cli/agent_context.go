package cli

import "github.com/spf13/cobra"

func newAgentContextCmd() *cobra.Command {
	return &cobra.Command{Use: "agent-context", Short: "Emit structured tool description for agents", RunE: func(cmd *cobra.Command, args []string) error {
		return printJSON(cmd.OutOrStdout(), map[string]any{"name": "google-analytics-pp-cli", "binary": "google-analytics-pp-cli", "purpose": "GA4-only analytics CLI with raw API wrappers and one-call novel reports", "auth": "Google service account JSON via --credentials or GOOGLE_APPLICATION_CREDENTIALS; analytics.readonly scope", "property_resolution": "--property, else GA4_PROPERTY_ID; health can accept --properties for fleet checks", "global_flags": []string{"--agent", "--json", "--compact", "--no-input", "--yes", "--property", "--credentials", "--timeout"}, "raw_commands": []string{"report", "pivot", "batch", "realtime", "metadata", "compatibility", "properties", "property", "streams"}, "novel_commands": []string{"channels", "sources", "top-pages", "events", "conversions", "funnel", "compare", "whats-changed", "revenue", "audience", "cohort", "health", "doctor"}, "examples": []string{"google-analytics-pp-cli health --properties 280199692,540652239 --agent", "google-analytics-pp-cli channels --property 280199692 --start 28daysAgo --end yesterday --agent", "google-analytics-pp-cli compare --property 280199692 --metrics sessions,totalRevenue --period wow --agent", "google-analytics-pp-cli funnel --property 280199692 --steps view_item,add_to_cart,begin_checkout,purchase --agent"}})
	}}
}

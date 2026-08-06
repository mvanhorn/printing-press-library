package cli

import "github.com/spf13/cobra"

func newAgentContextCmd() *cobra.Command {
	return &cobra.Command{Use: "agent-context", Short: "Emit structured tool description for agents", RunE: func(cmd *cobra.Command, args []string) error {
		return printJSON(cmd.OutOrStdout(), map[string]any{
			"name":                "google-analytics-pp-cli",
			"binary":              "google-analytics-pp-cli",
			"purpose":             "GA4-only analytics CLI with raw API wrappers, one-call novel reports, and a gated GA4 Admin write surface",
			"auth":                "Google service-account JSON key or ADC authorized_user JSON (client_id + client_secret + refresh_token); resolution is --credentials, then GOOGLE_ANALYTICS_ADC, then GOOGLE_APPLICATION_CREDENTIALS",
			"scopes":              map[string]string{"read": "analytics.readonly", "write": "analytics.edit (also requires Editor or Administrator on the property)"},
			"property_resolution": "--property, else GA4_PROPERTY_ID; health can accept --properties for fleet checks",
			"global_flags":        []string{"--agent", "--json", "--compact", "--no-input", "--yes", "--property", "--credentials", "--timeout"},
			"raw_commands":        []string{"report", "pivot", "batch", "realtime", "metadata", "compatibility", "properties", "property", "streams"},
			"novel_commands":      []string{"channels", "sources", "top-pages", "events", "conversions", "funnel", "compare", "whats-changed", "revenue", "audience", "cohort", "health", "doctor"},
			"write_commands":      []string{"key-events create|patch|delete", "custom-dimensions create|archive", "custom-metrics create|archive", "data-streams create|patch|delete", "admin POST|PATCH|PUT|DELETE"},
			"destructive_gate":    "Every delete, plus custom-dimensions archive and custom-metrics archive, requires --yes to appear on the command line. --agent expands to include --yes but does NOT satisfy this gate; a human must add --yes explicitly.",
			"gotchas": []string{
				"accessBindings exists only in Admin API v1alpha; v1beta returns 404 with an HTML page instead of JSON. Use: admin GET properties/<id>/accessBindings --api v1alpha",
				"A 403 on a write means the token carried the scope but the account lacks Editor/Administrator on the property.",
				"To prove write access without mutating anything, create a key event that already exists and expect 409 ALREADY_EXISTS.",
				"An authorized_user token carries whatever its original grant allowed; health reports scope_requested, not scope granted.",
			},
			"examples": []string{
				"google-analytics-pp-cli health --properties $GA4_PROPERTY_IDS --agent",
				"google-analytics-pp-cli channels --property $GA4_PROPERTY_ID --start 28daysAgo --end yesterday --agent",
				"google-analytics-pp-cli compare --property $GA4_PROPERTY_ID --metrics sessions,totalRevenue --period wow --agent",
				"google-analytics-pp-cli funnel --property $GA4_PROPERTY_ID --steps view_item,add_to_cart,begin_checkout,purchase --agent",
				"google-analytics-pp-cli key-events list --property $GA4_PROPERTY_ID --agent",
				"google-analytics-pp-cli key-events create --property $GA4_PROPERTY_ID --event generate_lead --agent",
				"google-analytics-pp-cli key-events delete --name properties/123/keyEvents/abc --agent --yes",
				"google-analytics-pp-cli custom-dimensions create --property $GA4_PROPERTY_ID --parameter plan --display-name Plan --scope EVENT --agent",
				"google-analytics-pp-cli admin GET properties/123/accessBindings --api v1alpha --agent",
				"google-analytics-pp-cli admin PATCH properties/123/keyEvents/abc --update-mask countingMethod --body '{\"countingMethod\":\"ONCE_PER_SESSION\"}' --agent",
			},
		})
	}}
}

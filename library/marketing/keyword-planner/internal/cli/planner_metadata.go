package cli

func init() {
	whichIndex = append([]whichEntry{
		{Command: "ideas", Description: "Discover keyword ideas from subject-level seeds with complete pagination.", Group: "Collect evidence"},
		{Command: "historical", Description: "Retrieve historical monthly search metrics for chosen keywords.", Group: "Collect evidence"},
		{Command: "portfolio list", Description: "List saved snapshot collections and completeness status offline.", Group: "Inspect evidence"},
		{Command: "portfolio show", Description: "Show a saved snapshot, request, raw receipts, and coverage.", Group: "Inspect evidence"},
		{Command: "portfolio search", Description: "Search stored keyword monthly rows using local filters.", Group: "Inspect evidence"},
		{Command: "portfolio export", Description: "Export stored monthly evidence as JSON or CSV.", Group: "Inspect evidence"},
		{Command: "doctor", Description: "Check offline readiness or explicitly test live Google Ads authentication.", Group: "Setup"},
	}, whichIndex...)
}

// PlannerAuthMetadata describes bindings without loading credential values.
func PlannerAuthMetadata() map[string]any {
	auth := plannerAgentAuth()
	return map[string]any{
		"type": auth.Mode, "env_vars": auth.EnvVars,
		"credential_file":          "~/.env",
		"credential_file_override": "KEYWORD_PLANNER_ENV_FILE",
		"access_token_storage":     "memory only",
		"operating_target":         "GOOGLE_ADS_CUSTOMER_ID",
		"manager_context":          "GOOGLE_ADS_LOGIN_CUSTOMER_ID",
	}
}

func plannerAgentAuth() agentContextAuth {
	return agentContextAuth{
		Mode: "oauth2_refresh",
		EnvVars: []agentContextAuthEnvVar{
			{Name: "GOOGLE_ADS_CLIENT_ID", Kind: "auth_flow_input", Required: true, Description: "OAuth client associated with the approved refresh grant."},
			{Name: "GOOGLE_ADS_CLIENT_SECRET", Kind: "auth_flow_input", Required: true, Sensitive: true, Description: "OAuth client secret; loaded only for live calls."},
			{Name: "GOOGLE_ADS_REFRESH_TOKEN", Kind: "auth_flow_input", Required: true, Sensitive: true, Description: "Approved AdWords refresh grant; access tokens stay in memory."},
			{Name: "GOOGLE_ADS_DEVELOPER_TOKEN", Kind: "per_call", Required: true, Sensitive: true, Description: "Google Ads developer-token header."},
			{Name: "GOOGLE_ADS_CUSTOMER_ID", Kind: "per_call", Required: true, Description: "Separately verified operating customer ID."},
			{Name: "GOOGLE_ADS_LOGIN_CUSTOMER_ID", Kind: "per_call", Description: "Optional manager login context; distinct from the operating target."},
		},
	}
}

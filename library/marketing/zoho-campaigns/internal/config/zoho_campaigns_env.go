// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored companion to the generated Load: maps the canonical
// ZOHO_CAMPAIGNS_* credential fields (classified as auth-flow inputs) onto the
// OAuth client fields the token-refresh path actually reads. Without this,
// env-var-only headless runs (cron, CI, agent sandboxes with no config file
// and no prior `auth login`) send unauthenticated requests and 401.

package config

// applyZohoCampaignsFlowInputs promotes flow-input credentials to the active
// OAuth fields when nothing else supplied them. Existing values always win so
// config-file and credentials-file auth behave exactly as before.
func (c *Config) applyZohoCampaignsFlowInputs() {
	if c.ClientID == "" && c.ZohoCampaignsClientId != "" {
		c.ClientID = c.ZohoCampaignsClientId
	}
	if c.ClientSecret == "" && c.ZohoCampaignsClientSecret != "" {
		c.ClientSecret = c.ZohoCampaignsClientSecret
	}
	if c.RefreshToken == "" && c.ZohoCampaignsRefreshToken != "" {
		c.RefreshToken = c.ZohoCampaignsRefreshToken
		c.AuthSource = "oauth2_refresh"
	}
}

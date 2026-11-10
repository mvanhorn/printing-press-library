// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"

// RequireCampaignCompliance shares the CLI send preflight with MCP execution.
func RequireCampaignCompliance(c *client.Client, campaignID string) error {
	return requireCampaignCompliance(c, campaignID)
}

// RequireTemplateCompliance shares the CLI template assignment preflight with MCP execution.
func RequireTemplateCompliance(c *client.Client, templateID string) error {
	return requireTemplateCompliance(c, templateID)
}

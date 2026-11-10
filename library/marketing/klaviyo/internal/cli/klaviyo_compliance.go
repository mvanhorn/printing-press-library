// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"

// RequireCampaignCompliance shares the CLI send preflight with MCP execution.
func RequireCampaignCompliance(c *client.Client, campaignID string) error {
	return requireCampaignCompliance(c, campaignID)
}

// RequireCampaignMessageScheduleCompliance checks the message being scheduled by CLI or MCP.
func RequireCampaignMessageScheduleCompliance(c *client.Client, body any) error {
	payload, _ := body.(map[string]any)
	return requireCampaignCompliance(c, stringFromMapPath(payload, "data.relationships.campaign-message.data.id"))
}

// RequireTemplateCompliance shares the CLI template assignment preflight with MCP execution.
func RequireTemplateCompliance(c *client.Client, templateID string) error {
	return requireTemplateCompliance(c, templateID)
}

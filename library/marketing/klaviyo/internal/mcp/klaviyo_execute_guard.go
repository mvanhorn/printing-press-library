// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
)

func requireKlaviyoExecuteCompliance(c *client.Client, path string, body any) error {
	payload, _ := body.(map[string]any)
	data, _ := payload["data"].(map[string]any)
	switch strings.TrimRight(path, "/") {
	case "/api/campaign-send-jobs":
		id, _ := data["id"].(string)
		return cli.RequireCampaignCompliance(c, id)
	case "/api/campaign-message-assign-template":
		relationships, _ := data["relationships"].(map[string]any)
		template, _ := relationships["template"].(map[string]any)
		reference, _ := template["data"].(map[string]any)
		id, _ := reference["id"].(string)
		return cli.RequireTemplateCompliance(c, id)
	default:
		return nil
	}
}

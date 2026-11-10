// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package client

import (
	"net/url"
	"strings"
)

const campaignMessageIDHint = "IDs from before revision 2026-10-15 are campaign message IDs now: try `campaign-messages get-id <id>` or `campaigns get-id <container id>`"

// Shared API errors reach the generated CLI, typed MCP, and code orchestration unchanged.
func klaviyoCampaignIDErrorHint(err *APIError) string {
	if err.Method != "GET" || err.StatusCode != 404 {
		return ""
	}
	parsed, parseErr := url.Parse(err.Path)
	if parseErr != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) == 3 && parts[0] == "api" && parts[1] == "campaigns" && parts[2] != "" {
		return "\nhint: " + campaignMessageIDHint
	}
	return ""
}

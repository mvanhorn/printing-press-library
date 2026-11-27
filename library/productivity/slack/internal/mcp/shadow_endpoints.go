// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/mcp/bound"
)

// shadowCodeOrchEndpoints registers the hand-added Slack endpoints missing from
// the archived OpenAPI spec (bookmarks CRUD, search.files/search.all,
// auth.teams.list, team.preferences.list, team.billing.info, and opt-in
// xoxc+cookie web-client RPCs) into codeOrchEndpoints so slack_search and
// slack_execute can discover and invoke every pp:endpoint command.
var shadowCodeOrchEndpoints = []codeOrchEndpoint{
	{
		ID:             "bookmarks.list",
		Method:         "GET",
		Path:           "/bookmarks.list",
		Summary:        "List bookmarks in a channel",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams: []codeOrchParamBinding{
			{PublicName: "channel_id", WireName: "channel_id"},
			{PublicName: "channel", WireName: "channel_id"},
		},
		keywords: codeOrchKeywords("bookmarks", "list", "List bookmarks in a channel", "/bookmarks.list"),
	},
	{
		ID:             "bookmarks.add",
		Method:         "POST",
		Path:           "/bookmarks.add",
		Summary:        "Add a bookmark to a channel",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("bookmarks", "add", "Add a bookmark to a channel", "/bookmarks.add"),
	},
	{
		ID:             "bookmarks.edit",
		Method:         "POST",
		Path:           "/bookmarks.edit",
		Summary:        "Edit an existing channel bookmark",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("bookmarks", "edit", "Edit an existing channel bookmark", "/bookmarks.edit"),
	},
	{
		ID:             "bookmarks.remove",
		Method:         "POST",
		Path:           "/bookmarks.remove",
		Summary:        "Remove a bookmark from a channel",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("bookmarks", "remove", "Remove a bookmark from a channel", "/bookmarks.remove"),
	},
	{
		ID:             "search_api.files",
		Method:         "GET",
		Path:           "/search.files",
		Summary:        "Search files matching a query (uses SLACK_USER_TOKEN)",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams: []codeOrchParamBinding{
			{PublicName: "query", WireName: "query"},
			{PublicName: "count", WireName: "count"},
			{PublicName: "page", WireName: "page"},
			{PublicName: "sort", WireName: "sort"},
			{PublicName: "sort_dir", WireName: "sort_dir"},
		},
		keywords: codeOrchKeywords("search_api", "files", "Search files matching a query", "/search.files"),
	},
	{
		ID:             "search_api.all",
		Method:         "GET",
		Path:           "/search.all",
		Summary:        "Search both messages and files matching a query (uses SLACK_USER_TOKEN)",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams: []codeOrchParamBinding{
			{PublicName: "query", WireName: "query"},
			{PublicName: "count", WireName: "count"},
			{PublicName: "page", WireName: "page"},
			{PublicName: "sort", WireName: "sort"},
			{PublicName: "sort_dir", WireName: "sort_dir"},
		},
		keywords: codeOrchKeywords("search_api", "all", "Search both messages and files matching a query", "/search.all"),
	},
	{
		ID:             "auth_api.teams",
		Method:         "GET",
		Path:           "/auth.teams.list",
		Summary:        "List workspaces accessible to the authenticated token",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams: []codeOrchParamBinding{
			{PublicName: "cursor", WireName: "cursor"},
			{PublicName: "limit", WireName: "limit"},
			{PublicName: "include_icon", WireName: "include_icon"},
		},
		keywords: codeOrchKeywords("auth_api", "teams", "List workspaces accessible to the authenticated token", "/auth.teams.list"),
	},
	{
		ID:             "team.preferences",
		Method:         "GET",
		Path:           "/team.preferences.list",
		Summary:        "List workspace preferences",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("team", "preferences", "List workspace preferences", "/team.preferences.list"),
	},
	{
		ID:             "team.billing",
		Method:         "GET",
		Path:           "/team.billing.info",
		Summary:        "Get workspace billing plan information",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("team", "billing", "Get workspace billing plan information", "/team.billing.info"),
	},
	{
		ID:             "web_client.unreads",
		Method:         "POST",
		Path:           "/client.counts",
		Summary:        "Fetch live channel, DM, and thread unread counts (requires SLACK_SESSION_TOKEN xoxc + d cookie)",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("web_client", "unreads", "Fetch live channel DM and thread unread counts", "/client.counts"),
	},
	{
		ID:             "web_client.later",
		Method:         "POST",
		Path:           "/saved.list",
		Summary:        "List Slack Later / saved items (requires SLACK_SESSION_TOKEN xoxc + d cookie)",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("web_client", "later", "List Slack Later saved items", "/saved.list"),
	},
	{
		ID:             "web_client.drafts",
		Method:         "POST",
		Path:           "/drafts.list",
		Summary:        "List unsent message drafts (requires SLACK_SESSION_TOKEN xoxc + d cookie)",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("web_client", "drafts", "List unsent message drafts", "/drafts.list"),
	},
	{
		ID:             "web_client.activity",
		Method:         "POST",
		Path:           "/activity.feed",
		Summary:        "Read the Slack Activity feed of mentions, reactions, and replies (requires SLACK_SESSION_TOKEN xoxc + d cookie)",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("web_client", "activity", "Read Slack Activity feed mentions reactions replies", "/activity.feed"),
	},
	{
		ID:             "web_client.threads",
		Method:         "POST",
		Path:           "/subscriptions.thread.getView",
		Summary:        "Read subscribed threads view (requires SLACK_SESSION_TOKEN xoxc + d cookie)",
		Positional:     []string{},
		TemplateParams: []codeOrchParamBinding{},
		QueryParams:    []codeOrchParamBinding{},
		keywords:       codeOrchKeywords("web_client", "threads", "Read subscribed threads view", "/subscriptions.thread.getView"),
	},
}

func init() {
	existing := make(map[string]bool, len(codeOrchEndpoints))
	for _, ep := range codeOrchEndpoints {
		existing[ep.ID] = true
	}
	for _, ep := range shadowCodeOrchEndpoints {
		if !existing[ep.ID] {
			codeOrchEndpoints = append(codeOrchEndpoints, ep)
		}
	}
}

func isShadowWebClientEndpoint(id string) bool {
	return strings.HasPrefix(id, "web_client.")
}

func isShadowUserTokenEndpoint(id string) bool {
	switch id {
	case "search_api.messages", "search_api.files", "search_api.all":
		return true
	default:
		return false
	}
}

func executeShadowWebClientMCP(ctx context.Context, ep *codeOrchEndpoint, params map[string]any) (*mcplib.CallToolResult, error) {
	body := make(map[string]any, len(params)+2)
	for k, v := range params {
		body[k] = v
	}
	if ep.ID == "web_client.unreads" {
		if _, ok := body["thread_counts_by_channel"]; !ok {
			body["thread_counts_by_channel"] = true
		}
		if _, ok := body["org_wide_aware"]; !ok {
			body["org_wide_aware"] = true
		}
	}
	raw, err := client.PostWebSession(ctx, ep.Path, body, 60*time.Second)
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(bound.EndpointResponse(ep.Method, raw)), nil
}

// checkSlackMCPBodyError detects Slack's HTTP 200 {"ok":false,"error":"..."}
// error envelope so rejected OAuth API calls (e.g. missing_scope on
// bookmarks.add) surface as MCP tool errors instead of looking successful.
func checkSlackMCPBodyError(data json.RawMessage) error {
	var resp struct {
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		Needed string `json:"needed"`
	}
	if json.Unmarshal(data, &resp) != nil {
		return nil
	}
	if resp.OK || resp.Error == "" {
		return nil
	}
	if resp.Error == "missing_scope" && resp.Needed != "" {
		return fmt.Errorf("missing Slack scope: %s (add it in your Slack app OAuth & Permissions settings and reinstall)", resp.Needed)
	}
	return fmt.Errorf("Slack API error: %s", resp.Error)
}

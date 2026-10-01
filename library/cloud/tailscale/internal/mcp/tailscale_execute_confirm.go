// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

// Write gate for tailscale_execute. The CLI's hand-written writes need --yes
// and --agent turns every other mutation into a dry run without it. The
// execute tool reaches the same endpoints, including the whole-set route
// replace and the whole-policy write, so it follows the same rule: a write
// returns the planned request unless the agent passes confirm=true.

import (
	"encoding/json"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// codeOrchReadOnlyPosts are POST endpoints that never change tailnet state.
var codeOrchReadOnlyPosts = map[string]bool{
	"tailnet.acl.preview":              true,
	"tailnet.acl.validate":             true,
	"tailnet.aws-external-id.validate": true,
}

// codeOrchNeedsConfirm reports whether ep changes state and so needs an
// explicit confirm=true before tailscale_execute sends it.
func codeOrchNeedsConfirm(ep *codeOrchEndpoint) bool {
	if ep.Method == "GET" {
		return ep.Mutating
	}
	return !codeOrchReadOnlyPosts[ep.ID]
}

type codeOrchWritePlan struct {
	PlanOnly bool              `json:"plan_only"`
	Endpoint string            `json:"endpoint_id"`
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Query    map[string]string `json:"query,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     any               `json:"body,omitempty"`
	Next     string            `json:"next"`
}

// codeOrchPlanResult describes the request tailscale_execute would send.
// Nothing reaches the API.
func codeOrchPlanResult(ep *codeOrchEndpoint, path string, query, hdrs map[string]string, body any) (*mcplib.CallToolResult, error) {
	if len(query) == 0 {
		query = nil
	}
	plan := codeOrchWritePlan{
		PlanOnly: true,
		Endpoint: ep.ID,
		Method:   ep.Method,
		Path:     path,
		Query:    query,
		Headers:  hdrs,
		Body:     body,
		Next:     "Nothing was sent. Call tailscale_execute again with the same endpoint_id and params plus confirm=true to send this request.",
	}
	b, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return mcplib.NewToolResultError(err.Error()), nil
	}
	return mcplib.NewToolResultText(string(b)), nil
}

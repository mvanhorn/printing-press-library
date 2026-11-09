package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// codeOrchTestInvoke drives a code-orchestration call the way an agent does under Printing Press 4.34.2: endpoints that
// do not need confirmation go through conduyt-crm_read, gated ones through conduyt-crm_execute with confirm: true (an
// unconfirmed execute is only a preview). The hand-written guard tests (parity rounds, #95, #324, #930) were written
// against the 4.28.0 single execute tool; this keeps their intent: the call must reach the API, or be refused before it.
func codeOrchTestInvoke(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	id, _ := args["endpoint_id"].(string)
	ep := findCodeOrchEndpoint(codeOrchLegacyEndpointIDs.resolve(id))
	if ep != nil && !codeOrchRefuseForReadOnlySwitch(ep, ep.Method, false) && ep.Method == "GET" {
		return handleCodeOrchRead(ctx, req)
	}
	withConfirm := map[string]any{}
	for k, v := range args {
		withConfirm[k] = v
	}
	withConfirm["confirm"] = true
	req.Params.Arguments = withConfirm
	return handleCodeOrchExecute(ctx, req)
}

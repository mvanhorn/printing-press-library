// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

const mcpTenantGateOwnerKey = "pp:tenant-gate"

var verifyFreshMCPInvocation = cli.VerifyMCPInvocation

// installFreshTenantGate wraps every tool registered on this server, including
// tools added by code-orch and novel registrars after RegisterTools returns.
// Cobra mirrors are the sole exception: the child CLI owns their one live gate.
func installFreshTenantGate(s *server.MCPServer) {
	s.Use(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return requireFreshTenantGate(s, next)
	})
}

func requireFreshTenantGate(s *server.MCPServer, next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		if entry := s.GetTool(request.Params.Name); entry != nil && entry.Tool.Meta != nil && entry.Tool.Meta.AdditionalFields != nil {
			if owner, _ := entry.Tool.Meta.AdditionalFields[mcpTenantGateOwnerKey].(string); owner == "child-cli" {
				return next(ctx, request)
			}
		}
		session, err := verifyFreshMCPInvocation(ctx)
		if err != nil {
			return finalizeMCPResult(nil, request.GetArguments(), mcplib.NewToolResultError(err.Error())), nil
		}
		if session == nil {
			return finalizeMCPResult(nil, request.GetArguments(), mcplib.NewToolResultError("MCP tenant gate is not configured")), nil
		}
		defer session.ZeroCredentials()
		result, resultErr := next(platform.ContextWithSession(ctx, session), request)
		if result != nil {
			result = finalizeMCPResult(session, request.GetArguments(), result)
		}
		return result, resultErr
	}
}

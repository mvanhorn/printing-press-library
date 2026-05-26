// Copyright 2026 markvandeven. Licensed under Apache-2.0. See LICENSE.

// MCP server entry. Supports both stdio (the default for local Claude
// Desktop / Claude Code clients) and streamable HTTP (for cloud-hosted
// agents that must connect over the network).
//
// Transport selection:
//
//	MCP_TRANSPORT=stdio (default)  -- stdio; the convention for locally-installed servers.
//	MCP_TRANSPORT=http             -- streamable HTTP on MCP_HTTP_ADDR (default :8080), endpoint /mcp.
//
// Run `pdok-location-pp-mcp` (no args) for the stdio path; set
// `MCP_TRANSPORT=http` to expose the same tool surface over HTTP.
package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	mcptools "github.com/mvanhorn/printing-press-library/library/developer-tools/pdok-location/internal/mcp"
)

func main() {
	s := server.NewMCPServer(
		"Pdok Location",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	mcptools.RegisterTools(s)

	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "" {
		transport = "stdio"
	}

	switch transport {
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error (stdio): %v\n", err)
			os.Exit(1)
		}
	case "http":
		addr := os.Getenv("MCP_HTTP_ADDR")
		if addr == "" {
			addr = ":8080"
		}
		httpServer := server.NewStreamableHTTPServer(s)
		fmt.Fprintf(os.Stderr, "pdok-location-pp-mcp: listening on http://%s/mcp\n", addr)
		if err := httpServer.Start(addr); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error (http): %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "MCP server: unknown MCP_TRANSPORT=%q (expected stdio or http)\n", transport)
		os.Exit(2)
	}
}

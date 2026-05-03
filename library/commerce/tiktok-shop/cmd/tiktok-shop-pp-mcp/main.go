// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.
// Scaffolded for CLI Printing Press. Endpoint implementations are intentionally deferred.

package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	mcptools "github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/mcp"
)

func main() {
	s := server.NewMCPServer(
		"TikTok Shop",
		"0.1.0",
		server.WithToolCapabilities(false),
	)

	mcptools.RegisterTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

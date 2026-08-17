// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	mcptools "github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/mcp"
)

var version = "2026.8.2"

func main() {
	s := server.NewMCPServer(
		"Gemini Notebook (NotebookLM)",
		version,
		server.WithToolCapabilities(false),
	)
	mcptools.RegisterTools(s)
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

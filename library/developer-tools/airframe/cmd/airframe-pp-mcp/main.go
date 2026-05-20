// Copyright 2026 chris-drit. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	mcptools "github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/mcp"
)

func main() {
	s := server.NewMCPServer(
		"airframe",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	mcptools.RegisterTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

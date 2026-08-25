package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	mcptools "github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/mcp"
)

var version = "2026.8.1"

func main() {
	s := server.NewMCPServer(
		"The Lancet",
		version,
		server.WithToolCapabilities(false),
	)

	mcptools.RegisterTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
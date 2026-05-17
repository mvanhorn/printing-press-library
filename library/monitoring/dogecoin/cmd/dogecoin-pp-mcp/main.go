package main

import (
	"fmt"
	"os"

	mcptools "github.com/mvanhorn/printing-press-library/library/monitoring/dogecoin/internal/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewMCPServer(
		"Dogecoin",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	mcptools.RegisterTools(s)

	// Support both stdio (for local MCP clients) and HTTP (for remote agents)
	httpAddr := os.Getenv("DOGECOIN_MCP_HTTP_ADDR")
	if httpAddr != "" {
		httpServer := server.NewStreamableHTTPServer(s, server.WithHeartbeatInterval(30))
		fmt.Fprintf(os.Stderr, "MCP HTTP server listening on %s\n", httpAddr)
		if err := httpServer.Start(httpAddr); err != nil {
			fmt.Fprintf(os.Stderr, "MCP HTTP server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

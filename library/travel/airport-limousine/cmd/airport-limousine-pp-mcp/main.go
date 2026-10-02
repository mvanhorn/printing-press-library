// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-owned Airport Limousine MCP entrypoint; the recorded safe-page call site must survive regeneration.

package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/cli"
	mcptools "github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/internal/mcp"
)

// version is the printed MCP server's version, overridable at build time via ldflags.
var version = "0.0.0-dev"

func main() {
	// Pin the learn-event surface for this process and every walker
	// shell-out child, so usage events record surface=mcp.
	_ = os.Setenv("AIRPORT_LIMOUSINE_LEARN_SURFACE", "mcp")
	if err := cli.BindMCPServerProfile(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP client-profile bind failed: %v\n", err)
		os.Exit(1)
	}
	s := server.NewMCPServer(
		"Airport Limousine",
		version,
		server.WithToolCapabilities(false),
	)

	mcptools.RegisterAirportTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

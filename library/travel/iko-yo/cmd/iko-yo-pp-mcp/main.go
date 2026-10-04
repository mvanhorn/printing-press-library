// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Source-specific MCP entrypoint; preserve the Trip metadata registration.

package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cli"
	mcptools "github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/mcp"
)

// version is the printed MCP server's version, overridable at build time via ldflags.
var version = "2026.10.1"

func main() {
	// Pin the learn-event surface for this process and every walker
	// shell-out child, so usage events record surface=mcp.
	_ = os.Setenv("IKO_YO_LEARN_SURFACE", "mcp")
	if err := cli.BindMCPServerProfile(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP client-profile bind failed: %v\n", err)
		os.Exit(1)
	}
	s := server.NewMCPServer(
		"Iko-yo Trip",
		version,
		server.WithToolCapabilities(false),
	)

	mcptools.RegisterTools(s)
	mcptools.RegisterTripSurface(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}

// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

// peloton-pp-mcp is the MCP server companion to peloton-pp-cli. It exposes
// the same read-side workflows as MCP tools so an agent can answer
// "what's my Peloton history" without shelling out.
//
// Auth is shared with the CLI: the bearer token harvested by
// `peloton-pp-cli auth login` lives in
// ~/.config/peloton-pp-cli/config.toml, and this server reads from there.
// The MCP cannot itself drive the browser; bootstrap once from a CLI
// shell, then long-lived agents can run against the saved token.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/server"

	mcptools "github.com/mvanhorn/printing-press-library/library/other/peloton/internal/mcp"
)

const defaultHTTPAddr = ":7777"

// defaultTransport reads PP_MCP_TRANSPORT env when set, otherwise falls
// back to "stdio". Container-hosted agents pin transport via env without
// a flag — matches how hosted-agent process supervisors typically pass
// configuration in the rest of the catalog.
func defaultTransport() string {
	if t := os.Getenv("PP_MCP_TRANSPORT"); t != "" {
		return t
	}
	return "stdio"
}

func main() {
	s := server.NewMCPServer(
		"Peloton",
		"0.1.0",
		server.WithToolCapabilities(false),
	)
	mcptools.RegisterTools(s)

	transport := flag.String("transport", defaultTransport(), "MCP transport: stdio | http")
	addr := flag.String("addr", defaultHTTPAddr, "bind address for http transport (host:port or :port)")
	flag.Parse()

	switch strings.ToLower(*transport) {
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	case "http":
		httpSrv := server.NewStreamableHTTPServer(s)
		fmt.Fprintf(os.Stderr, "peloton-pp-mcp serving MCP over streamable HTTP at %s\n", *addr)
		if err := httpSrv.Start(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown --transport %q (supported: stdio, http)\n", *transport)
		os.Exit(2)
	}
}

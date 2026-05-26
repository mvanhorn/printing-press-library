package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/server"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/cli"
	imcp "github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/mcp"
)

// Standalone MCP server binary. The same tool surface is also reachable as the
// `safari-history-pp-cli mcp` subcommand; this dedicated entry point exists so
// the CLI ships as a Claude Desktop MCPB bundle (server.entry_point references
// this binary). Transport selection: --transport flag, then PP_MCP_TRANSPORT
// env, then stdio.

const defaultHTTPAddr = ":7777"

func main() {
	// The in-process MCP tools shell back into this same executable (via
	// os.Executable()) to run CLI subcommands. When invoked that way the first
	// argument is a subcommand name, so act as the CLI. Without this branch the
	// re-invocation would start a second MCP server reading an empty stdin and
	// every tool would return no data.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		root := cli.NewRootCmd()
		if err := root.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(cli.ExitCodeForError(err))
		}
		return
	}

	s := imcp.NewServer()

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
		fmt.Fprintf(os.Stderr, "safari-history-pp-mcp serving MCP over streamable HTTP at %s\n", *addr)
		if err := httpSrv.Start(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown --transport %q (supported: stdio, http)\n", *transport)
		os.Exit(2)
	}
}

func defaultTransport() string {
	if t := os.Getenv("PP_MCP_TRANSPORT"); t != "" {
		return t
	}
	return "stdio"
}

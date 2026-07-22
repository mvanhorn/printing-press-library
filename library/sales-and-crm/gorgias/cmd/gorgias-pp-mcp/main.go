// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/client"
	mcptools "github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/mcp"
)

// Transport selection order: --transport flag, then PP_MCP_TRANSPORT env,
// then the first transport declared in the spec (see MCPConfig.Transport).
// The flag surface lets one binary serve stdio locally and streamable HTTP
// when hosted in a container or remote sandbox, matching the Anthropic
// guidance that production agents need a remote option.

const (
	defaultHTTPAddr = ":7777"
)

// version is stamped at link time via `-X main.version=<release>` in
// .goreleaser.yaml. The "0.0.0-dev" default is overridden in three ways,
// in priority order: ldflag (goreleaser builds), module version from
// BuildInfo (`go install <module>@vX.Y.Z` builds), or this fallback
// (local `go run` / `go build` from a working tree).
var version = "2026.7.1"

// resolveVersion mirrors the CLI's cli.Version() logic so both binaries
// report the same value when installed via `go install ...@vX.Y.Z`.
func resolveVersion() string {
	if version != "0.0.0-dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}

func main() {
	transport := flag.String("transport", defaultTransport(), "MCP transport: stdio | http")
	addr := flag.String("addr", defaultHTTPAddr, "bind address for http transport (host:port or :port)")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	v := resolveVersion()

	if *showVersion {
		fmt.Printf("gorgias-pp-mcp %s\n", v)
		return
	}

	// Tag MCP-originated API calls with this binary's version so Gorgias's
	// access logs can distinguish CLI traffic from MCP traffic.
	client.SetVersion(v)

	s := server.NewMCPServer(
		"Gorgias",
		v,
		server.WithToolCapabilities(false),
	)

	mcptools.RegisterTools(s)

	switch strings.ToLower(*transport) {
	case "stdio":
		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	case "http":
		httpSrv := server.NewStreamableHTTPServer(s)
		fmt.Fprintf(os.Stderr, "gorgias-pp-mcp serving MCP over streamable HTTP at %s\n", *addr)
		if err := httpSrv.Start(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown --transport %q (supported: stdio, http)\n", *transport)
		os.Exit(2)
	}
}

// defaultTransport reads PP_MCP_TRANSPORT env when set, otherwise falls back
// to "stdio" so running the binary with no args keeps today's behavior.
// Container-hosted agents can pin the transport via env without a flag, which
// matches how hosted-agent process supervisors typically pass configuration.
func defaultTransport() string {
	if t := os.Getenv("PP_MCP_TRANSPORT"); t != "" {
		return t
	}
	return "stdio"
}

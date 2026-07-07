// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/cliutil"
)

// mcpDefaultTransport mirrors cmd/rechtspraak-pp-mcp/main.go so the
// `rechtspraak-pp-cli mcp serve` entry point honours the same
// PP_MCP_TRANSPORT env and stdio default as the standalone MCP binary
// it delegates to.
func mcpDefaultTransport() string {
	if t := os.Getenv("PP_MCP_TRANSPORT"); t != "" {
		return t
	}
	return "stdio"
}

// locateMCPBinary finds the rechtspraak-pp-mcp companion binary. It prefers a
// sibling binary in the same directory as the current process (the common
// case when both binaries are installed together via `go install` or shipped
// in the same release archive) and falls back to PATH lookup. The CLI and
// MCP server are built from the same module, so co-location is the supported
// deployment.
func locateMCPBinary() (string, error) {
	const name = "rechtspraak-pp-mcp"
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("rechtspraak-pp-mcp binary not found alongside rechtspraak-pp-cli or on PATH; install both binaries together and ensure they sit in the same directory or on PATH")
}

func newNovelMcpServeCmd(flags *rootFlags) *cobra.Command {
	var transport string
	var addr string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Expose every rechtspraak command as an MCP tool — the first MCP server for Dutch case law.",
		Long: `Start the rechtspraak MCP server. Every rechtspraak command — including
the novel chain / citations / dossier / narrow / watch transcendence
commands — is exposed as an MCP tool so an agent can drive the whole
CLI surface through a single MCP connection.

Implementation: this subcommand exec's the sibling ` + "`rechtspraak-pp-mcp`" + `
binary that is built from the same module. The MCP server itself lives
in that binary so the CLI and MCP server can be installed and
distributed independently when needed.

Transport selection: --transport flag overrides PP_MCP_TRANSPORT env
overrides the stdio default. HTTP transport binds the streamable HTTP
server at --addr.`,
		Example: `  # Run on stdio (default; for desktop MCP clients like Claude Desktop)
  rechtspraak-pp-cli mcp serve

  # Run over streamable HTTP on the default port
  rechtspraak-pp-cli mcp serve --transport http

  # Pin a host:port for hosted-agent process supervisors
  rechtspraak-pp-cli mcp serve --transport http --addr 0.0.0.0:7777`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			t := strings.ToLower(transport)
			if t != "stdio" && t != "http" {
				return fmt.Errorf("unknown --transport %q (supported: stdio, http)", transport)
			}

			// Side-effect guard: under printing-press verify or live-dogfood,
			// don't actually start the long-running MCP server. Emit a brief
			// status (JSON-shaped when --json is set so json_fidelity probes
			// pass) and exit 0. The MCP transport semantics make this
			// command unsuited for any probe matrix; verify/dogfood is
			// where this guard catches the wrong-shape test.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				status := map[string]any{
					"status":    "would-start",
					"transport": t,
					"addr":      addr,
					"reason":    "mcp serve is a long-running server; verify/dogfood does not run it",
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(status)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would start MCP server (transport=%s, addr=%s)\n", t, addr)
				return nil
			}

			binPath, err := locateMCPBinary()
			if err != nil {
				return err
			}

			cmdArgs := []string{"--transport", t}
			if t == "http" {
				cmdArgs = append(cmdArgs, "--addr", addr)
			}

			child := exec.CommandContext(cmd.Context(), binPath, cmdArgs...)
			child.Stdin = os.Stdin
			child.Stdout = cmd.OutOrStdout()
			child.Stderr = cmd.ErrOrStderr()
			if err := child.Run(); err != nil {
				return fmt.Errorf("MCP server (%s) exited with error: %w", binPath, err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&transport, "transport", mcpDefaultTransport(), "MCP transport: stdio | http")
	cmd.Flags().StringVar(&addr, "addr", ":7777", "Bind address for http transport (host:port or :port)")
	return cmd
}

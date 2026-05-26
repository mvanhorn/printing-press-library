// Novel command: mcp serve. The dedicated cmd/exchangerate-api-pp-mcp binary
// is the production MCP server entry point. This command on the main CLI is
// the discoverable shortcut: it exec's the MCP binary (looked up on PATH next
// to the CLI's own argv[0]) so agents can wire 'exchangerate-api-pp-cli mcp
// serve' instead of needing to know the second binary's name.
//
// We can't import internal/mcp here because that package imports internal/cli
// (the cobratree walker needs the root command), so an in-process start would
// produce a Go import cycle. Subprocess exec sidesteps that cleanly.
package cli

// PATCH exchangerate-novel-mcp-subprocess-wrapper: mcp serve subprocess wrapper to the standalone MCP binary (avoids internal/cli<->internal/mcp import cycle).

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newMCPCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run as an MCP (Model Context Protocol) server",
		Long:  "MCP server entry point. 'mcp serve' starts a stdio server that mirrors the full Cobra command tree as agent tools, with read-only annotations on safe queries.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMCPServeCmd(flags))
	return cmd
}

func newMCPServeCmd(flags *rootFlags) *cobra.Command {
	var binPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the stdio MCP server (every CLI command becomes an MCP tool)",
		Long: `Starts the stdio MCP server. Every user-facing Cobra command becomes
an MCP tool; read-only commands carry readOnlyHint: true so MCP hosts
(Claude Desktop, Cursor, Windsurf) can group them safely.

Implementation note: this exec's the standalone exchangerate-api-pp-mcp
binary that ships alongside the CLI. Override the path with --bin if you
have the MCP binary installed elsewhere.

Security note: lookup order is (1) --bin if set, (2) the binary sitting
next to the running CLI (via os.Executable), (3) $PATH. For hardened
deployments where $PATH is not under your control, pass an absolute
--bin so no PATH entry can substitute a malicious exchangerate-api-pp-mcp.`,
		Example:     "  exchangerate-api-pp-cli mcp serve\n  # Claude Desktop:\n  #   { \"command\": \"exchangerate-api-pp-cli\", \"args\": [\"mcp\", \"serve\"] }",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// --json is incoherent here: the MCP server speaks the MCP wire
			// protocol on stdout, not a single JSON object. Emit a clean
			// envelope so JSON consumers (and the dogfood json_fidelity
			// probe) get parseable output instead of multi-frame MCP traffic.
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"command": "mcp serve",
					"note":    "MCP servers stream the wire protocol on stdout; --json is not meaningful. Remove --json and invoke directly to start the server.",
					"hint":    "Wire it as: { \"command\": \"exchangerate-api-pp-cli\", \"args\": [\"mcp\", \"serve\"] }",
				}, flags)
			}
			candidates := []string{}
			if binPath != "" {
				candidates = append(candidates, binPath)
			}
			// 1) Same directory as our own binary.
			if self, err := os.Executable(); err == nil {
				candidates = append(candidates, filepath.Join(filepath.Dir(self), "exchangerate-api-pp-mcp"))
			}
			// 2) $PATH lookup.
			if p, err := exec.LookPath("exchangerate-api-pp-mcp"); err == nil {
				candidates = append(candidates, p)
			}
			var found string
			for _, c := range candidates {
				if info, err := os.Stat(c); err == nil && !info.IsDir() {
					found = c
					break
				}
			}
			if found == "" {
				return fmt.Errorf("exchangerate-api-pp-mcp binary not found; install it via 'go install ./cmd/exchangerate-api-pp-mcp' or pass --bin <path>")
			}
			child := exec.CommandContext(cmd.Context(), found)
			child.Stdin = os.Stdin
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			child.Env = os.Environ()
			if err := child.Run(); err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) {
					return fmt.Errorf("MCP server exited %d", exitErr.ExitCode())
				}
				return fmt.Errorf("MCP server: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&binPath, "bin", "", "Override path to exchangerate-api-pp-mcp binary")
	return cmd
}

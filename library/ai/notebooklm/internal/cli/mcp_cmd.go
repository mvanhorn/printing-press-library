// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

// RootCommand exposes the cobra root for MCP cobratree registration.
func RootCommand() *cobra.Command {
	var flags rootFlags
	return newRootCmd(&flags)
}

func newMCPCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "mcp",
		Short:  "Run as an MCP (Model Context Protocol) server",
		Hidden: true,
	}
	stdio := newMCPStdioCmd(flags)
	stdio.Aliases = []string{"serve"}
	cmd.AddCommand(stdio)
	return cmd
}

func newMCPStdioCmd(flags *rootFlags) *cobra.Command {
	var binPath string
	cmd := &cobra.Command{
		Use:   "stdio",
		Short: "Start the stdio MCP server that mirrors CLI commands",
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.asJSON {
				return printJSON(map[string]any{
					"command": "mcp stdio",
					"note":    "MCP servers stream the wire protocol on stdout; remove --json to start the server.",
				})
			}
			found, err := resolveMCPBinary(binPath)
			if err != nil {
				return err
			}
			// #nosec G204 -- resolved sibling MCP binary path, not user shell input
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
	cmd.Flags().StringVar(&binPath, "bin", "", "Override path to notebooklm-pp-mcp binary")
	return cmd
}

func resolveMCPBinary(binPath string) (string, error) {
	candidates := []string{}
	if binPath != "" {
		candidates = append(candidates, binPath)
	}
	if self, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(self), "notebooklm-pp-mcp"))
	}
	if p, err := exec.LookPath("notebooklm-pp-mcp"); err == nil {
		candidates = append(candidates, p)
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("notebooklm-pp-mcp binary not found; build with: go build -o notebooklm-pp-mcp ./cmd/notebooklm-pp-mcp")
}

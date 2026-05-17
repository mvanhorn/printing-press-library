// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

// Subprocess runner for intents that compose multiple CLI commands. The
// in-process path (via newMCPClient + direct c.Get calls) is preferred when
// the intent makes a small fixed set of calls; the subprocess path is used
// for intents that ride on top of more complex command logic (coin batch's
// fixture parser, coin pop-curve's grade fanout) that already lives in the
// command's RunE body and would be expensive to duplicate.

package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/pcgs/internal/mcp/cobratree"
)

// runCLISubprocess invokes the companion CLI binary with the given args and
// returns its stdout as a string. Resolves the binary via the same lookup
// the cobratree shellout uses (sibling-of-mcp, PCGS_CLI_PATH env, PATH).
func runCLISubprocess(ctx context.Context, args []string) (string, error) {
	binPath, err := cobratree.SiblingCLIPath()
	if err != nil {
		return "", fmt.Errorf("companion CLI binary not found: %w", err)
	}
	cmd := exec.CommandContext(ctx, binPath, args...)
	// Drop PP_MCP_TRANSPORT so the subprocess CLI doesn't think it's the MCP server.
	env := os.Environ()
	filtered := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, "PP_MCP_TRANSPORT=") {
			filtered = append(filtered, e)
		}
	}
	cmd.Env = filtered
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("cli %s %s: %w", filepath.Base(binPath), strings.Join(args, " "), err)
	}
	return string(out), nil
}

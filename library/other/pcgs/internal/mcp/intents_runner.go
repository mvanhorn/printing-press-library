// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

// Subprocess runner for intents that compose multiple CLI commands. The
// in-process path (via newMCPClient + direct c.Get calls) is preferred when
// the intent makes a small fixed set of calls; the subprocess path is used
// for intents that ride on top of more complex command logic (coin batch's
// fixture parser, coin pop-curve's grade fanout) that already lives in the
// command's RunE body and would be expensive to duplicate.

package mcp

import (
	"bytes"
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
	// PATCH intents-runner-split-stdout-stderr: capture stdout and stderr
	// separately so intent callers can json.Unmarshal stdout without the
	// root PersistentPostRunE's `[quota] X/1000 used, Y left` line (always
	// written to stderr) corrupting the trailing bytes. CombinedOutput
	// merged the two, which silently turned every forecast/curve parse in
	// handleBatchWithQuotaGuard and handlePopScarcityReport into a nil
	// result. Greptile P1 finding on PR #630 (review 7).
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("cli %s %s: %w (stderr: %s)", filepath.Base(binPath), strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

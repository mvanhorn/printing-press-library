// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored destructive-operation confirmation gate for GitHub DELETE
// commands. Generated handlers call this before sending the request.

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// confirmDestructive gates irreversible remote mutations behind explicit
// consent. --yes and --dry-run skip the prompt. Non-interactive callers
// (including piped --stdin JSON) fail closed with exit 2 instead of
// consuming the body as a y/N answer.
func confirmDestructive(cmd *cobra.Command, flags *rootFlags) error {
	if flags.dryRun || flags.yes {
		return nil
	}
	if flags.noInput || flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) || !isInteractiveReader(cmd.InOrStdin()) {
		return usageErr(fmt.Errorf("destructive operation requires --yes (or --dry-run to preview the request)"))
	}
	fmt.Fprint(cmd.ErrOrStderr(), "This permanently modifies or deletes remote data and cannot be undone. Continue? [y/N]: ")
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	}
	return usageErr(fmt.Errorf("aborted; re-run with --yes to confirm"))
}

func isInteractiveReader(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

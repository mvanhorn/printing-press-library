// Copyright 2026 Brandon Nye and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored destructive-operation confirmation gate for GitHub DELETE
// commands. Generated handlers call this before sending the request.

package cli

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// confirmDestructive gates irreversible remote mutations behind explicit
// consent. --yes and --dry-run skip the prompt. Non-interactive callers
// fail closed with exit 2 instead of hanging or silently deleting.
func confirmDestructive(cmd *cobra.Command, flags *rootFlags) error {
	if flags.dryRun || flags.yes {
		return nil
	}
	if flags.noInput || flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
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

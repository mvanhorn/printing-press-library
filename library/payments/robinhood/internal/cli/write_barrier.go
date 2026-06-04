// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func defaultWriteDryRun(cmd *cobra.Command, flags *rootFlags) bool {
	if flags == nil || flags.dryRun || flags.liveWrite {
		return false
	}
	switch commandRiskLevel(cmd) {
	case RiskWriteSafe, RiskWriteMutate, RiskWriteDestructive:
		fmt.Fprintf(os.Stderr, "[WRITES TO LIVE ROBINHOOD] %s defaults to --dry-run. Pass --live-write and set ROBINHOOD_PP_ALLOW_WRITES=1 only after explicit approval.\n", cmd.CommandPath())
		return true
	default:
		return false
	}
}

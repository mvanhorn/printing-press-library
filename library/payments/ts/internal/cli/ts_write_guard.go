// Copyright 2026 Dickie and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored safety guard for capital-committing writes. Not generated.
// `subscribe` commits real capital and `maturity-action` (PUT) changes a live
// rollover/redeem instruction. Both refuse to execute unless the caller
// explicitly confirms with --yes. --dry-run previews without confirmation, and
// the verifier sandbox never reaches the live call.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/ts/internal/cliutil"
)

// requireCapitalWriteConfirm blocks a capital-committing operation unless the
// caller passed --yes. Returns nil (allow) under --dry-run or the verifier
// sandbox, where no live mutation occurs. Agent/CI-safe: no stdin prompt — the
// caller must opt in explicitly with --yes.
func requireCapitalWriteConfirm(cmd *cobra.Command, flags *rootFlags, action string) error {
	if flags.dryRun || cliutil.IsVerifyEnv() || flags.yes {
		return nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"Refusing to %s without confirmation. Preview with --dry-run, then re-run with --yes to commit.\n", action)
	return usageErr(fmt.Errorf("%s is a capital-committing operation; pass --yes to confirm", action))
}

// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// Honest stubs — commands that print a clear "deferred implementation" message
// and exit 0. Keep the user informed about what they *would* do and point at
// working alternatives.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// stubMessage writes the stub notice to stdout so users and `--json` consumers
// see it. We intentionally do not exit non-zero — the stub is a deferred
// feature, not a failure.
func stubMessage(cmd *cobra.Command, msg string) {
	fmt.Fprintln(cmd.OutOrStdout(), msg)
}

func newExpenseWatchStubCmd(flags *rootFlags) *cobra.Command {
	var policy string
	cmd := &cobra.Command{
		Use:   "watch <dir>",
		Short: "[stub] Watch a directory for new receipt files and auto-file them",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "<dir>"
			if len(args) > 0 {
				target = args[0]
			}
			stubMessage(cmd, fmt.Sprintf(
				"watch daemon (stub — implementation deferred): would monitor %q (policy=%q) for new receipts and auto-file. Use `expense attach` to manually attach files for now.",
				target, policy))
			return nil
		},
	}
	cmd.Flags().StringVar(&policy, "policy", "", "Policy ID to file expenses under once implemented")
	return cmd
}

func newUndoStubCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "undo",
		Short: "[stub] Revert the last action from the local action log",
		RunE: func(cmd *cobra.Command, args []string) error {
			stubMessage(cmd,
				"undo (stub — implementation deferred): would revert the last action from the local action log. Track manually via `expense delete` / `report reopen` for now.")
			return nil
		},
	}
}

func newCloseStubCmd(flags *rootFlags) *cobra.Command {
	var month, template, label string
	cmd := &cobra.Command{
		Use:   "close",
		Short: "[stub] End-of-month orchestrator",
		RunE: func(cmd *cobra.Command, args []string) error {
			stubMessage(cmd, fmt.Sprintf(
				"close orchestrator (stub — implementation deferred): would list reports for month=%s, run `export run --template %s` and mark-as-exported with label %q. Chain `export run` + `export download` + `admin report_set_status` manually for now.",
				month, template, label))
			return nil
		},
	}
	cmd.Flags().StringVar(&month, "month", "current", "Month to close (current | previous | YYYY-MM)")
	cmd.Flags().StringVar(&template, "template", "netsuite", "Export template (netsuite | qbo | xero | sage | csv | json)")
	cmd.Flags().StringVar(&label, "label", "", "Label used for mark-as-exported idempotency")
	return cmd
}

func newAdminPolicyDiffStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy-diff <policy-id> <file>",
		Short: "[stub] Diff a local YAML policy file against live config",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := "<file>"
			if len(args) >= 2 {
				file = args[1]
			}
			stubMessage(cmd, fmt.Sprintf(
				"policy diff (stub — implementation deferred): would diff local YAML %q against live policy config. Use `admin policy_get <id> --json` then manually diff for now.",
				file))
			return nil
		},
	}
	return cmd
}

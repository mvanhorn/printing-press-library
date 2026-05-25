// PATCH: novel `db` command group — safe-mutation lifecycle (plan/apply/receipt/revert/lint/diff/decompose). Not in the Management API.
package cli

import (
	"github.com/spf13/cobra"
)

func newDBTopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Safe production-mutation lifecycle: plan, apply (policy or approved hash), decompose, receipt, revert, lint, diff",
		Long: `The safe-mutation lifecycle. Every database change becomes a structured,
inspectable, reversible, attestable transaction:

  plan      Parse SQL into a typed change manifest + deterministic plan_hash.
  apply     Execute ONLY an authorized plan; refuse on SQL drift; transaction-wrapped.
            Authorize with --approved <hash> (human) or --policy <name|file> (policy).
  decompose Rewrite destructive changes (DROP COLUMN, type change) into a safe
            expand/contract sequence; refuse the irreducible ones with a reason.
  receipt   Snapshot affected objects + auto-derive the inverse before mutating.
  revert    One-command undo from a receipt.
  lint      Run splinter-style checks locally against a migration BEFORE apply.
  diff      Compare local supabase/migrations/ against remote applied history.

The lifecycle makes autonomous production migrations real without removing the
guardrails. The human-in-the-loop approver can be replaced by a policy that
self-authorizes ONLY additive, reversible, lint-clean, zero-data-row changes;
the plan_hash drift gate, savepoint transaction, and reversal receipt are
unchanged. Destructive changes route through 'decompose' into safe steps or are
refused with a structured machine-readable reason — never silently applied.`,
	}
	cmd.AddCommand(newDBPlanCmd(flags))
	cmd.AddCommand(newDBApplyCmd(flags))
	cmd.AddCommand(newDBDecomposeCmd(flags))
	cmd.AddCommand(newDBReceiptCmd(flags))
	cmd.AddCommand(newDBRevertCmd(flags))
	cmd.AddCommand(newDBLintCmd(flags))
	cmd.AddCommand(newDBDiffCmd(flags))
	return cmd
}

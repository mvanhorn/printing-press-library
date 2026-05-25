// PATCH: novel `db apply --approved <plan-hash>` — executes ONLY the approved plan, refuses on hash drift, transaction + savepoint wrapped. Not a single Management API endpoint.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/dbchange"
)

func newDBApplyCmd(flags *rootFlags) *cobra.Command {
	var inlineSQL string
	var approved string
	var policySpec string
	var stageBranch string
	var skipReceipt bool
	var allowDestructive bool

	cmd := &cobra.Command{
		Use:   "apply <ref> [file.sql]",
		Short: "Execute an authorized plan against a project; refuses on SQL drift",
		Long: `Re-plans the SQL, recomputes the plan_hash, and refuses to run if it differs
from the authorized hash (the SQL changed since approval). Authorized statements
run inside a single transaction with a savepoint per statement: any statement
error rolls back to its savepoint and aborts the whole apply, so a partial
migration never lands.

AUTHORIZATION — two interchangeable approvers, the safety primitives are the same:

  --approved <plan-hash>   Human approver. You run 'db plan', inspect it, and
                           paste back its plan_hash. Drift-checked.
  --policy <name|file>     Policy approver. The typed manifest is evaluated
                           against a safe envelope and self-authorizes ONLY when
                           every statement is in an allowed class, reversible,
                           lint-clean, and touches zero existing data rows. Use
                           'safe-default' for the built-in additive+reversible
                           envelope, or a path to a policy JSON file.

The policy replaces the human approver, not the safety primitives: the plan_hash
drift gate, the savepoint transaction, and the receipt still run unchanged.

Destructive changes (DROP COLUMN, type change) are not authorized by the
safe-default policy; route them through 'db decompose' which rewrites them into
an expand/contract sequence whose steps each fall inside the safe envelope.

Before mutating, a receipt is captured (object snapshots + mechanical inverse)
so the change is reversible with 'db revert'.`,
		Example: `  supabase-pp-cli db apply abcdefgh migrations/008_rls.sql --approved sha256:... --json
  supabase-pp-cli db apply abcdefgh migrations/008_rls.sql --policy safe-default --json
  supabase-pp-cli db apply abcdefgh --sql "..." --policy ./policies/team.json --stage-branch preview-123`,
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/v1/projects/{ref}/database/query"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref := args[0]
			if approved == "" && policySpec == "" {
				return usageErr(fmt.Errorf("authorization required: pass --approved <plan-hash> (human) or --policy <name|file> (policy). Run 'db plan' first to see the plan_hash"))
			}
			if approved != "" && policySpec != "" {
				return usageErr(fmt.Errorf("pass either --approved or --policy, not both"))
			}
			sql, src, err := loadSQL(args[1:], inlineSQL)
			if err != nil {
				return err
			}
			m := dbchange.Parse(sql, src)

			// Authorization. Either the human paste (--approved, drift-checked) or
			// the policy approver (--policy) must clear before anything mutates.
			var policyVerdict *dbchange.PolicyVerdict
			if approved != "" {
				// Human approver — the original drift gate.
				if m.PlanHash != approved {
					return apiErr(fmt.Errorf("plan-hash drift: SQL hashes to %s but you approved %s.\nThe SQL changed since approval — re-run 'db plan' and approve the new hash", m.PlanHash, approved))
				}
			} else {
				// Policy approver — self-authorize within a declared envelope. The
				// plan_hash is still computed and recorded; the policy replaces only
				// the human paste, so a re-planned hash that drifts from a separately
				// recorded authorization is still detectable downstream.
				pol, perr := resolvePolicy(policySpec)
				if perr != nil {
					return usageErr(perr)
				}
				// "lint-clean" is evaluated with the same ruleset 'db lint' uses.
				errorLintCount := 0
				for _, f := range lintManifest(m, sql) {
					if f.Level == "ERROR" {
						errorLintCount++
					}
				}
				verdict := pol.Evaluate(m, errorLintCount)
				policyVerdict = &verdict
				if !verdict.Authorized {
					// Structured, machine-readable refusal + no-op. NEVER a silent
					// pass-through. Emit the full verdict as JSON in agent/JSON mode
					// so a caller can branch on the reasons.
					out := cmd.OutOrStdout()
					if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
						_ = printJSONFiltered(out, map[string]any{
							"applied":     false,
							"authorized":  false,
							"project_ref": ref,
							"plan_hash":   m.PlanHash,
							"policy":      verdict.Policy,
							"reasons":     verdict.Reasons,
						}, flags)
					}
					return apiErr(fmt.Errorf("policy %q refused this plan (%d reason(s)); not applied. First: %s",
						verdict.Policy, len(verdict.Reasons), firstReason(verdict.Reasons)))
				}
			}

			// Destructive override remains available for the human-approved path
			// (the policy path already gated destructiveness via the envelope).
			if approved != "" && m.HasDestructive && !allowDestructive {
				return usageErr(fmt.Errorf("plan contains destructive statement(s) (DROP/TRUNCATE/DELETE); pass --allow-destructive, or route through 'db decompose' for an expand/contract sequence"))
			}

			// Branch-first staging gate: a plan is only prod-eligible after it
			// applies cleanly to a Supabase preview branch with advisors green.
			if stageBranch != "" && !flags.dryRun {
				if serr := stageOnBranch(cmd, flags, stageBranch, m, sql, src); serr != nil {
					return serr
				}
			}

			// Assemble one transactional SQL string. The Management API
			// database/query endpoint runs the posted SQL as a single unit;
			// wrapping in BEGIN/COMMIT with per-statement savepoints gives the
			// all-or-nothing guarantee and bounded rollback target.
			txSQL := assembleTransaction(m)
			path := replacePathParam("/v1/projects/{ref}/database/query", "ref", ref)

			// Dry-run before any client work: show the transaction, take no
			// snapshots, send nothing.
			if flags.dryRun {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "[dry-run] would POST %d-statement transaction to %s (plan_hash %s)\n", m.StatementN, path, m.PlanHash)
				fmt.Fprintln(out, txSQL)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Capture the receipt (snapshots + inverse) BEFORE mutating.
			var rcpt *receipt
			if !skipReceipt {
				rcpt = buildReceipt(cmd, c, ref, m)
				// Record how this apply was authorized so the receipt is a full
				// audit artifact: either a human plan-hash paste or a named policy.
				if policyVerdict != nil {
					rcpt.Authorization = "policy:" + policyVerdict.Policy
				} else {
					rcpt.Authorization = "human:approved-plan-hash"
				}
			}

			body := map[string]any{"query": txSQL}
			data, status, err := c.Post(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if rcpt != nil {
				rcpt.Applied = true
				rpath, werr := writeReceipt(rcpt)
				if werr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: apply succeeded but receipt write failed: %v\n", werr)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "receipt: %s\n", rpath)
				}
			}

			out := cmd.OutOrStdout()
			result := map[string]any{
				"applied":     true,
				"project_ref": ref,
				"plan_hash":   m.PlanHash,
				"statements":  m.StatementN,
				"status":      status,
			}
			if rcpt != nil {
				result["receipt_id"] = rcpt.Receipt
				result["reversible"] = !m.HasIrreversible
			}
			if policyVerdict != nil {
				result["authorized"] = true
				result["policy"] = policyVerdict.Policy
			}
			if len(data) > 0 {
				result["result"] = trimRaw(data)
			}
			if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(out, result, flags)
			}
			fmt.Fprintf(out, "Applied %d statement(s) to %s (plan_hash %s).\n", m.StatementN, ref, m.PlanHash)
			if rcpt != nil {
				if m.HasIrreversible {
					fmt.Fprintf(out, "Receipt %s written (NOTE: plan has irreversible statements — revert is partial).\n", rcpt.Receipt)
				} else {
					fmt.Fprintf(out, "Receipt %s written. Undo with: supabase-pp-cli db revert %s\n", rcpt.Receipt, rcpt.Receipt)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&inlineSQL, "sql", "", "Inline SQL to apply (instead of a file argument)")
	cmd.Flags().StringVar(&approved, "approved", "", "Human approver: the plan_hash from 'db plan' authorizing this apply")
	cmd.Flags().StringVar(&policySpec, "policy", "", "Policy approver: 'safe-default' or a path to a policy JSON file (self-authorizes within the envelope)")
	cmd.Flags().StringVar(&stageBranch, "stage-branch", "", "Require the plan to apply cleanly to this preview branch with advisors green before prod apply")
	cmd.Flags().BoolVar(&skipReceipt, "skip-receipt", false, "Do not capture a reversal receipt (not recommended)")
	cmd.Flags().BoolVar(&allowDestructive, "allow-destructive", false, "Human-approved path only: permit destructive statements. The policy path uses 'db decompose' instead")
	return cmd
}

// resolvePolicy turns the --policy value into a *dbchange.Policy. The literal
// "safe-default" maps to the built-in Shane-approved additive+reversible
// envelope; any other value is treated as a path to a policy JSON file.
// PATCH(amend-2026-05-25: policy approver resolution).
func resolvePolicy(spec string) (*dbchange.Policy, error) {
	if spec == "safe-default" {
		return dbchange.SafeDefaultPolicy(), nil
	}
	return dbchange.LoadPolicy(spec)
}

// firstReason returns the first refusal message, for a one-line error summary.
func firstReason(rs []dbchange.PolicyReason) string {
	if len(rs) == 0 {
		return ""
	}
	return rs[0].Message
}

// assembleTransaction wraps the approved statements in a BEGIN/COMMIT block with
// a named savepoint before each statement. Postgres aborts the whole
// transaction on any unhandled error, so a failing statement rolls the entire
// apply back — no partial migration. The savepoints make the failure target
// explicit and survive in the receipt for audit.
func assembleTransaction(m *dbchange.Manifest) string {
	var b strings.Builder
	b.WriteString("BEGIN;\n")
	for i, s := range m.Statements {
		fmt.Fprintf(&b, "SAVEPOINT pp_sp_%d;\n", i)
		stmt := strings.TrimSpace(s.SQL)
		if !strings.HasSuffix(stmt, ";") {
			stmt += ";"
		}
		b.WriteString(stmt)
		b.WriteString("\n")
	}
	b.WriteString("COMMIT;\n")
	return b.String()
}

// trimRaw returns the raw JSON if it parses, else nil, to avoid embedding
// invalid bytes in the result envelope.
func trimRaw(data []byte) any {
	var v any
	if err := jsonUnmarshal(data, &v); err != nil {
		return nil
	}
	return v
}

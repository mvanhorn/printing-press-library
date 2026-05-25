// PATCH(amend-2026-05-25: novel `db decompose` — rewrite destructive operation
// classes into an expand/contract step sequence whose individual steps each
// fall inside the safe envelope, or refuse with a structured machine-readable
// reason and a no-op). Replaces the blunt --allow-destructive boolean. Not in
// the Management API.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/dbchange"
)

func newDBDecomposeCmd(flags *rootFlags) *cobra.Command {
	var inlineSQL string

	cmd := &cobra.Command{
		Use:   "decompose [file.sql]",
		Short: "Rewrite destructive changes into a safe expand/contract step sequence (or refuse with a reason)",
		Long: `Take a migration containing destructive operation classes and rewrite each one
into an expand/contract sequence whose individual steps each fall inside the
safe envelope:

  DROP COLUMN          -> tombstone-rename now (reversible), hard drop deferred
  ALTER COLUMN ... TYPE -> add new typed column (expand), app backfill, drop old (contract)

Classes that cannot be safely decomposed are REFUSED with a structured,
machine-readable reason and a no-op — never silently dropped:

  TRUNCATE             -> refused (irreversible, removes all rows)
  unbounded DELETE     -> refused (no WHERE, removes all rows)
  DROP TABLE / SCHEMA  -> refused (may have dependents / cascades)

Only expand-phase steps are auto-eligible; backfill and contract steps are
surfaced as manual gates that a later run must apply explicitly. Exit code is
non-zero (5) if any statement was refused, so it gates a pipeline.`,
		Example:     `  supabase-pp-cli db decompose migrations/012_drop_legacy.sql --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			sql, src, err := loadSQL(args, inlineSQL)
			if err != nil {
				return err
			}
			m := dbchange.Parse(sql, src)
			results, anyRefused := dbchange.DecomposeManifest(m)

			out := cmd.OutOrStdout()
			if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
				refusedCount := 0
				for _, r := range results {
					if !r.Decomposed {
						refusedCount++
					}
				}
				payload := map[string]any{
					"source":         src,
					"plan_hash":      m.PlanHash,
					"statement_count": m.StatementN,
					"refused_count":  refusedCount,
					"results":        results,
				}
				if perr := printJSONFiltered(out, payload, flags); perr != nil {
					return perr
				}
			} else {
				printDecomposeTable(cmd, src, results)
			}
			if anyRefused {
				return apiErr(fmt.Errorf("one or more statements could not be safely decomposed (refused, no-op)"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&inlineSQL, "sql", "", "Inline SQL to decompose (instead of a file argument)")
	return cmd
}

func printDecomposeTable(cmd *cobra.Command, src string, results []dbchange.DecompResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Decompose: %s\n\n", src)
	for _, r := range results {
		if !r.Decomposed {
			fmt.Fprintf(out, "[%d] REFUSED (%s): %s\n", r.OriginalIndex, r.RefusedRule, r.RefusedReason)
			fmt.Fprintf(out, "      original: %s\n\n", truncate(r.OriginalSQL, 70))
			continue
		}
		fmt.Fprintf(out, "[%d] %s -> %d step(s):\n", r.OriginalIndex, r.OpClass, len(r.Steps))
		for _, st := range r.Steps {
			auto := "manual"
			if st.AutoApply {
				auto = "auto"
			}
			fmt.Fprintf(out, "      (%s, %s) %s\n", st.Phase, auto, truncate(st.SQL, 64))
			if st.Note != "" {
				fmt.Fprintf(out, "        note: %s\n", st.Note)
			}
		}
		fmt.Fprintln(out)
	}
}

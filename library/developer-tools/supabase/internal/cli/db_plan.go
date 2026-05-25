// PATCH: novel `db plan` — parse SQL into a typed change manifest + deterministic plan_hash. Not in the Management API.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/dbchange"
)

func newDBPlanCmd(flags *rootFlags) *cobra.Command {
	var inlineSQL string

	cmd := &cobra.Command{
		Use:   "plan [file.sql]",
		Short: "Parse SQL into a typed change manifest with a deterministic plan_hash",
		Long: `Parse a migration (file or --sql) into a typed change manifest. Each statement
is classified by operation class (ddl | grant | rls | data-write | destructive),
the objects it touches, an estimated-rows note, and a reversibility verdict
(auto | snapshot | none). A deterministic plan_hash is emitted over the
normalized statement list.

The plan_hash is the approval token: 'db apply --approved <plan_hash>' refuses
to run if the SQL changed since the plan was produced.`,
		Example: `  supabase-pp-cli db plan migrations/008_rls.sql --json
  supabase-pp-cli db plan --sql "grant execute on function public.f to anon;" --json`,
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

			out := cmd.OutOrStdout()
			if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(out, m, flags)
			}
			return printPlanTable(cmd, m)
		},
	}
	cmd.Flags().StringVar(&inlineSQL, "sql", "", "Inline SQL to plan (instead of a file argument)")
	return cmd
}

func printPlanTable(cmd *cobra.Command, m *dbchange.Manifest) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Plan: %s\n", m.SourceLabel)
	fmt.Fprintf(out, "plan_hash: %s\n", m.PlanHash)
	fmt.Fprintf(out, "%d statement(s)", m.StatementN)
	if m.HasDestructive {
		fmt.Fprintf(out, "  [DESTRUCTIVE]")
	}
	if m.HasIrreversible {
		fmt.Fprintf(out, "  [IRREVERSIBLE]")
	}
	fmt.Fprintf(out, "\n\n")
	fmt.Fprintf(out, "%-3s %-12s %-13s %-26s %s\n", "#", "CLASS", "REVERSIBLE", "OBJECTS", "FLAGS")
	fmt.Fprintf(out, "%-3s %-12s %-13s %-26s %s\n", "-", "-----", "----------", "-------", "-----")
	for _, s := range m.Statements {
		objs := ""
		if len(s.Objects) > 0 {
			objs = s.Objects[0]
		}
		flag := ""
		if s.Destructive {
			flag = "destructive"
		}
		fmt.Fprintf(out, "%-3d %-12s %-13s %-26s %s\n", s.Index, s.OpClass, s.Reversibility, truncate(objs, 24), flag)
	}
	fmt.Fprintf(out, "\nApprove with: supabase-pp-cli db apply <ref> %s --approved %s\n",
		flagOrFile(m.SourceLabel), m.PlanHash)
	return nil
}

func flagOrFile(src string) string {
	if src == "--sql" || src == "" {
		return "--sql \"...\""
	}
	return src
}

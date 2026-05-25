// PATCH: novel top-level `query` — read-only-by-default SQL with --write opt-in, over the Management API database/query endpoint.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newQueryTopCmd(flags *rootFlags) *cobra.Command {
	var inlineSQL string
	var write bool
	var fromStdin bool

	cmd := &cobra.Command{
		Use:   "query <ref> [sql]",
		Short: "Run SQL against a project (read-only by default; --write to mutate)",
		Long: `Execute SQL via the Management API database/query endpoint. Read-only by
default (read_only=true) so an accidental UPDATE/DELETE is rejected by the
database, not just by convention. Pass --write to opt into mutations.

For structured, reversible, attestable mutations prefer the 'db plan' ->
'db apply --approved' lifecycle; --write is the escape hatch for ad-hoc writes.`,
		Example: `  supabase-pp-cli query abcdefgh "select count(*) from auth.users" --json
  supabase-pp-cli query abcdefgh --sql "select * from public.profiles limit 5" --csv
  supabase-pp-cli query abcdefgh "update public.flags set on=true where id=1" --write`,
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/v1/projects/{ref}/database/query"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref := args[0]

			var sql string
			switch {
			case fromStdin:
				b, rerr := io.ReadAll(os.Stdin)
				if rerr != nil {
					return fmt.Errorf("reading stdin: %w", rerr)
				}
				sql = string(b)
			case inlineSQL != "":
				sql = inlineSQL
			case len(args) > 1:
				sql = strings.Join(args[1:], " ")
			default:
				return usageErr(fmt.Errorf("no SQL provided: pass it positionally, via --sql, or --stdin"))
			}
			if strings.TrimSpace(sql) == "" {
				return usageErr(fmt.Errorf("empty SQL"))
			}

			if dryRunOK(flags) {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "[dry-run] would run (read_only=%v) against %s:\n%s\n", !write, ref, sql)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/v1/projects/{ref}/database/query", "ref", ref)
			body := map[string]any{"query": sql, "read_only": !write}
			data, statusCode, err := c.Post(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			out := cmd.OutOrStdout()
			if wantsHumanTable(out, flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if terr := printAutoTable(out, items); terr == nil {
						return nil
					}
				}
			}
			if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":    "query",
					"resource":  "database",
					"read_only": !write,
					"status":    statusCode,
					"success":   statusCode >= 200 && statusCode < 300,
				}
				if len(filtered) > 0 {
					var parsed any
					if json.Unmarshal(filtered, &parsed) == nil {
						envelope["data"] = parsed
					}
				}
				b, merr := json.Marshal(envelope)
				if merr != nil {
					return merr
				}
				return printOutput(out, json.RawMessage(b), true)
			}
			return printOutputWithFlags(out, data, flags)
		},
	}
	cmd.Flags().StringVar(&inlineSQL, "sql", "", "SQL to run (instead of the positional argument)")
	cmd.Flags().BoolVar(&write, "write", false, "Opt into mutations (sets read_only=false)")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read SQL from stdin")
	return cmd
}

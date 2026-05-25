// PATCH: novel `db revert <receipt>` — one-command undo from a receipt's inverse SQL. Not in the Management API.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newDBRevertCmd(flags *rootFlags) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "revert <receipt-id-or-path>",
		Short: "Undo an applied change from its receipt's inverse SQL",
		Long: `Replay the mechanical inverse captured in a receipt, in reverse statement
order, inside a transaction. Receipts with irreversible statements (data DELETE,
TRUNCATE, UPDATE) cannot be fully reverted; revert refuses unless --force is
given, and even then it only replays the inverses it has, leaving irreversible
statements un-undone (the inverse script marks them with comments).`,
		Example:     `  supabase-pp-cli db revert abcdefgh-20260525T193300Z --json`,
		Annotations: map[string]string{"pp:method": "POST", "pp:path": "/v1/projects/{ref}/database/query"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			r, err := readReceipt(args[0])
			if err != nil {
				return err
			}
			if r.Manifest != nil && r.Manifest.HasIrreversible && !force {
				return usageErr(fmt.Errorf("receipt %s contains irreversible statement(s); revert would be partial. Pass --force to replay the derivable inverses only", r.Receipt))
			}
			if strings.TrimSpace(stripInverseComments(r.InverseSQL)) == "" {
				return apiErr(fmt.Errorf("receipt %s has no executable inverse SQL — nothing to revert mechanically", r.Receipt))
			}

			revertSQL := "BEGIN;\n" + r.InverseSQL + "\nCOMMIT;\n"

			if dryRunOK(flags) {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "[dry-run] would revert receipt %s against %s:\n%s\n", r.Receipt, r.Ref, revertSQL)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/v1/projects/{ref}/database/query", "ref", r.Ref)
			data, status, err := c.Post(path, map[string]any{"query": revertSQL})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			out := cmd.OutOrStdout()
			result := map[string]any{
				"reverted":    true,
				"receipt_id":  r.Receipt,
				"project_ref": r.Ref,
				"plan_hash":   r.PlanHash,
				"status":      status,
				"partial":     r.Manifest != nil && r.Manifest.HasIrreversible,
			}
			if len(data) > 0 {
				result["result"] = trimRaw(data)
			}
			if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(out, result, flags)
			}
			fmt.Fprintf(out, "Reverted receipt %s against %s.\n", r.Receipt, r.Ref)
			if r.Manifest != nil && r.Manifest.HasIrreversible {
				fmt.Fprintln(out, "NOTE: partial revert — irreversible statements (data writes) were NOT undone.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Revert even when the receipt has irreversible statements (replays derivable inverses only)")
	return cmd
}

// stripInverseComments removes the -- comment marker lines so we can tell
// whether any executable inverse SQL remains.
func stripInverseComments(s string) string {
	var keep []string
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		keep = append(keep, line)
	}
	return strings.Join(keep, "\n")
}

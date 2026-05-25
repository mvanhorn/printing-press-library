// Hand-written novel command: bulk delete members matching a local-store predicate.
// Mandatory --dry-run is the default — explicit --apply required to mutate.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/auth/memberstack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/auth/memberstack/internal/store"
)

func newBulkCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Bulk member operations driven by a local-store SQL predicate",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBulkDeleteCmd(flags))
	return cmd
}

type bulkDeleteResult struct {
	Planned   []string        `json:"planned"`
	Deleted   []string        `json:"deleted"`
	Errors    []bulkDeleteErr `json:"errors"`
	DryRun    bool            `json:"dryRun"`
	Predicate string          `json:"predicate"`
}

type bulkDeleteErr struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

func newBulkDeleteCmd(flags *rootFlags) *cobra.Command {
	var where string
	var dbPath string
	var apply bool
	var deleteStripeCustomer bool
	var cancelSubs bool
	var limit int

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Wipe members matching a SQL WHERE clause; --dry-run by default.",
		Long: `Selects members from the local store using a safe subset of SQL applied to
the materialised JSON fields (id, email, lastLogin, createdAt, verified, plan
counts). Without --apply, prints what would be deleted and exits 0.

WARNING: --apply hits DELETE /members/:id for every matched row. Always preview
first with the default dry-run.`,
		Example: `  memberstack-pp-cli bulk delete --where "email LIKE '%@test.local'"          # dry-run preview
  memberstack-pp-cli bulk delete --where "days_since_login > 365" --apply     # commit`,
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"pp:typed-exit-codes": "0,2,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && isTerminal(cmd.OutOrStdout()) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if flags.asJSON {
					out := map[string]any{"dryRun": true, "predicate": where, "would": "select matching members; --apply required to commit"}
					data, _ := json.Marshal(out)
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would dry-run bulk delete where: %s\n", where)
				return nil
			}
			if where == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--where is required"))
			}
			if !isSafePredicate(where) {
				return usageErr(fmt.Errorf("predicate contains a disallowed keyword (only SELECT-style predicates over allowed columns are permitted)"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("memberstack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			selectSQL := `
				SELECT id,
				       json_extract(data, '$.auth.email')           AS email,
				       json_extract(data, '$.lastLogin')            AS lastLogin,
				       json_extract(data, '$.createdAt')            AS createdAt,
				       json_extract(data, '$.verified')             AS verified,
				       json_array_length(COALESCE(json_extract(data, '$.planConnections'), json('[]'))) AS plan_count,
				       CAST((julianday('now') - julianday(COALESCE(json_extract(data, '$.lastLogin'), '1970-01-01'))) AS INTEGER) AS days_since_login
				FROM resources WHERE resource_type IN ('members','member') AND (` + where + `)`
			if limit > 0 {
				selectSQL += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), selectSQL)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			ids := []string{}
			for rows.Next() {
				var (
					id        string
					email     sql.NullString
					lastLogin sql.NullString
					createdAt sql.NullString
					verified  sql.NullBool
					planCount sql.NullInt64
					daysSince sql.NullInt64
				)
				if err := rows.Scan(&id, &email, &lastLogin, &createdAt, &verified, &planCount, &daysSince); err != nil {
					continue
				}
				ids = append(ids, id)
			}

			result := bulkDeleteResult{Planned: ids, Predicate: where, DryRun: !apply}

			if !apply {
				return emitBulkResult(cmd, flags, result, fmt.Sprintf("DRY-RUN: would delete %d member(s). Re-run with --apply to commit.", len(ids)))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			for _, id := range ids {
				body := map[string]any{
					"deleteStripeCustomer":      deleteStripeCustomer,
					"cancelStripeSubscriptions": cancelSubs,
				}
				if err := deleteMember(cmd, c, id, body); err != nil {
					result.Errors = append(result.Errors, bulkDeleteErr{ID: id, Error: err.Error()})
					continue
				}
				result.Deleted = append(result.Deleted, id)
			}

			return emitBulkResult(cmd, flags, result, fmt.Sprintf("Deleted %d / %d member(s); %d error(s).", len(result.Deleted), len(ids), len(result.Errors)))
		},
	}
	cmd.Flags().StringVar(&where, "where", "", "SQL WHERE clause over local member rows (email, lastLogin, days_since_login, plan_count, verified, id)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually delete (without it, prints the planned set and exits 0)")
	cmd.Flags().BoolVar(&deleteStripeCustomer, "delete-stripe-customer", false, "Also delete the Stripe customer record")
	cmd.Flags().BoolVar(&cancelSubs, "cancel-stripe-subscriptions", false, "Cancel active Stripe subscriptions before delete")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap planned-set size (0 = unlimited)")
	return cmd
}

func deleteMember(cmd *cobra.Command, c *client.Client, id string, body map[string]any) error {
	_, _, err := c.DeleteWithBody(cmd.Context(), "/members/"+id, body)
	return err
}

func emitBulkResult(cmd *cobra.Command, flags *rootFlags, r bulkDeleteResult, human string) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		data, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return err
		}
		return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), human)
	for _, id := range r.Planned {
		mark := "(planned)"
		if !r.DryRun {
			mark = "(deleted)"
			for _, e := range r.Errors {
				if e.ID == id {
					mark = "(ERROR: " + e.Error + ")"
				}
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s\n", id, mark)
	}
	return nil
}

// isSafePredicate keeps DML/DDL out of the user-supplied WHERE clause.
// Read-only filter, not full SQL injection defence — the predicate is composed
// into a SELECT we control. Reject mutation keywords and statement terminators.
func isSafePredicate(s string) bool {
	lower := strings.ToLower(s)
	banned := []string{
		";", "--", "/*", "*/",
		"insert", "update", "delete", "drop", "alter", "create",
		"attach", "detach", "pragma", "vacuum", "reindex", "replace",
	}
	for _, b := range banned {
		if strings.Contains(lower, b) {
			return false
		}
	}
	return true
}

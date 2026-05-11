package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/store"
	"github.com/spf13/cobra"
)

func newMembersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "members",
		Short: "Local member-load audits over the synced store",
	}
	cmd.AddCommand(newMembersAuditCmd(flags))
	return cmd
}

type memberAuditRow struct {
	AccountID     string `json:"account_id"`
	RoleCount     int    `json:"role_count"`
	EntityCount   int    `json:"entity_count"`
	CommunityIDs  string `json:"communities"`
	OrphanReason  string `json:"orphan_reason,omitempty"`
}

func newMembersAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var community string
	var orphansOnly bool
	cmd := &cobra.Command{
		Use:     "audit",
		Short:   "Per-account role/entity counts across the local store",
		Long:    "Reads the local store synced by `chainels-pp-cli sync`. Joins accounts + accounts_entities + companies_members to compute, per account: role_count (rows in accounts_entities), entity_count (distinct entity ids), and the communities the account participates in.",
		Example: "  chainels-pp-cli members audit --community <community-id> --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("chainels-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			filter := ""
			qargs := []any{}
			if community != "" {
				filter = "WHERE cm.companies_id = ?"
				qargs = append(qargs, community)
			}
			q := `
				SELECT a.id AS account_id,
				       COUNT(DISTINCT ae.id) AS role_count,
				       COUNT(DISTINCT json_extract(ae.data,'$.entity_id')) AS entity_count,
				       COALESCE(GROUP_CONCAT(DISTINCT cm.companies_id), '') AS communities
				FROM accounts a
				LEFT JOIN accounts_entities ae ON ae.accounts_id = a.id
				LEFT JOIN companies_members cm ON cm.data LIKE '%' || a.id || '%'
				` + filter + `
				GROUP BY a.id
				ORDER BY role_count DESC, account_id ASC`
			rows, err := db.DB().QueryContext(cmd.Context(), q, qargs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			out := make([]memberAuditRow, 0)
			for rows.Next() {
				var r memberAuditRow
				var rc, ec sql.NullInt64
				if err := rows.Scan(&r.AccountID, &rc, &ec, &r.CommunityIDs); err != nil {
					return err
				}
				r.RoleCount = int(rc.Int64)
				r.EntityCount = int(ec.Int64)
				if r.RoleCount == 0 && r.EntityCount == 0 {
					r.OrphanReason = "no role or entity references"
				} else if r.CommunityIDs == "" {
					r.OrphanReason = "no community membership references"
				}
				if orphansOnly && r.OrphanReason == "" {
					continue
				}
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	cmd.Flags().StringVar(&community, "community", "", "Filter to a single community (companies_id)")
	cmd.Flags().BoolVar(&orphansOnly, "orphans-only", false, "Return only accounts flagged as orphans")
	return cmd
}

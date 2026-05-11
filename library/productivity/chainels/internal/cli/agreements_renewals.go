package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/store"
	"github.com/spf13/cobra"
)

type agreementRenewalRow struct {
	ID            string `json:"id"`
	CommunityID   string `json:"community_id"`
	EntityID      string `json:"entity_id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	EndDate       string `json:"end_date"`
	NoticeDate    string `json:"notice_date"`
	DaysUntilEnd  int    `json:"days_until_end"`
}

func newAgreementsRenewalsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var withinDays string
	var community string
	cmd := &cobra.Command{
		Use:     "renewals",
		Short:   "List agreements with end-of-term inside an N-day window",
		Long:    "Reads the local store synced by `chainels-pp-cli sync`. Filters agreements by end_date falling between today and today+N, optionally scoped to one community. Sorted by soonest renewal first.",
		Example: "  chainels-pp-cli agreements renewals --within 90d --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			days, err := parseDaysFlag(withinDays)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("chainels-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			now := time.Now()
			until := now.Add(time.Duration(days) * 24 * time.Hour)
			filter := ""
			qargs := []any{now.Format(time.RFC3339), until.Format(time.RFC3339)}
			if community != "" {
				filter = "AND COALESCE(community_id,'') = ?"
				qargs = append(qargs, community)
			}
			q := `
				SELECT id,
				       COALESCE(community_id,''),
				       COALESCE(entity_id,''),
				       COALESCE(name,''),
				       COALESCE(type,''),
				       COALESCE(status,''),
				       COALESCE(end_date,''),
				       COALESCE(notice_date,'')
				FROM agreements
				WHERE end_date IS NOT NULL AND end_date >= ? AND end_date <= ?
				` + filter + `
				ORDER BY end_date ASC`
			rows, err := db.DB().QueryContext(cmd.Context(), q, qargs...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			out := make([]agreementRenewalRow, 0)
			for rows.Next() {
				var r agreementRenewalRow
				var end, notice sql.NullString
				if err := rows.Scan(&r.ID, &r.CommunityID, &r.EntityID, &r.Name, &r.Type, &r.Status, &end, &notice); err != nil {
					return err
				}
				if end.Valid {
					r.EndDate = end.String
					if t, err := time.Parse(time.RFC3339, end.String); err == nil {
						r.DaysUntilEnd = int(time.Until(t).Hours() / 24)
					}
				}
				if notice.Valid {
					r.NoticeDate = notice.String
				}
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	cmd.Flags().StringVar(&withinDays, "within", "90d", "Renewal window in days (90d, 180d)")
	cmd.Flags().StringVar(&community, "community", "", "Filter to a single community id")
	return cmd
}

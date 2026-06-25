// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored transcendence command: expiring / low-balance watchlist.
//
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/sutra-fitness/internal/cliutil"
)

type expiringRow struct {
	ClientName    string `json:"client_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	PurchaseID    string `json:"purchase_id"`
	PlanName      string `json:"plan_name"`
	Type          string `json:"type"`
	EndDate       string `json:"end_date"`
	RemainingUses *int   `json:"remaining_uses"`
	Reason        string `json:"reason"`
}

func newNovelExpiringCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var within string
	var lowCredits bool
	var threshold int

	cmd := &cobra.Command{
		Use:   "expiring",
		Short: "List active memberships and class-packs expiring within a window or running low on credits, with client contact info.",
		Long: `List active memberships and class-packs that need renewal outreach.

Reads your locally synced purchases and flags ACTIVE plans whose end date falls
within --within (default 7d) or, with --low-credits, whose remaining uses are at
or below --threshold. Each row carries the client's name, email, and phone so
the output is a ready-to-use outreach list.

Run 'sutra-fitness-pp-cli sync' first to populate purchases and clients.

Use this command for deterministic renewal outreach (date/credit threshold, act
now). For behavioral lapse signals use 'churn'.`,
		Example:     "  sutra-fitness-pp-cli expiring --within 7d\n  sutra-fitness-pp-cli expiring --within 14d --low-credits --csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(within) == "" {
				within = "7d"
			}
			window, err := cliutil.ParseDurationLoose(within)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --within %q: %w", within, err))
			}

			db, ready, err := openAnalyticsStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if !ready {
				return emitAnalytics(cmd, flags, []expiringRow{})
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "purchases") {
				hintIfStale(cmd, db, "purchases", flags.maxAge)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT p.id, COALESCE(p.type,''), COALESCE(p.name,''), COALESCE(p.end_date,''), p.remaining_uses,
				       COALESCE(cl.first_name,''), COALESCE(cl.last_name,''), COALESCE(cl.email,''), COALESCE(cl.phone,'')
				FROM purchases p
				LEFT JOIN clients cl ON p.client_id = cl.id
				WHERE UPPER(COALESCE(p.status,'')) = 'ACTIVE'`)
			if err != nil {
				return fmt.Errorf("querying purchases: %w", err)
			}
			defer rows.Close()

			now := time.Now()
			cutoff := now.Add(window)
			out := make([]expiringRow, 0)
			for rows.Next() {
				var id, ptype, pname, endDate, first, last, email, phone string
				var remaining sql.NullInt64
				if err := rows.Scan(&id, &ptype, &pname, &endDate, &remaining, &first, &last, &email, &phone); err != nil {
					continue
				}

				reasons := []string{}
				if endTime, ok := parseLocalTime(endDate); ok && endTime.Before(cutoff) {
					reasons = append(reasons, "expiring")
				}
				var remPtr *int
				if remaining.Valid {
					r := int(remaining.Int64)
					remPtr = &r
					if lowCredits && r <= threshold {
						reasons = append(reasons, "low_credits")
					}
				}
				if len(reasons) == 0 {
					continue
				}

				out = append(out, expiringRow{
					ClientName:    strings.TrimSpace(first + " " + last),
					Email:         email,
					Phone:         phone,
					PurchaseID:    id,
					PlanName:      pname,
					Type:          ptype,
					EndDate:       endDate,
					RemainingUses: remPtr,
					Reason:        strings.Join(reasons, "+"),
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating purchases: %w", err)
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].EndDate < out[j].EndDate
			})

			return emitAnalytics(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&within, "within", "7d", "Expiry window (e.g. 7d, 2w, 24h)")
	cmd.Flags().BoolVar(&lowCredits, "low-credits", false, "Also include plans at or below --threshold remaining uses")
	cmd.Flags().IntVar(&threshold, "threshold", 3, "Remaining-uses threshold for --low-credits")
	return cmd
}

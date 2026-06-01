// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for CLI Printing Press.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelFleetHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Single-command account-wide health snapshot: mailbox count, oversight queue depth, webhook delivery rate, domain verification status, usage vs plan limits.",
		Long: `Fleet health joins synced mailboxes, webhooks, domains, usage, oversight
pending, and suppression data in local SQLite to produce an account-wide
health report. No single API endpoint returns this view.

Run 'multimail-pp-cli sync' first to populate local data.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openStoreForRead(ctx, "multimail-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				return fmt.Errorf("no local data. Run 'multimail-pp-cli sync' first")
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "mailboxes")
			hintIfStale(cmd, db, "mailboxes", flags.maxAge)

			sqlDB := db.DB()

			// Mailbox stats
			type mailboxStat struct {
				Total    int            `json:"total"`
				Active   int            `json:"active"`
				ByMode   map[string]int `json:"by_oversight_mode"`
			}
			var ms mailboxStat
			ms.ByMode = map[string]int{}
			rows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'mailboxes'`)
			if err != nil {
				return fmt.Errorf("querying mailboxes: %w", err)
			}
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					rows.Close()
					return err
				}
				ms.Total++
				var m map[string]any
				if json.Unmarshal([]byte(raw), &m) == nil {
					if active, ok := m["is_active"]; ok {
						switch v := active.(type) {
						case float64:
							if v == 1 {
								ms.Active++
							}
						case bool:
							if v {
								ms.Active++
							}
						}
					}
					if mode, ok := m["oversight_mode"].(string); ok && mode != "" {
						ms.ByMode[mode]++
					}
				}
			}
			rows.Close()

			// Oversight pending count — filter by status since the store
			// accumulates historical records across syncs.
			var oversightPending int
			oversightRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'oversight'`)
			if err == nil {
				for oversightRows.Next() {
					var raw string
					if oversightRows.Scan(&raw) != nil {
						break
					}
					var o map[string]any
					if json.Unmarshal([]byte(raw), &o) != nil {
						continue
					}
					status, _ := o["status"].(string)
					// Count only pending items; approved/rejected are historical
					if status == "" || status == "pending" || status == "awaiting_decision" {
						oversightPending++
					}
				}
				oversightRows.Close()
			}

			// Domain stats
			type domainStat struct {
				Total    int `json:"total"`
				Verified int `json:"verified"`
				Pending  int `json:"pending"`
			}
			var ds domainStat
			domainRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'domains'`)
			if err == nil {
				for domainRows.Next() {
					var raw string
					if err := domainRows.Scan(&raw); err != nil {
						break
					}
					ds.Total++
					var d map[string]any
					if json.Unmarshal([]byte(raw), &d) == nil {
						status, _ := d["status"].(string)
						if status == "verified" || status == "active" {
							ds.Verified++
						} else {
							ds.Pending++
						}
					}
				}
				domainRows.Close()
			}

			// Webhook stats
			type webhookStat struct {
				Total  int `json:"total"`
				Active int `json:"active"`
			}
			var ws webhookStat
			whRows, err := sqlDB.QueryContext(ctx, `SELECT COUNT(*) FROM resources WHERE resource_type = 'webhooks'`)
			if err == nil && whRows.Next() {
				whRows.Scan(&ws.Total)
				ws.Active = ws.Total // webhooks are active if they exist
			}
			if whRows != nil {
				whRows.Close()
			}

			// Webhook delivery stats
			type deliveryStat struct {
				Total   int     `json:"total"`
				Success int     `json:"success"`
				Failed  int     `json:"failed"`
				Rate    float64 `json:"success_rate_pct"`
			}
			var dels deliveryStat
			delRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'webhook-deliveries'`)
			if err == nil {
				for delRows.Next() {
					var raw string
					if err := delRows.Scan(&raw); err != nil {
						break
					}
					dels.Total++
					var d map[string]any
					if json.Unmarshal([]byte(raw), &d) == nil {
						if status, ok := d["status"].(string); ok {
							if status == "success" || status == "delivered" {
								dels.Success++
							} else {
								dels.Failed++
							}
						} else if code, ok := d["status_code"].(float64); ok {
							if code >= 200 && code < 300 {
								dels.Success++
							} else {
								dels.Failed++
							}
						}
					}
				}
				delRows.Close()
			}
			if dels.Total > 0 {
				dels.Rate = float64(dels.Success) / float64(dels.Total) * 100
			}

			// Usage stats
			type usageStat struct {
				EmailsSent     int    `json:"emails_sent_this_month,omitempty"`
				MonthlyQuota   int    `json:"monthly_quota,omitempty"`
				StorageUsed    int64  `json:"storage_used_bytes,omitempty"`
				MaxStorage     int64  `json:"max_storage_bytes,omitempty"`
				Plan           string `json:"plan,omitempty"`
			}
			var us usageStat
			acctRow := sqlDB.QueryRowContext(ctx, `SELECT data FROM resources WHERE resource_type = 'account' LIMIT 1`)
			var acctRaw string
			if acctRow.Scan(&acctRaw) == nil {
				var a map[string]any
				if json.Unmarshal([]byte(acctRaw), &a) == nil {
					if v, ok := a["emails_sent_this_month"].(float64); ok {
						us.EmailsSent = int(v)
					}
					if v, ok := a["monthly_email_quota"].(float64); ok {
						us.MonthlyQuota = int(v)
					}
					if v, ok := a["storage_used_bytes"].(float64); ok {
						us.StorageUsed = int64(v)
					}
					if v, ok := a["max_storage_bytes"].(float64); ok {
						us.MaxStorage = int64(v)
					}
					us.Plan, _ = a["plan"].(string)
				}
			}

			// Suppression count
			var suppressionCount int
			suppRow := sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE resource_type = 'suppression'`)
			suppRow.Scan(&suppressionCount)

			result := map[string]any{
				"generated_at":      time.Now().UTC().Format(time.RFC3339),
				"mailboxes":         ms,
				"oversight_pending": oversightPending,
				"domains":           ds,
				"webhooks":          ws,
				"webhook_deliveries": dels,
				"usage":             us,
				"suppression_count": suppressionCount,
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for CLI Printing Press.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelWebhooksHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Per-webhook success rate, failure count, last delivery timestamp, and consecutive failure streak.",
		Long: `Webhook health aggregates synced webhook delivery records to compute
per-webhook success rate, failure count, last delivery timestamp, and
consecutive failure streak. No single API endpoint returns this view.

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

			hintIfUnsynced(cmd, db, "webhook-deliveries")
			hintIfStale(cmd, db, "webhook-deliveries", flags.maxAge)

			sqlDB := db.DB()

			// Load webhook definitions
			type webhookInfo struct {
				ID        string
				URL       string
				MailboxID string
				Events    []string
			}
			webhooks := map[string]webhookInfo{}
			whRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'webhooks'`)
			if err == nil {
				for whRows.Next() {
					var raw string
					if whRows.Scan(&raw) != nil {
						break
					}
					var w map[string]any
					if json.Unmarshal([]byte(raw), &w) != nil {
						continue
					}
					id, _ := w["id"].(string)
					url, _ := w["url"].(string)
					mbID, _ := w["mailbox_id"].(string)
					var events []string
					if evts, ok := w["events"].([]any); ok {
						for _, e := range evts {
							if s, ok := e.(string); ok {
								events = append(events, s)
							}
						}
					}
					if id != "" {
						webhooks[id] = webhookInfo{ID: id, URL: url, MailboxID: mbID, Events: events}
					}
				}
				whRows.Close()
			}

			// Aggregate delivery stats per webhook
			type deliveryRecord struct {
				WebhookID  string
				Success    bool
				Timestamp  time.Time
				StatusCode int
			}
			deliveriesByWebhook := map[string][]deliveryRecord{}

			delRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'webhook-deliveries'`)
			if err != nil {
				return fmt.Errorf("querying webhook-deliveries: %w", err)
			}
			for delRows.Next() {
				var raw string
				if delRows.Scan(&raw) != nil {
					break
				}
				var d map[string]any
				if json.Unmarshal([]byte(raw), &d) != nil {
					continue
				}

				webhookID, _ := d["webhook_id"].(string)
				if webhookID == "" {
					webhookID, _ = d["subscription_id"].(string)
				}
				if webhookID == "" {
					continue
				}

				success := false
				if s, ok := d["status"].(string); ok {
					success = s == "success" || s == "delivered" || s == "ok"
				} else if code, ok := d["status_code"].(float64); ok {
					success = code >= 200 && code < 300
				}

				var ts time.Time
				if t, ok := parseNumericTime(d["delivered_at"]); ok {
					ts = t
				} else if t, ok := parseNumericTime(d["created_at"]); ok {
					ts = t
				}

				statusCode := 0
				if code, ok := d["status_code"].(float64); ok {
					statusCode = int(code)
				}

				deliveriesByWebhook[webhookID] = append(deliveriesByWebhook[webhookID], deliveryRecord{
					WebhookID:  webhookID,
					Success:    success,
					Timestamp:  ts,
					StatusCode: statusCode,
				})
			}
			delRows.Close()

			// Compute per-webhook health
			type webhookHealth struct {
				WebhookID             string   `json:"webhook_id"`
				URL                   string   `json:"url,omitempty"`
				MailboxID             string   `json:"mailbox_id,omitempty"`
				Events                []string `json:"events,omitempty"`
				TotalDeliveries       int      `json:"total_deliveries"`
				SuccessCount          int      `json:"success_count"`
				FailureCount          int      `json:"failure_count"`
				SuccessRate           float64  `json:"success_rate_pct"`
				LastDelivery          string   `json:"last_delivery,omitempty"`
				LastSuccess           string   `json:"last_success,omitempty"`
				LastFailure           string   `json:"last_failure,omitempty"`
				ConsecutiveFailures   int      `json:"consecutive_failures"`
				Status                string   `json:"status"` // healthy, degraded, failing
			}

			var results []webhookHealth
			for whID, deliveries := range deliveriesByWebhook {
				// Sort by timestamp descending for consecutive failure calculation
				sort.Slice(deliveries, func(i, j int) bool {
					return deliveries[i].Timestamp.After(deliveries[j].Timestamp)
				})

				wh := webhookHealth{
					WebhookID:       whID,
					TotalDeliveries: len(deliveries),
				}

				if info, ok := webhooks[whID]; ok {
					wh.URL = info.URL
					wh.MailboxID = info.MailboxID
					wh.Events = info.Events
				}

				var lastSuccess, lastFailure time.Time
				for _, d := range deliveries {
					if d.Success {
						wh.SuccessCount++
						if lastSuccess.IsZero() || d.Timestamp.After(lastSuccess) {
							lastSuccess = d.Timestamp
						}
					} else {
						wh.FailureCount++
						if lastFailure.IsZero() || d.Timestamp.After(lastFailure) {
							lastFailure = d.Timestamp
						}
					}
				}

				// Consecutive failures (from most recent)
				for _, d := range deliveries {
					if d.Success {
						break
					}
					wh.ConsecutiveFailures++
				}

				if wh.TotalDeliveries > 0 {
					wh.SuccessRate = float64(wh.SuccessCount) / float64(wh.TotalDeliveries) * 100
				}

				if len(deliveries) > 0 && !deliveries[0].Timestamp.IsZero() {
					wh.LastDelivery = deliveries[0].Timestamp.Format(time.RFC3339)
				}
				if !lastSuccess.IsZero() {
					wh.LastSuccess = lastSuccess.Format(time.RFC3339)
				}
				if !lastFailure.IsZero() {
					wh.LastFailure = lastFailure.Format(time.RFC3339)
				}

				// Determine status
				switch {
				case wh.ConsecutiveFailures >= 5:
					wh.Status = "failing"
				case wh.SuccessRate < 95 || wh.ConsecutiveFailures >= 2:
					wh.Status = "degraded"
				default:
					wh.Status = "healthy"
				}

				results = append(results, wh)
			}

			// Also include webhooks with no deliveries
			for whID, info := range webhooks {
				if _, has := deliveriesByWebhook[whID]; !has {
					results = append(results, webhookHealth{
						WebhookID: whID,
						URL:       info.URL,
						MailboxID: info.MailboxID,
						Events:    info.Events,
						Status:    "no_deliveries",
					})
				}
			}

			sort.Slice(results, func(i, j int) bool {
				// Failing first, then degraded, then healthy
				statusOrder := map[string]int{"failing": 0, "degraded": 1, "no_deliveries": 2, "healthy": 3}
				return statusOrder[results[i].Status] < statusOrder[results[j].Status]
			})

			output := map[string]any{
				"webhooks":     results,
				"total":        len(results),
				"generated_at": time.Now().UTC().Format(time.RFC3339),
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}
	return cmd
}

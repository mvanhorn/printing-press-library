// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature for CLI Printing Press.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelOversightVelocityCmd(flags *rootFlags) *cobra.Command {
	var flagDays int

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "See approval/rejection rates and median decision latency per mailbox across your entire fleet.",
		Long: `Oversight velocity joins synced audit-log events with oversight decisions
to compute approval/rejection rates and median decision latency per mailbox.

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

			hintIfUnsynced(cmd, db, "audit-log")
			hintIfStale(cmd, db, "audit-log", flags.maxAge)

			sqlDB := db.DB()
			cutoff := time.Now().AddDate(0, 0, -flagDays).UTC().Format(time.RFC3339)

			// Build mailbox name map
			mailboxNames := map[string]string{}
			mbRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'mailboxes'`)
			if err == nil {
				for mbRows.Next() {
					var raw string
					if mbRows.Scan(&raw) != nil {
						break
					}
					var m map[string]any
					if json.Unmarshal([]byte(raw), &m) == nil {
						id, _ := m["id"].(string)
						addr, _ := m["address"].(string)
						if id != "" && addr != "" {
							mailboxNames[id] = addr
						}
					}
				}
				mbRows.Close()
			}

			// Collect oversight decisions from audit log
			type decision struct {
				MailboxID string
				Action    string // "approve" or "reject"
				CreatedAt time.Time
				Latency   time.Duration // time between email creation and decision
			}

			var decisions []decision
			auditRows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'audit-log'`)
			if err != nil {
				return fmt.Errorf("querying audit-log: %w", err)
			}
			for auditRows.Next() {
				var raw string
				if auditRows.Scan(&raw) != nil {
					break
				}
				var entry map[string]any
				if json.Unmarshal([]byte(raw), &entry) != nil {
					continue
				}
				action, _ := entry["action"].(string)
				if action != "oversight_approve" && action != "oversight_reject" &&
					action != "approve" && action != "reject" {
					continue
				}
				// parseNumericTime handles RFC3339 strings, epoch-as-string,
				// and float64 epoch values in a single call — no fallthrough gap.
				created, ok := parseNumericTime(entry["created_at"])
				if !ok || created.Before(mustParseTime(cutoff)) {
					continue
				}

				mailboxID := ""
				if meta, ok := entry["metadata"].(map[string]any); ok {
					mailboxID, _ = meta["mailbox_id"].(string)
				}
				if mailboxID == "" {
					mailboxID, _ = entry["resource_id"].(string)
				}

				normalAction := "approve"
				if action == "oversight_reject" || action == "reject" {
					normalAction = "reject"
				}

				decisions = append(decisions, decision{
					MailboxID: mailboxID,
					Action:    normalAction,
					CreatedAt: created,
					Latency:   extractDecisionLatency(entry, created),
				})
			}
			auditRows.Close()

			// Aggregate per mailbox
			type mailboxVelocity struct {
				MailboxID                    string  `json:"mailbox_id"`
				MailboxAddress               string  `json:"mailbox_address,omitempty"`
				Approved                     int     `json:"approved"`
				Rejected                     int     `json:"rejected"`
				Total                        int     `json:"total"`
				ApprovalRate                 float64 `json:"approval_rate_pct"`
				MedianDecisionLatencySeconds float64 `json:"median_decision_latency_seconds"`
			}

			byMailbox := map[string]*mailboxVelocity{}
			latenciesByMailbox := map[string][]float64{}
			for _, d := range decisions {
				mv, ok := byMailbox[d.MailboxID]
				if !ok {
					mv = &mailboxVelocity{
						MailboxID:      d.MailboxID,
						MailboxAddress: mailboxNames[d.MailboxID],
					}
					byMailbox[d.MailboxID] = mv
				}
				mv.Total++
				if d.Action == "approve" {
					mv.Approved++
				} else {
					mv.Rejected++
				}
				if d.Latency > 0 {
					latenciesByMailbox[d.MailboxID] = append(latenciesByMailbox[d.MailboxID], d.Latency.Seconds())
				}
			}

			var results []mailboxVelocity
			for _, mv := range byMailbox {
				if mv.Total > 0 {
					mv.ApprovalRate = float64(mv.Approved) / float64(mv.Total) * 100
				}
				if latencies := latenciesByMailbox[mv.MailboxID]; len(latencies) > 0 {
					sort.Float64s(latencies)
					mv.MedianDecisionLatencySeconds = median(latencies)
				}
				results = append(results, *mv)
			}
			sort.Slice(results, func(i, j int) bool {
				return results[i].Total > results[j].Total
			})

			output := map[string]any{
				"period_days":     flagDays,
				"total_decisions": len(decisions),
				"mailboxes":       results,
				"generated_at":    time.Now().UTC().Format(time.RFC3339),
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of days to look back")
	return cmd
}

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// median computes the median of a sorted slice of float64 values.
func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func extractDecisionLatency(entry map[string]any, decisionAt time.Time) time.Duration {
	emailCreatedAt, ok := parseDecisionSourceCreatedAt(entry)
	if !ok || !decisionAt.After(emailCreatedAt) {
		return 0
	}
	return decisionAt.Sub(emailCreatedAt)
}

func parseDecisionSourceCreatedAt(entry map[string]any) (time.Time, bool) {
	for _, key := range []string{"email_created_at", "resource_created_at", "pending_created_at"} {
		if ts, ok := parseNumericTime(entry[key]); ok {
			return ts, true
		}
	}

	if metadata, ok := entry["metadata"].(map[string]any); ok {
		for _, key := range []string{"email_created_at", "resource_created_at", "pending_created_at", "created_at"} {
			if ts, ok := parseNumericTime(metadata[key]); ok {
				return ts, true
			}
		}
	}

	return time.Time{}, false
}

// parseNumericTime parses either RFC3339 or epoch seconds into a time.Time.
func parseNumericTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case string:
		parsed, err := time.Parse(time.RFC3339, t)
		if err == nil {
			return parsed, true
		}
		// Try epoch string
		if secs, err := strconv.ParseInt(t, 10, 64); err == nil {
			return time.Unix(secs, 0), true
		}
		return time.Time{}, false
	case float64:
		if t > 1e12 {
			return time.UnixMilli(int64(t)), true
		}
		return time.Unix(int64(t), 0), true
	default:
		return time.Time{}, false
	}
}

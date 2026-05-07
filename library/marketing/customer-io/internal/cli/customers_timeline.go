// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/customer-io/internal/store"

	"github.com/spf13/cobra"
)

// newCustomersTimelineCmd renders a chronological per-customer event stream
// merging deliveries, suppression events, and (live) segment membership for
// one customer ID or email. The deliveries + suppressions reads are local
// SQL; the segment lookup hits the live API.
func newCustomersTimelineCmd(flags *rootFlags) *cobra.Command {
	var sinceFlag string
	var limit int
	var includeSegments bool
	var envID string
	cmd := &cobra.Command{
		Use:   "timeline <customer-id-or-email>",
		Short: "Chronological per-customer event stream merging deliveries, suppressions, and segment membership",
		Long: `Customer 360 timeline: render a per-customer event stream from local
synced deliveries + suppressions, optionally enriched with live segment
membership.

Run 'customer-io-pp-cli sync --resources deliveries,suppressions --since 30d'
first to populate the local store. Without sync data, the timeline is empty
or partial; the JSON envelope's "data_source" field tells you which.`,
		Example: strings.Trim(`
  customer-io-pp-cli customers timeline alice@example.com
  customer-io-pp-cli customers timeline alice@example.com --since 30d --json
  customer-io-pp-cli customers timeline cust_123 --segments
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			customer := strings.TrimSpace(args[0])
			if customer == "" {
				return usageErr(fmt.Errorf("customer ID or email required"))
			}

			cutoff, err := parseSinceCutoff(sinceFlag)
			if err != nil {
				return usageErr(err)
			}

			db, err := openTimelineStore()
			if err != nil {
				return apiErr(fmt.Errorf("opening local store: %w (run 'customer-io-pp-cli sync' first)", err))
			}
			defer db.Close()

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			deliveries, err := loadCustomerDeliveries(ctx, db.DB(), customer, cutoff, limit)
			if err != nil {
				return apiErr(fmt.Errorf("reading deliveries: %w", err))
			}
			suppressions, err := loadCustomerSuppressions(ctx, db.DB(), customer, cutoff)
			if err != nil {
				return apiErr(fmt.Errorf("reading suppressions: %w", err))
			}

			events := append(deliveriesToEvents(deliveries), suppressionsToEvents(suppressions)...)
			sort.Slice(events, func(i, j int) bool {
				return events[i].Timestamp < events[j].Timestamp
			})

			out := map[string]any{
				"customer":       customer,
				"events":         events,
				"deliveries_count":  len(deliveries),
				"suppressions_count": len(suppressions),
			}
			if includeSegments {
				if envID == "" {
					out["segments_error"] = "--environment-id is required to fetch live segment membership"
				} else {
					c, clientErr := flags.newClient()
					if clientErr != nil {
						return clientErr
					}
					segData, getErr := c.Get("/v1/environments/"+envID+"/customers/"+customer+"/segments", nil)
					if getErr == nil {
						var raw json.RawMessage = segData
						out["segments"] = raw
					} else {
						out["segments_error"] = getErr.Error()
					}
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Customer: %s\n", customer)
			fmt.Fprintf(cmd.OutOrStdout(), "Events: %d (%d deliveries, %d suppressions)\n\n", len(events), len(deliveries), len(suppressions))
			for _, ev := range events {
				when := time.Unix(ev.Timestamp, 0).UTC().Format(time.RFC3339)
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-12s  %s\n", when, ev.Kind, ev.Summary)
			}
			if len(events) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  (no events; run 'customer-io-pp-cli sync --resources deliveries,suppressions' first)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Only show events newer than this duration (e.g. 24h, 7d, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum events to return")
	cmd.Flags().BoolVar(&includeSegments, "segments", false, "Also fetch live segment membership for the customer (requires --environment-id)")
	cmd.Flags().StringVar(&envID, "environment-id", "", "Environment ID for live API enrichment (--segments)")
	return cmd
}

type timelineEvent struct {
	Timestamp int64  `json:"timestamp"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
	Detail    any    `json:"detail,omitempty"`
}

// openTimelineStore is a thin wrapper around store.OpenWithContext that
// resolves the same default DB path other commands use.
func openTimelineStore() (*store.Store, error) {
	dbPath := defaultDBPath("customer-io-pp-cli")
	return store.OpenWithContext(context.Background(), dbPath)
}

func parseSinceCutoff(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	dur, err := parseSimpleDuration(s)
	if err != nil {
		return 0, err
	}
	return time.Now().Add(-dur).Unix(), nil
}

// loadCustomerDeliveries returns rows where data.customer_id or data.recipient
// matches the supplied identifier (treats both as a string match).
func loadCustomerDeliveries(ctx context.Context, db *sql.DB, customer string, cutoff int64, limit int) ([]json.RawMessage, error) {
	q := `SELECT data FROM deliveries
	      WHERE json_extract(data, '$.customer_id') = ?
	         OR json_extract(data, '$.recipient') = ?`
	args := []any{customer, customer}
	if cutoff > 0 {
		q += ` AND IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) >= ?`
		args = append(args, cutoff)
	}
	q += ` ORDER BY IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(append([]byte{}, data...)))
	}
	return out, rows.Err()
}

func loadCustomerSuppressions(ctx context.Context, db *sql.DB, customer string, cutoff int64) ([]json.RawMessage, error) {
	q := `SELECT data FROM suppressions
	      WHERE json_extract(data, '$.email') = ?
	         OR json_extract(data, '$.id') = ?
	         OR json_extract(data, '$.customer_id') = ?`
	args := []any{customer, customer, customer}
	if cutoff > 0 {
		q += ` AND IFNULL(json_extract(data, '$.created_at'), json_extract(data, '$.created')) >= ?`
		args = append(args, cutoff)
	}
	q += ` ORDER BY IFNULL(json_extract(data, '$.created_at'), json_extract(data, '$.created')) DESC`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(append([]byte{}, data...)))
	}
	return out, rows.Err()
}

func deliveriesToEvents(rows []json.RawMessage) []timelineEvent {
	events := make([]timelineEvent, 0, len(rows))
	for _, row := range rows {
		var d struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			State      string `json:"state"`
			Subject    string `json:"subject"`
			CampaignID string `json:"campaign_id"`
			Created    int64  `json:"created"`
			Updated    int64  `json:"updated"`
		}
		_ = json.Unmarshal(row, &d)
		ts := d.Created
		if ts == 0 {
			ts = d.Updated
		}
		events = append(events, timelineEvent{
			Timestamp: ts,
			Kind:      "delivery." + nonEmpty(d.State, "unknown"),
			Summary:   fmt.Sprintf("[%s] %s subject=%q campaign=%s", nonEmpty(d.Type, "?"), d.ID, d.Subject, d.CampaignID),
			Detail:    json.RawMessage(row),
		})
	}
	return events
}

func suppressionsToEvents(rows []json.RawMessage) []timelineEvent {
	events := make([]timelineEvent, 0, len(rows))
	for _, row := range rows {
		var s struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			Reason    string `json:"reason"`
			CreatedAt int64  `json:"created_at"`
		}
		_ = json.Unmarshal(row, &s)
		events = append(events, timelineEvent{
			Timestamp: s.CreatedAt,
			Kind:      "suppression",
			Summary:   fmt.Sprintf("%s (%s)", nonEmpty(s.Email, s.ID), nonEmpty(s.Reason, "no-reason")),
			Detail:    json.RawMessage(row),
		})
	}
	return events
}

func nonEmpty(a, fallback string) string {
	if strings.TrimSpace(a) == "" {
		return fallback
	}
	return a
}

// parseSimpleDuration accepts e.g. "30s", "10m", "2h", "7d", "30d".
func parseSimpleDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var n int
		if _, err := fmt.Sscanf(days, "%d", &n); err != nil {
			return 0, fmt.Errorf("invalid days: %s", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

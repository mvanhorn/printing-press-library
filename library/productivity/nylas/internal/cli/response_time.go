// Copyright 2026 nathan-kettles. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/nylas/internal/store"
)

// rtBucket is one row of the response-time report. p50 and p90 are
// reported in minutes; n is the number of reply intervals that
// contributed to the bucket.
type rtBucket struct {
	Bucket   string `json:"bucket"`
	N        int    `json:"n"`
	P50Min   int64  `json:"p50_minutes"`
	P90Min   int64  `json:"p90_minutes"`
	GrantsID string `json:"grants_id,omitempty"`
}

func newResponseTimeCmd(flags *rootFlags) *cobra.Command {
	var groupBy string
	var since string
	var grantID string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "response-time",
		Short: "Compute first-response latency on threads where you replied",
		Long: `Reconstruct the thread timeline from the local mirror and compute
first-response latency: time between an inbound message and the first
outbound message from the grant-holder on the same thread.

Group by 'domain' (counterparty email domain) or 'grant'. Use --since to
restrict to recent threads only.`,
		Example: strings.Trim(`
  # P50 / P90 by counterparty domain over the last 30 days
  nylas-pp-cli response-time --group-by domain --since 30d --agent

  # Per-grant SLA breakdown
  nylas-pp-cli response-time --group-by grant --since 7d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if groupBy != "domain" && groupBy != "grant" {
				return fmt.Errorf("--group-by must be 'domain' or 'grant' (got %q)", groupBy)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("nylas-pp-cli")
			}
			autoRefreshIfStale(cmd.Context(), dbPath, cmd.ErrOrStderr())
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'nylas-pp-cli sync' first.", err)
			}
			defer db.Close()

			where := []string{}
			params := []any{}
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return fmt.Errorf("invalid --since: %w", err)
				}
				cutoff := ts.UTC().Format("2006-01-02 15:04:05")
				where = append(where, "synced_at >= ?")
				params = append(params, cutoff)
			}
			if grantID != "" {
				where = append(where, "grants_id = ?")
				params = append(params, grantID)
			}
			whereSQL := ""
			if len(where) > 0 {
				whereSQL = " AND " + strings.Join(where, " AND ")
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(grants_id,'') AS grants_id,
				        COALESCE(json_extract(data,'$.thread_id'), '') AS thread_id,
				        COALESCE(json_extract(data,'$.from[0].email'), '') AS from_email,
				        COALESCE(json_extract(data,'$.date'), 0) AS date_ts,
				        COALESCE(json_extract(data,'$.grant_id'), '') AS grant_id_in_data
				   FROM grants_messages WHERE 1=1`+whereSQL+` ORDER BY thread_id, date_ts ASC`, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			// Walk per-thread; first replier-from-grant after an inbound
			// message gives one interval. We classify "outbound" as a
			// message whose from-email's domain belongs to a known grant
			// or matches grant-derived address heuristics. Without that
			// metadata, fall back to: the message is outbound if its
			// `from_email` ever appears as a `to_email` in the same
			// thread (it's the grant holder's address).
			grantAddrs := loadGrantAddresses(db)

			type msg struct {
				grant, thread, fromEmail string
				ts                       int64
			}
			byThread := make(map[string][]msg)
			for rows.Next() {
				var m msg
				var dataGrant string
				if err := rows.Scan(&m.grant, &m.thread, &m.fromEmail, &m.ts, &dataGrant); err != nil {
					continue
				}
				if m.grant == "" {
					m.grant = dataGrant
				}
				if m.thread == "" || m.ts == 0 {
					continue
				}
				m.fromEmail = strings.ToLower(strings.TrimSpace(m.fromEmail))
				byThread[m.thread] = append(byThread[m.thread], m)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating grants_messages: %w", err)
			}

			type interval struct {
				delta  time.Duration
				domain string
				grant  string
			}
			intervals := make([]interval, 0, 128)
			for _, msgs := range byThread {
				// msgs are already in date_ts ASC order from the SQL.
				var lastInbound *msg
				for i := range msgs {
					m := &msgs[i]
					isOut := false
					if _, ok := grantAddrs[m.fromEmail]; ok {
						isOut = true
					}
					if isOut && lastInbound != nil {
						delta := time.Duration(m.ts-lastInbound.ts) * time.Second
						if delta > 0 && delta < 30*24*time.Hour {
							intervals = append(intervals, interval{
								delta:  delta,
								domain: domainOf(lastInbound.fromEmail),
								grant:  m.grant,
							})
						}
						lastInbound = nil
					} else if !isOut {
						lastInbound = m
					}
				}
			}

			buckets := make(map[string][]time.Duration)
			for _, iv := range intervals {
				key := iv.domain
				if groupBy == "grant" {
					key = iv.grant
				}
				if key == "" {
					continue
				}
				buckets[key] = append(buckets[key], iv.delta)
			}
			out := make([]rtBucket, 0, len(buckets))
			for k, ds := range buckets {
				sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
				row := rtBucket{Bucket: k, N: len(ds)}
				if groupBy == "grant" {
					row.GrantsID = k
				}
				row.P50Min = int64(percentile(ds, 0.5).Minutes())
				row.P90Min = int64(percentile(ds, 0.9).Minutes())
				out = append(out, row)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].N > out[j].N })
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&groupBy, "group-by", "domain", "Bucket by 'domain' (counterparty) or 'grant'")
	cmd.Flags().StringVar(&since, "since", "", "Restrict to threads synced within this duration (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&grantID, "grant", "", "Scope to one grant ID (default: all grants)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite database")
	return cmd
}

func domainOf(email string) string {
	at := strings.LastIndexByte(email, '@')
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := p * float64(len(sorted)-1)
	lo := int(rank)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return sorted[lo] + time.Duration(float64(sorted[hi]-sorted[lo])*frac)
}

// loadGrantAddresses returns the set of email addresses associated with
// each grant in the local store. We treat any message whose from_email
// matches one of these addresses as outbound. Falls back to empty set
// if grants table or shape isn't present, in which case the report
// shows zero intervals (no false positives).
func loadGrantAddresses(db *store.Store) map[string]struct{} {
	out := make(map[string]struct{})
	rows, err := db.DB().Query(`SELECT COALESCE(json_extract(data,'$.email'), '') FROM grants`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err == nil && email != "" {
			out[strings.ToLower(email)] = struct{}{}
		}
	}
	if rows.Err() != nil {
		// Return empty set so callers produce zero intervals rather than
		// false positives from misclassified inbound/outbound messages.
		return make(map[string]struct{})
	}
	return out
}

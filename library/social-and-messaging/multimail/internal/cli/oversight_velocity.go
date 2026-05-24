package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"multimail-pp-cli/internal/store"
)

type velocityRow struct {
	Mailbox        string  `json:"mailbox"`
	MailboxID      string  `json:"mailbox_id"`
	TotalDecisions int     `json:"total_decisions"`
	Approved       int     `json:"approved"`
	Rejected       int     `json:"rejected"`
	ApprovalRate   float64 `json:"approval_rate"`
	MedianLatency  string  `json:"median_latency"`
	PendingCount   int     `json:"pending_count"`
}

func newOversightVelocityCmd(flags *rootFlags) *cobra.Command {
	var (
		days   int
		dbPath string
	)
	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "See approval/rejection rates and median decision latency per mailbox across your entire fleet",
		Long: `Oversight velocity shows how fast oversight decisions are being made
across all mailboxes. It joins audit events with oversight decisions in the
local SQLite cache to compute approval rates and decision latency.

Requires synced data (run 'multimail-pp-cli sync --full' first).`,
		Example: strings.Trim(`
  multimail-pp-cli oversight velocity --json
  multimail-pp-cli oversight velocity --days 7 --json --select mailbox,approval_rate,median_latency
  multimail-pp-cli oversight velocity --days 30`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("multimail-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

			// Get mailbox names for display
			type mbInfo struct {
				ID   string
				Name string
			}
			var mailboxes []mbInfo
			mbRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id, json_extract(data, '$.address') FROM resources WHERE resource_type = 'mailboxes'`)
			if err != nil {
				return fmt.Errorf("querying mailboxes: %w", err)
			}
			for mbRows.Next() {
				var m mbInfo
				if err := mbRows.Scan(&m.ID, &m.Name); err != nil {
					continue
				}
				if m.Name == "" {
					m.Name = m.ID
				}
				mailboxes = append(mailboxes, m)
			}
			mbRows.Close()

			if len(mailboxes) == 0 {
				return fmt.Errorf("no mailboxes in local cache — run 'multimail-pp-cli sync --full' first")
			}

			var results []velocityRow
			for _, mb := range mailboxes {
				// PATCH: Use event timestamp ($.created_at) instead of synced_at
				// for --days window filtering. synced_at is when the record was
				// cached locally — on first sync all records land with synced_at ≈ now,
				// making the window include all historical events.
				// PATCH: Check multiple fields for mailbox association — the audit-log
				// schema may store mailbox_id at the top level, in metadata, or as
				// resource_id. Match all three for consistency with trust_status.
				var approved, rejected int
				row := db.DB().QueryRowContext(cmd.Context(),
					`SELECT
						COALESCE(SUM(CASE WHEN json_extract(data, '$.action') LIKE '%approve%' THEN 1 ELSE 0 END), 0),
						COALESCE(SUM(CASE WHEN json_extract(data, '$.action') LIKE '%reject%' THEN 1 ELSE 0 END), 0)
					FROM resources
					WHERE resource_type = 'audit-log'
					AND (json_extract(data, '$.mailbox_id') = ?
					  OR json_extract(data, '$.resource_id') = ?
					  OR json_extract(data, '$.metadata.mailbox_id') = ?)
					AND json_extract(data, '$.created_at') > ?`, mb.ID, mb.ID, mb.ID, cutoff)
				if err := row.Scan(&approved, &rejected); err != nil {
					continue
				}

				total := approved + rejected
				var rate float64
				if total > 0 {
					rate = float64(approved) / float64(total) * 100
				}

				// PATCH: Compute median latency by joining audit-log
				// decisions with oversight items. The sync layer only
				// fetches /v1/oversight/pending, so decided items never
				// re-appear with updated status. Instead, correlate
				// audit-log approve/reject events (which have the decision
				// timestamp) with oversight items (which retain received_at)
				// via resource_id.
				medianLatency := "—"
				latencyRows, lerr := db.DB().QueryContext(cmd.Context(),
					`SELECT
						COALESCE(json_extract(o.data, '$.received_at'), json_extract(o.data, '$.created_at'), ''),
						json_extract(a.data, '$.created_at')
					FROM resources a
					JOIN resources o ON o.resource_type = 'oversight'
						AND o.id = json_extract(a.data, '$.resource_id')
					WHERE a.resource_type = 'audit-log'
					AND (json_extract(a.data, '$.action') LIKE '%approve%'
					  OR json_extract(a.data, '$.action') LIKE '%reject%')
					AND json_extract(o.data, '$.mailbox_id') = ?
					AND json_extract(a.data, '$.created_at') > ?`, mb.ID, cutoff)
				if lerr == nil {
					var latencies []float64
					for latencyRows.Next() {
						var createdStr, decidedStr string
						if latencyRows.Scan(&createdStr, &decidedStr) != nil {
							continue
						}
						if createdStr == "" || decidedStr == "" {
							continue
						}
						created, cerr := time.Parse(time.RFC3339, createdStr)
						decided, derr := time.Parse(time.RFC3339, decidedStr)
						if cerr != nil || derr != nil {
							continue
						}
						dur := decided.Sub(created).Seconds()
						if dur >= 0 {
							latencies = append(latencies, dur)
						}
					}
					latencyRows.Close()
					if len(latencies) > 0 {
						sort.Float64s(latencies)
						var mid float64
						n := len(latencies)
						if n%2 == 1 {
							mid = latencies[n/2]
						} else {
							mid = (latencies[n/2-1] + latencies[n/2]) / 2
						}
						switch {
						case mid < 60:
							medianLatency = fmt.Sprintf("%.0fs", mid)
						case mid < 3600:
							medianLatency = fmt.Sprintf("%.1fm", mid/60)
						default:
							medianLatency = fmt.Sprintf("%.1fh", mid/3600)
						}
					}
				}

				// PATCH: Count currently pending within the --days window
				// for consistency with the windowed decision counts above.
				// Use received_at (event time) instead of synced_at.
				var pending int
				pendingRow := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*) FROM resources
					WHERE resource_type = 'oversight'
					AND json_extract(data, '$.mailbox_id') = ?
					AND json_extract(data, '$.status') = 'pending'
					AND COALESCE(json_extract(data, '$.received_at'), json_extract(data, '$.created_at')) > ?`, mb.ID, cutoff)
				_ = pendingRow.Scan(&pending)

				results = append(results, velocityRow{
					Mailbox:        mb.Name,
					MailboxID:      mb.ID,
					TotalDecisions: total,
					Approved:       approved,
					Rejected:       rejected,
					ApprovalRate:   rate,
					MedianLatency:  medianLatency,
					PendingCount:   pending,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].PendingCount > results[j].PendingCount
			})

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Look-back window in days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

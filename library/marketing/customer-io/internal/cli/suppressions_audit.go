// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newSuppressionsAuditCmd attributes every suppression in a window to the
// triggering bounce or complaint delivery (or "manual" if no preceding
// delivery is found in the local store). Pure local SQL over the synced
// suppressions × deliveries tables.
func newSuppressionsAuditCmd(flags *rootFlags) *cobra.Command {
	var since string
	var reasonFilter string
	var limit int
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Attribute every suppression to a triggering bounce/complaint delivery (or 'manual')",
		Long: `Joins synced suppressions × deliveries in the local SQLite store. For each
suppression, the most recent preceding delivery to the same recipient (within
24 hours) with a 'bounced', 'dropped', 'failed', or 'spammed' state is
recorded as the attribution. Suppressions with no qualifying preceding
delivery are tagged 'manual'.

Run 'customer-io-pp-cli sync --resources suppressions,deliveries' first.`,
		Example: strings.Trim(`
  customer-io-pp-cli suppressions audit --since 30d
  customer-io-pp-cli suppressions audit --reason bounce --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := parseSinceCutoff(since)
			if err != nil {
				return usageErr(err)
			}

			db, err := openTimelineStore()
			if err != nil {
				return apiErr(fmt.Errorf("opening local store: %w (run 'customer-io-pp-cli sync --resources suppressions,deliveries' first)", err))
			}
			defer db.Close()

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			records, err := loadSuppressionsForAudit(ctx, db.DB(), cutoff, limit)
			if err != nil {
				return apiErr(fmt.Errorf("reading suppressions: %w", err))
			}
			audited := make([]map[string]any, 0, len(records))
			counts := map[string]int{}
			for _, sup := range records {
				attrib, deliveryID := attributeSuppression(ctx, db.DB(), sup)
				if reasonFilter != "" && !strings.EqualFold(reasonFilter, attrib) {
					continue
				}
				audited = append(audited, map[string]any{
					"suppression_id":         sup.ID,
					"recipient":              sup.Recipient,
					"reason_recorded":        sup.Reason,
					"reason_attributed":      attrib,
					"created_at":             sup.CreatedAt,
					"triggering_delivery_id": deliveryID,
				})
				counts[attrib]++
			}

			out := map[string]any{
				"window":      since,
				"reason":      reasonFilter,
				"audited":     len(audited),
				"by_reason":   counts,
				"records":     audited,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Audited %d suppressions (window=%s)\n\n", len(audited), nonEmpty(since, "all"))
			fmt.Fprintln(cmd.OutOrStdout(), "By attributed reason:")
			for r, n := range counts {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %d\n", r, n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only audit suppressions newer than this duration (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&reasonFilter, "reason", "", "Filter to suppressions attributed to this reason: bounce, complaint, dropped, failed, manual")
	cmd.Flags().IntVar(&limit, "limit", 5000, "Maximum suppressions to audit")
	return cmd
}

type suppressionRecord struct {
	ID        string
	Recipient string
	Reason    string
	CreatedAt int64
}

func loadSuppressionsForAudit(ctx context.Context, db *sql.DB, cutoff int64, limit int) ([]suppressionRecord, error) {
	q := `SELECT
	          IFNULL(json_extract(data, '$.id'), id) AS id,
	          IFNULL(json_extract(data, '$.email'), json_extract(data, '$.customer_id')) AS recipient,
	          json_extract(data, '$.reason') AS reason,
	          IFNULL(json_extract(data, '$.created_at'), json_extract(data, '$.created')) AS created_at
	      FROM suppressions`
	args := []any{}
	if cutoff > 0 {
		q += ` WHERE IFNULL(json_extract(data, '$.created_at'), json_extract(data, '$.created')) >= ?`
		args = append(args, cutoff)
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []suppressionRecord
	for rows.Next() {
		var (
			id       sql.NullString
			recip    sql.NullString
			reason   sql.NullString
			created  sql.NullInt64
		)
		if scanErr := rows.Scan(&id, &recip, &reason, &created); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, suppressionRecord{
			ID:        id.String,
			Recipient: recip.String,
			Reason:    reason.String,
			CreatedAt: created.Int64,
		})
	}
	return out, rows.Err()
}

// attributeSuppression returns ("bounce" / "complaint" / "dropped" / "failed"
// / "manual", deliveryID-or-empty).
func attributeSuppression(ctx context.Context, db *sql.DB, sup suppressionRecord) (string, string) {
	if sup.Recipient == "" {
		return strings.ToLower(nonEmpty(sup.Reason, "manual")), ""
	}
	floor := sup.CreatedAt - int64(24*time.Hour/time.Second)
	row := db.QueryRowContext(ctx, `SELECT
	    IFNULL(json_extract(data, '$.id'), id) AS id,
	    json_extract(data, '$.state') AS state
	    FROM deliveries
	    WHERE (json_extract(data, '$.customer_id') = ? OR json_extract(data, '$.recipient') = ?)
	      AND IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) <= ?
	      AND IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) >= ?
	      AND LOWER(IFNULL(json_extract(data, '$.state'), '')) IN ('bounced', 'dropped', 'failed', 'spammed', 'complaint', 'undeliverable')
	    ORDER BY IFNULL(json_extract(data, '$.created'), json_extract(data, '$.updated')) DESC
	    LIMIT 1`, sup.Recipient, sup.Recipient, sup.CreatedAt, floor)
	var (
		deliveryID sql.NullString
		state      sql.NullString
	)
	if err := row.Scan(&deliveryID, &state); err != nil {
		return "manual", ""
	}
	switch strings.ToLower(state.String) {
	case "spammed", "complaint":
		return "complaint", deliveryID.String
	case "bounced", "undeliverable":
		return "bounce", deliveryID.String
	case "dropped":
		return "dropped", deliveryID.String
	case "failed":
		return "failed", deliveryID.String
	}
	return "manual", ""
}

// silence the unused import warning from json — used in future expansions.
var _ = json.Marshal

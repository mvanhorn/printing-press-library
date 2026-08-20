// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH: add `dispute-win-rate` analytics command. Computes historical dispute
// outcome ratio (won / total resolved) with breakdowns by reason and by
// amount-band.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

type disputeBucketRow struct {
	Label       string  `json:"label"`
	Resolved    int     `json:"resolved"`
	Won         int     `json:"won"`
	Lost        int     `json:"lost"`
	WonRatePct  float64 `json:"won_rate_pct"`
}

type disputeWinReport struct {
	Since          string             `json:"since"`
	By             string             `json:"by"`
	IncludePending bool               `json:"include_pending"`
	Resolved       int                `json:"resolved"`
	Won            int                `json:"won"`
	Lost           int                `json:"lost"`
	Pending        int                `json:"pending"`
	WonRatePct     float64            `json:"won_rate_pct"`
	ByReason       []disputeBucketRow `json:"by_reason,omitempty"`
	ByAmountBand   []disputeBucketRow `json:"by_amount_band,omitempty"`
}

// Amount bands in cents: <$10, $10–100, $100–500, $500–2k, >$2k. Stripe
// disputes are denominated in the same minor unit as their charge.
var amountBands = []struct {
	label string
	min   int64
	max   int64
}{
	{"<$10", 0, 999},
	{"$10-$100", 1000, 9999},
	{"$100-$500", 10000, 49999},
	{"$500-$2k", 50000, 199999},
	{">$2k", 200000, 1<<62 - 1},
}

func newDisputeWinRateCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var by string
	var includePending bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dispute-win-rate",
		Short: "Historical dispute outcome ratio with reason and amount-band breakdowns",
		Long: `Walk every dispute since the cutoff. Partition by status: 'won' and 'lost'
are resolved; the warning_* and needs_response / under_review states are
pending. won_rate_pct = won / resolved. --by reason adds a per-reason row;
--by amount-band adds an amount-bucket row. --include-pending widens the
denominator to all disputes, useful for tracking trajectory.`,
		Example: `  # Last 90 days, both breakdowns
  stripe-pp-cli dispute-win-rate --json

  # Last year, reason-only
  stripe-pp-cli dispute-win-rate --since 365d --by reason --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since, err := parseHumanDuration(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since: %w", err))
			}
			if since == 0 {
				since = 90 * 24 * time.Hour
			}
			by = strings.ToLower(strings.TrimSpace(by))
			switch by {
			case "", "all", "reason", "amount-band":
				// ok
			default:
				return usageErr(fmt.Errorf("--by must be one of: reason, amount-band, all (got %q)", by))
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' first.", path, err))
			}
			defer db.Close()

			report, err := computeDisputeWinRate(db.DB(), since, by, includePending)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}

	cmd.Flags().StringVar(&sinceStr, "since", "90d", "Lookback window (e.g. 90d, 365d)")
	cmd.Flags().StringVar(&by, "by", "all", "Breakdown axis: reason, amount-band, or all (default)")
	cmd.Flags().BoolVar(&includePending, "include-pending", false, "Include pending disputes in the denominator")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	return cmd
}

// computeDisputeWinRate aggregates dispute outcomes within the window.
func computeDisputeWinRate(db *sql.DB, since time.Duration, by string, includePending bool) (disputeWinReport, error) {
	now := time.Now().UTC()
	cutoff := now.Add(-since).Unix()

	rs, err := db.Query(`SELECT id, data FROM resources WHERE resource_type='disputes'
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) >= ?`, cutoff)
	if err != nil {
		return disputeWinReport{}, fmt.Errorf("querying disputes: %w", err)
	}
	defer rs.Close()

	var total bucket
	pending := 0
	byReason := make(map[string]*bucket)
	byBand := make(map[string]*bucket)

	for rs.Next() {
		var id, data string
		_ = id
		if err := rs.Scan(&id, &data); err != nil {
			return disputeWinReport{}, err
		}
		raw := json.RawMessage(data)
		status, _ := jsonGet(raw, "status")
		reason, _ := jsonGet(raw, "reason")
		if reason == "" {
			reason = "unspecified"
		}
		amount, _ := jsonGetInt(raw, "amount")
		bandLabel := amountBandLabel(amount)

		isWon := status == "won"
		isLost := status == "lost"
		isResolved := isWon || isLost
		isPending := strings.HasPrefix(status, "warning_") || status == "needs_response" || status == "under_review"

		switch {
		case isWon:
			total.resolved++
			total.won++
			incBucket(byReason, reason, true, false)
			incBucket(byBand, bandLabel, true, false)
		case isLost:
			total.resolved++
			total.lost++
			incBucket(byReason, reason, false, true)
			incBucket(byBand, bandLabel, false, true)
		case isPending:
			pending++
			if includePending {
				incBucket(byReason, reason, false, false)
				incBucket(byBand, bandLabel, false, false)
			}
		}
		_ = isResolved
	}
	if err := rs.Err(); err != nil {
		return disputeWinReport{}, err
	}

	denominator := total.resolved
	if includePending {
		denominator += pending
	}
	wonPct := 0.0
	if denominator > 0 {
		wonPct = float64(total.won) / float64(denominator) * 100
	}

	report := disputeWinReport{
		Since:          since.String(),
		By:             by,
		IncludePending: includePending,
		Resolved:       total.resolved,
		Won:            total.won,
		Lost:           total.lost,
		Pending:        pending,
		WonRatePct:     roundTo(wonPct, 2),
	}

	if by == "" || by == "all" || by == "reason" {
		report.ByReason = flattenBuckets(byReason, includePending)
	}
	if by == "" || by == "all" || by == "amount-band" {
		report.ByAmountBand = flattenBucketsOrdered(byBand, amountBandOrder(), includePending)
	}
	return report, nil
}

// incBucket increments the (resolved/won/lost) counts on a label bucket,
// initializing the entry lazily.
func incBucket(m map[string]*bucket, label string, won, lost bool) {
	b, ok := m[label]
	if !ok {
		b = &bucket{}
		m[label] = b
	}
	if won {
		b.resolved++
		b.won++
	} else if lost {
		b.resolved++
		b.lost++
	}
	// pending case (neither won nor lost) widens denominator only.
}

type bucket struct {
	resolved int
	won      int
	lost     int
}

// flattenBuckets emits rows in count-desc order. includePending is a hint
// for documentation only — the buckets already reflect inclusion semantics.
func flattenBuckets(m map[string]*bucket, _ bool) []disputeBucketRow {
	out := make([]disputeBucketRow, 0, len(m))
	for label, b := range m {
		pct := 0.0
		if b.resolved > 0 {
			pct = float64(b.won) / float64(b.resolved) * 100
		}
		out = append(out, disputeBucketRow{
			Label:      label,
			Resolved:   b.resolved,
			Won:        b.won,
			Lost:       b.lost,
			WonRatePct: roundTo(pct, 2),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Resolved > out[j].Resolved
	})
	return out
}

// flattenBucketsOrdered emits rows in the explicit `order` slice's order
// (used for amount-band so bands always appear ascending, not by frequency).
func flattenBucketsOrdered(m map[string]*bucket, order []string, _ bool) []disputeBucketRow {
	out := make([]disputeBucketRow, 0, len(order))
	for _, label := range order {
		b, ok := m[label]
		if !ok {
			continue
		}
		pct := 0.0
		if b.resolved > 0 {
			pct = float64(b.won) / float64(b.resolved) * 100
		}
		out = append(out, disputeBucketRow{
			Label:      label,
			Resolved:   b.resolved,
			Won:        b.won,
			Lost:       b.lost,
			WonRatePct: roundTo(pct, 2),
		})
	}
	return out
}

// amountBandLabel maps an amount (in cents) to its band label.
func amountBandLabel(amount int64) string {
	for _, b := range amountBands {
		if amount >= b.min && amount <= b.max {
			return b.label
		}
	}
	return amountBands[len(amountBands)-1].label
}

// amountBandOrder returns the canonical band ordering for output.
func amountBandOrder() []string {
	out := make([]string, len(amountBands))
	for i, b := range amountBands {
		out[i] = b.label
	}
	return out
}

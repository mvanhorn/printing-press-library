// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH: add `daily-digest` analytics command. One-command markdown (or JSON)
// report for a period: charges, subscriptions, disputes, payouts. Reads from
// local store only.

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

type digestCharges struct {
	GrossCents       int64                     `json:"gross_cents"`
	RefundsCents     int64                     `json:"refunds_cents"`
	NetCents         int64                     `json:"net_cents"`
	SuccessfulCount  int                       `json:"successful_count"`
	FailedCount      int                       `json:"failed_count"`
	SuccessRatePct   float64                   `json:"success_rate_pct"`
	AOVCents         int64                     `json:"aov_cents"`
	Days             []digestRevenueDay        `json:"days"`
	TopFailureCodes  []digestFailureCodeRow    `json:"top_failure_codes,omitempty"`
}

type digestRevenueDay struct {
	Date         string `json:"date"`
	ChargeCount  int    `json:"charge_count"`
	GrossCents   int64  `json:"gross_cents"`
	RefundsCents int64  `json:"refunds_cents"`
	NetCents     int64  `json:"net_cents"`
}

type digestFailureCodeRow struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

type digestSubs struct {
	// MRRCents is the sum of monthly-normalized recurring revenue across ALL
	// currencies, in minor units. For multi-currency merchants this is a
	// face-value sum, not a converted total — use ByCurrencyCents for the
	// honest per-currency breakdown.
	MRRCents            int64            `json:"mrr_cents"`
	ARRCents            int64            `json:"arr_cents"`
	ByCurrencyCents     map[string]int64 `json:"by_currency_cents,omitempty"`
	ActiveSubscribers   int              `json:"active_subscribers"`
	TrialingSubscribers int              `json:"trialing_subscribers"`
	NewInWindow         int              `json:"new_in_window"`
	ChurnedInWindow     int              `json:"churned_in_window"`
}

type digestDisputes struct {
	OpenCount   int   `json:"open_count"`
	OpenTotal   int64 `json:"open_total_cents"`
	WonInWindow int   `json:"won_in_window"`
	LostInWindow int  `json:"lost_in_window"`
}

type digestPayouts struct {
	NextPayoutCents      int64  `json:"next_payout_cents,omitempty"`
	NextPayoutArrival    string `json:"next_payout_arrival,omitempty"`
	LastPayoutCents      int64  `json:"last_payout_cents,omitempty"`
	LastPayoutArrival    string `json:"last_payout_arrival,omitempty"`
	LastPayoutStatus     string `json:"last_payout_status,omitempty"`
}

type dailyDigest struct {
	From     string          `json:"from"`
	To       string          `json:"to"`
	Sections []string        `json:"sections"`
	Charges  *digestCharges  `json:"charges,omitempty"`
	Subs     *digestSubs     `json:"subs,omitempty"`
	Disputes *digestDisputes `json:"disputes,omitempty"`
	Payouts  *digestPayouts  `json:"payouts,omitempty"`
}

func newDailyDigestCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var fromStr string
	var toStr string
	var format string
	var sections string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "daily-digest",
		Short: "One-command period report: charges, subs, disputes, payouts",
		Long: `Compose a multi-section report for a period — equivalent to running the
existing customer-360, dunning-queue, and payout-reconcile commands plus a sql
query for the revenue rollup, but in one shot. Reads from the local SQLite
mirror only — no API calls. Default format is markdown; pass --json for a
structured envelope.`,
		Example: `  # Last 7 days, markdown
  stripe-pp-cli daily-digest

  # Explicit date range, JSON envelope, charges + subs only
  stripe-pp-cli daily-digest --from 2026-05-01 --to 2026-05-28 --json --sections charges,subs`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			from, to, err := resolveDigestRange(sinceStr, fromStr, toStr)
			if err != nil {
				return usageErr(err)
			}
			sectionList := parseSections(sections)
			if len(sectionList) == 0 {
				return usageErr(fmt.Errorf("--sections must include at least one of: charges, subs, disputes, payouts"))
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' first.", path, err))
			}
			defer db.Close()

			digest, err := buildDigest(db.DB(), from, to, sectionList)
			if err != nil {
				return apiErr(err)
			}

			switch strings.ToLower(format) {
			case "json":
				return printJSONFiltered(cmd.OutOrStdout(), digest, flags)
			case "md", "markdown", "":
				_, err := fmt.Fprint(cmd.OutOrStdout(), renderDigestMarkdown(digest))
				return err
			default:
				return usageErr(fmt.Errorf("--format must be md or json (got %q)", format))
			}
		},
	}

	cmd.Flags().StringVar(&sinceStr, "since", "7d", "Window ending now (e.g. 7d, 30d). Ignored if --from/--to are set.")
	cmd.Flags().StringVar(&fromStr, "from", "", "Explicit start date (YYYY-MM-DD). Requires --to.")
	cmd.Flags().StringVar(&toStr, "to", "", "Explicit end date (YYYY-MM-DD). Requires --from.")
	cmd.Flags().StringVar(&format, "format", "md", "Output format: md or json")
	cmd.Flags().StringVar(&sections, "sections", "charges,subs,disputes,payouts", "Comma-separated section list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	// Allow --json shorthand: maps to --format=json. Provided as a separate
	// flag for ergonomic symmetry with the rest of the CLI; the actual JSON
	// envelope is the same one printJSONFiltered would emit.
	cmd.Flags().BoolP("json", "j", false, "Shortcut for --format=json")
	cmd.PreRunE = func(c *cobra.Command, _ []string) error {
		if c.Flags().Changed("json") {
			isJSON, _ := c.Flags().GetBool("json")
			if isJSON {
				if err := c.Flags().Set("format", "json"); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return cmd
}

// resolveDigestRange computes the (from, to) UTC date pair. Explicit
// --from/--to override --since. Validates ordering and bounds.
func resolveDigestRange(sinceStr, fromStr, toStr string) (time.Time, time.Time, error) {
	if (fromStr != "") != (toStr != "") {
		return time.Time{}, time.Time{}, fmt.Errorf("--from and --to must both be set, or neither")
	}
	if fromStr != "" {
		from, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from (want YYYY-MM-DD): %w", err)
		}
		to, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to (want YYYY-MM-DD): %w", err)
		}
		if to.Before(from) {
			return time.Time{}, time.Time{}, fmt.Errorf("--from must be on or before --to")
		}
		// end-of-day for `to`
		to = to.Add(24*time.Hour - time.Second)
		return from, to, nil
	}
	since, err := parseHumanDuration(sinceStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --since: %w", err)
	}
	if since == 0 {
		since = 7 * 24 * time.Hour
	}
	now := time.Now().UTC()
	from := now.Add(-since)
	return from, now, nil
}

// parseSections splits a comma-list and lowercases entries, dropping empties.
func parseSections(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		switch p {
		case "charges", "subs", "subscriptions", "disputes", "payouts":
			// normalize 'subscriptions' to 'subs' for canonical comparison
			if p == "subscriptions" {
				p = "subs"
			}
			if !seen[p] {
				out = append(out, p)
				seen[p] = true
			}
		}
	}
	return out
}

func sectionEnabled(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func buildDigest(db *sql.DB, from, to time.Time, sections []string) (dailyDigest, error) {
	digest := dailyDigest{
		From:     from.Format("2006-01-02"),
		To:       to.Format("2006-01-02"),
		Sections: sections,
	}
	fromTS := from.Unix()
	toTS := to.Unix()

	if sectionEnabled(sections, "charges") {
		c, err := buildChargesSection(db, fromTS, toTS, from, to)
		if err != nil {
			return digest, err
		}
		digest.Charges = &c
	}
	if sectionEnabled(sections, "subs") {
		s, err := buildSubsSection(db, fromTS, toTS)
		if err != nil {
			return digest, err
		}
		digest.Subs = &s
	}
	if sectionEnabled(sections, "disputes") {
		d, err := buildDisputesSection(db, fromTS, toTS)
		if err != nil {
			return digest, err
		}
		digest.Disputes = &d
	}
	if sectionEnabled(sections, "payouts") {
		p, err := buildPayoutsSection(db)
		if err != nil {
			return digest, err
		}
		digest.Payouts = &p
	}
	return digest, nil
}

func buildChargesSection(db *sql.DB, fromTS, toTS int64, from, to time.Time) (digestCharges, error) {
	rs, err := db.Query(`SELECT data FROM resources WHERE resource_type='charges'
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) >= ?
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) <= ?`, fromTS, toTS)
	if err != nil {
		return digestCharges{}, fmt.Errorf("querying charges: %w", err)
	}
	defer rs.Close()

	dayMap := make(map[string]*digestRevenueDay)
	for d := from; !d.After(to); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		dayMap[key] = &digestRevenueDay{Date: key}
	}

	var grossTotal int64
	successCount := 0
	failedCount := 0
	failureCodes := make(map[string]int)

	for rs.Next() {
		var data string
		if err := rs.Scan(&data); err != nil {
			return digestCharges{}, err
		}
		raw := json.RawMessage(data)
		status, _ := jsonGet(raw, "status")
		amount, _ := jsonGetInt(raw, "amount")
		ts, _ := jsonGetInt(raw, "created")
		day := time.Unix(ts, 0).UTC().Format("2006-01-02")

		row := dayMap[day]
		if row == nil {
			row = &digestRevenueDay{Date: day}
			dayMap[day] = row
		}
		switch status {
		case "succeeded":
			successCount++
			grossTotal += amount
			row.ChargeCount++
			row.GrossCents += amount
		case "failed":
			failedCount++
			code, _ := jsonGet(raw, "failure_code")
			if code == "" {
				code = "unknown"
			}
			failureCodes[code]++
		}
	}
	if err := rs.Err(); err != nil {
		return digestCharges{}, err
	}

	// Refunds within the window
	refundRows, err := db.Query(`SELECT data FROM resources WHERE resource_type='refunds'
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) >= ?
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) <= ?`, fromTS, toTS)
	if err != nil {
		return digestCharges{}, fmt.Errorf("querying refunds: %w", err)
	}
	defer refundRows.Close()

	var refundTotal int64
	for refundRows.Next() {
		var data string
		if err := refundRows.Scan(&data); err != nil {
			return digestCharges{}, err
		}
		raw := json.RawMessage(data)
		amount, _ := jsonGetInt(raw, "amount")
		ts, _ := jsonGetInt(raw, "created")
		day := time.Unix(ts, 0).UTC().Format("2006-01-02")
		refundTotal += amount
		if row, ok := dayMap[day]; ok {
			row.RefundsCents += amount
		}
	}
	if err := refundRows.Err(); err != nil {
		return digestCharges{}, err
	}

	days := make([]digestRevenueDay, 0, len(dayMap))
	for _, row := range dayMap {
		row.NetCents = row.GrossCents - row.RefundsCents
		days = append(days, *row)
	}
	sort.SliceStable(days, func(i, j int) bool { return days[i].Date < days[j].Date })

	successRate := 0.0
	denom := successCount + failedCount
	if denom > 0 {
		successRate = float64(successCount) / float64(denom) * 100
	}
	var aov int64
	if successCount > 0 {
		aov = grossTotal / int64(successCount)
	}

	topCodes := make([]digestFailureCodeRow, 0, len(failureCodes))
	for code, n := range failureCodes {
		topCodes = append(topCodes, digestFailureCodeRow{Code: code, Count: n})
	}
	sort.SliceStable(topCodes, func(i, j int) bool { return topCodes[i].Count > topCodes[j].Count })
	if len(topCodes) > 5 {
		topCodes = topCodes[:5]
	}

	return digestCharges{
		GrossCents:      grossTotal,
		RefundsCents:    refundTotal,
		NetCents:        grossTotal - refundTotal,
		SuccessfulCount: successCount,
		FailedCount:     failedCount,
		SuccessRatePct:  roundTo(successRate, 2),
		AOVCents:        aov,
		Days:            days,
		TopFailureCodes: topCodes,
	}, nil
}

func buildSubsSection(db *sql.DB, fromTS, toTS int64) (digestSubs, error) {
	rs, err := db.Query(`SELECT id, data FROM resources WHERE resource_type='subscriptions'`)
	if err != nil {
		return digestSubs{}, fmt.Errorf("querying subscriptions: %w", err)
	}
	defer rs.Close()

	var mrr int64
	byCurrency := make(map[string]int64)
	active := 0
	trialing := 0
	newSubs := 0
	churned := 0
	for rs.Next() {
		var id, data string
		_ = id
		if err := rs.Scan(&id, &data); err != nil {
			return digestSubs{}, err
		}
		raw := json.RawMessage(data)
		status, _ := jsonGet(raw, "status")
		created, _ := jsonGetInt(raw, "created")
		canceled, _ := jsonGetInt(raw, "canceled_at")

		switch status {
		case "active":
			active++
		case "trialing":
			trialing++
		}
		if status == "active" || status == "trialing" {
			// subscriptionMonthlyMRR returns (total monthly-normalized minor
			// units, primary currency code, per-currency breakdown for
			// multi-item subs with mixed currencies). Accumulate per-currency
			// so the markdown renderer can show an honest breakdown instead
			// of a single label that misrepresents non-USD merchants.
			m, cur, perCur := subscriptionMonthlyMRR(raw)
			mrr += m
			if len(perCur) > 0 {
				for c, v := range perCur {
					byCurrency[normalizeCurrencyCode(c)] += v
				}
			} else if cur != "" {
				byCurrency[normalizeCurrencyCode(cur)] += m
			}
		}
		if created >= fromTS && created <= toTS {
			newSubs++
		}
		if canceled > 0 && canceled >= fromTS && canceled <= toTS {
			churned++
		}
	}
	if err := rs.Err(); err != nil {
		return digestSubs{}, err
	}

	return digestSubs{
		MRRCents:            mrr,
		ARRCents:            mrr * 12,
		ByCurrencyCents:     byCurrency,
		ActiveSubscribers:   active,
		TrialingSubscribers: trialing,
		NewInWindow:         newSubs,
		ChurnedInWindow:     churned,
	}, nil
}

func normalizeCurrencyCode(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "unknown"
	}
	return strings.ToUpper(c)
}

func buildDisputesSection(db *sql.DB, fromTS, toTS int64) (digestDisputes, error) {
	// Open disputes (regardless of window — open status is the property)
	openRows, err := db.Query(`SELECT data FROM resources WHERE resource_type='disputes'
		AND json_extract(data,'$.status') IN ('needs_response','warning_needs_response','under_review','warning_under_review')`)
	if err != nil {
		return digestDisputes{}, fmt.Errorf("querying open disputes: %w", err)
	}
	defer openRows.Close()

	openCount := 0
	var openTotal int64
	for openRows.Next() {
		var data string
		if err := openRows.Scan(&data); err != nil {
			return digestDisputes{}, err
		}
		raw := json.RawMessage(data)
		openCount++
		if amt, ok := jsonGetInt(raw, "amount"); ok {
			openTotal += amt
		}
	}
	if err := openRows.Err(); err != nil {
		return digestDisputes{}, err
	}

	// Resolved-in-window
	winRows, err := db.Query(`SELECT data FROM resources WHERE resource_type='disputes'
		AND json_extract(data,'$.status') IN ('won','lost')
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) >= ?
		AND CAST(IFNULL(json_extract(data,'$.created'),0) AS INTEGER) <= ?`, fromTS, toTS)
	if err != nil {
		return digestDisputes{}, fmt.Errorf("querying resolved disputes: %w", err)
	}
	defer winRows.Close()

	won := 0
	lost := 0
	for winRows.Next() {
		var data string
		if err := winRows.Scan(&data); err != nil {
			return digestDisputes{}, err
		}
		raw := json.RawMessage(data)
		status, _ := jsonGet(raw, "status")
		switch status {
		case "won":
			won++
		case "lost":
			lost++
		}
	}
	if err := winRows.Err(); err != nil {
		return digestDisputes{}, err
	}

	return digestDisputes{
		OpenCount:    openCount,
		OpenTotal:    openTotal,
		WonInWindow:  won,
		LostInWindow: lost,
	}, nil
}

func buildPayoutsSection(db *sql.DB) (digestPayouts, error) {
	rs, err := db.Query(`SELECT data FROM resources WHERE resource_type='payouts'
		ORDER BY CAST(IFNULL(json_extract(data,'$.arrival_date'),0) AS INTEGER) DESC LIMIT 30`)
	if err != nil {
		return digestPayouts{}, fmt.Errorf("querying payouts: %w", err)
	}
	defer rs.Close()

	type payout struct {
		amount  int64
		arrival int64
		status  string
	}
	var pending, paid []payout
	for rs.Next() {
		var data string
		if err := rs.Scan(&data); err != nil {
			return digestPayouts{}, err
		}
		raw := json.RawMessage(data)
		amt, _ := jsonGetInt(raw, "amount")
		arr, _ := jsonGetInt(raw, "arrival_date")
		st, _ := jsonGet(raw, "status")
		p := payout{amount: amt, arrival: arr, status: st}
		switch st {
		case "pending", "in_transit":
			pending = append(pending, p)
		case "paid":
			paid = append(paid, p)
		}
	}
	if err := rs.Err(); err != nil {
		return digestPayouts{}, err
	}

	out := digestPayouts{}
	if len(pending) > 0 {
		sort.SliceStable(pending, func(i, j int) bool { return pending[i].arrival < pending[j].arrival })
		out.NextPayoutCents = pending[0].amount
		out.NextPayoutArrival = time.Unix(pending[0].arrival, 0).UTC().Format("2006-01-02")
	}
	if len(paid) > 0 {
		// paid list is already DESC by arrival_date thanks to ORDER BY
		// (skipping `pending` may have lifted later ones up, but `paid` order
		// among itself is preserved by SQL — re-sort for safety).
		sort.SliceStable(paid, func(i, j int) bool { return paid[i].arrival > paid[j].arrival })
		out.LastPayoutCents = paid[0].amount
		out.LastPayoutArrival = time.Unix(paid[0].arrival, 0).UTC().Format("2006-01-02")
		out.LastPayoutStatus = paid[0].status
	}
	return out, nil
}

// renderDigestMarkdown produces the human-friendly markdown report.
// Section ordering: revenue, subscriptions, disputes, payouts.
func renderDigestMarkdown(d dailyDigest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Stripe Daily Digest\n")
	fmt.Fprintf(&b, "**Period:** %s -> %s\n\n", d.From, d.To)
	fmt.Fprintf(&b, "---\n\n")

	if d.Charges != nil {
		fmt.Fprintf(&b, "## Revenue\n\n")
		fmt.Fprintf(&b, "| Metric | Value |\n|---|---|\n")
		fmt.Fprintf(&b, "| Gross | USD %s |\n", centsToDollars(d.Charges.GrossCents))
		fmt.Fprintf(&b, "| Refunds | USD %s |\n", centsToDollars(d.Charges.RefundsCents))
		fmt.Fprintf(&b, "| **Net** | **USD %s** |\n", centsToDollars(d.Charges.NetCents))
		fmt.Fprintf(&b, "| Successful charges | %d |\n", d.Charges.SuccessfulCount)
		fmt.Fprintf(&b, "| Failed charges | %d |\n", d.Charges.FailedCount)
		fmt.Fprintf(&b, "| Success rate | %.2f%% |\n", d.Charges.SuccessRatePct)
		fmt.Fprintf(&b, "| AOV | USD %s |\n\n", centsToDollars(d.Charges.AOVCents))

		if len(d.Charges.Days) > 0 {
			fmt.Fprintf(&b, "### Day-by-day\n\n| Date | Charges | Gross | Refunds | Net |\n|---|---|---|---|---|\n")
			for _, row := range d.Charges.Days {
				fmt.Fprintf(&b, "| %s | %d | USD %s | USD %s | USD %s |\n",
					row.Date, row.ChargeCount,
					centsToDollars(row.GrossCents),
					centsToDollars(row.RefundsCents),
					centsToDollars(row.NetCents))
			}
			fmt.Fprintln(&b)
		}
		if len(d.Charges.TopFailureCodes) > 0 {
			fmt.Fprintf(&b, "### Top failure codes\n\n| Code | Count |\n|---|---|\n")
			for _, r := range d.Charges.TopFailureCodes {
				fmt.Fprintf(&b, "| %s | %d |\n", r.Code, r.Count)
			}
			fmt.Fprintln(&b)
		}
		fmt.Fprintf(&b, "---\n\n")
	}

	if d.Subs != nil {
		// Choose the MRR label honestly. Single-currency merchant gets that
		// currency code; multi-currency gets a "mixed" label with a per-
		// currency breakdown table below; absent currency data gets a dash.
		mrrLabel := "Total (mixed currency)"
		switch len(d.Subs.ByCurrencyCents) {
		case 1:
			for c := range d.Subs.ByCurrencyCents {
				mrrLabel = c
			}
		case 0:
			mrrLabel = "—"
		}
		fmt.Fprintf(&b, "## Subscriptions\n\n| Metric | Value |\n|---|---|\n")
		fmt.Fprintf(&b, "| MRR | %s %s |\n", mrrLabel, centsToDollars(d.Subs.MRRCents))
		fmt.Fprintf(&b, "| ARR | %s %s |\n", mrrLabel, centsToDollars(d.Subs.ARRCents))
		fmt.Fprintf(&b, "| Active subscribers | %d |\n", d.Subs.ActiveSubscribers)
		fmt.Fprintf(&b, "| Trialing subscribers | %d |\n", d.Subs.TrialingSubscribers)
		fmt.Fprintf(&b, "| New in window | %d |\n", d.Subs.NewInWindow)
		fmt.Fprintf(&b, "| Churned in window | %d |\n\n", d.Subs.ChurnedInWindow)
		if len(d.Subs.ByCurrencyCents) > 1 {
			// Multi-currency: render per-currency monthly breakdown so the
			// summed MRR row above is interpretable. Sort by code so the
			// table is diff-stable across runs.
			fmt.Fprintf(&b, "Per-currency MRR (monthly normalized):\n\n| Currency | MRR |\n|---|---|\n")
			codes := make([]string, 0, len(d.Subs.ByCurrencyCents))
			for c := range d.Subs.ByCurrencyCents {
				codes = append(codes, c)
			}
			sort.Strings(codes)
			for _, c := range codes {
				fmt.Fprintf(&b, "| %s | %s |\n", c, centsToDollars(d.Subs.ByCurrencyCents[c]))
			}
			fmt.Fprintln(&b)
		}
		fmt.Fprintf(&b, "---\n\n")
	}

	if d.Disputes != nil {
		fmt.Fprintf(&b, "## Disputes\n\n")
		fmt.Fprintf(&b, "Open: **%d** (total USD %s)\n", d.Disputes.OpenCount, centsToDollars(d.Disputes.OpenTotal))
		fmt.Fprintf(&b, "Resolved in window: %d won, %d lost\n\n", d.Disputes.WonInWindow, d.Disputes.LostInWindow)
		fmt.Fprintf(&b, "---\n\n")
	}

	if d.Payouts != nil {
		fmt.Fprintf(&b, "## Payouts\n\n")
		if d.Payouts.NextPayoutCents > 0 {
			fmt.Fprintf(&b, "Next: USD %s arriving %s\n", centsToDollars(d.Payouts.NextPayoutCents), d.Payouts.NextPayoutArrival)
		} else {
			fmt.Fprintf(&b, "Next: _none pending_\n")
		}
		if d.Payouts.LastPayoutCents > 0 {
			fmt.Fprintf(&b, "Last: USD %s on %s (%s)\n\n", centsToDollars(d.Payouts.LastPayoutCents), d.Payouts.LastPayoutArrival, d.Payouts.LastPayoutStatus)
		} else {
			fmt.Fprintf(&b, "Last: _none on record_\n\n")
		}
	}

	return b.String()
}

// centsToDollars renders an integer minor-unit amount as a fixed-2-decimal
// dollar string. Stripe stores amounts as integer cents; this helper is the
// inverse render for human-readable markdown.
func centsToDollars(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	dollars := amount / 100
	cents := amount % 100
	sign := ""
	if neg {
		sign = "-"
	}
	return fmt.Sprintf("%s%d.%02d", sign, dollars, cents)
}

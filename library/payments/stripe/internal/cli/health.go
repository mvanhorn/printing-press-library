// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

// healthFactor describes a single deduction or addition applied to the base
// score, with a human-readable evidence string for the consumer.
type healthFactor struct {
	Name     string `json:"name"`
	Delta    int    `json:"delta"`
	Evidence string `json:"evidence"`
}

// healthInputs is the deterministic input to the scoring function.
// Pure data, no DB handles — that lets the unit tests exercise the formula
// without needing SQLite.
type healthInputs struct {
	OpenDisputes        int
	FailedChargesIn30d  int
	SubscriptionStatus  string // "active", "past_due", "canceled", "unpaid", "" (none)
	ActiveSubscriptions int
	AccountAgeDays      int
	DaysSinceLastCharge int // -1 = never had a successful charge (treated as no signal)
}

// computeHealthScore implements the deterministic 0-100 scoring formula.
// Decreasing factors:
//   - dispute: -25 each (no cap; one dispute is bad enough)
//   - failed_charge_30d: -5 each, capped at -30
//   - subscription_status: past_due -20, canceled -10, unpaid -25
//   - account_age <30d: -5 (new and unproven)
//   - last_successful_charge >60d: -10
//
// Increasing factors:
//   - active_subscriptions >=2: +5
//
// Score is clamped to [0, 100].
func computeHealthScore(in healthInputs) (int, []healthFactor) {
	score := 100
	factors := make([]healthFactor, 0)

	if in.OpenDisputes > 0 {
		d := -25 * in.OpenDisputes
		score += d
		factors = append(factors, healthFactor{
			Name:     "open_disputes",
			Delta:    d,
			Evidence: fmt.Sprintf("%d open dispute(s)", in.OpenDisputes),
		})
	}
	if in.FailedChargesIn30d > 0 {
		d := -5 * in.FailedChargesIn30d
		if d < -30 {
			d = -30
		}
		score += d
		factors = append(factors, healthFactor{
			Name:     "failed_charges_30d",
			Delta:    d,
			Evidence: fmt.Sprintf("%d failed charge(s) in last 30d", in.FailedChargesIn30d),
		})
	}
	switch in.SubscriptionStatus {
	case "past_due":
		score -= 20
		factors = append(factors, healthFactor{Name: "subscription_past_due", Delta: -20, Evidence: "subscription past_due"})
	case "canceled":
		score -= 10
		factors = append(factors, healthFactor{Name: "subscription_canceled", Delta: -10, Evidence: "subscription canceled"})
	case "unpaid":
		score -= 25
		factors = append(factors, healthFactor{Name: "subscription_unpaid", Delta: -25, Evidence: "subscription unpaid"})
	}
	if in.AccountAgeDays >= 0 && in.AccountAgeDays < 30 {
		score -= 5
		factors = append(factors, healthFactor{
			Name:     "new_account",
			Delta:    -5,
			Evidence: fmt.Sprintf("account age %dd (<30d)", in.AccountAgeDays),
		})
	}
	if in.DaysSinceLastCharge > 60 {
		score -= 10
		factors = append(factors, healthFactor{
			Name:     "stale_revenue",
			Delta:    -10,
			Evidence: fmt.Sprintf("last successful charge %dd ago", in.DaysSinceLastCharge),
		})
	}
	if in.ActiveSubscriptions >= 2 {
		score += 5
		factors = append(factors, healthFactor{
			Name:     "multiple_active_subscriptions",
			Delta:    +5,
			Evidence: fmt.Sprintf("%d active subscriptions", in.ActiveSubscriptions),
		})
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, factors
}

type healthRow struct {
	CustomerID string         `json:"customer_id"`
	Email      string         `json:"email,omitempty"`
	Score      int            `json:"score"`
	Factors    []healthFactor `json:"factors"`
	ComputedAt string         `json:"computed_at"`
}

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var allCustomers bool
	var sinceStr string
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "health [customer-id]",
		Short: "Compute a 0-100 health score for one or many customers from local SQLite",
		Long: `Health is computed deterministically from the local SQLite mirror — no LLM,
no live API calls. Run 'stripe-pp-cli sync' first to populate the mirror.`,
		Example: `  # Single customer
  stripe-pp-cli health cus_NffrFeUfNV2Hib

  # Bottom-10 customers, JSON
  stripe-pp-cli health --all --limit 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !allCustomers {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' first.", path, err))
			}
			defer db.Close()

			since, err := parseHumanDuration(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since: %w", err))
			}
			if since == 0 {
				since = 30 * 24 * time.Hour
			}
			now := time.Now()
			windowStart := now.Add(-since)

			var customerIDs []string
			if allCustomers {
				customerIDs, err = listCustomerIDs(cmd.Context(), db.DB(), limit)
				if err != nil {
					return apiErr(err)
				}
			} else {
				customerIDs = []string{args[0]}
			}

			rows := make([]healthRow, 0, len(customerIDs))
			for _, cid := range customerIDs {
				row, err := scoreCustomer(cmd.Context(), db.DB(), cid, now, windowStart)
				if err != nil {
					return apiErr(err)
				}
				if row == nil {
					if !allCustomers {
						return notFoundErr(fmt.Errorf("customer %s not found in local store", cid))
					}
					continue
				}
				rows = append(rows, *row)
			}

			// Sort low-score-first for the --all path so the most at-risk
			// customers float to the top of the JSON array. Stable secondary
			// sort by id so the output is deterministic in tie cases.
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Score != rows[j].Score {
					return rows[i].Score < rows[j].Score
				}
				return rows[i].CustomerID < rows[j].CustomerID
			})

			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}

			if !allCustomers {
				return printJSONFiltered(cmd.OutOrStdout(), rows[0], flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().BoolVar(&allCustomers, "all", false, "Score every customer in the local mirror")
	cmd.Flags().StringVar(&sinceStr, "since", "30d", "Window for failed-charge / staleness factors (e.g. 30d, 4w, 720h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap output to the lowest-scoring N customers")

	return cmd
}

func listCustomerIDs(ctx interface{ Done() <-chan struct{} }, db *sql.DB, limit int) ([]string, error) {
	q := `SELECT id FROM resources WHERE resource_type = 'customers' ORDER BY id`
	if limit > 0 {
		// Don't pre-truncate — we need every customer to compute the score
		// and only THEN sort by health. Keep the query unbounded; the caller
		// trims after sorting.
		_ = q
	}
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// scoreCustomer pulls every relevant resource for cid out of the local store
// and runs computeHealthScore. Returns (nil, nil) when the customer doesn't
// exist in the mirror.
func scoreCustomer(_ interface{ Done() <-chan struct{} }, db *sql.DB, cid string, now, windowStart time.Time) (*healthRow, error) {
	var custData string
	err := db.QueryRow(`SELECT data FROM resources WHERE resource_type='customers' AND id=?`, cid).Scan(&custData)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	email, _ := jsonGet(json.RawMessage(custData), "email")
	created, _ := jsonGetInt(json.RawMessage(custData), "created")

	in := healthInputs{DaysSinceLastCharge: -1, AccountAgeDays: -1}
	if created > 0 {
		in.AccountAgeDays = int(now.Sub(time.Unix(created, 0)).Hours() / 24)
	}

	// Open disputes for this customer. Stripe denormalizes 'customer' on
	// disputes via the parent charge, but in the resources table we have
	// the raw object. Match by data->>customer when present.
	in.OpenDisputes = countWhere(db, "disputes",
		`json_extract(data,'$.customer')=? AND json_extract(data,'$.status')!='lost' AND json_extract(data,'$.status')!='won'`,
		cid)

	// Failed charges in the rolling window (default 30d). Stripe charge
	// status: succeeded | pending | failed.
	in.FailedChargesIn30d = countWhere(db, "charges",
		`json_extract(data,'$.customer')=? AND json_extract(data,'$.status')='failed' AND json_extract(data,'$.created') >= ?`,
		cid, windowStart.Unix())

	// Subscription rollup — pick the worst status across active subs.
	subStatuses := selectStrings(db, "subscriptions",
		`json_extract(data,'$.customer')=?`, cid)
	in.SubscriptionStatus, in.ActiveSubscriptions = rollupSubStatuses(subStatuses)

	// Days since last successful charge.
	var lastChargeUnix sql.NullInt64
	_ = db.QueryRow(
		`SELECT MAX(json_extract(data,'$.created')) FROM resources
		 WHERE resource_type='charges' AND json_extract(data,'$.customer')=?
		 AND json_extract(data,'$.status')='succeeded'`,
		cid,
	).Scan(&lastChargeUnix)
	if lastChargeUnix.Valid && lastChargeUnix.Int64 > 0 {
		in.DaysSinceLastCharge = int(now.Sub(time.Unix(lastChargeUnix.Int64, 0)).Hours() / 24)
	}

	score, factors := computeHealthScore(in)
	return &healthRow{
		CustomerID: cid,
		Email:      email,
		Score:      score,
		Factors:    factors,
		ComputedAt: now.UTC().Format(time.RFC3339),
	}, nil
}

// rollupSubStatuses picks the worst (most-deductive) status among a customer's
// subscriptions. unpaid > past_due > canceled > active. Returns the chosen
// status string and the count of statuses considered "active" (active +
// trialing) for the multi-sub bonus.
func rollupSubStatuses(statuses []string) (string, int) {
	priority := map[string]int{"unpaid": 4, "past_due": 3, "canceled": 2, "active": 1, "trialing": 1, "": 0}
	worst := ""
	worstP := -1
	active := 0
	for _, s := range statuses {
		if s == "active" || s == "trialing" {
			active++
		}
		p, ok := priority[s]
		if !ok {
			continue
		}
		if p > worstP {
			worstP = p
			worst = s
		}
	}
	return worst, active
}

func countWhere(db *sql.DB, resourceType, where string, args ...any) int {
	q := fmt.Sprintf(`SELECT COUNT(*) FROM resources WHERE resource_type=? AND %s`, where)
	full := append([]any{resourceType}, args...)
	var n int
	_ = db.QueryRow(q, full...).Scan(&n)
	return n
}

func selectStrings(db *sql.DB, resourceType, where string, args ...any) []string {
	q := fmt.Sprintf(`SELECT json_extract(data,'$.status') FROM resources WHERE resource_type=? AND %s`, where)
	full := append([]any{resourceType}, args...)
	rows, err := db.Query(q, full...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var s sql.NullString
		if err := rows.Scan(&s); err == nil && s.Valid {
			out = append(out, s.String)
		}
	}
	return out
}

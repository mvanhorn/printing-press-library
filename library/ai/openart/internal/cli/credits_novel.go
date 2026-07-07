package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// addCreditsNovelSubcommands attaches balance / burn / forecast to the
// existing spec-derived `credits` command. Called from root.go before the
// command is added to the rootCmd.
func addCreditsNovelSubcommands(creditsCmd *cobra.Command, flags *rootFlags) {
	creditsCmd.AddCommand(newCreditsBalanceCmd(flags))
	creditsCmd.AddCommand(newCreditsBurnCmd(flags))
	creditsCmd.AddCommand(newCreditsForecastCmd(flags))
}

func newCreditsBalanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Show your current OpenArt credit balance and subscription state",
		Example: `  openart-pp-cli credits balance
  openart-pp-cli credits balance --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, _, err := c.Post("/user/my-info", map[string]any{})
			if err != nil {
				return err
			}
			var u map[string]any
			if err := json.Unmarshal(raw, &u); err != nil {
				return err
			}
			// OpenArt has multiple credit pools. The number shown in the
			// suite UI badge is the sum of subscription_monthly_credit +
			// free_credit_balance + trial_credit_balance. Surface each pool
			// individually so users (and agents) can reason about which
			// pool funds which generation.
			sub, _ := u["subscription_monthly_credit"].(float64)
			free, _ := u["free_credit_balance"].(float64)
			trial, _ := u["trial_credit_balance"].(float64)
			dalle2, _ := u["dalle2_credit_balance"].(float64)
			total := int(sub + free + trial)
			out := map[string]any{
				"balance":                  total,
				"subscription_credits":     int(sub),
				"free_credits":             int(free),
				"trial_credits":            int(trial),
				"dalle2_credits":           int(dalle2),
				"subscription_active":      u["subscription_active"],
				"subscription_type":        u["subscription_type"],
				"subscription_billing":     u["subscription_billing_interval"],
				"display_name":             u["displayName"],
				"username":                 u["username"],
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newCreditsBurnCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		by    string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "burn",
		Short: "Aggregate credit spend over a window, grouped by model / tool / day / project",
		Long: `Reads the local credits ledger (synced from /suite/api/credits/logs)
and groups CONSUME entries by the chosen dimension.

Run 'openart-pp-cli sync' first to populate the local store.`,
		Example: `  openart-pp-cli credits burn --since 7d --by model
  openart-pp-cli credits burn --since 30d --by day --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cutoff := parseSince(since)
			db, err := openLocalStore()
			if err != nil {
				return fmt.Errorf("open local store: %w (run 'openart-pp-cli sync' first)", err)
			}
			defer db.Close()

			// PATCH(credits-bucket-by-first-seen-at): bucket and filter
			// on first_seen_at - the first time the local store observed
			// each credit record - instead of synced_at, which is bumped
			// on every full resync and would collapse historical spends
			// into the resync-day bucket. The captured /credits/logs
			// contract has no per-record timestamp, so first_seen_at is
			// the closest local proxy for spend time. New records get an
			// accurate first_seen_at within seconds of the consume event
			// because the autorefresh path syncs on every CLI invocation
			// (greptile P1).
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT amount, json_extract(data, '$.reference.businessType') AS business,
				 first_seen_at FROM credits WHERE type = 'CONSUME'`)
			if err != nil {
				return fmt.Errorf("query credits: %w", err)
			}
			defer rows.Close()
			type entry struct {
				amount   int
				business string
				ts       time.Time
			}
			var entries []entry
			for rows.Next() {
				var amount sql.NullInt64
				var business sql.NullString
				var tsStr string
				if err := rows.Scan(&amount, &business, &tsStr); err != nil {
					continue
				}
				ts, _ := time.Parse(time.RFC3339, tsStr)
				if ts.IsZero() {
					ts, _ = time.Parse("2006-01-02 15:04:05", tsStr)
				}
				if !cutoff.IsZero() && ts.Before(cutoff) {
					continue
				}
				e := entry{amount: int(amount.Int64), business: business.String, ts: ts}
				if e.business == "" {
					e.business = "unknown"
				}
				entries = append(entries, e)
			}

			groups := map[string]int{}
			for _, e := range entries {
				key := bucketKey(by, e)
				groups[key] += -e.amount // CONSUME is negative; flip to positive spend
			}

			type row struct {
				Bucket string `json:"bucket"`
				Spent  int    `json:"credits_spent"`
				Count  int    `json:"events"`
			}
			counts := map[string]int{}
			for _, e := range entries {
				counts[bucketKey(by, e)]++
			}
			out := make([]row, 0, len(groups))
			for k, v := range groups {
				out = append(out, row{Bucket: k, Spent: v, Count: counts[k]})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Spent > out[j].Spent })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			total := 0
			for _, r := range out {
				total += r.Spent
			}
			result := map[string]any{
				"since":       since,
				"group_by":    by,
				"event_count": len(entries),
				"buckets":     out,
				"total_spent": total,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window: e.g. 24h, 7d, 30d, 90d")
	cmd.Flags().StringVar(&by, "by", "model", "Group by: model | day | week")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap number of buckets (default unlimited)")
	return cmd
}

func newCreditsForecastCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forecast",
		Short: "Project remaining credit runway from recent burn rate",
		Long: `Combines local credits ledger (rolling 4-week average burn) with the
live balance from /suite/api/user/my-info to estimate weeks of runway.`,
		Example: `  openart-pp-cli credits forecast`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, _, err := c.Post("/user/my-info", map[string]any{})
			if err != nil {
				return err
			}
			var u struct {
				FreeCreditBalance         int `json:"free_credit_balance"`
				SubscriptionMonthlyCredit int `json:"subscription_monthly_credit"`
				TrialCreditBalance        int `json:"trial_credit_balance"`
			}
			_ = json.Unmarshal(raw, &u)
			balance := u.SubscriptionMonthlyCredit + u.FreeCreditBalance + u.TrialCreditBalance

			db, err := openLocalStore()
			if err != nil {
				return fmt.Errorf("open local store: %w (run 'openart-pp-cli sync' first)", err)
			}
			defer db.Close()

			cutoff := time.Now().Add(-28 * 24 * time.Hour)
			// PATCH(credits-forecast-first-seen-at): anchor the 28-day
			// burn window on first_seen_at (when the local store first
			// observed the consume event) rather than synced_at, which
			// is rewritten on every resync and inflates the window to
			// "everything ever synced" on a full historical sync.
			row := db.QueryRowContext(cmd.Context(),
				`SELECT COALESCE(SUM(-amount), 0), COUNT(*)
				 FROM credits WHERE type = 'CONSUME' AND first_seen_at >= ?`,
				cutoff.Format("2006-01-02 15:04:05"))
			var spent int64
			var events int64
			if err := row.Scan(&spent, &events); err != nil && err != sql.ErrNoRows {
				return err
			}
			weeklyBurn := float64(spent) / 4.0
			weeksLeft := 0.0
			if weeklyBurn > 0 {
				weeksLeft = float64(balance) / weeklyBurn
			}

			out := map[string]any{
				"balance":             balance,
				"subscription_credits": u.SubscriptionMonthlyCredit,
				"free_credits":        u.FreeCreditBalance,
				"trial_credits":       u.TrialCreditBalance,
				"weekly_burn_avg":     int(weeklyBurn),
				"events_28d":          events,
				"weeks_of_runway":     int(weeksLeft + 0.5),
				"runway_basis_window": "28d",
				"hint":                "burn baseline is from local store; run 'openart-pp-cli sync' for fresh data",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// bucketKey derives the group key for an entry given the --by flag value.
func bucketKey(by string, e creditsEntry) string {
	switch strings.ToLower(by) {
	case "day":
		return e.ts.Format("2006-01-02")
	case "week":
		yr, wk := e.ts.ISOWeek()
		return fmt.Sprintf("%d-W%02d", yr, wk)
	case "month":
		return e.ts.Format("2006-01")
	default: // "model" / "capability" / "business"
		return e.business
	}
}

// creditsEntry mirrors the inline `entry` struct in burn but is exported
// so bucketKey can take it.
type creditsEntry = struct {
	amount   int
	business string
	ts       time.Time
}

// parseSince accepts "7d", "24h", "30d", "12w", or empty (= no cutoff).
func parseSince(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d)
	}
	// Custom suffixes: d=days, w=weeks
	if len(s) < 2 {
		return time.Time{}
	}
	suffix := s[len(s)-1]
	num := 0
	for _, r := range s[:len(s)-1] {
		if r < '0' || r > '9' {
			return time.Time{}
		}
		num = num*10 + int(r-'0')
	}
	switch suffix {
	case 'd':
		return time.Now().Add(-time.Duration(num) * 24 * time.Hour)
	case 'w':
		return time.Now().Add(-time.Duration(num) * 7 * 24 * time.Hour)
	case 'h':
		return time.Now().Add(-time.Duration(num) * time.Hour)
	}
	return time.Time{}
}

// openLocalStore opens the same SQLite DB the generator uses for the
// store package, but as a raw *sql.DB for ad-hoc novel queries.
func openLocalStore() (*sql.DB, error) {
	path := localStorePath()
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("database not found at %s", path)
	}
	return openSQLite(path)
}

func localStorePath() string {
	if env := os.Getenv("OPENART_DB_PATH"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "openart-pp-cli", "data.db")
}

// openSQLite is a thin wrapper that uses the same driver the generated
// store registers ("sqlite" via modernc.org/sqlite). Importing the store
// package indirectly registers it.
func openSQLite(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

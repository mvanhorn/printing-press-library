// Copyright 2026 lokisbo. Licensed under Apache-2.0. See LICENSE.
//
// quota command tree.
//   quota status — read av_quota_log, show today's count vs 25/day cap.
//   quota plan   — mechanical cost estimator for a follow-on subcommand.
//
// Both subcommands are read-only and make zero AV calls. status reads from
// the local SQLite log written by the API client; plan is a static
// cost-per-subcommand map and never opens the store at all.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

// avDailyQuotaCap is the documented Alpha Vantage free-tier ceiling. Users on
// paid tiers can override via the --max flag on `quota status`.
const avDailyQuotaCap = 25

func newQuotaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quota",
		Short: "Local quota tracking for the Alpha Vantage free-tier 25/day budget",
		Long: `Local quota tracking for the Alpha Vantage free-tier 25/day budget.

Subcommands:
  status — show today's used/remaining count, with recent call history.
  plan   — preview how many AV calls a command would burn, without running it.

Both subcommands read local state only — they make no API call and burn
no quota themselves.`,
	}
	cmd.AddCommand(newQuotaStatusCmd(flags))
	cmd.AddCommand(newQuotaPlanCmd(flags))
	return cmd
}

// quotaStatusResult is the JSON shape emitted by `quota status`.
//
// `used` counts every row in av_quota_log for today — including attempts that
// returned a rate-limit envelope or other error. Server-side, AV counts every
// attempt against the cap, so this is the value to compare against `max`. For
// agents wanting "how many calls actually delivered data", `successful_today`
// breaks out the OK=1 subset; `errors_today` is the complement (used - successful).
//
// When `used > max` the local log has recorded more attempts than the
// documented cap — this is normal when the user has been retrying after
// rate-limit responses; the `notes` field surfaces that explicitly so a
// human reader doesn't mistake the row for a counter bug.
type quotaStatusResult struct {
	Used            int             `json:"used"`
	SuccessfulToday int             `json:"successful_today"`
	ErrorsToday     int             `json:"errors_today"`
	Max             int             `json:"max"`
	Remaining       int             `json:"remaining"`
	ResetAt         string          `json:"reset_at"`
	RecentCalls     []quotaCallInfo `json:"recent_calls"`
	Notes           []string        `json:"notes,omitempty"`
}

type quotaCallInfo struct {
	Function string `json:"function"`
	CalledAt string `json:"called_at"`
	OK       bool   `json:"ok"`
}

func newQuotaStatusCmd(flags *rootFlags) *cobra.Command {
	var max int

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show today's Alpha Vantage call count vs daily budget",
		Long: `Show how many of today's Alpha Vantage calls the local quota log has
recorded, and how many remain against the daily cap. Reads from the local
SQLite quota log only — no API call.`,
		Example: strings.Trim(`
  alphavantage-pp-cli quota status --json
  alphavantage-pp-cli quota status --max 75 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// Alpha Vantage resets at 00:00 UTC. Compute today's UTC midnight as
			// the inclusive lower bound, then the next UTC midnight as reset_at.
			now := time.Now().UTC()
			todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			resetAt := todayUTC.Add(24 * time.Hour)

			var used, successful int
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*), COALESCE(SUM(CASE WHEN ok = 1 THEN 1 ELSE 0 END), 0)
				 FROM av_quota_log WHERE called_at >= ?`,
				todayUTC.Format("2006-01-02 15:04:05"),
			).Scan(&used, &successful)
			if err != nil {
				return fmt.Errorf("reading quota log: %w", err)
			}
			errors := used - successful
			if errors < 0 {
				errors = 0
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT function, called_at, ok FROM av_quota_log
				 WHERE called_at >= ? ORDER BY called_at DESC LIMIT 10`,
				todayUTC.Format("2006-01-02 15:04:05"),
			)
			if err != nil {
				return fmt.Errorf("reading recent calls: %w", err)
			}
			defer rows.Close()

			recent := []quotaCallInfo{}
			for rows.Next() {
				var ci quotaCallInfo
				var okInt int
				if err := rows.Scan(&ci.Function, &ci.CalledAt, &okInt); err != nil {
					continue
				}
				ci.OK = okInt != 0
				recent = append(recent, ci)
			}

			cap := max
			if cap <= 0 {
				cap = avDailyQuotaCap
			}
			remaining := cap - used
			if remaining < 0 {
				remaining = 0
			}

			var notes []string
			if used > cap {
				notes = append(notes,
					fmt.Sprintf("local quota log recorded %d attempts (cap %d). The log counts every HTTP attempt — including rate-limit responses — so `used` can legitimately exceed `max` when calls are retried after a 25/day refusal. `successful_today` is the count that actually returned data.", used, cap))
			}

			result := quotaStatusResult{
				Used:            used,
				SuccessfulToday: successful,
				ErrorsToday:     errors,
				Max:             cap,
				Remaining:       remaining,
				ResetAt:         resetAt.Format(time.RFC3339),
				RecentCalls:     recent,
				Notes:           notes,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&max, "max", avDailyQuotaCap, "Daily quota cap (free tier: 25; paid tiers vary)")
	return cmd
}

// quotaPlanResult is the JSON shape emitted by `quota plan`.
type quotaPlanResult struct {
	WouldCall         int                  `json:"would_call"`
	FunctionBreakdown []quotaPlanBreakdown `json:"function_breakdown"`
	UsedToday         int                  `json:"used_today"`
	Max               int                  `json:"max"`
	RemainingNow      int                  `json:"remaining_now"`
	RemainingAfter    int                  `json:"remaining_after"`
	OK                bool                 `json:"ok"`
	Notes             []string             `json:"notes,omitempty"`
}

type quotaPlanBreakdown struct {
	Subcommand string `json:"subcommand"`
	Calls      int    `json:"calls"`
	Reason     string `json:"reason"`
}

func newQuotaPlanCmd(flags *rootFlags) *cobra.Command {
	var max int

	cmd := &cobra.Command{
		Use:                "plan [subcommand args...]",
		Short:              "Preview how many AV calls a command would burn",
		DisableFlagParsing: true,
		Long: `Estimate how many Alpha Vantage API calls a command would consume before
running it. Reads the static cost map and the local quota log; makes no
network calls itself.

Examples of what plan accepts (anything you'd type after 'alphavantage-pp-cli'):
  quota plan news sweep --tickers AAPL,MSFT,NVDA
  quota plan briefing earnings AAPL
  quota plan pulse us
  quota plan macro snapshot

Cost map (per command):
  news sweep         1 call per --tickers entry (watchlist size when --watchlist)
  briefing earnings  up to 4 calls (EARNINGS_CALENDAR + EARNINGS + NEWS + QUOTE)
  movers brief       1 call
  macro snapshot     up to 5 calls (cached endpoints return 0 on subsequent runs)
  pulse us           up to 2 calls
  sync news          1 call per ticker in the watchlist
  sync movers        1 call
  sync earnings      1 call`,
		Example: strings.Trim(`
  alphavantage-pp-cli quota plan news sweep --tickers AAPL,MSFT,NVDA --json
  alphavantage-pp-cli quota plan pulse us --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// DisableFlagParsing is true so we own the args list. Split it into
			// (plan's own flags, inner-command-args) by scanning explicitly.
			// Honors --help, --json, --dry-run, --max on the plan command;
			// everything else passes through to estimateCallsForArgs.
			planArgs, dryRun, asJSON, helpReq := splitPlanArgs(args, &max)
			if helpReq {
				return cmd.Help()
			}
			if asJSON {
				flags.asJSON = true
			}
			if dryRun || dryRunOK(flags) {
				return nil
			}
			if len(planArgs) == 0 {
				return cmd.Help()
			}

			calls, breakdown, notes := estimateCallsForArgs(planArgs)

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			now := time.Now().UTC()
			todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			var used int
			_ = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM av_quota_log WHERE called_at >= ?`,
				todayUTC.Format("2006-01-02 15:04:05"),
			).Scan(&used)

			cap := max
			if cap <= 0 {
				cap = avDailyQuotaCap
			}
			remainingNow := cap - used
			if remainingNow < 0 {
				remainingNow = 0
			}
			remainingAfter := remainingNow - calls
			if remainingAfter < 0 {
				remainingAfter = 0
			}
			ok := calls <= remainingNow

			result := quotaPlanResult{
				WouldCall:         calls,
				FunctionBreakdown: breakdown,
				UsedToday:         used,
				Max:               cap,
				RemainingNow:      remainingNow,
				RemainingAfter:    remainingAfter,
				OK:                ok,
				Notes:             notes,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&max, "max", avDailyQuotaCap, "Daily quota cap (free tier: 25; paid tiers vary)")
	return cmd
}

// estimateCallsForArgs walks the args slice and looks for a known subcommand
// pattern, then returns its expected AV call cost. Unknown patterns return
// (0, breakdown with "unknown command — pass through" reason, nil).
func estimateCallsForArgs(args []string) (int, []quotaPlanBreakdown, []string) {
	if len(args) == 0 {
		return 0, nil, nil
	}

	// Two-token leading subcommands first (most specific match wins).
	if len(args) >= 2 {
		head := args[0] + " " + args[1]
		rest := args[2:]
		switch head {
		case "news sweep":
			n := countTickers(rest)
			return n, []quotaPlanBreakdown{{
				Subcommand: head,
				Calls:      n,
				Reason:     fmt.Sprintf("1 NEWS_SENTIMENT call per ticker (%d tickers)", n),
			}}, nil
		case "briefing earnings":
			return 4, []quotaPlanBreakdown{{
				Subcommand: head,
				Calls:      4,
				Reason:     "EARNINGS_CALENDAR + EARNINGS + NEWS_SENTIMENT + GLOBAL_QUOTE",
			}}, nil
		case "movers brief":
			return 1, []quotaPlanBreakdown{{
				Subcommand: head,
				Calls:      1,
				Reason:     "TOP_GAINERS_LOSERS",
			}}, nil
		case "macro snapshot":
			return 5, []quotaPlanBreakdown{{
				Subcommand: head,
				Calls:      5,
				Reason:     "CPI + FFR + TREASURY_YIELD + UNEMPLOYMENT + NONFARM_PAYROLL (cached after first run)",
			}}, []string{"After the first run, cached indicators return 0 calls until their natural TTL expires."}
		case "pulse us":
			return 2, []quotaPlanBreakdown{{
				Subcommand: head,
				Calls:      2,
				Reason:     "TOP_GAINERS_LOSERS + at most 1 macro indicator refresh",
			}}, []string{"After the first run today, pulse us is a 0-call read from local state."}
		case "watchlist sentiment":
			return 0, []quotaPlanBreakdown{{Subcommand: head, Calls: 0, Reason: "pure local SQL query"}}, nil
		case "news timeline":
			return 0, []quotaPlanBreakdown{{Subcommand: head, Calls: 0, Reason: "pure local SQL aggregation"}}, nil
		case "news search":
			return 0, []quotaPlanBreakdown{{Subcommand: head, Calls: 0, Reason: "pure local FTS5 query"}}, nil
		case "sync news":
			n := countTickers(rest)
			return n, []quotaPlanBreakdown{{
				Subcommand: head,
				Calls:      n,
				Reason:     fmt.Sprintf("1 NEWS_SENTIMENT call per ticker (%d tickers)", n),
			}}, nil
		case "sync movers":
			return 1, []quotaPlanBreakdown{{Subcommand: head, Calls: 1, Reason: "TOP_GAINERS_LOSERS"}}, nil
		case "sync earnings-calendar":
			return 1, []quotaPlanBreakdown{{Subcommand: head, Calls: 1, Reason: "EARNINGS_CALENDAR (CSV)"}}, nil
		}
	}

	// One-token subcommand fallbacks for common leaves.
	switch args[0] {
	case "quote":
		return 1, []quotaPlanBreakdown{{Subcommand: "quote", Calls: 1, Reason: "GLOBAL_QUOTE"}}, nil
	case "search":
		return 1, []quotaPlanBreakdown{{Subcommand: "search", Calls: 1, Reason: "SYMBOL_SEARCH"}}, nil
	case "screen":
		return 0, []quotaPlanBreakdown{{Subcommand: "screen", Calls: 0, Reason: "pure local SQL query"}}, nil
	}

	return 0, []quotaPlanBreakdown{{
		Subcommand: strings.Join(args, " "),
		Calls:      0,
		Reason:     "command not in static cost map; assume 0 (use --dry-run for an accurate trace)",
	}}, []string{"Cost not modeled for this subcommand; estimate is 0. Use --dry-run on the real command for a precise trace."}
}

// countTickers walks args looking for --tickers=A,B or --tickers A,B (and the
// same shape for --watchlist when the watchlist size is unknown). Returns at
// least 1 if neither was seen, so quota plan defaults to "burn at least one call".
func countTickers(args []string) int {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--tickers="):
			return countComma(strings.TrimPrefix(a, "--tickers="))
		case a == "--tickers" && i+1 < len(args):
			return countComma(args[i+1])
		case strings.HasPrefix(a, "--watchlist="), a == "--watchlist":
			// Watchlist size is dynamic and not known here — assume 5 as a
			// conservative midpoint. Users can re-run with --tickers for an
			// exact count.
			return 5
		}
	}
	return 1
}

func countComma(s string) int {
	if s == "" {
		return 0
	}
	n := 1
	for _, r := range s {
		if r == ',' {
			n++
		}
	}
	return n
}

// splitPlanArgs separates plan's own flags (--json, --dry-run, --max, --help)
// from the pass-through args meant for estimateCallsForArgs. We can't rely on
// cobra here because DisableFlagParsing is true (we need it true so cobra
// doesn't choke on inner-command flags like --tickers it doesn't recognize).
func splitPlanArgs(args []string, maxOut *int) (planArgs []string, dryRun bool, asJSON bool, helpReq bool) {
	planArgs = []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--help" || a == "-h":
			helpReq = true
		case a == "--dry-run":
			dryRun = true
		case a == "--json":
			asJSON = true
		case a == "--max" && i+1 < len(args):
			if v, err := strconvAtoiSafe(args[i+1]); err == nil {
				*maxOut = v
			}
			i++
		case strings.HasPrefix(a, "--max="):
			if v, err := strconvAtoiSafe(strings.TrimPrefix(a, "--max=")); err == nil {
				*maxOut = v
			}
		default:
			planArgs = append(planArgs, a)
		}
	}
	return
}

func strconvAtoiSafe(s string) (int, error) {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// logQuotaCall is a helper used by every novel command that issues a live AV
// call. It inserts one row into av_quota_log; failures are silently swallowed
// so quota tracking can never break the surrounding workflow.
func logQuotaCall(cmd *cobra.Command, db *store.Store, function, symbol string, ok bool, message string) {
	okInt := 1
	if !ok {
		okInt = 0
	}
	_, _ = db.DB().ExecContext(cmd.Context(),
		`INSERT INTO av_quota_log (called_at, function, symbol, ok, message)
		 VALUES (CURRENT_TIMESTAMP, ?, ?, ?, ?)`,
		function, symbol, okInt, message,
	)
}

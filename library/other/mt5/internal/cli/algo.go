package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/store"
)

// ── history (mirror reads) ───────────────────────────────────────────────────

func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "history", Short: "Historical orders + deals from the local mirror"}
	cmd.AddCommand(newHistoryDealsCmd())
	cmd.AddCommand(newHistoryOrdersCmd())
	return cmd
}

func newHistoryDealsCmd() *cobra.Command {
	var (
		from, to string
		symbol   string
		magic    int64
	)
	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Print historical deals from the local mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, toT, err := parseRange(from, to)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}

			q := "SELECT ticket, time_ms, symbol, type, entry, volume, price, profit, commission, swap, fee, magic, position_id, comment FROM deals WHERE account_login = ? AND time_ms BETWEEN ? AND ?"
			args2 := []any{acct, fromT.UnixMilli(), toT.UnixMilli()}
			if symbol != "" {
				q += " AND symbol = ?"
				args2 = append(args2, symbol)
			}
			if magic != 0 {
				q += " AND magic = ?"
				args2 = append(args2, magic)
			}
			q += " ORDER BY time_ms ASC"

			rows, err := db.QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return err
			}
			defer rows.Close()

			type d struct {
				Ticket     int64   `json:"ticket"`
				TimeMS     int64   `json:"time_ms"`
				Symbol     string  `json:"symbol"`
				Type       string  `json:"type"`
				Entry      string  `json:"entry"`
				Volume     float64 `json:"volume"`
				Price      float64 `json:"price"`
				Profit     float64 `json:"profit"`
				Commission float64 `json:"commission"`
				Swap       float64 `json:"swap"`
				Fee        float64 `json:"fee"`
				Magic      int64   `json:"magic"`
				PositionID int64   `json:"position_id"`
				Comment    string  `json:"comment"`
			}
			var out []d
			for rows.Next() {
				var x d
				if err := rows.Scan(&x.Ticket, &x.TimeMS, &x.Symbol, &x.Type, &x.Entry,
					&x.Volume, &x.Price, &x.Profit, &x.Commission, &x.Swap, &x.Fee,
					&x.Magic, &x.PositionID, &x.Comment); err != nil {
					return err
				}
				out = append(out, x)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no deals in [%s, %s]. Did you `mt5-pp-cli sync deals`?\n", from, to)
			}
			return emit(cmd, out, func(w io.Writer, v any) {
				items := v.([]d)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "TIME\tTICKET\tSYMBOL\tTYPE\tENTRY\tVOLUME\tPRICE\tP&L\tCOMM\tSWAP\tMAGIC")
				fmt.Fprintln(tw, "────\t──────\t──────\t────\t─────\t──────\t─────\t───\t────\t────\t─────")
				for _, x := range items {
					fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%g\t%g\t%+.2f\t%+.2f\t%+.2f\t%d\n",
						time.UnixMilli(x.TimeMS).Format("2006-01-02 15:04:05"),
						x.Ticket, x.Symbol, x.Type, x.Entry,
						x.Volume, x.Price, x.Profit, x.Commission, x.Swap, x.Magic)
				}
				tw.Flush()
				fmt.Fprintf(w, "\n(%d deal%s)\n", len(items), pluralS(len(items)))
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "30d", "ISO date or relative")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date or relative")
	cmd.Flags().StringVar(&symbol, "symbol", "", "Restrict to one symbol")
	cmd.Flags().Int64Var(&magic, "magic", 0, "Restrict to one EA magic number")
	return cmd
}

func newHistoryOrdersCmd() *cobra.Command {
	var (
		from, to string
		symbol   string
	)
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "Print historical orders from the local mirror",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, toT, err := parseRange(from, to)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			q := "SELECT ticket, time_setup_ms, time_done_ms, symbol, type, state, volume_initial, price_open, sl, tp, magic, comment FROM history_orders WHERE account_login = ? AND time_setup_ms BETWEEN ? AND ?"
			args2 := []any{acct, fromT.UnixMilli(), toT.UnixMilli()}
			if symbol != "" {
				q += " AND symbol = ?"
				args2 = append(args2, symbol)
			}
			q += " ORDER BY time_setup_ms ASC"
			rows, err := db.QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return err
			}
			defer rows.Close()
			type o struct {
				Ticket    int64   `json:"ticket"`
				Setup     int64   `json:"time_setup_ms"`
				Done      int64   `json:"time_done_ms"`
				Symbol    string  `json:"symbol"`
				Type      string  `json:"type"`
				State     string  `json:"state"`
				Volume    float64 `json:"volume_initial"`
				PriceOpen float64 `json:"price_open"`
				SL        float64 `json:"sl"`
				TP        float64 `json:"tp"`
				Magic     int64   `json:"magic"`
				Comment   string  `json:"comment"`
			}
			var out []o
			for rows.Next() {
				var x o
				if err := rows.Scan(&x.Ticket, &x.Setup, &x.Done, &x.Symbol, &x.Type, &x.State,
					&x.Volume, &x.PriceOpen, &x.SL, &x.TP, &x.Magic, &x.Comment); err != nil {
					return err
				}
				out = append(out, x)
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no orders in [%s, %s]. Did you `mt5-pp-cli sync history-orders`?\n", from, to)
			}
			return emit(cmd, out, func(w io.Writer, v any) {
				items := v.([]o)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "SETUP\tDONE\tTICKET\tSYMBOL\tTYPE\tSTATE\tVOLUME\tPRICE\tSL\tTP\tMAGIC")
				fmt.Fprintln(tw, "─────\t────\t──────\t──────\t────\t─────\t──────\t─────\t──\t──\t─────")
				for _, x := range items {
					fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\t%g\t%g\t%g\t%g\t%d\n",
						time.UnixMilli(x.Setup).Format("2006-01-02 15:04"),
						time.UnixMilli(x.Done).Format("2006-01-02 15:04"),
						x.Ticket, x.Symbol, x.Type, x.State, x.Volume, x.PriceOpen, x.SL, x.TP, x.Magic)
				}
				tw.Flush()
				fmt.Fprintf(w, "\n(%d order%s)\n", len(items), pluralS(len(items)))
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "30d", "ISO date or relative")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date or relative")
	cmd.Flags().StringVar(&symbol, "symbol", "", "Restrict to one symbol")
	return cmd
}

// ── stats summary + grouped ─────────────────────────────────────────────────

func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "stats", Short: "Trading statistics over historical deals"}
	cmd.AddCommand(newStatsSummaryCmd())
	cmd.AddCommand(newStatsBy("by-symbol", "symbol", "SYMBOL"))
	cmd.AddCommand(newStatsBy("by-magic", "magic", "MAGIC"))
	cmd.AddCommand(newStatsBy("by-hour",
		"strftime('%H', last_ms/1000, 'unixepoch')",
		"HOUR_UTC"))
	cmd.AddCommand(newStatsBy("by-day-of-week",
		"CASE strftime('%w', last_ms/1000, 'unixepoch') "+
			"WHEN '0' THEN 'Sun' WHEN '1' THEN 'Mon' WHEN '2' THEN 'Tue' "+
			"WHEN '3' THEN 'Wed' WHEN '4' THEN 'Thu' WHEN '5' THEN 'Fri' "+
			"WHEN '6' THEN 'Sat' END",
		"DAY"))
	cmd.AddCommand(newStatsStreaksCmd())
	cmd.AddCommand(newStatsDrawdownCmd())
	return cmd
}

func newStatsSummaryCmd() *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Sharpe (daily $), Sortino, win rate, profit factor, max DD, expectancy",
		Long: `Aggregate statistics across closed positions in the local mirror.

A "closed position" is a position_id whose deals include an 'out' entry.
Per-position P&L = sum(profit + commission + swap + fee) across all its deals.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, err := parseSince(since)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			s, err := computeStatsSummary(cmd.Context(), db, acct, fromT.UnixMilli(), time.Now().UnixMilli())
			if err != nil {
				return err
			}
			return emit(cmd, s, printStatsSummary)
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window: ISO date or relative (e.g. 90d, 1y)")
	return cmd
}

// ── stats summary (Phase 3 carry-over — unchanged) ──────────────────────────

// StatsSummary is the JSON-shaped output of `stats summary`.
type StatsSummary struct {
	FromMS         int64   `json:"from_ms"`
	ToMS           int64   `json:"to_ms"`
	TotalTrades    int     `json:"total_trades"`
	Wins           int     `json:"wins"`
	Losses         int     `json:"losses"`
	BreakEven      int     `json:"break_even"`
	WinRate        float64 `json:"win_rate"`
	NetProfit      float64 `json:"net_profit"`
	GrossProfit    float64 `json:"gross_profit"`
	GrossLoss      float64 `json:"gross_loss"`
	ProfitFactor   float64 `json:"profit_factor"`
	AvgWin         float64 `json:"avg_win"`
	AvgLoss        float64 `json:"avg_loss"`
	Expectancy     float64 `json:"expectancy"`
	LargestWin     float64 `json:"largest_win"`
	LargestLoss    float64 `json:"largest_loss"`
	MaxDrawdown    float64 `json:"max_drawdown"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	SharpeDaily    float64 `json:"sharpe_daily_pnl"`
	SortinoDaily   float64 `json:"sortino_daily_pnl"`
	TradingDays    int     `json:"trading_days"`
}

func computeStatsSummary(ctx context.Context, db *sql.DB, acct, fromMS, toMS int64) (*StatsSummary, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT
		  position_id,
		  MIN(time_ms) AS first_time,
		  MAX(time_ms) AS last_time,
		  SUM(profit + commission + swap + fee) AS pnl,
		  SUM(CASE WHEN entry='out' THEN 1 ELSE 0 END) AS outs
		FROM deals
		WHERE account_login = ? AND time_ms BETWEEN ? AND ?
		  AND position_id <> 0
		GROUP BY position_id
		HAVING outs > 0
		ORDER BY last_time ASC
	`, acct, fromMS, toMS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type trade struct {
		ID, FirstMS, LastMS int64
		PnL                 float64
	}
	var trades []trade
	for rows.Next() {
		var t trade
		var outs int
		if err := rows.Scan(&t.ID, &t.FirstMS, &t.LastMS, &t.PnL, &outs); err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}

	s := &StatsSummary{FromMS: fromMS, ToMS: toMS, TotalTrades: len(trades)}
	if len(trades) == 0 {
		return s, nil
	}
	var winSum, lossSum float64
	for _, t := range trades {
		switch {
		case t.PnL > 0:
			s.Wins++
			winSum += t.PnL
			if t.PnL > s.LargestWin {
				s.LargestWin = t.PnL
			}
		case t.PnL < 0:
			s.Losses++
			lossSum += t.PnL
			if t.PnL < s.LargestLoss {
				s.LargestLoss = t.PnL
			}
		default:
			s.BreakEven++
		}
	}
	s.GrossProfit = winSum
	s.GrossLoss = -lossSum
	s.NetProfit = winSum + lossSum
	if s.Wins+s.Losses > 0 {
		s.WinRate = float64(s.Wins) / float64(s.Wins+s.Losses)
	}
	if s.GrossLoss > 0 {
		s.ProfitFactor = s.GrossProfit / s.GrossLoss
	} else if s.GrossProfit > 0 {
		s.ProfitFactor = math.Inf(1)
	}
	if s.Wins > 0 {
		s.AvgWin = winSum / float64(s.Wins)
	}
	if s.Losses > 0 {
		s.AvgLoss = lossSum / float64(s.Losses)
	}
	s.Expectancy = s.NetProfit / float64(s.TotalTrades)

	var peak, equity float64
	for _, t := range trades {
		equity += t.PnL
		if equity > peak {
			peak = equity
		}
		dd := peak - equity
		if dd > s.MaxDrawdown {
			s.MaxDrawdown = dd
			if peak > 0 {
				s.MaxDrawdownPct = dd / peak * 100
			}
		}
	}

	dayPnL := map[string]float64{}
	for _, t := range trades {
		key := time.UnixMilli(t.LastMS).UTC().Format("2006-01-02")
		dayPnL[key] += t.PnL
	}
	s.TradingDays = len(dayPnL)
	if s.TradingDays >= 2 {
		days := make([]float64, 0, s.TradingDays)
		for _, v := range dayPnL {
			days = append(days, v)
		}
		s.SharpeDaily = annualized(meanStd(days))
		s.SortinoDaily = annualized(meanDownsideStd(days))
	}
	return s, nil
}

func meanStd(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range xs {
		sum += v
	}
	mean := sum / float64(len(xs))
	var ssd float64
	for _, v := range xs {
		ssd += (v - mean) * (v - mean)
	}
	return mean, math.Sqrt(ssd / float64(len(xs)-1))
}

func meanDownsideStd(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	negs := xs[:0:0]
	for _, v := range xs {
		sum += v
		if v < 0 {
			negs = append(negs, v)
		}
	}
	mean := sum / float64(len(xs))
	if len(negs) < 2 {
		return mean, 0
	}
	var ssd float64
	for _, v := range negs {
		ssd += v * v
	}
	return mean, math.Sqrt(ssd / float64(len(negs)-1))
}

func annualized(mean, std float64) float64 {
	if std == 0 {
		return 0
	}
	return mean / std * math.Sqrt(252)
}

func printStatsSummary(w io.Writer, v any) {
	s := v.(*StatsSummary)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	rows := [][2]string{
		{"window", fmt.Sprintf("%s → %s", time.UnixMilli(s.FromMS).Format("2006-01-02"), time.UnixMilli(s.ToMS).Format("2006-01-02"))},
		{"trades", fmt.Sprintf("%d (wins=%d losses=%d be=%d)", s.TotalTrades, s.Wins, s.Losses, s.BreakEven)},
		{"win rate", fmt.Sprintf("%.1f%%", s.WinRate*100)},
		{"net profit", fmt.Sprintf("%+.2f", s.NetProfit)},
		{"gross profit", fmt.Sprintf("%+.2f", s.GrossProfit)},
		{"gross loss", fmt.Sprintf("-%.2f", s.GrossLoss)},
		{"profit factor", formatPF(s.ProfitFactor)},
		{"avg win / loss", fmt.Sprintf("%+.2f / %+.2f", s.AvgWin, s.AvgLoss)},
		{"expectancy", fmt.Sprintf("%+.2f", s.Expectancy)},
		{"largest win/loss", fmt.Sprintf("%+.2f / %+.2f", s.LargestWin, s.LargestLoss)},
		{"max drawdown", fmt.Sprintf("-%.2f (%.1f%% of peak)", s.MaxDrawdown, s.MaxDrawdownPct)},
		{"trading days", fmt.Sprintf("%d", s.TradingDays)},
		{"Sharpe (daily $, ann.)", fmt.Sprintf("%.2f", s.SharpeDaily)},
		{"Sortino (daily $, ann.)", fmt.Sprintf("%.2f", s.SortinoDaily)},
	}
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\n", r[0], r[1])
	}
	tw.Flush()
	if s.TotalTrades == 0 {
		fmt.Fprintln(w, "\nNo trades in window. Run `mt5-pp-cli sync deals --from ...` first.")
	}
}

func formatPF(v float64) string {
	if math.IsInf(v, 1) {
		return "∞ (no losses)"
	}
	if v == 0 {
		return "—"
	}
	return fmt.Sprintf("%.2f", v)
}

// ── grouped stats: by-symbol / by-hour / by-day-of-week / by-magic ──────────

// GroupedStatsRow is one row of a `stats by-*` table.
type GroupedStatsRow struct {
	Key          string  `json:"key"`
	Trades       int     `json:"trades"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	NetProfit    float64 `json:"net_profit"`
	GrossProfit  float64 `json:"gross_profit"`
	GrossLoss    float64 `json:"gross_loss"`
	WinRate      float64 `json:"win_rate"`
	ProfitFactor float64 `json:"profit_factor"`
	AvgPnL       float64 `json:"avg_pnl"`
	LargestWin   float64 `json:"largest_win"`
	LargestLoss  float64 `json:"largest_loss"`
}

// newStatsBy returns a `mt5-pp-cli stats <use>` subcommand that groups closed
// positions by the given SQL expression. The expression runs against the
// inner per-position query, so columns symbol/magic/last_ms are available.
func newStatsBy(use, groupExpr, label string) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Stats grouped by %s", label),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, err := parseSince(since)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			rows, err := computeStatsBy(cmd.Context(), db, acct, fromT.UnixMilli(), time.Now().UnixMilli(), groupExpr)
			if err != nil {
				return err
			}
			return emit(cmd, rows, func(w io.Writer, v any) { printStatsByTable(w, v.([]GroupedStatsRow), label) })
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "Window: ISO date or relative")
	return cmd
}

func computeStatsBy(ctx context.Context, db *sql.DB, acct, fromMS, toMS int64, groupExpr string) ([]GroupedStatsRow, error) {
	q := fmt.Sprintf(`
		WITH positions_pnl AS (
		  SELECT
		    position_id,
		    MAX(symbol)   AS symbol,
		    MAX(magic)    AS magic,
		    MAX(time_ms)  AS last_ms,
		    SUM(profit + commission + swap + fee) AS pnl
		  FROM deals
		  WHERE account_login = ? AND time_ms BETWEEN ? AND ? AND position_id <> 0
		  GROUP BY position_id
		  HAVING SUM(CASE WHEN entry='out' THEN 1 ELSE 0 END) > 0
		)
		SELECT
		  %s                                          AS k,
		  COUNT(*)                                    AS trades,
		  SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END)    AS wins,
		  SUM(CASE WHEN pnl < 0 THEN 1 ELSE 0 END)    AS losses,
		  COALESCE(SUM(pnl), 0)                       AS net,
		  COALESCE(SUM(CASE WHEN pnl > 0 THEN pnl  END), 0) AS gp,
		  COALESCE(SUM(CASE WHEN pnl < 0 THEN -pnl END), 0) AS gl,
		  COALESCE(AVG(pnl), 0)                       AS avg_pnl,
		  COALESCE(MAX(pnl), 0)                       AS lw,
		  COALESCE(MIN(pnl), 0)                       AS ll
		FROM positions_pnl
		GROUP BY k
		ORDER BY net DESC
	`, groupExpr)
	rows, err := db.QueryContext(ctx, q, acct, fromMS, toMS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupedStatsRow
	for rows.Next() {
		var r GroupedStatsRow
		var keyNullable sql.NullString
		if err := rows.Scan(&keyNullable, &r.Trades, &r.Wins, &r.Losses,
			&r.NetProfit, &r.GrossProfit, &r.GrossLoss, &r.AvgPnL,
			&r.LargestWin, &r.LargestLoss); err != nil {
			return nil, err
		}
		r.Key = keyNullable.String
		if r.Wins+r.Losses > 0 {
			r.WinRate = float64(r.Wins) / float64(r.Wins+r.Losses)
		}
		if r.GrossLoss > 0 {
			r.ProfitFactor = r.GrossProfit / r.GrossLoss
		} else if r.GrossProfit > 0 {
			r.ProfitFactor = math.Inf(1)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func printStatsByTable(w io.Writer, rows []GroupedStatsRow, label string) {
	if len(rows) == 0 {
		fmt.Fprintln(w, "No closed trades in window. Run `mt5-pp-cli sync deals --from ...` first.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "%s\tTRADES\tWINS\tLOSSES\tWIN%%\tNET\tPF\tAVG\tLARGEST_W\tLARGEST_L\n",
		label)
	fmt.Fprintf(tw, "─────\t──────\t────\t──────\t────\t───\t──\t───\t─────────\t─────────\n")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.1f%%\t%+.2f\t%s\t%+.2f\t%+.2f\t%+.2f\n",
			r.Key, r.Trades, r.Wins, r.Losses, r.WinRate*100,
			r.NetProfit, formatPF(r.ProfitFactor), r.AvgPnL,
			r.LargestWin, r.LargestLoss)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n(%d group%s)\n", len(rows), pluralS(len(rows)))
}

// ── streaks ─────────────────────────────────────────────────────────────────

func newStatsStreaksCmd() *cobra.Command {
	var (
		since string
		top   int
	)
	cmd := &cobra.Command{
		Use:   "streaks",
		Short: "Longest winning/losing runs and post-streak behavior",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, err := parseSince(since)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			trades, err := loadTradeTimeline(cmd.Context(), db, acct, fromT.UnixMilli(), time.Now().UnixMilli())
			if err != nil {
				return err
			}
			out := computeStreaks(trades, top)
			return emit(cmd, out, printStreaks)
		},
	}
	cmd.Flags().StringVar(&since, "since", "1y", "Window: ISO date or relative")
	cmd.Flags().IntVar(&top, "top", 5, "How many of each streak type to show")
	return cmd
}

type Streak struct {
	Kind      string  `json:"kind"` // "win" | "loss"
	Length    int     `json:"length"`
	FirstMS   int64   `json:"first_ms"`
	LastMS    int64   `json:"last_ms"`
	GrossPnL  float64 `json:"gross_pnl"`
	NextPnL   float64 `json:"next_pnl"`    // first trade after the streak (0 if streak still open)
	NextIsRev bool    `json:"next_is_rev"` // true if next trade flipped sign
}

type StreaksOut struct {
	Wins        []Streak `json:"top_wins"`
	Losses      []Streak `json:"top_losses"`
	CurrentOpen *Streak  `json:"current_open,omitempty"`
	PostRevRate float64  `json:"reversal_after_streak_rate"` // P(next flips sign | streak length ≥ 2)
	Total       int      `json:"trades_in_window"`
}

type tradeRow struct {
	ID     int64
	LastMS int64
	PnL    float64
	Symbol string
	Magic  int64
}

func loadTradeTimeline(ctx context.Context, db *sql.DB, acct, fromMS, toMS int64) ([]tradeRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT position_id,
		       MAX(time_ms) AS last_ms,
		       SUM(profit + commission + swap + fee) AS pnl,
		       MAX(symbol) AS symbol,
		       MAX(magic) AS magic
		FROM deals
		WHERE account_login = ? AND time_ms BETWEEN ? AND ? AND position_id <> 0
		GROUP BY position_id
		HAVING SUM(CASE WHEN entry='out' THEN 1 ELSE 0 END) > 0
		ORDER BY last_ms ASC
	`, acct, fromMS, toMS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tradeRow
	for rows.Next() {
		var t tradeRow
		if err := rows.Scan(&t.ID, &t.LastMS, &t.PnL, &t.Symbol, &t.Magic); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func computeStreaks(trades []tradeRow, top int) *StreaksOut {
	out := &StreaksOut{Total: len(trades)}
	if len(trades) == 0 {
		return out
	}
	var all []Streak
	i := 0
	for i < len(trades) {
		sign := 0
		switch {
		case trades[i].PnL > 0:
			sign = 1
		case trades[i].PnL < 0:
			sign = -1
		default:
			i++ // break-even trades are skipped (don't extend either kind of streak)
			continue
		}
		j := i + 1
		for j < len(trades) {
			s := 0
			switch {
			case trades[j].PnL > 0:
				s = 1
			case trades[j].PnL < 0:
				s = -1
			default:
				s = 0
			}
			if s != sign {
				break
			}
			j++
		}
		streak := Streak{
			Length:  j - i,
			FirstMS: trades[i].LastMS,
			LastMS:  trades[j-1].LastMS,
		}
		for k := i; k < j; k++ {
			streak.GrossPnL += trades[k].PnL
		}
		if sign > 0 {
			streak.Kind = "win"
		} else {
			streak.Kind = "loss"
		}
		// Look at the trade immediately after the streak.
		if j < len(trades) {
			streak.NextPnL = trades[j].PnL
			if (sign > 0 && trades[j].PnL < 0) || (sign < 0 && trades[j].PnL > 0) {
				streak.NextIsRev = true
			}
		} else {
			out.CurrentOpen = &Streak{Kind: streak.Kind, Length: streak.Length, FirstMS: streak.FirstMS, LastMS: streak.LastMS, GrossPnL: streak.GrossPnL}
		}
		all = append(all, streak)
		i = j
	}
	// Top-N for each side.
	winsList := filterStreaks(all, "win")
	lossList := filterStreaks(all, "loss")
	sort.Slice(winsList, func(a, b int) bool { return winsList[a].Length > winsList[b].Length })
	sort.Slice(lossList, func(a, b int) bool { return lossList[a].Length > lossList[b].Length })
	out.Wins = headN(winsList, top)
	out.Losses = headN(lossList, top)

	// Reversal rate: of streaks ≥ length 2 that have a "next" trade, what % flipped sign?
	var qualifying, reversed int
	for _, s := range all {
		if s.Length < 2 || (out.CurrentOpen != nil && s.FirstMS == out.CurrentOpen.FirstMS) {
			continue
		}
		qualifying++
		if s.NextIsRev {
			reversed++
		}
	}
	if qualifying > 0 {
		out.PostRevRate = float64(reversed) / float64(qualifying)
	}
	return out
}

func filterStreaks(all []Streak, kind string) []Streak {
	out := make([]Streak, 0, len(all))
	for _, s := range all {
		if s.Kind == kind {
			out = append(out, s)
		}
	}
	return out
}

func headN(s []Streak, n int) []Streak {
	if n > len(s) {
		n = len(s)
	}
	return s[:n]
}

func printStreaks(w io.Writer, v any) {
	s := v.(*StreaksOut)
	if s.Total == 0 {
		fmt.Fprintln(w, "No closed trades in window.")
		return
	}
	fmt.Fprintf(w, "Trades in window: %d\n", s.Total)
	if s.CurrentOpen != nil {
		fmt.Fprintf(w, "Current open streak: %s of %d (%+.2f)\n",
			s.CurrentOpen.Kind, s.CurrentOpen.Length, s.CurrentOpen.GrossPnL)
	}
	fmt.Fprintf(w, "Post-streak reversal rate (length ≥ 2): %.1f%%\n\n", s.PostRevRate*100)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "KIND\tLEN\tSTART\tEND\tGROSS\tNEXT_PNL\tREVERSED")
	fmt.Fprintln(tw, "────\t───\t─────\t───\t─────\t────────\t────────")
	for _, st := range append(append([]Streak{}, s.Wins...), s.Losses...) {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%+.2f\t%+.2f\t%v\n",
			st.Kind, st.Length,
			time.UnixMilli(st.FirstMS).Format("2006-01-02"),
			time.UnixMilli(st.LastMS).Format("2006-01-02"),
			st.GrossPnL, st.NextPnL, st.NextIsRev)
	}
	tw.Flush()
}

// ── drawdown timeline ──────────────────────────────────────────────────────

func newStatsDrawdownCmd() *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "drawdown",
		Short: "Every drawdown period: depth, duration, recovery",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, err := parseSince(since)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			trades, err := loadTradeTimeline(cmd.Context(), db, acct, fromT.UnixMilli(), time.Now().UnixMilli())
			if err != nil {
				return err
			}
			out := computeDrawdowns(trades)
			return emit(cmd, out, printDrawdowns)
		},
	}
	cmd.Flags().StringVar(&since, "since", "1y", "Window: ISO date or relative")
	return cmd
}

type DrawdownPeriod struct {
	PeakMS            int64   `json:"peak_ms"`
	TroughMS          int64   `json:"trough_ms"`
	RecoveryMS        int64   `json:"recovery_ms"` // 0 if still open
	PeakEquity        float64 `json:"peak_equity"`
	TroughEquity      float64 `json:"trough_equity"`
	Depth             float64 `json:"depth"`
	DepthPct          float64 `json:"depth_pct"`
	DurationToTrough  int64   `json:"duration_to_trough_days"`
	DurationToRecover int64   `json:"duration_to_recover_days"` // -1 if not recovered
	StillOpen         bool    `json:"still_open"`
}

type DrawdownsOut struct {
	Periods        []DrawdownPeriod `json:"periods"`
	MaxDepth       float64          `json:"max_depth"`
	MaxDepthPct    float64          `json:"max_depth_pct"`
	OpenDrawdown   *DrawdownPeriod  `json:"open_drawdown,omitempty"`
	TradesInWindow int              `json:"trades_in_window"`
}

func computeDrawdowns(trades []tradeRow) *DrawdownsOut {
	out := &DrawdownsOut{TradesInWindow: len(trades)}
	if len(trades) == 0 {
		return out
	}
	var equity, peak float64
	var peakMS int64
	type openDD struct {
		PeakMS, TroughMS int64
		PeakEq, TroughEq float64
	}
	var open *openDD
	for _, t := range trades {
		equity += t.PnL
		if equity >= peak {
			// Recovery (or new peak): close any open DD.
			if open != nil && equity >= open.PeakEq {
				dur := (t.LastMS - open.PeakMS) / 1000 / 86400
				troughDays := (open.TroughMS - open.PeakMS) / 1000 / 86400
				p := DrawdownPeriod{
					PeakMS:            open.PeakMS,
					TroughMS:          open.TroughMS,
					RecoveryMS:        t.LastMS,
					PeakEquity:        open.PeakEq,
					TroughEquity:      open.TroughEq,
					Depth:             open.PeakEq - open.TroughEq,
					DepthPct:          pctOfPeak(open.PeakEq, open.TroughEq),
					DurationToTrough:  troughDays,
					DurationToRecover: dur,
				}
				out.Periods = append(out.Periods, p)
				if p.Depth > out.MaxDepth {
					out.MaxDepth = p.Depth
					out.MaxDepthPct = p.DepthPct
				}
				open = nil
			}
			peak = equity
			peakMS = t.LastMS
			continue
		}
		// equity < peak: in (or starting) a drawdown
		if open == nil {
			open = &openDD{PeakMS: peakMS, PeakEq: peak, TroughMS: t.LastMS, TroughEq: equity}
		}
		if equity < open.TroughEq {
			open.TroughEq = equity
			open.TroughMS = t.LastMS
		}
	}
	if open != nil {
		p := DrawdownPeriod{
			PeakMS:            open.PeakMS,
			TroughMS:          open.TroughMS,
			RecoveryMS:        0,
			PeakEquity:        open.PeakEq,
			TroughEquity:      open.TroughEq,
			Depth:             open.PeakEq - open.TroughEq,
			DepthPct:          pctOfPeak(open.PeakEq, open.TroughEq),
			DurationToTrough:  (open.TroughMS - open.PeakMS) / 1000 / 86400,
			DurationToRecover: -1,
			StillOpen:         true,
		}
		out.Periods = append(out.Periods, p)
		out.OpenDrawdown = &p
		if p.Depth > out.MaxDepth {
			out.MaxDepth = p.Depth
			out.MaxDepthPct = p.DepthPct
		}
	}
	return out
}

func pctOfPeak(peak, trough float64) float64 {
	if peak <= 0 {
		return 0
	}
	return (peak - trough) / peak * 100
}

func printDrawdowns(w io.Writer, v any) {
	d := v.(*DrawdownsOut)
	if d.TradesInWindow == 0 {
		fmt.Fprintln(w, "No closed trades in window.")
		return
	}
	fmt.Fprintf(w, "Trades: %d   Max drawdown: %.2f (%.1f%% of peak)\n",
		d.TradesInWindow, d.MaxDepth, d.MaxDepthPct)
	if d.OpenDrawdown != nil {
		fmt.Fprintf(w, "OPEN drawdown: -%.2f since %s (%.1f%% off peak)\n\n",
			d.OpenDrawdown.Depth,
			time.UnixMilli(d.OpenDrawdown.PeakMS).Format("2006-01-02"),
			d.OpenDrawdown.DepthPct)
	} else {
		fmt.Fprintln(w)
	}
	if len(d.Periods) == 0 {
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PEAK\tTROUGH\tRECOVERED\tDEPTH\tDEPTH%\tTO_TROUGH\tTO_RECOVER")
	fmt.Fprintln(tw, "────\t──────\t─────────\t─────\t──────\t─────────\t──────────")
	for _, p := range d.Periods {
		rec := "—"
		recDur := "open"
		if p.RecoveryMS > 0 {
			rec = time.UnixMilli(p.RecoveryMS).Format("2006-01-02")
			recDur = fmt.Sprintf("%dd", p.DurationToRecover)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t-%.2f\t%.1f%%\t%dd\t%s\n",
			time.UnixMilli(p.PeakMS).Format("2006-01-02"),
			time.UnixMilli(p.TroughMS).Format("2006-01-02"),
			rec, p.Depth, p.DepthPct, p.DurationToTrough, recDur)
	}
	tw.Flush()
}

// ── r-multiples ─────────────────────────────────────────────────────────────

func newRMultiplesCmd() *cobra.Command {
	var (
		since        string
		riskPerTrade float64
		balance      float64
	)
	cmd := &cobra.Command{
		Use:   "r-multiples",
		Short: "Express each closed trade as a multiple of risk",
		Long: `Compute R = pnl / risk for each closed position.

Risk preference:
  1. If the entry order has a stop loss in history_orders, use:
       risk = volume × contract_size × |entry_price - sl|        (for FX-style)
       risk = volume × |entry_price - sl|                        (when contract_size is missing)
  2. Otherwise fall back to --risk-per-trade × --balance (or current equity).

Output:
  - per-trade R
  - distribution: mean R, std R, % of positive R-trades
  - expectancy in R (= mean R)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, err := parseSince(since)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()

			// The 'accounts' table doesn't store balance today, so when
			// --balance is 0 we let computeRMultiples fall back per-trade
			// to 0 risk and emit 'unknown' rather than guess. A future
			// commit could backfill accounts.balance from AccountInfo on
			// each sync and wire it here.
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			out, err := computeRMultiples(cmd.Context(), db, acct, fromT.UnixMilli(), time.Now().UnixMilli(), riskPerTrade, balance)
			if err != nil {
				return err
			}
			return emit(cmd, out, printRMultiples)
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "Window")
	cmd.Flags().Float64Var(&riskPerTrade, "risk-per-trade", 0.01, "Fallback risk as fraction of balance when SL is missing")
	cmd.Flags().Float64Var(&balance, "balance", 0, "Balance for fallback risk (default: cached account balance)")
	return cmd
}

type RMultiple struct {
	PositionID int64   `json:"position_id"`
	Symbol     string  `json:"symbol"`
	LastMS     int64   `json:"last_ms"`
	PnL        float64 `json:"pnl"`
	Risk       float64 `json:"risk"`
	R          float64 `json:"r"`
	RiskSource string  `json:"risk_source"` // "sl" | "fallback"
}

type RMultiplesOut struct {
	Trades        []RMultiple `json:"trades"`
	MeanR         float64     `json:"mean_r"`
	StdR          float64     `json:"std_r"`
	PositivePct   float64     `json:"positive_r_pct"`
	ExpectancyR   float64     `json:"expectancy_r"`
	BestR         float64     `json:"best_r"`
	WorstR        float64     `json:"worst_r"`
	SLBackedCount int         `json:"sl_backed_count"`
	FallbackCount int         `json:"fallback_count"`
}

func computeRMultiples(ctx context.Context, db *sql.DB, acct, fromMS, toMS int64, riskPct, fallbackBalance float64) (*RMultiplesOut, error) {
	// One join: per closed position, the entry deal's order ticket → history_orders SL.
	rows, err := db.QueryContext(ctx, `
		WITH per_pos AS (
		  SELECT
		    position_id,
		    MAX(symbol)   AS symbol,
		    MAX(time_ms)  AS last_ms,
		    SUM(profit + commission + swap + fee) AS pnl,
		    MAX(CASE WHEN entry='in' THEN order_ticket END) AS entry_order
		  FROM deals
		  WHERE account_login = ? AND time_ms BETWEEN ? AND ? AND position_id <> 0
		  GROUP BY position_id
		  HAVING SUM(CASE WHEN entry='out' THEN 1 ELSE 0 END) > 0
		)
		SELECT
		  p.position_id, p.symbol, p.last_ms, p.pnl,
		  COALESCE(o.price_open, 0) AS entry_price,
		  COALESCE(o.sl, 0)         AS sl,
		  COALESCE(o.volume_initial, 0) AS volume,
		  COALESCE(s.contract_size, 0)  AS contract_size
		FROM per_pos p
		LEFT JOIN history_orders o ON o.ticket = p.entry_order AND o.account_login = ?
		LEFT JOIN symbols       s ON s.symbol = p.symbol      AND s.account_login = ?
		ORDER BY p.last_ms ASC
	`, acct, fromMS, toMS, acct, acct)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// fallbackBalance: caller passes --balance N to enable per-trade risk
	// imputation when SL is missing. If it's 0, we leave it 0; risk and R
	// for trades without SL come out as 0 ("unknown") rather than a guess.

	var trades []RMultiple
	for rows.Next() {
		var pos RMultiple
		var entryPrice, sl, volume, contractSize float64
		if err := rows.Scan(&pos.PositionID, &pos.Symbol, &pos.LastMS, &pos.PnL,
			&entryPrice, &sl, &volume, &contractSize); err != nil {
			return nil, err
		}
		switch {
		case sl > 0 && entryPrice > 0 && volume > 0:
			perUnit := math.Abs(entryPrice - sl)
			if contractSize > 0 {
				pos.Risk = volume * contractSize * perUnit
			} else {
				pos.Risk = volume * perUnit
			}
			pos.RiskSource = "sl"
		case fallbackBalance > 0 && riskPct > 0:
			pos.Risk = fallbackBalance * riskPct
			pos.RiskSource = "fallback"
		default:
			// No risk info — skip this trade for R analysis.
			continue
		}
		if pos.Risk > 0 {
			pos.R = pos.PnL / pos.Risk
		}
		trades = append(trades, pos)
	}

	out := &RMultiplesOut{Trades: trades}
	if len(trades) == 0 {
		return out, nil
	}
	var rsum float64
	pos := 0
	for _, t := range trades {
		rsum += t.R
		if t.R > 0 {
			pos++
		}
		if t.R > out.BestR {
			out.BestR = t.R
		}
		if t.R < out.WorstR {
			out.WorstR = t.R
		}
		switch t.RiskSource {
		case "sl":
			out.SLBackedCount++
		case "fallback":
			out.FallbackCount++
		}
	}
	out.MeanR = rsum / float64(len(trades))
	out.ExpectancyR = out.MeanR
	out.PositivePct = float64(pos) / float64(len(trades)) * 100
	if len(trades) > 1 {
		var ssd float64
		for _, t := range trades {
			ssd += (t.R - out.MeanR) * (t.R - out.MeanR)
		}
		out.StdR = math.Sqrt(ssd / float64(len(trades)-1))
	}
	return out, nil
}

func printRMultiples(w io.Writer, v any) {
	o := v.(*RMultiplesOut)
	if len(o.Trades) == 0 {
		fmt.Fprintln(w, "No closed trades with derivable risk in window.")
		fmt.Fprintln(w, "Tip: run `mt5-pp-cli sync history-orders --from ...` so SL prices are available.")
		return
	}
	fmt.Fprintf(w, "Trades: %d   SL-backed: %d   fallback: %d\n",
		len(o.Trades), o.SLBackedCount, o.FallbackCount)
	fmt.Fprintf(w, "Mean R: %+.2f   Std R: %.2f   Expectancy: %+.2fR   Positive: %.1f%%\n",
		o.MeanR, o.StdR, o.ExpectancyR, o.PositivePct)
	fmt.Fprintf(w, "Best: %+.2fR   Worst: %+.2fR\n\n", o.BestR, o.WorstR)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tSYMBOL\tPNL\tRISK\tR\tSRC")
	fmt.Fprintln(tw, "────\t──────\t───\t────\t─\t───")
	for _, t := range o.Trades {
		fmt.Fprintf(tw, "%s\t%s\t%+.2f\t%.2f\t%+.2f\t%s\n",
			time.UnixMilli(t.LastMS).Format("2006-01-02"),
			t.Symbol, t.PnL, t.Risk, t.R, t.RiskSource)
	}
	tw.Flush()
}

// ── correlation ─────────────────────────────────────────────────────────────

func newCorrelationCmd() *cobra.Command {
	var (
		symbols []string
		window  string
		tf      string
	)
	cmd := &cobra.Command{
		Use:   "correlation",
		Short: "Pairwise Pearson correlation of log returns across symbols",
		Long: `Compute pairwise Pearson correlation of log returns over the same time window.

Reads bars_<TF> from the local mirror — run 'mt5-pp-cli sync bars' first for each symbol.
Bars are intersected by timestamp; missing bars in any series drop that timestamp.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(symbols) < 2 {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbols needs at least 2 entries")}
			}
			fromT, err := parseSince(window)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			tf = strings.ToUpper(tf)
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()

			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			out, err := computeCorrelation(cmd.Context(), db, acct, symbols, tf, fromT.UnixMilli(), time.Now().UnixMilli())
			if err != nil {
				return err
			}
			return emit(cmd, out, printCorrelation)
		},
	}
	cmd.Flags().StringSliceVar(&symbols, "symbols", nil, "Symbols (≥ 2, required)")
	cmd.Flags().StringVar(&window, "window", "30d", "Window")
	cmd.Flags().StringVar(&tf, "tf", "H1", "Timeframe (M1 M5 M15 M30 H1 H4 D1 W1 MN1)")
	return cmd
}

type CorrelationOut struct {
	Symbols []string    `json:"symbols"`
	TF      string      `json:"tf"`
	BarsN   int         `json:"aligned_bars"`
	Matrix  [][]float64 `json:"matrix"`
}

func computeCorrelation(ctx context.Context, db *sql.DB, acct int64, symbols []string, tf string, fromMS, toMS int64) (*CorrelationOut, error) {
	allowedTFs := map[string]bool{
		"M1": true, "M5": true, "M15": true, "M30": true,
		"H1": true, "H4": true, "D1": true, "W1": true, "MN1": true,
	}
	if !allowedTFs[tf] {
		return nil, fmt.Errorf("--tf %q not in store schema", tf)
	}
	table := "bars_" + tf

	// Load closes per symbol keyed by time_ms.
	series := make([]map[int64]float64, len(symbols))
	for i, sym := range symbols {
		q := fmt.Sprintf(`SELECT time_ms, c FROM %s WHERE account_login = ? AND symbol = ? AND time_ms BETWEEN ? AND ? ORDER BY time_ms ASC`, table)
		rows, err := db.QueryContext(ctx, q, acct, sym, fromMS, toMS)
		if err != nil {
			return nil, err
		}
		series[i] = map[int64]float64{}
		for rows.Next() {
			var t int64
			var c float64
			if err := rows.Scan(&t, &c); err != nil {
				rows.Close()
				return nil, err
			}
			series[i][t] = c
		}
		rows.Close()
		if len(series[i]) == 0 {
			return nil, fmt.Errorf("no bars for %s on %s in window — run `mt5-pp-cli sync bars --symbol %s --tf %s ...`", sym, tf, sym, tf)
		}
	}

	// Intersect timestamps.
	commonT := keysOf(series[0])
	for i := 1; i < len(series); i++ {
		commonT = intersectAsc(commonT, keysOf(series[i]))
	}
	sort.Slice(commonT, func(a, b int) bool { return commonT[a] < commonT[b] })

	if len(commonT) < 3 {
		return nil, fmt.Errorf("not enough overlapping bars (%d) — sync the same window across all symbols first", len(commonT))
	}

	// Build aligned log-return matrix [series][transition]. A transition is
	// kept only when every symbol has positive closes at both ends — dropping
	// it for all series at once keeps rows time-aligned, where skipping it for
	// one symbol would pair returns from different bars in the Pearson sums.
	returns := make([][]float64, len(symbols))
	for k := 1; k < len(commonT); k++ {
		valid := true
		for i := range symbols {
			if series[i][commonT[k-1]] <= 0 || series[i][commonT[k]] <= 0 {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		for i := range symbols {
			returns[i] = append(returns[i], math.Log(series[i][commonT[k]]/series[i][commonT[k-1]]))
		}
	}
	minLen := len(returns[0])
	if minLen < 2 {
		return nil, fmt.Errorf("not enough valid overlapping returns (%d) — sync the same window across all symbols first", minLen)
	}

	mtx := make([][]float64, len(symbols))
	for i := range mtx {
		mtx[i] = make([]float64, len(symbols))
	}
	for i := range symbols {
		mtx[i][i] = 1
		for j := i + 1; j < len(symbols); j++ {
			r := pearson(returns[i], returns[j])
			mtx[i][j] = r
			mtx[j][i] = r
		}
	}

	return &CorrelationOut{Symbols: symbols, TF: tf, BarsN: minLen, Matrix: mtx}, nil
}

func pearson(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 2 {
		return 0
	}
	var sa, sb float64
	for i := 0; i < n; i++ {
		sa += a[i]
		sb += b[i]
	}
	ma, mb := sa/float64(n), sb/float64(n)
	var num, da, dbb float64
	for i := 0; i < n; i++ {
		x, y := a[i]-ma, b[i]-mb
		num += x * y
		da += x * x
		dbb += y * y
	}
	d := math.Sqrt(da * dbb)
	if d == 0 {
		return 0
	}
	return num / d
}

func keysOf(m map[int64]float64) []int64 {
	out := make([]int64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func intersectAsc(a, b []int64) []int64 {
	set := make(map[int64]struct{}, len(a))
	for _, v := range a {
		set[v] = struct{}{}
	}
	out := make([]int64, 0)
	for _, v := range b {
		if _, ok := set[v]; ok {
			out = append(out, v)
		}
	}
	return out
}

func printCorrelation(w io.Writer, v any) {
	c := v.(*CorrelationOut)
	fmt.Fprintf(w, "Symbols: %s   TF: %s   Aligned bars: %d\n\n",
		strings.Join(c.Symbols, ", "), c.TF, c.BarsN)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	hdr := []string{""}
	hdr = append(hdr, c.Symbols...)
	fmt.Fprintln(tw, strings.Join(hdr, "\t"))
	sep := []string{""}
	for range c.Symbols {
		sep = append(sep, "──────")
	}
	fmt.Fprintln(tw, strings.Join(sep, "\t"))
	for i, sym := range c.Symbols {
		cells := []string{sym}
		for j := range c.Symbols {
			cells = append(cells, fmt.Sprintf("%+.2f", c.Matrix[i][j]))
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	tw.Flush()
}

// ── magic audit ─────────────────────────────────────────────────────────────

func newMagicCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "magic", Short: "EA magic-number tools"}
	var (
		since    string
		deadDays int
	)
	audit := &cobra.Command{
		Use:   "audit",
		Short: "Group deals by magic number; surface dead and runaway EAs",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromT, err := parseSince(since)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			out, err := computeMagicAudit(cmd.Context(), db, acct, fromT.UnixMilli(), time.Now().UnixMilli(), deadDays)
			if err != nil {
				return err
			}
			return emit(cmd, out, printMagicAudit)
		},
	}
	audit.Flags().StringVar(&since, "since", "365d", "Window")
	audit.Flags().IntVar(&deadDays, "dead-days", 14, "EAs with no activity in this window are flagged dead")
	cmd.AddCommand(audit)
	return cmd
}

type MagicRow struct {
	Magic     int64   `json:"magic"`
	Trades    int     `json:"trades"`
	NetProfit float64 `json:"net_profit"`
	WinRate   float64 `json:"win_rate"`
	FirstMS   int64   `json:"first_ms"`
	LastMS    int64   `json:"last_ms"`
	DaysSince int64   `json:"days_since_last"`
	Dead      bool    `json:"dead"`
	Runaway   bool    `json:"runaway"` // recent rolling P&L sharply negative
}

func computeMagicAudit(ctx context.Context, db *sql.DB, acct, fromMS, toMS int64, deadDays int) ([]MagicRow, error) {
	rows, err := db.QueryContext(ctx, `
		WITH per_pos AS (
		  SELECT
		    position_id, MAX(magic) AS magic, MAX(time_ms) AS last_ms,
		    SUM(profit + commission + swap + fee) AS pnl
		  FROM deals
		  WHERE account_login = ? AND time_ms BETWEEN ? AND ? AND position_id <> 0
		  GROUP BY position_id
		  HAVING SUM(CASE WHEN entry='out' THEN 1 ELSE 0 END) > 0
		)
		SELECT
		  magic,
		  COUNT(*) AS trades,
		  SUM(pnl) AS net,
		  SUM(CASE WHEN pnl > 0 THEN 1 ELSE 0 END) AS wins,
		  SUM(CASE WHEN pnl < 0 THEN 1 ELSE 0 END) AS losses,
		  MIN(last_ms) AS first_ms, MAX(last_ms) AS last_ms
		FROM per_pos
		GROUP BY magic
		ORDER BY net DESC
	`, acct, fromMS, toMS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now().UnixMilli()
	var out []MagicRow
	for rows.Next() {
		var r MagicRow
		var wins, losses int
		if err := rows.Scan(&r.Magic, &r.Trades, &r.NetProfit, &wins, &losses, &r.FirstMS, &r.LastMS); err != nil {
			return nil, err
		}
		if wins+losses > 0 {
			r.WinRate = float64(wins) / float64(wins+losses)
		}
		r.DaysSince = (now - r.LastMS) / 1000 / 86400
		if r.DaysSince > int64(deadDays) {
			r.Dead = true
		}
		out = append(out, r)
	}
	// Mark runaway: bottom-5 by P&L over the last 7 days.
	cutoff := time.Now().Add(-7 * 24 * time.Hour).UnixMilli()
	type recent struct {
		magic int64
		net   float64
	}
	var recents []recent
	rrows, err := db.QueryContext(ctx, `
		WITH per_pos AS (
		  SELECT position_id, MAX(magic) AS magic, MAX(time_ms) AS last_ms,
		         SUM(profit + commission + swap + fee) AS pnl
		  FROM deals WHERE account_login = ? AND time_ms >= ? AND position_id <> 0
		  GROUP BY position_id
		  HAVING SUM(CASE WHEN entry='out' THEN 1 ELSE 0 END) > 0
		)
		SELECT magic, SUM(pnl) FROM per_pos GROUP BY magic ORDER BY 2 ASC LIMIT 5
	`, acct, cutoff)
	if err == nil {
		defer rrows.Close()
		for rrows.Next() {
			var x recent
			if err := rrows.Scan(&x.magic, &x.net); err == nil {
				if x.net < 0 {
					recents = append(recents, x)
				}
			}
		}
	}
	runawaySet := map[int64]bool{}
	for _, r := range recents {
		runawaySet[r.magic] = true
	}
	for i := range out {
		if runawaySet[out[i].Magic] {
			out[i].Runaway = true
		}
	}
	return out, nil
}

func printMagicAudit(w io.Writer, v any) {
	rows := v.([]MagicRow)
	if len(rows) == 0 {
		fmt.Fprintln(w, "No closed trades in window. Run `mt5-pp-cli sync deals --from ...` first.")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "MAGIC\tTRADES\tNET\tWIN%\tFIRST\tLAST\tDAYS_SINCE\tFLAG")
	fmt.Fprintln(tw, "─────\t──────\t───\t────\t─────\t────\t──────────\t────")
	for _, r := range rows {
		flag := "ok"
		if r.Dead {
			flag = "DEAD"
		}
		if r.Runaway {
			flag = "RUNAWAY"
		}
		fmt.Fprintf(tw, "%d\t%d\t%+.2f\t%.1f%%\t%s\t%s\t%d\t%s\n",
			r.Magic, r.Trades, r.NetProfit, r.WinRate*100,
			time.UnixMilli(r.FirstMS).Format("2006-01-02"),
			time.UnixMilli(r.LastMS).Format("2006-01-02"),
			r.DaysSince, flag)
	}
	tw.Flush()
}

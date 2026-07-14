package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/bridge"
	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/safety"
	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/store"
)

// ── symbols ──────────────────────────────────────────────────────────────────
//
// `symbols list` reads the local mirror (run `mt5-pp-cli sync symbols` first).
// `symbols info` is a live bridge call — spread, bid, ask move in real time.

func newSymbolsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "symbols", Short: "Symbol catalog"}

	var filter string
	list := &cobra.Command{
		Use:   "list",
		Short: "List symbols from the local mirror (optionally --filter EUR*)",
		Long: `List the broker's tradeable symbols from the local mirror.

Reads from the symbols table — run 'mt5-pp-cli sync symbols' first if it's empty.
--filter is a glob pattern translated to SQL LIKE (* → %, ? → _).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}

			q := "SELECT symbol, description, digits, point, spread, volume_min, volume_max, volume_step, base_currency, profit_currency FROM symbols WHERE account_login = ?"
			rowsArgs := []any{acct}
			if filter != "" {
				q += " AND symbol LIKE ?"
				rowsArgs = append(rowsArgs, globToLike(filter))
			}
			q += " ORDER BY symbol"

			rows, err := db.QueryContext(cmd.Context(), q, rowsArgs...)
			if err != nil {
				return err
			}
			defer rows.Close()

			type sym struct {
				Symbol         string  `json:"symbol"`
				Description    string  `json:"description"`
				Digits         int     `json:"digits"`
				Point          float64 `json:"point"`
				Spread         int     `json:"spread"`
				VolumeMin      float64 `json:"volume_min"`
				VolumeMax      float64 `json:"volume_max"`
				VolumeStep     float64 `json:"volume_step"`
				BaseCurrency   string  `json:"base_currency"`
				ProfitCurrency string  `json:"profit_currency"`
			}
			var out []sym
			for rows.Next() {
				var s sym
				if err := rows.Scan(&s.Symbol, &s.Description, &s.Digits, &s.Point, &s.Spread,
					&s.VolumeMin, &s.VolumeMax, &s.VolumeStep, &s.BaseCurrency, &s.ProfitCurrency); err != nil {
					return err
				}
				out = append(out, s)
			}
			if len(out) == 0 {
				if filter != "" {
					var total int
					_ = db.QueryRowContext(cmd.Context(),
						"SELECT count(*) FROM symbols WHERE account_login = ?", acct).Scan(&total)
					fmt.Fprintf(cmd.ErrOrStderr(), "no symbols match %q (account %d mirror has %d total)\n", filter, acct, total)
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "mirror is empty for this account — run `mt5-pp-cli sync symbols` first.")
				}
			}
			return emit(cmd, out, func(w io.Writer, v any) {
				items := v.([]sym)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "SYMBOL\tDESCRIPTION\tDIGITS\tSPREAD\tVMIN\tVMAX\tBASE/QUOTE")
				fmt.Fprintln(tw, "──────\t───────────\t──────\t──────\t────\t────\t──────────")
				for _, s := range items {
					fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%g\t%g\t%s/%s\n",
						s.Symbol, truncate(s.Description, 40), s.Digits, s.Spread,
						s.VolumeMin, s.VolumeMax, s.BaseCurrency, s.ProfitCurrency)
				}
				tw.Flush()
				fmt.Fprintf(w, "\n(%d symbol%s)\n", len(items), pluralS(len(items)))
			})
		},
	}
	list.Flags().StringVar(&filter, "filter", "", "Glob pattern, e.g. EUR* or *USD")
	cmd.AddCommand(list)

	cmd.AddCommand(&cobra.Command{
		Use:   "info <SYMBOL>",
		Short: "Full live symbol_info() dump",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			info, err := b.SymbolInfo(args[0])
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, info, printSymbolInfo)
		},
	})

	return cmd
}

func printSymbolInfo(w io.Writer, v any) {
	s := v.(*bridge.SymbolInfoFull)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	rows := [][2]string{
		{"name", s.Name},
		{"description", s.Description},
		{"base / profit", s.Currency + " / " + s.CurrencyProfit},
		{"digits", fmt.Sprintf("%d", s.Digits)},
		{"point", fmt.Sprintf("%g", s.Point)},
		{"bid", fmt.Sprintf("%.*f", s.Digits, s.Bid)},
		{"ask", fmt.Sprintf("%.*f", s.Digits, s.Ask)},
		{"spread (pts)", fmt.Sprintf("%d (float=%v)", s.Spread, s.SpreadFloat)},
		{"contract size", fmt.Sprintf("%g", s.TradeContractSize)},
		{"tick size", fmt.Sprintf("%g", s.TradeTickSize)},
		{"tick value", fmt.Sprintf("%g", s.TradeTickValue)},
		{"volume min/step/max", fmt.Sprintf("%g / %g / %g", s.VolumeMin, s.VolumeStep, s.VolumeMax)},
		{"trade mode", fmt.Sprintf("%d", s.TradeMode)},
		{"selected", fmt.Sprintf("%v (visible=%v)", s.Select, s.Visible)},
		{"last tick", time.Unix(s.Time, 0).Format(time.RFC3339)},
	}
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\n", r[0], r[1])
	}
	tw.Flush()
}

// ── quote ────────────────────────────────────────────────────────────────────

type quoteOut struct {
	Symbol    string  `json:"symbol"`
	Bid       float64 `json:"bid"`
	Ask       float64 `json:"ask"`
	Last      float64 `json:"last"`
	SpreadPts int     `json:"spread_pts"`
	Time      int64   `json:"time"`
	Digits    int     `json:"digits"`
}

func newQuoteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quote <SYMBOL>",
		Short: "Last tick: bid / ask / last / spread / time",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			info, err := b.SymbolInfo(args[0])
			if err != nil {
				return mapBridgeErr(err)
			}
			tick, err := b.SymbolInfoTick(args[0])
			if err != nil {
				return mapBridgeErr(err)
			}
			if tick.Time == 0 && info.Bid > 0 {
				// Some brokers return symbol_info_tick zero on a fresh select;
				// the symbol_info fields are populated synchronously. Fall back.
				tick.Bid, tick.Ask, tick.Last, tick.Time = info.Bid, info.Ask, info.Last, info.Time
			}
			out := &quoteOut{
				Symbol: args[0], Bid: tick.Bid, Ask: tick.Ask, Last: tick.Last,
				SpreadPts: int((tick.Ask - tick.Bid) / info.Point),
				Time:      tick.Time, Digits: info.Digits,
			}
			return emit(cmd, out, func(w io.Writer, v any) {
				q := v.(*quoteOut)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintf(tw, "symbol\t%s\n", q.Symbol)
				fmt.Fprintf(tw, "bid\t%.*f\n", q.Digits, q.Bid)
				fmt.Fprintf(tw, "ask\t%.*f\n", q.Digits, q.Ask)
				fmt.Fprintf(tw, "last\t%.*f\n", q.Digits, q.Last)
				fmt.Fprintf(tw, "spread\t%d pts\n", q.SpreadPts)
				if q.Time > 0 {
					fmt.Fprintf(tw, "time\t%s\n", time.Unix(q.Time, 0).Format(time.RFC3339))
				} else {
					fmt.Fprintf(tw, "time\t(no recent tick — symbol may not be tradeable right now)\n")
				}
				tw.Flush()
			})
		},
	}
}

// ── book (DOM) ───────────────────────────────────────────────────────────────

func newBookCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "book <SYMBOL>",
		Short: "Market Depth snapshot (requires broker DOM support)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			items, err := b.MarketBookGet(args[0])
			if err != nil {
				return mapBridgeErr(err)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no depth returned — broker may not provide DOM for this symbol")
			}
			return emit(cmd, items, func(w io.Writer, v any) {
				rows := v.([]bridge.BookItem)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "SIDE\tPRICE\tVOLUME")
				fmt.Fprintln(tw, "────\t─────\t──────")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%g\t%g\n", bookSideName(r.Type), r.Price, r.Volume)
				}
				tw.Flush()
			})
		},
	}
}

// ── positions list ───────────────────────────────────────────────────────────

func newPositionsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "positions", Short: "Open positions"}
	var (
		filter string
		symbol string
	)
	list := &cobra.Command{
		Use:   "list",
		Short: "Print all open positions (live snapshot)",
		Long: `Live snapshot of currently open positions (no mirror read).

--symbol restricts to one symbol at the bridge call boundary.
--filter is a glob pattern matched in-process against symbol and comment.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			req := map[string]any{}
			if symbol != "" {
				req["symbol"] = symbol
			}
			ps, err := b.PositionsGet(req)
			if err != nil {
				return mapBridgeErr(err)
			}
			ps = filterPositions(ps, filter)
			return emit(cmd, ps, printPositions)
		},
	}
	list.Flags().StringVar(&symbol, "symbol", "", "Restrict to one symbol")
	list.Flags().StringVar(&filter, "filter", "", "Glob match on symbol or comment (e.g. EUR*)")
	cmd.AddCommand(list)
	return cmd
}

func printPositions(w io.Writer, v any) {
	rows := v.([]bridge.Position)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TICKET\tSYMBOL\tTYPE\tVOLUME\tPRICE_OPEN\tPRICE_NOW\tSL\tTP\tPROFIT\tSWAP\tMAGIC\tOPENED")
	fmt.Fprintln(tw, "──────\t──────\t────\t──────\t──────────\t─────────\t──\t──\t──────\t────\t─────\t──────")
	for _, p := range rows {
		typ := "buy"
		if p.Type == 1 {
			typ = "sell"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%g\t%g\t%g\t%g\t%g\t%+.2f\t%+.2f\t%d\t%s\n",
			p.Ticket, p.Symbol, typ, p.Volume, p.PriceOpen, p.PriceCurrent,
			p.SL, p.TP, p.Profit, p.Swap, p.Magic, time.Unix(p.Time, 0).Format(time.RFC3339))
	}
	tw.Flush()
	fmt.Fprintf(w, "\n(%d position%s)\n", len(rows), pluralS(len(rows)))
}

func filterPositions(ps []bridge.Position, glob string) []bridge.Position {
	if glob == "" {
		return ps
	}
	out := make([]bridge.Position, 0, len(ps))
	for _, p := range ps {
		if globMatch(glob, p.Symbol) || globMatch(glob, p.Comment) {
			out = append(out, p)
		}
	}
	return out
}

// ── orders list ──────────────────────────────────────────────────────────────

func newOrdersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "orders", Short: "Active (pending) orders"}
	var symbol string
	list := &cobra.Command{
		Use:   "list",
		Short: "Print all active orders (live snapshot)",
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			req := map[string]any{}
			if symbol != "" {
				req["symbol"] = symbol
			}
			os, err := b.OrdersGet(req)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, os, printOrders)
		},
	}
	list.Flags().StringVar(&symbol, "symbol", "", "Restrict to one symbol")
	cmd.AddCommand(list)
	return cmd
}

func printOrders(w io.Writer, v any) {
	rows := v.([]bridge.Order)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TICKET\tSYMBOL\tTYPE\tVOLUME\tPRICE_OPEN\tSL\tTP\tSTATE\tSETUP")
	fmt.Fprintln(tw, "──────\t──────\t────\t──────\t──────────\t──\t──\t─────\t─────")
	for _, o := range rows {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%g\t%g\t%g\t%g\t%s\t%s\n",
			o.Ticket, o.Symbol, orderTypeNameInt(o.Type), o.VolumeCurrent, o.PriceOpen,
			o.SL, o.TP, orderStateNameInt(o.State), time.Unix(o.TimeSetup, 0).Format(time.RFC3339))
	}
	tw.Flush()
	fmt.Fprintf(w, "\n(%d order%s)\n", len(rows), pluralS(len(rows)))
}

// ── order check (read-only preview) ──────────────────────────────────────────

func newOrderCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "order", Short: "Place/preview a new order"}
	cmd.AddCommand(newOrderCheckCmd())
	cmd.AddCommand(newOrderSendCmd())
	return cmd
}

func newOrderCheckCmd() *cobra.Command {
	var (
		symbol, side string
		volume       float64
		price        float64
		sl, tp       float64
	)
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Preview margin + validity (mt5.order_check) — never sends",
		RunE: func(cmd *cobra.Command, args []string) error {
			if symbol == "" || side == "" || volume == 0 {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol, --side, --volume are required")}
			}
			action, err := parseOrderSide(side)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 10 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			tick, _ := b.SymbolInfoTick(symbol)
			if price == 0 && tick != nil {
				if action == 0 {
					price = tick.Ask
				} else {
					price = tick.Bid
				}
			}
			req := buildMarketRequest(symbol, action, volume, price, sl, tp, 0, "", 20)
			res, err := b.OrderCheck(req)
			if err != nil {
				return mapBridgeErr(err)
			}
			return emit(cmd, res, func(w io.Writer, v any) {
				m := v.(map[string]any)
				keys := make([]string, 0, len(m))
				for k := range m {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				for _, k := range keys {
					fmt.Fprintf(tw, "%s\t%v\n", k, m[k])
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	cmd.Flags().StringVar(&side, "side", "", "buy | sell (required)")
	cmd.Flags().Float64Var(&volume, "volume", 0, "Lot size (required)")
	cmd.Flags().Float64Var(&price, "price", 0, "Override execution price (default: current ask/bid)")
	cmd.Flags().Float64Var(&sl, "sl", 0, "Stop loss")
	cmd.Flags().Float64Var(&tp, "tp", 0, "Take profit")
	return cmd
}

// ── order send (write — full safety flow) ────────────────────────────────────

func newOrderSendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Place a market order (DRY-RUN by default; requires safety hash)",
		Long: `Place a market order on the connected broker.

Safety flow:
  1. First run prints a SHA-256 hash of the canonical request and exits 6.
  2. Re-run with --confirm <hash> within 60 seconds to actually send.
  3. Real-account writes additionally require MT5_LIVE=1 AND
     --i-understand-this-is-live (skipped on demo and contest accounts).
  4. Per-command guardrails from ~/.config/mt5-pp-cli/config.toml apply.
  5. Every attempt is appended to <store_dir>/audit.jsonl AND the audit DB
     table (queryable via 'mt5-pp-cli sql').`,
		RunE: func(cmd *cobra.Command, args []string) error {
			symbol, _ := cmd.Flags().GetString("symbol")
			sideStr, _ := cmd.Flags().GetString("side")
			volume, _ := cmd.Flags().GetFloat64("volume")
			price, _ := cmd.Flags().GetFloat64("price")
			sl, _ := cmd.Flags().GetFloat64("sl")
			tp, _ := cmd.Flags().GetFloat64("tp")
			magic, _ := cmd.Flags().GetInt64("magic")
			comment, _ := cmd.Flags().GetString("comment")
			deviation, _ := cmd.Flags().GetInt("deviation")

			if symbol == "" || sideStr == "" || volume == 0 {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol, --side, --volume are required")}
			}
			action, err := parseOrderSide(sideStr)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			ctx := writeCtx{cmd: cmd, opName: "order_send"}
			b, db, acc, g, err := ctx.openAll()
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()

			if price == 0 {
				tick, _ := b.SymbolInfoTick(symbol)
				if tick != nil {
					if action == 0 {
						price = tick.Ask
					} else {
						price = tick.Bid
					}
				}
			}
			req := buildMarketRequest(symbol, action, volume, price, sl, tp, magic, comment, deviation)

			if err := safety.CheckGuardrails(g, volume); err != nil {
				return ctx.reject(db, acc.Login, req, err)
			}
			// order_send opens a NEW position — wouldAdd=true.
			if err := checkPositionCapLive(cmd.Context(), b, g, true); err != nil {
				return ctx.reject(db, acc.Login, req, err)
			}
			if err := checkDailyLossLive(cmd.Context(), b, g); err != nil {
				return ctx.reject(db, acc.Login, req, err)
			}
			return ctx.runOrderSend(b, db, acc, req)
		},
	}
	addOrderFlags(cmd)
	safety.AddWriteFlags(cmd)
	return cmd
}

// checkPositionCapLive queries the live position count via the bridge and
// hands it to safety.CheckPositionCap. Fail-closed: a bridge error rejects
// the order rather than silently allowing it — a safety guardrail that
// disables itself on transient errors is worse than no guardrail because
// it builds false confidence.
func checkPositionCapLive(ctx context.Context, b *bridge.Bridge, g safety.Guardrails, wouldAdd bool) error {
	if g.MaxOpenPositions <= 0 {
		return nil
	}
	positions, err := b.PositionsGet(nil)
	if err != nil {
		return fmt.Errorf("max_open_positions guardrail: cannot determine current position count (bridge error: %w) — refusing the order; retry once the bridge is healthy or set max_open_positions=0 to disable", err)
	}
	return safety.CheckPositionCap(g, len(positions), wouldAdd)
}

// checkDailyLossLive sums today's realized P&L from live MT5 deal history
// using a UTC day boundary. Fail-closed: a bridge error rejects the order.
//
// Entry filter covers every MT5 entry type that carries realized P&L:
//   - out    - straight close
//   - inout  - reverse / partial close that flips side (netting)
//   - out_by - closed-by-opposite-position (netting/hedging)
//
// 'in' alone never realizes P&L. position_id<>0 strips balance/credit deals.
func checkDailyLossLive(ctx context.Context, b *bridge.Bridge, g safety.Guardrails) error {
	if g.MaxDailyLoss <= 0 {
		return nil
	}
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	deals, err := b.HistoryDealsGet(dayStart.Unix(), now.Unix())
	if err != nil {
		return fmt.Errorf("max_daily_loss guardrail: cannot fetch today's live deal history (bridge error: %w) - refusing the order; retry once the bridge is healthy or set max_daily_loss=0 to disable", err)
	}
	return safety.CheckDailyLoss(g, realizedPnLFromDeals(deals))
}

func realizedPnLFromDeals(deals []bridge.Deal) float64 {
	var realized float64
	for _, d := range deals {
		if d.PositionID == 0 {
			continue
		}
		switch d.Entry {
		case 1, 2, 3: // out, inout, out_by
			realized += d.Profit + d.Commission + d.Swap + d.Fee
		}
	}
	return realized
}

func addOrderFlags(cmd *cobra.Command) {
	cmd.Flags().String("symbol", "", "Symbol (required)")
	cmd.Flags().String("side", "", "buy | sell")
	cmd.Flags().Float64("volume", 0, "Lot size (required)")
	cmd.Flags().Float64("price", 0, "Override execution price (default: current ask/bid)")
	cmd.Flags().Float64("sl", 0, "Stop loss price")
	cmd.Flags().Float64("tp", 0, "Take profit price")
	cmd.Flags().Int64("magic", 0, "EA magic number")
	cmd.Flags().String("comment", "", "Order comment")
	cmd.Flags().Int("deviation", 20, "Max price deviation in points")
}

// ── position close / modify ──────────────────────────────────────────────────

func newPositionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "position", Short: "Operate on an open position by ticket"}

	closeCmd := &cobra.Command{
		Use:   "close <ticket>",
		Short: "Close a position (DRY-RUN by default; safety hash required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticket, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("ticket must be an integer: %w", err)}
			}
			partial, _ := cmd.Flags().GetFloat64("partial")

			ctx := writeCtx{cmd: cmd, opName: "position_close"}
			b, db, acc, g, err := ctx.openAll()
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()

			// Find the position so we know symbol, volume, side, current price.
			positions, err := b.PositionsGet(nil)
			if err != nil {
				return mapBridgeErr(err)
			}
			var pos *bridge.Position
			for i := range positions {
				if positions[i].Ticket == ticket {
					pos = &positions[i]
					break
				}
			}
			if pos == nil {
				return &ExitErr{Code: ExitNotFound, Err: fmt.Errorf("no open position with ticket %d", ticket)}
			}
			volume := pos.Volume
			if partial > 0 && partial < 1 {
				step := lookupVolumeStep(cmd.Context(), db, acc.Login, pos.Symbol)
				volume = roundLot(pos.Volume*partial, step)
				if volume == 0 {
					return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("partial=%g rounds to 0 lots (volume_step=%g)", partial, step)}
				}
			}
			req := buildCloseRequest(pos, volume)
			if err := safety.CheckGuardrails(g, volume); err != nil {
				return ctx.reject(db, acc.Login, req, err)
			}
			return ctx.runOrderSend(b, db, acc, req)
		},
	}
	closeCmd.Flags().Float64("partial", 0, "Close only this fraction of volume (0<x<1, rounded to 0.01 lots)")
	safety.AddWriteFlags(closeCmd)
	cmd.AddCommand(closeCmd)

	modify := &cobra.Command{
		Use:   "modify <ticket>",
		Short: "Modify SL/TP on an open position (DRY-RUN by default; safety hash required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ticket, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("ticket must be an integer: %w", err)}
			}
			sl, _ := cmd.Flags().GetFloat64("sl")
			tp, _ := cmd.Flags().GetFloat64("tp")
			if sl == 0 && tp == 0 {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("provide at least one of --sl or --tp")}
			}
			ctx := writeCtx{cmd: cmd, opName: "position_modify"}
			b, db, acc, _, err := ctx.openAll()
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()
			req := map[string]any{
				"action":   6, // TRADE_ACTION_SLTP
				"position": ticket,
				"sl":       sl,
				"tp":       tp,
			}
			return ctx.runOrderSend(b, db, acc, req)
		},
	}
	modify.Flags().Float64("sl", 0, "New stop loss")
	modify.Flags().Float64("tp", 0, "New take profit")
	safety.AddWriteFlags(modify)
	cmd.AddCommand(modify)

	return cmd
}

// ── close all --filter "<sql>" — the hero command's engine ──────────────────

func newCloseAllCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "close", Short: "Bulk operations across positions"}
	all := &cobra.Command{
		Use:   "all",
		Short: "Close every position matching --filter (SQL WHERE clause)",
		Long: `Close every open position matching a SQL-style WHERE clause.

Flow:
  1. Snapshot live positions from the bridge into the positions table.
  2. SELECT ticket, symbol, type, volume, profit FROM positions WHERE <filter>.
  3. Print the resolved ticket list, total exposure, and a single hash that
     covers the whole bulk close.
  4. Exit 6 unless --confirm <hash> was passed.
  5. On confirm: close each ticket sequentially; per-position retcode → audit
     log. The bulk operation succeeds even if some closes fail; exit code is
     5 (broker rejected) if any close was rejected, else 0.

Examples:
  mt5-pp-cli close all --filter "profit < -50"
  mt5-pp-cli close all --filter "symbol like 'XAU%' AND magic = 0"
  mt5-pp-cli close all --filter "1=1"      # everything (explicit on purpose)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			filter, _ := cmd.Flags().GetString("filter")
			if filter == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--filter is required (use \"1=1\" for everything; explicit is the point)")}
			}
			ctx := writeCtx{cmd: cmd, opName: "close_all"}
			b, db, acc, g, err := ctx.openAll()
			if err != nil {
				return err
			}
			defer b.Close()
			defer db.Close()

			// Validate the filter before it touches the SQL engine. The
			// driver blocks multi-statement injection (verified empirically
			// against modernc.org/sqlite), but a filter with comments or
			// statement separators is almost certainly a mistake or attack
			// and we'd rather reject than guess. UNION etc. is still
			// possible — the safety hash binds tickets+filter so a UI lie
			// would surface in the dry-run table.
			if err := validateCloseAllFilter(filter); err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}

			// Step 1: snapshot positions into the mirror so the filter has data.
			if _, err := store.SyncPositions(cmd.Context(), db, b, acc.Login); err != nil {
				return mapBridgeErr(err)
			}
			// Step 2: resolve the filter to a ticket list. ORDER BY ticket
			// is load-bearing: without it, SQLite's row order can vary
			// between dry-run and confirm (e.g., when sync inserts a new
			// position), which would invalidate the user's hash.
			rows, err := db.QueryContext(cmd.Context(),
				"SELECT ticket, symbol, type, volume, profit FROM positions WHERE account_login=? AND ("+filter+") ORDER BY ticket ASC",
				acc.Login)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("filter SQL: %w", err)}
			}
			defer rows.Close()
			type target struct {
				Ticket int64
				Symbol string
				Type   string
				Volume float64
				Profit float64
			}
			var targets []target
			for rows.Next() {
				var t target
				if err := rows.Scan(&t.Ticket, &t.Symbol, &t.Type, &t.Volume, &t.Profit); err != nil {
					return err
				}
				targets = append(targets, t)
			}
			// req: full payload (with live profit) for the audit log.
			// intent: stable subset for the hash so a price tick between
			// dry-run and confirm doesn't invalidate the user's hash.
			req := map[string]any{
				"op":      "close_all",
				"filter":  filter,
				"tickets": targets,
			}
			intentTickets := make([]map[string]any, len(targets))
			for i, t := range targets {
				intentTickets[i] = map[string]any{
					"ticket": t.Ticket, "symbol": t.Symbol,
					"type": t.Type, "volume": t.Volume,
				}
			}
			intent := map[string]any{"op": "close_all", "filter": filter, "tickets": intentTickets}

			fmt.Fprintf(cmd.OutOrStdout(), "\nclose all candidates (filter: %s)\n", filter)
			if len(targets) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no positions match — nothing to close)")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "TICKET\tSYMBOL\tTYPE\tVOLUME\tCURRENT P&L")
			fmt.Fprintln(tw, "──────\t──────\t────\t──────\t───────────")
			var totalPnL float64
			for _, t := range targets {
				totalPnL += t.Profit
				fmt.Fprintf(tw, "%d\t%s\t%s\t%g\t%+.2f\n", t.Ticket, t.Symbol, t.Type, t.Volume, t.Profit)
			}
			tw.Flush()
			fmt.Fprintf(cmd.OutOrStdout(), "\nrealized P&L if closed now: %+.2f  (%d position%s)\n",
				totalPnL, len(targets), pluralS(len(targets)))

			if err := safety.PrecheckWrite(cmd, g, acc.TradeMode); err != nil {
				return ctx.reject(db, acc.Login, req, err)
			}
			confirmed, err := safety.Confirmed(cmd, intent)
			if err != nil {
				return err
			}
			if !confirmed {
				h := safety.CurrentHash(intent)
				fmt.Fprintf(cmd.OutOrStdout(), "\nhash: %s  (60–120s bucketed window; covers tickets + filter, not live P&L)\n", h)
				fmt.Fprintf(cmd.OutOrStdout(), "to execute:  mt5-pp-cli close all --filter %q --confirm %s%s\n",
					filter, h, liveFlagsHint(acc.TradeMode))
				ctx.auditDryRun(db, acc.Login, req, h)
				return &ExitErr{Code: ExitSafetyRejected, Err: fmt.Errorf("dry-run — pass --confirm to execute")}
			}

			// Execute each close sequentially.
			anyFailed := false
			results := make([]map[string]any, 0, len(targets))
			for _, t := range targets {
				positions, _ := b.PositionsGet(map[string]any{"symbol": t.Symbol})
				var pos *bridge.Position
				for i := range positions {
					if positions[i].Ticket == t.Ticket {
						pos = &positions[i]
						break
					}
				}
				if pos == nil {
					results = append(results, map[string]any{"ticket": t.Ticket, "error": "position vanished between filter and close"})
					anyFailed = true
					continue
				}
				closeReq := buildCloseRequest(pos, pos.Volume)
				res, err := b.OrderSend(closeReq)
				if err != nil {
					results = append(results, map[string]any{"ticket": t.Ticket, "error": err.Error()})
					anyFailed = true
					continue
				}
				if !isOK(res.Retcode) {
					anyFailed = true
				}
				results = append(results, map[string]any{
					"ticket": t.Ticket, "retcode": res.Retcode, "comment": res.Comment,
					"price": res.Price, "deal": res.Deal,
				})
			}
			ctx.auditConfirmed(db, acc.Login, req, results)

			tw2 := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw2, "\nTICKET\tRETCODE\tCOMMENT")
			fmt.Fprintln(tw2, "──────\t───────\t───────")
			for _, r := range results {
				fmt.Fprintf(tw2, "%v\t%v\t%v\n", r["ticket"], r["retcode"], r["comment"])
			}
			tw2.Flush()

			if anyFailed {
				return &ExitErr{Code: ExitBrokerRejected, Err: fmt.Errorf("one or more closes failed")}
			}
			return nil
		},
	}
	all.Flags().String("filter", "", "SQL WHERE clause against the positions table (required)")
	safety.AddWriteFlags(all)
	cmd.AddCommand(all)
	return cmd
}

// ── write-flow helpers ──────────────────────────────────────────────────────

// writeCtx bundles the bridge/db/config plumbing every write command needs.
// One per command invocation; no shared state across calls.
type writeCtx struct {
	cmd    *cobra.Command
	opName string
}

func (w writeCtx) openAll() (*bridge.Bridge, *sql.DB, *bridge.AccountInfo, safety.Guardrails, error) {
	cfg, err := config.Load("")
	if err != nil {
		return nil, nil, nil, safety.Guardrails{}, &ExitErr{Code: ExitConfig, Err: err}
	}
	g := safety.Guardrails{
		MaxVolumePerOrder: cfg.Guardrails.MaxVolumePerOrder,
		MaxOpenPositions:  cfg.Guardrails.MaxOpenPositions,
		MaxDailyLoss:      cfg.Guardrails.MaxDailyLoss,
		KillSwitchFile:    cfg.Guardrails.KillSwitchFile,
	}
	b, err := bridge.New(bridge.Options{Stderr: w.cmd.ErrOrStderr(), CallTimeout: 30 * time.Second})
	if err != nil {
		return nil, nil, nil, g, err
	}
	if err := initBridge(w.cmd, b, 10000); err != nil {
		_ = b.Close()
		return nil, nil, nil, g, err
	}
	acc, err := b.AccountInfo()
	if err != nil {
		_ = b.Close()
		return nil, nil, nil, g, mapBridgeErr(err)
	}
	db, err := store.OpenAndMigrate("")
	if err != nil {
		_ = b.Close()
		return nil, nil, nil, g, &ExitErr{Code: ExitConfig, Err: err}
	}
	return b, db, acc, g, nil
}

// runOrderSend runs the dry-run-or-execute flow for a single order_send.
//
// The HASH covers user intent (symbol/side/volume/sl/tp/deviation/magic/
// position) only, not the live market price, which changes every tick and
// would invalidate every confirm. The broker's deviation field absorbs price
// moves within the 60-120s bucketed window.
func (w writeCtx) runOrderSend(b *bridge.Bridge, db *sql.DB, acc *bridge.AccountInfo, req map[string]any) error {
	cfg, _ := config.Load("")
	g := safety.Guardrails{
		MaxVolumePerOrder: cfg.Guardrails.MaxVolumePerOrder,
		KillSwitchFile:    cfg.Guardrails.KillSwitchFile,
	}
	if err := safety.PrecheckWrite(w.cmd, g, acc.TradeMode); err != nil {
		return w.reject(db, acc.Login, req, err)
	}

	intent := hashableIntent(req)
	confirmed, err := safety.Confirmed(w.cmd, intent)
	if err != nil {
		return err
	}
	if !confirmed {
		h := safety.CurrentHash(intent)
		w.printDryRun(req)
		fmt.Fprintf(w.cmd.OutOrStdout(), "hash: %s  (60–120s bucketed window; covers intent, not the live price)\n", h)
		fmt.Fprintf(w.cmd.OutOrStdout(), "to execute: re-run with  --confirm %s%s\n", h, liveFlagsHint(acc.TradeMode))
		w.auditDryRun(db, acc.Login, req, h)
		return &ExitErr{Code: ExitSafetyRejected, Err: fmt.Errorf("dry-run — pass --confirm to execute")}
	}

	res, err := b.OrderSend(req)
	if err != nil {
		w.auditError(db, acc.Login, req, err)
		return mapBridgeErr(err)
	}
	w.auditConfirmed(db, acc.Login, req, res)

	if !isOK(res.Retcode) {
		w.printOrderResult(res)
		return &ExitErr{Code: ExitBrokerRejected, Err: fmt.Errorf("retcode %d: %s", res.Retcode, res.Comment)}
	}
	w.printOrderResult(res)
	return nil
}

// hashableIntent strips fields that move with the market (price) so the hash
// reflects user intent and survives small price ticks within the 60-120s
// bucketed window. NOTE: 'deviation' (slippage tolerance) is intent — the
// user picked it, the user must confirm it. Stripping it would let a wider
// deviation slip through on --confirm than was shown in the dry-run.
func hashableIntent(req map[string]any) map[string]any {
	out := make(map[string]any, len(req))
	for k, v := range req {
		switch k {
		case "price":
			// skip — moves with the market between dry-run and confirm
		default:
			out[k] = v
		}
	}
	return out
}

func (w writeCtx) printDryRun(req map[string]any) {
	fmt.Fprintf(w.cmd.OutOrStdout(), "\nDRY-RUN: %s\n", w.opName)
	keys := make([]string, 0, len(req))
	for k := range req {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	tw := tabwriter.NewWriter(w.cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	for _, k := range keys {
		fmt.Fprintf(tw, "%s\t%v\n", k, req[k])
	}
	tw.Flush()
	fmt.Fprintln(w.cmd.OutOrStdout())
}

func (w writeCtx) printOrderResult(res *bridge.OrderSendResult) {
	tw := tabwriter.NewWriter(w.cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "retcode\t%d (%s)\n", res.Retcode, retcodeName(res.Retcode))
	fmt.Fprintf(tw, "comment\t%s\n", res.Comment)
	fmt.Fprintf(tw, "deal\t%d\n", res.Deal)
	fmt.Fprintf(tw, "order\t%d\n", res.Order)
	fmt.Fprintf(tw, "price\t%g\n", res.Price)
	fmt.Fprintf(tw, "volume\t%g\n", res.Volume)
	tw.Flush()
}

// writeAudit funnels every audit-write call so failures surface to stderr
// with a tag that makes them grep-able. We never abort the calling command
// on audit failure (the broker's already executed — the user needs to see
// what happened), but we DO scream.
//
// 'critical' is true for auditConfirmed and auditError — those record post-
// broker-write state and losing them silently is the worst outcome. For
// reject / dryRun the absence of an audit row is also detectable from the
// command exit code and stdout, so the warning is enough.
func (w writeCtx) writeAudit(db *sql.DB, e safety.AuditEntry, critical bool) {
	if err := safety.AppendAudit(w.cmd.Context(), db, store.AuditPath(), e); err != nil {
		prefix := "warn"
		if critical {
			prefix = "CRITICAL"
		}
		fmt.Fprintf(w.cmd.ErrOrStderr(),
			"AUDIT %s: failed to record %s entry: %v\n  (broker action, if any, has ALREADY occurred; reconcile from broker history)\n",
			prefix, w.opName, err)
	}
}

func (w writeCtx) reject(db *sql.DB, accLogin int64, req map[string]any, why error) error {
	buf, _ := json.Marshal(req)
	w.writeAudit(db, safety.AuditEntry{
		Command:      w.opName,
		Request:      buf,
		Confirmed:    false,
		Error:        why.Error(),
		AccountLogin: accLogin,
		Mode:         modeStr(),
	}, false)
	return &ExitErr{Code: ExitSafetyRejected, Err: why}
}

func (w writeCtx) auditDryRun(db *sql.DB, accLogin int64, req map[string]any, hash string) {
	buf, _ := json.Marshal(req)
	w.writeAudit(db, safety.AuditEntry{
		Command:      w.opName,
		Request:      buf,
		Hash:         hash,
		Confirmed:    false,
		AccountLogin: accLogin,
		Mode:         modeStr(),
	}, false)
}

func (w writeCtx) auditConfirmed(db *sql.DB, accLogin int64, req, res any) {
	reqBuf, _ := json.Marshal(req)
	resBuf, _ := json.Marshal(res)
	hashSrc := req
	if m, ok := req.(map[string]any); ok {
		hashSrc = hashableIntent(m)
	}
	w.writeAudit(db, safety.AuditEntry{
		Command:      w.opName,
		Request:      reqBuf,
		Hash:         safety.CurrentHash(hashSrc),
		Confirmed:    true,
		Response:     resBuf,
		AccountLogin: accLogin,
		Mode:         modeStr(),
	}, true)
}

func (w writeCtx) auditError(db *sql.DB, accLogin int64, req map[string]any, err error) {
	buf, _ := json.Marshal(req)
	w.writeAudit(db, safety.AuditEntry{
		Command:      w.opName,
		Request:      buf,
		Hash:         safety.CurrentHash(hashableIntent(req)),
		Confirmed:    true,
		Error:        err.Error(),
		AccountLogin: accLogin,
		Mode:         modeStr(),
	}, true)
}

func modeStr() string {
	if safety.CurrentMode() == safety.ModeLive {
		return "live"
	}
	return "paper"
}

// ── request builders ────────────────────────────────────────────────────────

// buildMarketRequest constructs a TRADE_ACTION_DEAL request for buy/sell.
func buildMarketRequest(symbol string, action int, volume, price, sl, tp float64, magic int64, comment string, deviation int) map[string]any {
	req := map[string]any{
		"action":    1, // TRADE_ACTION_DEAL
		"symbol":    symbol,
		"volume":    volume,
		"type":      action,
		"price":     price,
		"deviation": deviation,
	}
	if sl > 0 {
		req["sl"] = sl
	}
	if tp > 0 {
		req["tp"] = tp
	}
	if magic > 0 {
		req["magic"] = magic
	}
	if comment != "" {
		req["comment"] = comment
	}
	return req
}

// buildCloseRequest constructs the opposite-side TRADE_ACTION_DEAL request
// that closes the position. MT5 needs the position ticket so it can match.
func buildCloseRequest(p *bridge.Position, volume float64) map[string]any {
	// buy position → sell deal, sell position → buy deal
	closeType := 1 // sell
	if p.Type == 1 {
		closeType = 0 // buy
	}
	return map[string]any{
		"action":    1, // TRADE_ACTION_DEAL
		"symbol":    p.Symbol,
		"volume":    volume,
		"type":      closeType,
		"position":  p.Ticket,
		"price":     p.PriceCurrent,
		"deviation": 20,
	}
}

func isOK(retcode int) bool {
	// PLACED / DONE / NO_CHANGES — the broker treats a no-op SLTP modify
	// (price already matches) as 10025 NO_CHANGES, which is success from
	// the user's perspective. Flagging it as broker rejection would noise
	// up the audit log on idempotent modify calls.
	return retcode == 10008 || retcode == 10009 || retcode == 10025
}

func retcodeName(rc int) string {
	switch rc {
	case 10008:
		return "PLACED"
	case 10009:
		return "DONE"
	case 10025:
		return "NO_CHANGES"
	case 10004:
		return "REQUOTE"
	case 10013:
		return "INVALID"
	case 10014:
		return "INVALID_VOLUME"
	case 10015:
		return "INVALID_PRICE"
	case 10016:
		return "INVALID_STOPS"
	case 10017:
		return "TRADE_DISABLED"
	case 10018:
		return "MARKET_CLOSED"
	case 10019:
		return "NO_MONEY"
	case 10021:
		return "PRICE_OFF"
	case 10031:
		return "CONNECTION"
	}
	return fmt.Sprintf("retcode_%d", rc)
}

func liveFlagsHint(tradeMode int) string {
	if tradeMode == 2 {
		return "  --i-understand-this-is-live  (and: $env:MT5_LIVE=\"1\" once per shell)"
	}
	return ""
}

// validateCloseAllFilter is a cheap syntactic guard on the --filter string.
// We can't safely parameterize a SQL fragment (the user is supplying part of
// the WHERE clause), but we can reject the patterns that have no legitimate
// use in a WHERE expression — statement terminators and comments. The driver
// already blocks multi-statement queries; this is defense in depth so the
// dry-run UI can't be silently manipulated either.
//
// Inside a single-quoted string literal a `;` or `--` IS legal SQL — but a
// filter that needs that level of complexity is past what we want to support
// inline. Users with that need can fall back to `mt5-pp-cli sql` with --write.
func validateCloseAllFilter(f string) error {
	if strings.TrimSpace(f) == "" {
		return fmt.Errorf("--filter is empty (use e.g. \"profit < 0\")")
	}
	for _, banned := range []string{";", "--", "/*", "*/"} {
		if strings.Contains(f, banned) {
			return fmt.Errorf("--filter contains %q which is not allowed (no statement separators or comments)", banned)
		}
	}
	return nil
}

// roundLot snaps a volume to the broker's lot step. step is the symbol's
// volume_step from the symbols table; pass 0 for the safe 0.01 fallback
// (only when the symbol isn't in the mirror yet, e.g., user closing a
// position on a symbol they never synced — should not happen in practice
// because SyncPositions is called first).
//
// FX usually quotes 0.01, but XAU/indices/crypto can be 0.1 or 0.0001.
// Hardcoding 0.01 silently rejected partial closes on those.
func roundLot(v float64, step float64) float64 {
	if step <= 0 {
		step = 0.01
	}
	return math.Round(v/step) * step
}

// lookupVolumeStep reads the symbol's volume_step from the local mirror.
// Returns 0 if the symbol isn't there (caller's roundLot will fall back).
// Best-effort — never fails the calling write.
func lookupVolumeStep(ctx context.Context, db *sql.DB, acct int64, symbol string) float64 {
	var step float64
	_ = db.QueryRowContext(ctx,
		`SELECT volume_step FROM symbols WHERE account_login = ? AND symbol = ?`,
		acct, symbol).Scan(&step)
	return step
}

// ── risk preview ─────────────────────────────────────────────────────────────
//
// Joins order_calc_margin + order_calc_profit at ±N pips + current account
// state. Strictly a read; never goes through safety.

// riskProjection / riskPreview are package-level so the closure's type
// assertion matches what we build (anonymous local types make assertions fail).
type riskProjection struct {
	PipsOffset  int     `json:"pips"`
	PriceClose  float64 `json:"price_close"`
	Profit      float64 `json:"profit"`
	EquityAfter float64 `json:"equity_after"`
}

type riskPreview struct {
	Symbol          string           `json:"symbol"`
	Side            string           `json:"side"`
	Volume          float64          `json:"volume"`
	Entry           float64          `json:"entry_price"`
	Digits          int              `json:"digits"`
	PipSize         float64          `json:"pip_size"`
	Margin          float64          `json:"margin"`
	Equity          float64          `json:"equity"`
	MarginFreeAfter float64          `json:"margin_free_after"`
	Currency        string           `json:"currency"`
	Projections     []riskProjection `json:"projections"`
}

func newRiskCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "risk", Short: "Risk preview tools"}

	var (
		symbol, side string
		volume       float64
		pips         []int
	)
	preview := &cobra.Command{
		Use:   "preview",
		Short: "Project margin + P&L at ±N pips for a hypothetical position",
		RunE: func(cmd *cobra.Command, args []string) error {
			if symbol == "" || volume == 0 {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol and --volume are required")}
			}
			action, err := parseOrderSide(side)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			b, err := bridge.New(bridge.Options{Stderr: cmd.ErrOrStderr(), CallTimeout: 15 * time.Second})
			if err != nil {
				return err
			}
			defer b.Close()
			if err := initBridge(cmd, b, 10000); err != nil {
				return err
			}
			info, err := b.SymbolInfo(symbol)
			if err != nil {
				return mapBridgeErr(err)
			}
			acc, err := b.AccountInfo()
			if err != nil {
				return mapBridgeErr(err)
			}
			entry := info.Ask
			if action == 1 {
				entry = info.Bid
			}
			margin, err := b.OrderCalcMargin(action, symbol, volume, entry)
			if err != nil {
				return mapBridgeErr(err)
			}

			pipSize := pointPipSize(info.Digits, info.Point)

			projs := make([]riskProjection, 0, len(pips))
			direction := 1.0
			if action == 1 {
				direction = -1.0
			}
			for _, p := range pips {
				priceClose := entry + direction*float64(p)*pipSize
				profit, err := b.OrderCalcProfit(action, symbol, volume, entry, priceClose)
				if err != nil {
					return mapBridgeErr(err)
				}
				projs = append(projs, riskProjection{
					PipsOffset:  p,
					PriceClose:  priceClose,
					Profit:      profit,
					EquityAfter: acc.Equity + profit,
				})
			}
			out := &riskPreview{
				Symbol: symbol, Side: side, Volume: volume,
				Entry: entry, Digits: info.Digits, PipSize: pipSize,
				Margin: margin, Equity: acc.Equity,
				MarginFreeAfter: acc.MarginFree - margin,
				Currency:        acc.Currency, Projections: projs,
			}
			return emit(cmd, out, printRiskPreview)
		},
	}
	preview.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	preview.Flags().StringVar(&side, "side", "buy", "buy | sell")
	preview.Flags().Float64Var(&volume, "volume", 0, "Lot size (required)")
	preview.Flags().IntSliceVar(&pips, "pips", []int{-100, -50, -25, 25, 50, 100}, "Pip offsets to project P&L at")
	cmd.AddCommand(preview)
	return cmd
}

func printRiskPreview(w io.Writer, v any) {
	r := v.(*riskPreview)
	fmt.Fprintf(w, "%s %s %g lots @ %.*f  pip=%g %s\n",
		r.Symbol, r.Side, r.Volume, r.Digits, r.Entry, r.PipSize, r.Currency)
	fmt.Fprintf(w, "margin:  %.2f %s   (current equity %.2f → margin_free after entry %.2f)\n\n",
		r.Margin, r.Currency, r.Equity, r.MarginFreeAfter)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PIPS\tPRICE\tP&L\tEQUITY_AFTER")
	fmt.Fprintln(tw, "────\t─────\t───\t────────────")
	for _, p := range r.Projections {
		fmt.Fprintf(tw, "%+d\t%.*f\t%+.2f %s\t%.2f %s\n",
			p.PipsOffset, r.Digits, p.PriceClose, p.Profit, r.Currency, p.EquityAfter, r.Currency)
	}
	tw.Flush()
}

// ── small helpers ────────────────────────────────────────────────────────────

// parseOrderSide returns the MT5 ORDER_TYPE int. Phase 3 only needs buy/sell.
func parseOrderSide(side string) (int, error) {
	switch strings.ToLower(side) {
	case "", "buy":
		return 0, nil
	case "sell":
		return 1, nil
	case "buy_limit":
		return 2, nil
	case "sell_limit":
		return 3, nil
	case "buy_stop":
		return 4, nil
	case "sell_stop":
		return 5, nil
	}
	return -1, fmt.Errorf("unknown side %q (want buy/sell/buy_limit/sell_limit/buy_stop/sell_stop)", side)
}

func orderTypeNameInt(t int) string {
	switch t {
	case 0:
		return "buy"
	case 1:
		return "sell"
	case 2:
		return "buy_limit"
	case 3:
		return "sell_limit"
	case 4:
		return "buy_stop"
	case 5:
		return "sell_stop"
	}
	return fmt.Sprintf("type_%d", t)
}

func orderStateNameInt(s int) string {
	switch s {
	case 0:
		return "started"
	case 1:
		return "placed"
	case 2:
		return "canceled"
	case 3:
		return "partial"
	case 4:
		return "filled"
	case 5:
		return "rejected"
	case 6:
		return "expired"
	}
	return fmt.Sprintf("state_%d", s)
}

func bookSideName(t int) string {
	switch t {
	case 1, 3, 5, 7:
		return "sell"
	case 2, 4, 6, 8:
		return "buy"
	}
	return fmt.Sprintf("type_%d", t)
}

// pointPipSize: for 3/5-digit FX (broker fractional pip) the pip is 10×point.
// For 2/4-digit, the pip == point.
func pointPipSize(digits int, point float64) float64 {
	if digits == 3 || digits == 5 {
		return point * 10
	}
	return point
}

// globToLike converts a fnmatch-style glob to a SQL LIKE pattern.
func globToLike(g string) string {
	r := strings.NewReplacer("*", "%", "?", "_")
	return r.Replace(g)
}

// globMatch evaluates a glob against a string using path.Match semantics.
func globMatch(pattern, s string) bool {
	ok, _ := path.Match(pattern, s)
	return ok
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "…"
}

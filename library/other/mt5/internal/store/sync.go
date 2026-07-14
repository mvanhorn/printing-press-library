// Sync orchestration. Reads from the bridge, writes to the store. Idempotent —
// every sync writes through INSERT ... ON CONFLICT DO UPDATE, so re-runs cost
// only the network round-trip plus a small write amplification.
//
// Time fields in the store are unix milliseconds. MT5 hands us unix seconds
// (and msc for deals/ticks); we multiply on the way in.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/bridge"
)

// Counts is what each Sync* returns so the CLI can print a summary.
type Counts struct {
	Inserted int
	Updated  int
}

func (c *Counts) add(o Counts) { c.Inserted += o.Inserted; c.Updated += o.Updated }

// AllOptions configures `mt5-pp-cli sync all`.
type AllOptions struct {
	AccountLogin int64
	Since        time.Time
	OnlySymbols  []string // restrict bars/ticks to these (empty = all)
	IncludeBars  bool
	IncludeTicks bool
	BarsTFs      []string // timeframes to sync when IncludeBars (default: M5,H1,D1)
	Verbose      func(format string, args ...any)
}

// SyncAll runs the whole pipeline: symbols → positions → orders → history orders →
// history deals (since) → optionally bars/ticks per (symbol, tf). Returns a map
// of resource name → counts so the caller can print a summary.
func SyncAll(ctx context.Context, db *sql.DB, b *bridge.Bridge, opts AllOptions) (map[string]Counts, error) {
	if opts.Verbose == nil {
		opts.Verbose = func(string, ...any) {}
	}
	if len(opts.BarsTFs) == 0 {
		opts.BarsTFs = []string{"M5", "H1", "D1"}
	}

	results := map[string]Counts{}

	opts.Verbose("syncing symbols...")
	c, err := SyncSymbols(ctx, db, b, opts.AccountLogin, "")
	if err != nil {
		return results, fmt.Errorf("symbols: %w", err)
	}
	results["symbols"] = c

	opts.Verbose("syncing positions...")
	c, err = SyncPositions(ctx, db, b, opts.AccountLogin)
	if err != nil {
		return results, fmt.Errorf("positions: %w", err)
	}
	results["positions"] = c

	opts.Verbose("syncing active orders...")
	c, err = SyncOrders(ctx, db, b, opts.AccountLogin)
	if err != nil {
		return results, fmt.Errorf("orders: %w", err)
	}
	results["orders"] = c

	now := time.Now()
	opts.Verbose("syncing history orders since %s...", opts.Since.Format(time.RFC3339))
	c, err = SyncHistoryOrders(ctx, db, b, opts.AccountLogin, opts.Since, now)
	if err != nil {
		return results, fmt.Errorf("history_orders: %w", err)
	}
	results["history_orders"] = c

	opts.Verbose("syncing deals since %s...", opts.Since.Format(time.RFC3339))
	c, err = SyncDeals(ctx, db, b, opts.AccountLogin, opts.Since, now)
	if err != nil {
		return results, fmt.Errorf("deals: %w", err)
	}
	results["deals"] = c

	// Bars and ticks share the same symbol resolution: an explicit --symbols
	// list wins; otherwise fall back to every symbol mirrored for the account,
	// so `sync all --with-ticks` without a filter copies ticks instead of
	// silently iterating an empty slice.
	var symbols []string
	if opts.IncludeBars || opts.IncludeTicks {
		symbols = opts.OnlySymbols
		if len(symbols) == 0 {
			rows, err := db.QueryContext(ctx, "SELECT symbol FROM symbols WHERE account_login=?", opts.AccountLogin)
			if err != nil {
				return results, fmt.Errorf("query symbols for bars/ticks: %w", err)
			}
			for rows.Next() {
				var s string
				if err := rows.Scan(&s); err != nil {
					rows.Close()
					return results, err
				}
				symbols = append(symbols, s)
			}
			rows.Close()
		}
	}

	if opts.IncludeBars {
		for _, sym := range symbols {
			for _, tf := range opts.BarsTFs {
				opts.Verbose("syncing bars %s %s...", sym, tf)
				c, err := SyncBars(ctx, db, b, opts.AccountLogin, sym, tf, opts.Since, now)
				if err != nil {
					return results, fmt.Errorf("bars %s/%s: %w", sym, tf, err)
				}
				results[fmt.Sprintf("bars_%s_%s", sym, tf)] = c
			}
		}
	}

	if opts.IncludeTicks {
		for _, sym := range symbols {
			opts.Verbose("syncing ticks %s...", sym)
			c, err := SyncTicks(ctx, db, b, opts.AccountLogin, sym, opts.Since, now)
			if err != nil {
				return results, fmt.Errorf("ticks %s: %w", sym, err)
			}
			results["ticks_"+sym] = c
		}
	}

	return results, nil
}

// ── symbols ─────────────────────────────────────────────────────────────────

func SyncSymbols(ctx context.Context, db *sql.DB, b *bridge.Bridge, accountLogin int64, group string) (Counts, error) {
	items, err := b.SymbolsGet(group)
	if err != nil {
		return Counts{}, err
	}
	const q = `INSERT INTO symbols (
		account_login, symbol, description, digits, point, spread,
		contract_size, volume_min, volume_max, volume_step,
		trade_mode, trade_calc, base_currency, profit_currency,
		margin_initial, last_synced
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(account_login, symbol) DO UPDATE SET
		description=excluded.description, digits=excluded.digits, point=excluded.point,
		spread=excluded.spread, contract_size=excluded.contract_size,
		volume_min=excluded.volume_min, volume_max=excluded.volume_max,
		volume_step=excluded.volume_step,
		trade_mode=excluded.trade_mode, trade_calc=excluded.trade_calc,
		base_currency=excluded.base_currency, profit_currency=excluded.profit_currency,
		margin_initial=excluded.margin_initial, last_synced=excluded.last_synced`
	return bulkExec(ctx, db, q, len(items), func(stmt *sql.Stmt, i int) (sql.Result, error) {
		s := items[i]
		return stmt.ExecContext(ctx,
			accountLogin, s.Name, s.Description, s.Digits, s.Point, s.Spread,
			s.TradeContractSize, s.VolumeMin, s.VolumeMax, s.VolumeStep,
			s.TradeMode, s.TradeCalcMode, s.CurrencyBase, s.CurrencyProfit,
			s.MarginInitial, time.Now().UnixMilli())
	})
}

// ── positions / orders ──────────────────────────────────────────────────────

func SyncPositions(ctx context.Context, db *sql.DB, b *bridge.Bridge, accountLogin int64) (Counts, error) {
	items, err := b.PositionsGet(nil)
	if err != nil {
		return Counts{}, err
	}
	// Snapshot semantics: clear this account's positions and re-insert.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Counts{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM positions WHERE account_login=?", accountLogin); err != nil {
		return Counts{}, err
	}
	for _, p := range items {
		typ := "buy"
		if p.Type == 1 {
			typ = "sell"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO positions(
			account_login, ticket, symbol, type, volume,
			price_open, price_current, sl, tp, profit, swap,
			magic, comment, time_open_ms, time_update_ms
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			accountLogin, p.Ticket, p.Symbol, typ, p.Volume,
			p.PriceOpen, p.PriceCurrent, p.SL, p.TP, p.Profit, p.Swap,
			p.Magic, p.Comment, p.Time*1000, p.TimeUpdate*1000); err != nil {
			return Counts{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Counts{}, err
	}
	return Counts{Inserted: len(items)}, nil
}

func SyncOrders(ctx context.Context, db *sql.DB, b *bridge.Bridge, accountLogin int64) (Counts, error) {
	items, err := b.OrdersGet(nil)
	if err != nil {
		return Counts{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Counts{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM orders WHERE account_login=?", accountLogin); err != nil {
		return Counts{}, err
	}
	for _, o := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO orders(
			account_login, ticket, symbol, type, volume_initial, volume_current,
			price_open, sl, tp, time_setup_ms, time_expiration_ms,
			state, magic, comment
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			accountLogin, o.Ticket, o.Symbol, orderTypeName(o.Type),
			o.VolumeInitial, o.VolumeCurrent, o.PriceOpen, o.SL, o.TP,
			o.TimeSetup*1000, o.TimeExpiration*1000,
			orderStateName(o.State), o.Magic, o.Comment); err != nil {
			return Counts{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Counts{}, err
	}
	return Counts{Inserted: len(items)}, nil
}

// ── history orders / deals ──────────────────────────────────────────────────

func SyncHistoryOrders(ctx context.Context, db *sql.DB, b *bridge.Bridge, accountLogin int64, from, to time.Time) (Counts, error) {
	items, err := b.HistoryOrdersGet(from.Unix(), to.Unix())
	if err != nil {
		return Counts{}, err
	}
	const q = `INSERT INTO history_orders(
		account_login, ticket, symbol, type, state,
		volume_initial, volume_current, price_open, sl, tp,
		time_setup_ms, time_done_ms, magic, comment
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(account_login, ticket) DO UPDATE SET
		state=excluded.state, time_done_ms=excluded.time_done_ms,
		volume_current=excluded.volume_current`
	return bulkExec(ctx, db, q, len(items), func(stmt *sql.Stmt, i int) (sql.Result, error) {
		o := items[i]
		return stmt.ExecContext(ctx,
			accountLogin, o.Ticket, o.Symbol, orderTypeName(o.Type), orderStateName(o.State),
			o.VolumeInitial, o.VolumeCurrent, o.PriceOpen, o.SL, o.TP,
			o.TimeSetup*1000, o.TimeDone*1000, o.Magic, o.Comment)
	})
}

func SyncDeals(ctx context.Context, db *sql.DB, b *bridge.Bridge, accountLogin int64, from, to time.Time) (Counts, error) {
	items, err := b.HistoryDealsGet(from.Unix(), to.Unix())
	if err != nil {
		return Counts{}, err
	}
	const q = `INSERT INTO deals(
		account_login, ticket, order_ticket, position_id, symbol, type, entry,
		volume, price, commission, swap, profit, fee, time_ms, magic, comment
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(account_login, ticket) DO UPDATE SET
		profit=excluded.profit, commission=excluded.commission, swap=excluded.swap,
		fee=excluded.fee`
	return bulkExec(ctx, db, q, len(items), func(stmt *sql.Stmt, i int) (sql.Result, error) {
		d := items[i]
		tms := d.TimeMSC
		if tms == 0 {
			tms = d.Time * 1000
		}
		return stmt.ExecContext(ctx,
			accountLogin, d.Ticket, d.Order, d.PositionID, d.Symbol,
			dealTypeName(d.Type), dealEntryName(d.Entry),
			d.Volume, d.Price, d.Commission, d.Swap, d.Profit, d.Fee,
			tms, d.Magic, d.Comment)
	})
}

// ── bars / ticks ────────────────────────────────────────────────────────────

// allowedTFs gates which timeframe strings we accept against the per-tf tables
// in the schema. Keeps a typo from creating a table named bars_FOO at runtime.
var allowedTFs = map[string]bool{
	"M1": true, "M5": true, "M15": true, "M30": true,
	"H1": true, "H4": true, "D1": true, "W1": true, "MN1": true,
}

func SyncBars(ctx context.Context, db *sql.DB, b *bridge.Bridge, accountLogin int64, symbol, tf string, from, to time.Time) (Counts, error) {
	tf = strings.ToUpper(tf)
	if !allowedTFs[tf] {
		return Counts{}, fmt.Errorf("timeframe %q not in store schema (allowed: M1,M5,M15,M30,H1,H4,D1,W1,MN1)", tf)
	}
	bars, err := b.CopyRatesRange(symbol, tf, from.Unix(), to.Unix())
	if err != nil {
		return Counts{}, err
	}
	table := "bars_" + tf
	q := fmt.Sprintf(`INSERT INTO %s(
		account_login, symbol, time_ms, o, h, l, c,
		tick_volume, spread, real_volume
	) VALUES (?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(account_login, symbol, time_ms) DO UPDATE SET
		o=excluded.o, h=excluded.h, l=excluded.l, c=excluded.c,
		tick_volume=excluded.tick_volume, spread=excluded.spread,
		real_volume=excluded.real_volume`, table)
	return bulkExec(ctx, db, q, len(bars), func(stmt *sql.Stmt, i int) (sql.Result, error) {
		bar := bars[i]
		return stmt.ExecContext(ctx,
			accountLogin, symbol, bar.Time*1000,
			bar.Open, bar.High, bar.Low, bar.Close,
			bar.TickVolume, bar.Spread, bar.RealVolume)
	})
}

func SyncTicks(ctx context.Context, db *sql.DB, b *bridge.Bridge, accountLogin int64, symbol string, from, to time.Time) (Counts, error) {
	ticks, err := b.CopyTicksRange(symbol, from.Unix(), to.Unix(), "all")
	if err != nil {
		return Counts{}, err
	}
	const q = `INSERT INTO ticks(
		account_login, symbol, time_ms, bid, ask, last, volume_real, flags
	) VALUES (?,?,?,?,?,?,?,?)
	ON CONFLICT(account_login, symbol, time_ms) DO UPDATE SET
		bid=excluded.bid, ask=excluded.ask, last=excluded.last,
		volume_real=excluded.volume_real, flags=excluded.flags`
	return bulkExec(ctx, db, q, len(ticks), func(stmt *sql.Stmt, i int) (sql.Result, error) {
		t := ticks[i]
		tms := t.TimeMSC
		if tms == 0 {
			tms = t.Time * 1000
		}
		return stmt.ExecContext(ctx,
			accountLogin, symbol, tms, t.Bid, t.Ask, t.Last, t.VolumeReal, t.Flags)
	})
}

// ── helpers ─────────────────────────────────────────────────────────────────

// bulkExec wraps a prepared INSERT loop in one transaction for speed. n is the
// total row count, fill builds each ExecContext. Returns total rows affected
// as Inserted (SQLite's RowsAffected counts updates the same; the distinction
// only matters for reporting and isn't worth a second query here).
func bulkExec(ctx context.Context, db *sql.DB, q string, n int, fill func(*sql.Stmt, int) (sql.Result, error)) (Counts, error) {
	if n == 0 {
		return Counts{}, nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return Counts{}, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return Counts{}, fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()
	for i := 0; i < n; i++ {
		if _, err := fill(stmt, i); err != nil {
			return Counts{}, fmt.Errorf("exec row %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Counts{}, err
	}
	return Counts{Inserted: n}, nil
}

// MT5 numeric enums → readable strings (kept consistent with anything we may
// want a SQL WHERE to grep on later).
func orderTypeName(t int) string {
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
	case 6:
		return "buy_stop_limit"
	case 7:
		return "sell_stop_limit"
	case 8:
		return "close_by"
	default:
		return fmt.Sprintf("type_%d", t)
	}
}

func orderStateName(s int) string {
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
	default:
		return fmt.Sprintf("state_%d", s)
	}
}

func dealTypeName(t int) string {
	switch t {
	case 0:
		return "buy"
	case 1:
		return "sell"
	case 2:
		return "balance"
	case 3:
		return "credit"
	case 4:
		return "charge"
	case 5:
		return "correction"
	case 6:
		return "bonus"
	case 7:
		return "commission"
	default:
		return fmt.Sprintf("type_%d", t)
	}
}

func dealEntryName(e int) string {
	switch e {
	case 0:
		return "in"
	case 1:
		return "out"
	case 2:
		return "inout"
	case 3:
		return "out_by"
	default:
		return fmt.Sprintf("entry_%d", e)
	}
}

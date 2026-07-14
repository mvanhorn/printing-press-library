package cli

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/store"
)

// Allowed timeframes mirror the bars_<TF> tables created by migration 0001.
var quantAllowedTFs = map[string]bool{
	"M1": true, "M5": true, "M15": true, "M30": true,
	"H1": true, "H4": true, "D1": true, "W1": true, "MN1": true,
}

// timeframeMillis is used by the replay clock and backtest stepping.
var timeframeMillis = map[string]int64{
	"M1":  60_000,
	"M5":  5 * 60_000,
	"M15": 15 * 60_000,
	"M30": 30 * 60_000,
	"H1":  60 * 60_000,
	"H4":  4 * 60 * 60_000,
	"D1":  24 * 60 * 60_000,
	"W1":  7 * 24 * 60 * 60_000,
	"MN1": 30 * 24 * 60 * 60_000,
}

// ── bars / ticks copy + export ───────────────────────────────────────────────

func newBarsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "bars", Short: "Bar (OHLCV) operations"}
	cmd.AddCommand(newBarsCopyCmd())
	cmd.AddCommand(newBarsExportCmd())
	return cmd
}

func newBarsCopyCmd() *cobra.Command {
	var symbol, tf, from, to, out string
	cmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy bars from local mirror to csv/jsonl/stdout",
		Long: `Read bars from bars_<TF> in the local mirror and write them to a file or stdout.

--out forms:
  csv:path/to/file.csv      → CSV with header (time_ms,iso_time,o,h,l,c,tick_volume,spread,real_volume)
  jsonl:path/to/file.jsonl  → one JSON object per line
  -                         → JSONL to stdout

Parquet output is not implemented in v1 — use csv or jsonl, or convert
downstream with pandas / pyarrow.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if symbol == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol is required")}
			}
			tf = strings.ToUpper(tf)
			if !quantAllowedTFs[tf] {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--tf %q not in {M1 M5 M15 M30 H1 H4 D1 W1 MN1}", tf)}
			}
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

			rows, err := db.QueryContext(cmd.Context(), fmt.Sprintf(
				`SELECT time_ms, o, h, l, c, COALESCE(tick_volume,0), COALESCE(spread,0), COALESCE(real_volume,0)
				 FROM bars_%s WHERE account_login = ? AND symbol = ? AND time_ms BETWEEN ? AND ?
				 ORDER BY time_ms ASC`, tf), acct, symbol, fromT.UnixMilli(), toT.UnixMilli())
			if err != nil {
				return err
			}
			defer rows.Close()

			w, closer, format, err := openSink(out)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			defer closer()

			n, err := writeBars(w, format, rows)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d bar%s (%s %s) → %s\n",
				n, pluralS(n), symbol, tf, sinkDest(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	cmd.Flags().StringVar(&tf, "tf", "H1", "Timeframe: M1..MN1")
	cmd.Flags().StringVar(&from, "from", "30d", "ISO date or relative")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date or relative")
	cmd.Flags().StringVar(&out, "out", "-", "Output: csv:path | jsonl:path | -")
	return cmd
}

func newBarsExportCmd() *cobra.Command {
	var tf, symbols, since, format, outDir string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Bulk export many symbols at one timeframe into a directory",
		Long: `Export one file per symbol matching --symbols (comma-separated globs) at --tf.

  mt5-pp-cli bars export --tf H1 --symbols "EUR*,XAU*" --since 90d --format csv --out-dir ./exports`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tf = strings.ToUpper(tf)
			if !quantAllowedTFs[tf] {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--tf %q not allowed", tf)}
			}
			if format == "parquet" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("parquet not yet implemented; use --format csv|jsonl")}
			}
			if format != "csv" && format != "jsonl" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--format must be csv or jsonl")}
			}
			fromT, err := parseSince(since)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
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

			// Resolve symbols from the symbols table using the globs.
			resolved, err := resolveSymbolsByGlob(cmd.Context(), db, acct, symbols)
			if err != nil {
				return err
			}
			if len(resolved) == 0 {
				return &ExitErr{Code: ExitNotFound, Err: fmt.Errorf("no symbols matched %q in the local mirror", symbols)}
			}

			type fileReport struct {
				Symbol string `json:"symbol"`
				Rows   int    `json:"rows"`
				Path   string `json:"path"`
			}
			reports := make([]fileReport, 0, len(resolved))
			toMS := time.Now().UnixMilli()
			for _, sym := range resolved {
				fname := fmt.Sprintf("%s_%s.%s", sanitizeFilename(sym), tf, format)
				path := filepath.Join(outDir, fname)
				f, err := os.Create(path)
				if err != nil {
					return err
				}
				rows, err := db.QueryContext(cmd.Context(), fmt.Sprintf(
					`SELECT time_ms, o, h, l, c, COALESCE(tick_volume,0), COALESCE(spread,0), COALESCE(real_volume,0)
					 FROM bars_%s WHERE account_login = ? AND symbol = ? AND time_ms BETWEEN ? AND ?
					 ORDER BY time_ms ASC`, tf), acct, sym, fromT.UnixMilli(), toMS)
				if err != nil {
					f.Close()
					return err
				}
				bw := bufio.NewWriter(f)
				n, err := writeBars(bw, format, rows)
				rows.Close()
				bw.Flush()
				f.Close()
				if err != nil {
					return err
				}
				reports = append(reports, fileReport{Symbol: sym, Rows: n, Path: path})
			}

			return emit(cmd, reports, func(w io.Writer, v any) {
				rs := v.([]fileReport)
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "SYMBOL\tROWS\tPATH")
				fmt.Fprintln(tw, "──────\t────\t────")
				total := 0
				for _, r := range rs {
					total += r.Rows
					fmt.Fprintf(tw, "%s\t%d\t%s\n", r.Symbol, r.Rows, r.Path)
				}
				tw.Flush()
				fmt.Fprintf(w, "\nexported %d file%s, %d total bar%s\n",
					len(rs), pluralS(len(rs)), total, pluralS(total))
			})
		},
	}
	cmd.Flags().StringVar(&tf, "tf", "H1", "Timeframe")
	cmd.Flags().StringVar(&symbols, "symbols", "*", "Comma-separated globs (e.g. 'EUR*,XAU*')")
	cmd.Flags().StringVar(&since, "since", "1y", "Lower bound")
	cmd.Flags().StringVar(&format, "format", "csv", "csv | jsonl (parquet not yet supported)")
	cmd.Flags().StringVar(&outDir, "out-dir", "./exports", "Output directory")
	return cmd
}

func newTicksCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "ticks", Short: "Tick operations"}
	cmd.AddCommand(newTicksCopyCmd())
	return cmd
}

func newTicksCopyCmd() *cobra.Command {
	var symbol, from, to, out string
	cmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy ticks from local mirror to csv/jsonl/stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			if symbol == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol is required")}
			}
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

			rows, err := db.QueryContext(cmd.Context(),
				`SELECT time_ms, COALESCE(bid,0), COALESCE(ask,0), COALESCE(last,0),
				        COALESCE(volume_real,0), COALESCE(flags,0)
				 FROM ticks WHERE account_login = ? AND symbol = ? AND time_ms BETWEEN ? AND ?
				 ORDER BY time_ms ASC`, acct, symbol, fromT.UnixMilli(), toT.UnixMilli())
			if err != nil {
				return err
			}
			defer rows.Close()

			w, closer, format, err := openSink(out)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			defer closer()

			n, err := writeTicks(w, format, rows)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d tick%s (%s) → %s\n",
				n, pluralS(n), symbol, sinkDest(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	cmd.Flags().StringVar(&from, "from", "1d", "ISO date/time or relative")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date/time or relative")
	cmd.Flags().StringVar(&out, "out", "-", "Output: csv:path | jsonl:path | -")
	return cmd
}

// openSink parses an --out spec and returns a writer, a closer, and the format.
// Forms:
//
//	"-"                 → stdout, jsonl
//	"jsonl:path"        → file, jsonl
//	"csv:path"          → file, csv
//	"parquet:path"      → error (not implemented)
func openSink(out string) (io.Writer, func(), string, error) {
	if out == "" || out == "-" {
		bw := bufio.NewWriter(os.Stdout)
		return bw, func() { bw.Flush() }, "jsonl", nil
	}
	parts := strings.SplitN(out, ":", 2)
	if len(parts) != 2 {
		return nil, nil, "", fmt.Errorf("--out must be 'csv:path', 'jsonl:path', 'parquet:path', or '-'")
	}
	format, path := strings.ToLower(parts[0]), parts[1]
	switch format {
	case "csv", "jsonl":
		f, err := os.Create(path)
		if err != nil {
			return nil, nil, "", err
		}
		bw := bufio.NewWriter(f)
		return bw, func() { bw.Flush(); f.Close() }, format, nil
	case "parquet":
		return nil, nil, "", fmt.Errorf("parquet output not yet implemented; use csv or jsonl")
	default:
		return nil, nil, "", fmt.Errorf("unknown format %q (use csv, jsonl, or parquet)", format)
	}
}

func sinkDest(out string) string {
	if out == "" || out == "-" {
		return "stdout (jsonl)"
	}
	return out
}

// writeBars writes rows out as either csv or jsonl. Caller closes rows.
func writeBars(w io.Writer, format string, rows *sql.Rows) (int, error) {
	switch format {
	case "csv":
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"time_ms", "iso_time", "o", "h", "l", "c", "tick_volume", "spread", "real_volume"})
		n := 0
		for rows.Next() {
			var t int64
			var o, h, l, c float64
			var tv, sp, rv int64
			if err := rows.Scan(&t, &o, &h, &l, &c, &tv, &sp, &rv); err != nil {
				return n, err
			}
			_ = cw.Write([]string{
				strconv.FormatInt(t, 10),
				time.UnixMilli(t).UTC().Format(time.RFC3339),
				strconv.FormatFloat(o, 'f', -1, 64),
				strconv.FormatFloat(h, 'f', -1, 64),
				strconv.FormatFloat(l, 'f', -1, 64),
				strconv.FormatFloat(c, 'f', -1, 64),
				strconv.FormatInt(tv, 10),
				strconv.FormatInt(sp, 10),
				strconv.FormatInt(rv, 10),
			})
			n++
		}
		cw.Flush()
		return n, cw.Error()
	case "jsonl":
		enc := json.NewEncoder(w)
		n := 0
		for rows.Next() {
			var t int64
			var o, h, l, c float64
			var tv, sp, rv int64
			if err := rows.Scan(&t, &o, &h, &l, &c, &tv, &sp, &rv); err != nil {
				return n, err
			}
			if err := enc.Encode(map[string]any{
				"time_ms": t, "iso_time": time.UnixMilli(t).UTC().Format(time.RFC3339),
				"o": o, "h": h, "l": l, "c": c,
				"tick_volume": tv, "spread": sp, "real_volume": rv,
			}); err != nil {
				return n, err
			}
			n++
		}
		return n, nil
	}
	return 0, fmt.Errorf("unknown format %q", format)
}

func writeTicks(w io.Writer, format string, rows *sql.Rows) (int, error) {
	switch format {
	case "csv":
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"time_ms", "iso_time", "bid", "ask", "last", "volume_real", "flags"})
		n := 0
		for rows.Next() {
			var t int64
			var bid, ask, last, volReal float64
			var flags int64
			if err := rows.Scan(&t, &bid, &ask, &last, &volReal, &flags); err != nil {
				return n, err
			}
			_ = cw.Write([]string{
				strconv.FormatInt(t, 10),
				time.UnixMilli(t).UTC().Format(time.RFC3339),
				strconv.FormatFloat(bid, 'f', -1, 64),
				strconv.FormatFloat(ask, 'f', -1, 64),
				strconv.FormatFloat(last, 'f', -1, 64),
				strconv.FormatFloat(volReal, 'f', -1, 64),
				strconv.FormatInt(flags, 10),
			})
			n++
		}
		cw.Flush()
		return n, cw.Error()
	case "jsonl":
		enc := json.NewEncoder(w)
		n := 0
		for rows.Next() {
			var t int64
			var bid, ask, last, volReal float64
			var flags int64
			if err := rows.Scan(&t, &bid, &ask, &last, &volReal, &flags); err != nil {
				return n, err
			}
			if err := enc.Encode(map[string]any{
				"time_ms": t, "iso_time": time.UnixMilli(t).UTC().Format(time.RFC3339),
				"bid": bid, "ask": ask, "last": last,
				"volume_real": volReal, "flags": flags,
			}); err != nil {
				return n, err
			}
			n++
		}
		return n, nil
	}
	return 0, fmt.Errorf("unknown format %q", format)
}

// resolveSymbolsByGlob expands a comma-separated glob list against the symbols
// table. "*" means all symbols. Globs use simple star semantics.
func resolveSymbolsByGlob(ctx context.Context, db *sql.DB, acct int64, globs string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT symbol FROM symbols WHERE account_login = ? ORDER BY symbol`, acct)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		all = append(all, s)
	}
	patterns := strings.Split(globs, ",")
	for i := range patterns {
		patterns[i] = strings.TrimSpace(patterns[i])
	}
	var out []string
	for _, s := range all {
		for _, p := range patterns {
			if matchGlob(p, s) {
				out = append(out, s)
				break
			}
		}
	}
	return out, nil
}

// matchGlob is a tiny shell-glob (supports leading/trailing/middle *), enough
// for "EUR*", "*USD*", "XAU*", "*".
func matchGlob(pattern, s string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, p := range parts {
		if p == "" {
			continue
		}
		idx := strings.Index(s[pos:], p)
		if idx < 0 {
			return false
		}
		// Anchor first non-empty part to start when pattern doesn't begin with *.
		if i == 0 && !strings.HasPrefix(pattern, "*") && idx != 0 {
			return false
		}
		pos += idx + len(p)
	}
	// Anchor last non-empty part to end when pattern doesn't end with *.
	if !strings.HasSuffix(pattern, "*") {
		last := parts[len(parts)-1]
		if last != "" && !strings.HasSuffix(s, last) {
			return false
		}
	}
	return true
}

func sanitizeFilename(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return r.Replace(s)
}

// ── features build ──────────────────────────────────────────────────────────

func newFeaturesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "features", Short: "Derived feature engineering on the local mirror"}
	cmd.AddCommand(newFeaturesBuildCmd())
	return cmd
}

func newFeaturesBuildCmd() *cobra.Command {
	var symbol, tf, from, to string
	var atrPeriod, rsiPeriod, rvWindow int
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compute returns/log-returns/ATR(14)/RSI(14)/realized-vol(20) into the features table",
		Long: `Walk bars_<TF> for --symbol and compute:
  - ret           simple close-to-close return
  - log_ret       natural log of (c_t / c_{t-1})
  - atr_14        Wilder ATR over --atr bars
  - rsi_14        Wilder RSI over --rsi bars
  - realized_vol  stdev of log returns over --rv-window bars (sample stdev; not annualized)

Upserts into the 'features' table keyed by (account_login, symbol, tf, time_ms).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if symbol == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol is required")}
			}
			tf = strings.ToUpper(tf)
			if !quantAllowedTFs[tf] {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--tf %q not allowed", tf)}
			}
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

			n, err := buildFeatures(cmd.Context(), db, acct, symbol, tf,
				fromT.UnixMilli(), toT.UnixMilli(), atrPeriod, rsiPeriod, rvWindow)
			if err != nil {
				return err
			}
			return emit(cmd, map[string]any{
				"symbol": symbol, "tf": tf, "rows_written": n,
				"atr_period": atrPeriod, "rsi_period": rsiPeriod, "rv_window": rvWindow,
			}, func(w io.Writer, v any) {
				m := v.(map[string]any)
				fmt.Fprintf(w, "features built: %s %s — %d row(s)\n  atr=%d rsi=%d rv=%d\n",
					m["symbol"], m["tf"], m["rows_written"],
					m["atr_period"], m["rsi_period"], m["rv_window"])
			})
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	cmd.Flags().StringVar(&tf, "tf", "H1", "Timeframe")
	cmd.Flags().StringVar(&from, "from", "1y", "ISO date or relative")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date or relative")
	cmd.Flags().IntVar(&atrPeriod, "atr", 14, "ATR period (Wilder)")
	cmd.Flags().IntVar(&rsiPeriod, "rsi", 14, "RSI period (Wilder)")
	cmd.Flags().IntVar(&rvWindow, "rv-window", 20, "Realized-vol rolling window on log returns")
	return cmd
}

func buildFeatures(ctx context.Context, db *sql.DB, acct int64, symbol, tf string, fromMS, toMS int64,
	atrPeriod, rsiPeriod, rvWindow int) (int, error) {

	q := fmt.Sprintf(`SELECT time_ms, h, l, c FROM bars_%s
		WHERE account_login = ? AND symbol = ? AND time_ms BETWEEN ? AND ? ORDER BY time_ms ASC`, tf)
	rows, err := db.QueryContext(ctx, q, acct, symbol, fromMS, toMS)
	if err != nil {
		return 0, err
	}
	var (
		times  []int64
		highs  []float64
		lows   []float64
		closes []float64
	)
	for rows.Next() {
		var t int64
		var h, l, c float64
		if err := rows.Scan(&t, &h, &l, &c); err != nil {
			rows.Close()
			return 0, err
		}
		times = append(times, t)
		highs = append(highs, h)
		lows = append(lows, l)
		closes = append(closes, c)
	}
	rows.Close()
	if len(times) < 2 {
		return 0, fmt.Errorf("only %d bar(s) for %s %s in window — sync more first", len(times), symbol, tf)
	}

	n := len(times)
	rets := make([]float64, n)
	logRets := make([]float64, n)
	for i := 1; i < n; i++ {
		if closes[i-1] != 0 {
			rets[i] = closes[i]/closes[i-1] - 1.0
			logRets[i] = math.Log(closes[i] / closes[i-1])
		}
	}

	atr := wilderATR(highs, lows, closes, atrPeriod)
	rsi := wilderRSI(closes, rsiPeriod)
	rv := rollingStd(logRets, rvWindow)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO features(account_login, symbol, tf, time_ms, ret, log_ret, atr_14, rsi_14, realized_vol_20)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(account_login, symbol, tf, time_ms) DO UPDATE SET
		  ret=excluded.ret, log_ret=excluded.log_ret,
		  atr_14=excluded.atr_14, rsi_14=excluded.rsi_14,
		  realized_vol_20=excluded.realized_vol_20`)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	written := 0
	for i := 0; i < n; i++ {
		if _, err := stmt.ExecContext(ctx,
			acct, symbol, tf, times[i],
			nullIfNaN(rets[i]), nullIfNaN(logRets[i]),
			nullIfNaN(atr[i]), nullIfNaN(rsi[i]), nullIfNaN(rv[i]),
		); err != nil {
			_ = tx.Rollback()
			return written, err
		}
		written++
	}
	if err := tx.Commit(); err != nil {
		return written, err
	}
	return written, nil
}

// wilderATR: ATR using Wilder's smoothing. First period-1 values are NaN; the
// period-th is a simple average of TR; subsequent are smoothed.
func wilderATR(h, l, c []float64, period int) []float64 {
	n := len(c)
	atr := make([]float64, n)
	for i := range atr {
		atr[i] = math.NaN()
	}
	if n < period+1 || period < 1 {
		return atr
	}
	tr := make([]float64, n)
	for i := 1; i < n; i++ {
		tr[i] = math.Max(h[i]-l[i], math.Max(math.Abs(h[i]-c[i-1]), math.Abs(l[i]-c[i-1])))
	}
	sum := 0.0
	for i := 1; i <= period; i++ {
		sum += tr[i]
	}
	atr[period] = sum / float64(period)
	for i := period + 1; i < n; i++ {
		atr[i] = (atr[i-1]*float64(period-1) + tr[i]) / float64(period)
	}
	return atr
}

// wilderRSI: classic Wilder RSI on closes. First period values NaN.
func wilderRSI(c []float64, period int) []float64 {
	n := len(c)
	rsi := make([]float64, n)
	for i := range rsi {
		rsi[i] = math.NaN()
	}
	if n < period+1 || period < 1 {
		return rsi
	}
	gains, losses := 0.0, 0.0
	for i := 1; i <= period; i++ {
		d := c[i] - c[i-1]
		if d > 0 {
			gains += d
		} else {
			losses -= d
		}
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	rsi[period] = rsiFrom(avgGain, avgLoss)
	for i := period + 1; i < n; i++ {
		d := c[i] - c[i-1]
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		avgGain = (avgGain*float64(period-1) + g) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + l) / float64(period)
		rsi[i] = rsiFrom(avgGain, avgLoss)
	}
	return rsi
}

func rsiFrom(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50
		}
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// rollingStd: sample stdev of values over a rolling window. Values < window
// return NaN. Skips NaNs in input (treats them as 0 for the window count, which
// is fine for the index-1 boundary).
func rollingStd(vals []float64, window int) []float64 {
	n := len(vals)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if window < 2 || n < window {
		return out
	}
	for i := window - 1; i < n; i++ {
		var sum, sumSq float64
		cnt := 0
		for j := i - window + 1; j <= i; j++ {
			v := vals[j]
			if math.IsNaN(v) {
				continue
			}
			sum += v
			sumSq += v * v
			cnt++
		}
		if cnt < 2 {
			continue
		}
		mean := sum / float64(cnt)
		variance := (sumSq - float64(cnt)*mean*mean) / float64(cnt-1)
		if variance < 0 {
			variance = 0
		}
		out[i] = math.Sqrt(variance)
	}
	return out
}

func nullIfNaN(f float64) any {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	return f
}

// ── calendar ─────────────────────────────────────────────────────────────────

func newCalendarCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "calendar", Short: "Economic calendar (synced into local mirror)"}
	cmd.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Sync economic events (not provided by MT5 Python bridge — see help)",
		Long: `The MetaTrader 5 Python package does not expose an economic-calendar API.
Populate the calendar_events table yourself via:

  mt5-pp-cli sql --write "INSERT INTO calendar_events(time_ms, currency, importance, event_name, forecast, previous) VALUES (...)"

or import a CSV from a third-party source. 'mt5-pp-cli calendar near' will read
whatever is in the table.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return &ExitErr{Code: ExitConfig, Err: fmt.Errorf(
				"calendar sync not available: MT5 Python bridge has no calendar endpoint. " +
					"Import events via 'mt5-pp-cli sql --write' or a CSV loader. See 'mt5-pp-cli calendar sync --help'.")}
		},
	})
	cmd.AddCommand(newCalendarNearCmd())
	return cmd
}

func newCalendarNearCmd() *cobra.Command {
	var eventName, window, at string
	cmd := &cobra.Command{
		Use:   "near",
		Short: "Print events near the current time (or --at) from calendar_events",
		RunE: func(cmd *cobra.Command, args []string) error {
			anchor, _, err := parseRangeOne(at, time.Now())
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			d, err := parseRelativeDuration(window)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--window: %w", err)}
			}
			lo := anchor.Add(-d).UnixMilli()
			hi := anchor.Add(d).UnixMilli()

			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()

			q := `SELECT id, time_ms, COALESCE(currency,''), COALESCE(importance,0),
			             COALESCE(event_name,''), COALESCE(actual,0), COALESCE(forecast,0), COALESCE(previous,0)
			      FROM calendar_events
			      WHERE time_ms BETWEEN ? AND ?`
			argsQ := []any{lo, hi}
			if eventName != "" {
				q += " AND event_name LIKE ?"
				argsQ = append(argsQ, "%"+eventName+"%")
			}
			q += " ORDER BY time_ms ASC"
			rows, err := db.QueryContext(cmd.Context(), q, argsQ...)
			if err != nil {
				return err
			}
			defer rows.Close()

			type ev struct {
				ID         int64   `json:"id"`
				TimeMS     int64   `json:"time_ms"`
				ISOTime    string  `json:"iso_time"`
				Currency   string  `json:"currency"`
				Importance int     `json:"importance"`
				EventName  string  `json:"event_name"`
				Actual     float64 `json:"actual"`
				Forecast   float64 `json:"forecast"`
				Previous   float64 `json:"previous"`
			}
			var evs []ev
			for rows.Next() {
				var e ev
				if err := rows.Scan(&e.ID, &e.TimeMS, &e.Currency, &e.Importance,
					&e.EventName, &e.Actual, &e.Forecast, &e.Previous); err != nil {
					return err
				}
				e.ISOTime = time.UnixMilli(e.TimeMS).UTC().Format(time.RFC3339)
				evs = append(evs, e)
			}
			return emit(cmd, evs, func(w io.Writer, v any) {
				items := v.([]ev)
				if len(items) == 0 {
					fmt.Fprintf(w, "no events in calendar_events within ±%s of %s\n",
						window, anchor.UTC().Format(time.RFC3339))
					fmt.Fprintln(w, "(table is populated by you — see 'mt5-pp-cli calendar sync --help')")
					return
				}
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "TIME (UTC)\tCCY\tIMP\tEVENT\tACTUAL\tFORECAST\tPREVIOUS")
				fmt.Fprintln(tw, "──────────\t───\t───\t─────\t──────\t────────\t────────")
				for _, e := range items {
					fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%g\t%g\t%g\n",
						time.UnixMilli(e.TimeMS).UTC().Format("2006-01-02 15:04"),
						e.Currency, e.Importance, e.EventName,
						e.Actual, e.Forecast, e.Previous)
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&eventName, "event", "", "Event-name LIKE filter (e.g. NFP, FOMC)")
	cmd.Flags().StringVar(&window, "window", "1h", "Window radius around --at (e.g. 30m, 1h, 2d)")
	cmd.Flags().StringVar(&at, "at", "now", "Anchor time")
	return cmd
}

// ── replay ──────────────────────────────────────────────────────────────────

func newReplayCmd() *cobra.Command {
	var symbol, from, to, speed, granularity string
	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Stream historical bars/ticks from the local mirror to stdout (JSONL)",
		Long: `Tick-accurate replay engine fed from the local mirror.

  --granularity tick      stream ticks
  --granularity bar:M1    stream M1 bars (also M5, M15, M30, H1, H4, D1)
  --speed real            wall-clock pace (practical only for short windows —
                          a year of M1 bars at speed=real takes a year)
  --speed 10x|100x|max    accelerated (max = no sleep, default)

Outputs JSONL, one event per line — pipe to your harness. Works offline once
the mirror has the relevant data.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if symbol == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol is required")}
			}
			fromT, toT, err := parseRange(from, to)
			if err != nil {
				return &ExitErr{Code: ExitUsage, Err: err}
			}
			mult, err := parseSpeed(speed)
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

			w := bufio.NewWriter(cmd.OutOrStdout())
			defer w.Flush()

			isTick := granularity == "tick"
			if !isTick && !strings.HasPrefix(granularity, "bar:") {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--granularity must be 'tick' or 'bar:<TF>'")}
			}
			if isTick {
				return replayTicks(cmd.Context(), db, w, acct, symbol, fromT.UnixMilli(), toT.UnixMilli(), mult)
			}
			tf := strings.ToUpper(strings.TrimPrefix(granularity, "bar:"))
			if !quantAllowedTFs[tf] {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--granularity bar:%s not in allowed TFs", tf)}
			}
			return replayBars(cmd.Context(), db, w, acct, symbol, tf, fromT.UnixMilli(), toT.UnixMilli(), mult)
		},
	}
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	cmd.Flags().StringVar(&from, "from", "", "ISO date/time")
	cmd.Flags().StringVar(&to, "to", "now", "ISO date/time")
	cmd.Flags().StringVar(&speed, "speed", "max", "real | 10x | 100x | max")
	cmd.Flags().StringVar(&granularity, "granularity", "bar:M1", "tick | bar:M1 | bar:M5 | bar:M15 | bar:M30 | bar:H1 | bar:H4 | bar:D1")
	return cmd
}

// parseSpeed: "real" → 1.0, "10x" → 10.0, "max" → math.Inf(+1), "0.5x" → 0.5.
func parseSpeed(s string) (float64, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "", "real":
		return 1.0, nil
	case "max":
		return math.Inf(+1), nil
	}
	s = strings.TrimSuffix(s, "x")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0, fmt.Errorf("--speed %q not parseable (try real, 10x, 100x, max)", s)
	}
	return f, nil
}

func replayTicks(ctx context.Context, db *sql.DB, w *bufio.Writer, acct int64, symbol string, fromMS, toMS int64, speed float64) error {
	rows, err := db.QueryContext(ctx,
		`SELECT time_ms, bid, ask, COALESCE(last,0), COALESCE(volume_real,0), COALESCE(flags,0)
		 FROM ticks WHERE account_login = ? AND symbol = ? AND time_ms BETWEEN ? AND ?
		 ORDER BY time_ms ASC`, acct, symbol, fromMS, toMS)
	if err != nil {
		return err
	}
	defer rows.Close()
	enc := json.NewEncoder(w)

	var (
		streamStart int64
		wallStart   = time.Now()
		gotFirst    = false
		n           int
	)
	for rows.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var t int64
		var bid, ask, last, vol float64
		var flags int64
		if err := rows.Scan(&t, &bid, &ask, &last, &vol, &flags); err != nil {
			return err
		}
		if !gotFirst {
			streamStart = t
			gotFirst = true
		}
		gateReplay(t, streamStart, wallStart, speed)
		if err := enc.Encode(map[string]any{
			"type": "tick", "symbol": symbol,
			"time_ms": t, "iso_time": time.UnixMilli(t).UTC().Format(time.RFC3339),
			"bid": bid, "ask": ask, "last": last,
			"volume_real": vol, "flags": flags,
		}); err != nil {
			return err
		}
		n++
		if n%1024 == 0 {
			w.Flush()
		}
	}
	w.Flush()
	return nil
}

func replayBars(ctx context.Context, db *sql.DB, w *bufio.Writer, acct int64, symbol, tf string, fromMS, toMS int64, speed float64) error {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(
		`SELECT time_ms, o, h, l, c, COALESCE(tick_volume,0), COALESCE(spread,0), COALESCE(real_volume,0)
		 FROM bars_%s WHERE account_login = ? AND symbol = ? AND time_ms BETWEEN ? AND ?
		 ORDER BY time_ms ASC`, tf), acct, symbol, fromMS, toMS)
	if err != nil {
		return err
	}
	defer rows.Close()
	enc := json.NewEncoder(w)

	var (
		streamStart int64
		wallStart   = time.Now()
		gotFirst    = false
		n           int
	)
	for rows.Next() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var t int64
		var o, h, l, c float64
		var tv, sp, rv int64
		if err := rows.Scan(&t, &o, &h, &l, &c, &tv, &sp, &rv); err != nil {
			return err
		}
		if !gotFirst {
			streamStart = t
			gotFirst = true
		}
		gateReplay(t, streamStart, wallStart, speed)
		if err := enc.Encode(map[string]any{
			"type": "bar", "symbol": symbol, "tf": tf,
			"time_ms": t, "iso_time": time.UnixMilli(t).UTC().Format(time.RFC3339),
			"o": o, "h": h, "l": l, "c": c,
			"tick_volume": tv, "spread": sp, "real_volume": rv,
		}); err != nil {
			return err
		}
		n++
		if n%128 == 0 {
			w.Flush()
		}
	}
	w.Flush()
	return nil
}

// gateReplay sleeps so that the emitted event lands at the wall-clock time
// implied by --speed. speed=Inf (max) skips all sleeps.
func gateReplay(eventMS, streamStartMS int64, wallStart time.Time, speed float64) {
	if math.IsInf(speed, +1) {
		return
	}
	elapsedStream := time.Duration(eventMS-streamStartMS) * time.Millisecond
	target := wallStart.Add(time.Duration(float64(elapsedStream) / speed))
	if d := time.Until(target); d > 0 {
		time.Sleep(d)
	}
}

// ── backtest ────────────────────────────────────────────────────────────────

func newBacktestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "backtest", Short: "Event-loop backtester over the local mirror"}
	cmd.AddCommand(newBacktestRunCmd())
	cmd.AddCommand(newBacktestListCmd())
	return cmd
}

func newBacktestRunCmd() *cobra.Command {
	var (
		strategy, symbol, tf, from, to string
		deposit, costPerTrade          float64
		fastN, slowN                   int
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a built-in strategy against historical bars; persist result to the backtests table",
		Long: `Event-loop backtester. v1 ships with one built-in strategy:

  --strategy sma-cross   Long when fast SMA > slow SMA; flat otherwise.
                          Tunables: --fast 20 --slow 50

Pluggable Python strategy hosting will land in Phase 11 polish — keeping v1
self-contained means no subprocess plumbing in the hot loop, deterministic
runs, and a CI-friendly footprint. The result row in the 'backtests' table
stores params_json + metrics_json so you can rebuild equity curves later.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strategy != "sma-cross" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf(
					"--strategy %q not supported in v1 (only 'sma-cross'); see 'mt5-pp-cli backtest run --help'", strategy)}
			}
			if symbol == "" {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--symbol is required")}
			}
			tf = strings.ToUpper(tf)
			if !quantAllowedTFs[tf] {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("--tf %q not allowed", tf)}
			}
			if fastN < 2 || slowN <= fastN {
				return &ExitErr{Code: ExitUsage, Err: fmt.Errorf("require --fast >= 2 and --slow > --fast")}
			}
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

			res, err := runSMACrossBacktest(cmd.Context(), db, acct,
				symbol, tf, fromT.UnixMilli(), toT.UnixMilli(),
				deposit, costPerTrade, fastN, slowN)
			if err != nil {
				return err
			}

			// Persist. The backtests.metrics_json column is reserved for
			// per-trade equity curves a future strategy hosts will write; v1's
			// sma-cross has nothing extra to store, so it lands as an empty
			// JSON object rather than the literal string "null".
			params, _ := json.Marshal(map[string]any{
				"fast": fastN, "slow": slowN, "cost_per_trade": costPerTrade,
			})
			id, err := persistBacktestRow(cmd.Context(), db, backtestRowIn{
				AccountLogin: acct,
				Strategy:     "sma-cross", Symbol: symbol, TF: tf,
				FromMS: fromT.UnixMilli(), ToMS: toT.UnixMilli(),
				Deposit:      deposit,
				NetProfit:    res.NetProfit,
				ProfitFactor: res.ProfitFactor,
				Sharpe:       res.Sharpe,
				MaxDDPct:     res.MaxDDPct,
				Trades:       res.Trades,
				WinRate:      res.WinRate,
				ParamsJSON:   string(params),
				MetricsJSON:  "{}",
			})
			if err != nil {
				return err
			}
			res.ID = id
			return emit(cmd, res, printBacktestResult)
		},
	}
	cmd.Flags().StringVar(&strategy, "strategy", "sma-cross", "Built-in strategy ID (v1: sma-cross)")
	cmd.Flags().StringVar(&symbol, "symbol", "", "Symbol (required)")
	cmd.Flags().StringVar(&tf, "tf", "H1", "Timeframe")
	cmd.Flags().StringVar(&from, "from", "1y", "Start ISO date / relative")
	cmd.Flags().StringVar(&to, "to", "now", "End ISO date / relative")
	cmd.Flags().Float64Var(&deposit, "deposit", 10000, "Starting equity (account currency units)")
	cmd.Flags().Float64Var(&costPerTrade, "cost-per-trade", 0, "Flat cost subtracted per round-trip (account currency units)")
	cmd.Flags().IntVar(&fastN, "fast", 20, "Fast SMA period")
	cmd.Flags().IntVar(&slowN, "slow", 50, "Slow SMA period")
	return cmd
}

func newBacktestListCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List past backtests stored in the backtests table",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := store.OpenAndMigrate("")
			if err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			defer db.Close()
			// Scope to the current account, but also include legacy rows
			// (account_login NULL) so users who ran backtests before the
			// migration still see them.
			acct, err := resolveAccountLogin(cmd.Context(), db, cmd)
			if err != nil {
				return err
			}
			rows, err := db.QueryContext(cmd.Context(), `
				SELECT id, COALESCE(account_login,0), strategy, symbol, tf,
				       COALESCE(from_ms,0), COALESCE(to_ms,0),
				       COALESCE(deposit,0), COALESCE(net_profit,0), COALESCE(profit_factor,0),
				       COALESCE(sharpe,0), COALESCE(max_dd_pct,0), COALESCE(trades,0),
				       COALESCE(win_rate,0), created_at_ms
				FROM backtests
				WHERE account_login = ? OR account_login IS NULL
				ORDER BY id DESC LIMIT ?`, acct, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type row struct {
				ID           int64   `json:"id"`
				AccountLogin int64   `json:"account_login"`
				Strategy     string  `json:"strategy"`
				Symbol       string  `json:"symbol"`
				TF           string  `json:"tf"`
				FromMS       int64   `json:"from_ms"`
				ToMS         int64   `json:"to_ms"`
				CreatedAtMS  int64   `json:"created_at_ms"`
				Deposit      float64 `json:"deposit"`
				NetProfit    float64 `json:"net_profit"`
				ProfitFactor float64 `json:"profit_factor"`
				Sharpe       float64 `json:"sharpe"`
				MaxDDPct     float64 `json:"max_dd_pct"`
				Trades       int64   `json:"trades"`
				WinRate      float64 `json:"win_rate"`
			}
			var all []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.ID, &r.AccountLogin, &r.Strategy, &r.Symbol, &r.TF,
					&r.FromMS, &r.ToMS, &r.Deposit, &r.NetProfit, &r.ProfitFactor,
					&r.Sharpe, &r.MaxDDPct, &r.Trades, &r.WinRate, &r.CreatedAtMS); err != nil {
					return err
				}
				all = append(all, r)
			}
			return emit(cmd, all, func(w io.Writer, v any) {
				rs := v.([]row)
				if len(rs) == 0 {
					fmt.Fprintln(w, "no backtests recorded yet — run: mt5-pp-cli backtest run ...")
					return
				}
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tCREATED\tSTRATEGY\tSYMBOL\tTF\tNET\tPF\tSHARPE\tMAXDD%\tTRADES\tWIN%")
				fmt.Fprintln(tw, "──\t───────\t────────\t──────\t──\t───\t──\t──────\t──────\t──────\t────")
				for _, r := range rs {
					fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%+.2f\t%.2f\t%.2f\t%.2f\t%d\t%.1f\n",
						r.ID, time.UnixMilli(r.CreatedAtMS).Format("2006-01-02 15:04"),
						r.Strategy, r.Symbol, r.TF,
						r.NetProfit, r.ProfitFactor, r.Sharpe, r.MaxDDPct, r.Trades, r.WinRate*100)
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to return")
	return cmd
}

// BacktestResult is the structured output of a v1 sma-cross backtest.
type BacktestResult struct {
	ID           int64   `json:"id,omitempty"`
	Strategy     string  `json:"strategy"`
	Symbol       string  `json:"symbol"`
	TF           string  `json:"tf"`
	FromMS       int64   `json:"from_ms"`
	ToMS         int64   `json:"to_ms"`
	BarsScanned  int     `json:"bars_scanned"`
	Trades       int     `json:"trades"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	WinRate      float64 `json:"win_rate"`
	GrossProfit  float64 `json:"gross_profit"`
	GrossLoss    float64 `json:"gross_loss"`
	NetProfit    float64 `json:"net_profit"`
	ProfitFactor float64 `json:"profit_factor"`
	Sharpe       float64 `json:"sharpe_daily_annualized"`
	MaxDDPct     float64 `json:"max_dd_pct"`
	FinalEquity  float64 `json:"final_equity"`
	StartEquity  float64 `json:"start_equity"`
}

// runSMACrossBacktest: long-only SMA cross. Buy when fast > slow, flat when
// fast <= slow.
//
// FILL MODEL — lookahead within the bar.
// Signals are computed using bar i's close, and entries/exits also use bar
// i's close as the fill price. In reality you can't know the close before
// it happens, so real-trading results will differ. We document this rather
// than silently shifting entries to closes[i+1] because v1's goal is to
// validate the harness (bar loop, equity curve, persistence) — strategy
// honesty comes with v2's Python strategy hosting where the user controls
// the fill model. The mt5-pp-backtester sibling CLI is what you want for
// "what would my EA actually have done?".
//
// P&L is the log return of price × position × deposit (constant exposure
// model so the equity curve is dimensionless × deposit; this is a
// teaching backtest, not a sizing engine).
func runSMACrossBacktest(ctx context.Context, db *sql.DB,
	acct int64, symbol, tf string, fromMS, toMS int64, deposit, costPerTrade float64,
	fastN, slowN int) (*BacktestResult, error) {

	rows, err := db.QueryContext(ctx, fmt.Sprintf(
		`SELECT time_ms, c FROM bars_%s WHERE account_login = ? AND symbol = ? AND time_ms BETWEEN ? AND ?
		 ORDER BY time_ms ASC`, tf), acct, symbol, fromMS, toMS)
	if err != nil {
		return nil, err
	}
	var times []int64
	var closes []float64
	for rows.Next() {
		var t int64
		var c float64
		if err := rows.Scan(&t, &c); err != nil {
			rows.Close()
			return nil, err
		}
		times = append(times, t)
		closes = append(closes, c)
	}
	rows.Close()
	if len(closes) < slowN+2 {
		return nil, &ExitErr{Code: ExitNotFound, Err: fmt.Errorf(
			"only %d bars for %s %s in window — need at least %d (slow+2). Sync more first.",
			len(closes), symbol, tf, slowN+2)}
	}

	fast := simpleSMA(closes, fastN)
	slow := simpleSMA(closes, slowN)

	// Walk bars: on each bar close we read the SMAs computed from that same
	// bar's close and fill at the same close (the lookahead documented in
	// the docstring above). Slippage is approximated by costPerTrade on
	// round-trip.
	pos := 0 // 0 = flat, 1 = long
	entryPx := 0.0
	var (
		trades, wins, losses   int
		grossProfit, grossLoss float64
		netProfit              = 0.0
	)
	startEquity := deposit
	equity := deposit
	peak := equity
	maxDDPct := 0.0
	dailyEquity := map[int64]float64{} // last equity seen per UTC day for daily-rets

	// Process from index slowN (first index with both SMAs non-NaN).
	for i := slowN; i < len(closes); i++ {
		if math.IsNaN(fast[i]) || math.IsNaN(slow[i]) {
			continue
		}
		want := 0
		if fast[i] > slow[i] {
			want = 1
		}
		if want == 1 && pos == 0 {
			pos = 1
			entryPx = closes[i]
		} else if want == 0 && pos == 1 {
			// Close at this bar's close.
			exit := closes[i]
			ret := (exit - entryPx) / entryPx
			pnl := ret*deposit - costPerTrade
			netProfit += pnl
			if pnl >= 0 {
				grossProfit += pnl
				wins++
			} else {
				grossLoss += -pnl
				losses++
			}
			trades++
			equity = startEquity + netProfit
			if equity > peak {
				peak = equity
			}
			if peak > 0 {
				if dd := (peak - equity) / peak * 100; dd > maxDDPct {
					maxDDPct = dd
				}
			}
			day := time.UnixMilli(times[i]).UTC().Truncate(24 * time.Hour).UnixMilli()
			dailyEquity[day] = equity
			pos = 0
			entryPx = 0
		}
	}
	// Close any open position at the last bar.
	if pos == 1 {
		i := len(closes) - 1
		exit := closes[i]
		ret := (exit - entryPx) / entryPx
		pnl := ret*deposit - costPerTrade
		netProfit += pnl
		if pnl >= 0 {
			grossProfit += pnl
			wins++
		} else {
			grossLoss += -pnl
			losses++
		}
		trades++
		equity = startEquity + netProfit
		if equity > peak {
			peak = equity
		}
		if peak > 0 {
			if dd := (peak - equity) / peak * 100; dd > maxDDPct {
				maxDDPct = dd
			}
		}
		day := time.UnixMilli(times[i]).UTC().Truncate(24 * time.Hour).UnixMilli()
		dailyEquity[day] = equity
	}

	winRate := 0.0
	if trades > 0 {
		winRate = float64(wins) / float64(trades)
	}
	pf := math.Inf(+1)
	if grossLoss > 0 {
		pf = grossProfit / grossLoss
	}
	if grossProfit == 0 && grossLoss == 0 {
		pf = 0
	}
	sharpe := computeBTSharpe(dailyEquity, startEquity)

	return &BacktestResult{
		Strategy: "sma-cross",
		Symbol:   symbol,
		TF:       tf,
		FromMS:   fromMS, ToMS: toMS,
		BarsScanned:  len(closes),
		Trades:       trades,
		Wins:         wins,
		Losses:       losses,
		WinRate:      winRate,
		GrossProfit:  grossProfit,
		GrossLoss:    grossLoss,
		NetProfit:    netProfit,
		ProfitFactor: pf,
		Sharpe:       sharpe,
		MaxDDPct:     maxDDPct,
		FinalEquity:  startEquity + netProfit,
		StartEquity:  startEquity,
	}, nil
}

func simpleSMA(c []float64, period int) []float64 {
	n := len(c)
	out := make([]float64, n)
	for i := range out {
		out[i] = math.NaN()
	}
	if period <= 0 || n < period {
		return out
	}
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += c[i]
	}
	out[period-1] = sum / float64(period)
	for i := period; i < n; i++ {
		sum += c[i] - c[i-period]
		out[i] = sum / float64(period)
	}
	return out
}

// computeBTSharpe takes the per-day equity map and returns annualized Sharpe
// of daily returns (√252). Returns 0 if not enough days.
func computeBTSharpe(dailyEq map[int64]float64, startEq float64) float64 {
	if len(dailyEq) < 2 {
		return 0
	}
	days := make([]int64, 0, len(dailyEq))
	for d := range dailyEq {
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
	prev := startEq
	var rets []float64
	for _, d := range days {
		eq := dailyEq[d]
		if prev > 0 {
			rets = append(rets, eq/prev-1)
		}
		prev = eq
	}
	if len(rets) < 2 {
		return 0
	}
	var sum float64
	for _, r := range rets {
		sum += r
	}
	mean := sum / float64(len(rets))
	var sq float64
	for _, r := range rets {
		sq += (r - mean) * (r - mean)
	}
	std := math.Sqrt(sq / float64(len(rets)-1))
	if std == 0 {
		return 0
	}
	return (mean / std) * math.Sqrt(252)
}

type backtestRowIn struct {
	AccountLogin            int64
	Strategy, Symbol, TF    string
	FromMS, ToMS            int64
	Deposit, NetProfit      float64
	ProfitFactor, Sharpe    float64
	MaxDDPct                float64
	Trades                  int
	WinRate                 float64
	ParamsJSON, MetricsJSON string
}

func persistBacktestRow(ctx context.Context, db *sql.DB, in backtestRowIn) (int64, error) {
	res, err := db.ExecContext(ctx, `
		INSERT INTO backtests(account_login, strategy, symbol, tf, from_ms, to_ms, deposit, net_profit,
		                     profit_factor, sharpe, max_dd_pct, trades, win_rate,
		                     params_json, metrics_json)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.AccountLogin, in.Strategy, in.Symbol, in.TF, in.FromMS, in.ToMS, in.Deposit, in.NetProfit,
		profitFactorForSQL(in.ProfitFactor), in.Sharpe, in.MaxDDPct, in.Trades, in.WinRate,
		in.ParamsJSON, in.MetricsJSON)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// profitFactorForSQL clamps Inf to NULL-equivalent (-1) for storage so JSON
// reads work; the human printer surfaces "Inf" when grossLoss=0.
func profitFactorForSQL(pf float64) any {
	if math.IsInf(pf, 0) || math.IsNaN(pf) {
		return nil
	}
	return pf
}

func printBacktestResult(w io.Writer, v any) {
	r := v.(*BacktestResult)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	pf := fmt.Sprintf("%.2f", r.ProfitFactor)
	if math.IsInf(r.ProfitFactor, +1) {
		pf = "∞ (no losers)"
	}
	rows := [][2]string{
		{"id", fmt.Sprintf("%d", r.ID)},
		{"strategy", r.Strategy},
		{"symbol/tf", r.Symbol + " " + r.TF},
		{"window", fmt.Sprintf("%s → %s",
			time.UnixMilli(r.FromMS).UTC().Format("2006-01-02"),
			time.UnixMilli(r.ToMS).UTC().Format("2006-01-02"))},
		{"bars_scanned", fmt.Sprintf("%d", r.BarsScanned)},
		{"trades", fmt.Sprintf("%d (%d wins, %d losses)", r.Trades, r.Wins, r.Losses)},
		{"win_rate", fmt.Sprintf("%.1f%%", r.WinRate*100)},
		{"gross_profit", fmt.Sprintf("%+.2f", r.GrossProfit)},
		{"gross_loss", fmt.Sprintf("-%.2f", r.GrossLoss)},
		{"net_profit", fmt.Sprintf("%+.2f", r.NetProfit)},
		{"profit_factor", pf},
		{"sharpe (252)", fmt.Sprintf("%.2f", r.Sharpe)},
		{"max_dd_pct", fmt.Sprintf("%.2f%%", r.MaxDDPct)},
		{"start_equity", fmt.Sprintf("%.2f", r.StartEquity)},
		{"final_equity", fmt.Sprintf("%.2f", r.FinalEquity)},
	}
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%s\n", row[0], row[1])
	}
	tw.Flush()
}

// ── helper EA / watch (Phase 2 stubs unchanged) ─────────────────────────────

func newHelperCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "helper", Short: "MQL5 helper EA for live trade events (Phase 2)"}
	cmd.AddCommand(&cobra.Command{
		Use:   "install",
		Short: "Scaffold the helper EA directory tree (Phase 2 stub today)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImpl("Phase 2 helper EA — see library/other/mt5/helper/TODO.md for the design and what v2 will deliver")
		},
	})
	return cmd
}

func newWatchCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "watch", Short: "Tail live MT5 events"}
	trades := &cobra.Command{
		Use:   "trades",
		Short: "Tail trade events (Phase 2; requires helper EA)",
		Long: `Stream trade events as they happen.

v1 workaround until the helper EA lands: poll 'mt5-pp-cli positions list --json' on
an interval (every 1-5 seconds) and diff. The helper EA in Phase 2 replaces
polling with an OnTradeTransaction event stream.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImpl("Phase 2 (helper EA). v1 workaround: poll positions list on an interval.")
		},
	}
	trades.Flags().Bool("tail", true, "Stream until interrupted")
	trades.Flags().Duration("poll", 0, "v1 workaround: poll positions list at this interval instead")
	cmd.AddCommand(trades)
	return cmd
}

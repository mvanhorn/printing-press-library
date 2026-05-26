package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newStablesCmd() *cobra.Command {
	var chain, sortBy, with string
	cmd := &cobra.Command{
		Use:   "stables [symbol]",
		Short: "Stablecoin overview, with optional chain breakdown",
		Args:  cobra.MaximumNArgs(1),
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if err := ensureFresh(context.Background(), cx, []string{"stablecoins"}); err != nil {
				return err
			}
			lim := G.Limit
			if lim <= 0 {
				lim = 30
			}
			if chain != "" {
				return runStablesByChain(cx, chain, sortBy, with, lim)
			}
			if len(args) == 1 {
				return runStableDetail(cx, args[0])
			}
			sortCol := "circulating DESC"
			switch sortBy {
			case "", "circulating", "mcap":
				sortCol = "circulating DESC"
			case "symbol":
				sortCol = "symbol ASC"
			default:
				return fmt.Errorf("unknown --sort %q", sortBy)
			}
			q := fmt.Sprintf(`SELECT symbol, name, peg_type, peg_mechanism, price, circulating FROM stablecoins WHERE circulating > 0 ORDER BY %s LIMIT ?`, sortCol)
			rows, err := cx.S.Query(q, lim)
			if err != nil {
				return err
			}
			defer rows.Close()
			rec := format.NewRecorder([]string{"SYMBOL", "NAME", "PEG", "MECH", "PRICE", "CIRCULATING"})
			for rows.Next() {
				var sym, name, pegType, mech string
				var price, circ float64
				if err := rows.Scan(&sym, &name, &pegType, &mech, &price, &circ); err != nil {
					return err
				}
				rec.Append([]format.Cell{
					format.Str(sym), format.Str(name), format.Str(pegType), format.Str(mech),
					format.USD(price), format.USD(circ),
				})
			}
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{4, 5})})
		}),
	}
	cmd.Flags().StringVar(&chain, "chain", "", "show breakdown for a specific chain")
	cmd.Flags().StringVar(&sortBy, "sort", "", "sort key: circulating|symbol")
	cmd.Flags().StringVar(&with, "with", "", "extras: dominance")

	cmd.AddCommand(newStablesFlowCmd())
	return cmd
}

var newStablesFlowCmd = func() *cobra.Command {
	var period, sortBy string
	cmd := &cobra.Command{
		Use:   "flow",
		Short: "Per-chain stablecoin supply deltas over a period",
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			days, err := parsePeriod(period)
			if err != nil {
				return err
			}
			if err := ensureStablecoinHistory(cx); err != nil {
				return err
			}
			cutoff := ""
			if days > 0 {
				cutoff = fmt.Sprintf("-%d days", days)
			}

			// Per chain: latest circulating + circulating at period start.
			q := `SELECT chain,
			             (SELECT circulating FROM stablecoin_hist sh1
			              WHERE sh1.stablecoin_id = '_total' AND sh1.chain = h.chain
			              ORDER BY sh1.date DESC LIMIT 1) AS current_v,
			             (SELECT circulating FROM stablecoin_hist sh2
			              WHERE sh2.stablecoin_id = '_total' AND sh2.chain = h.chain`
			qargs := []any{}
			if cutoff != "" {
				q += ` AND sh2.date <= date('now', ?)`
				qargs = append(qargs, cutoff)
			}
			q += ` ORDER BY sh2.date DESC LIMIT 1) AS period_ago
			FROM (SELECT DISTINCT chain FROM stablecoin_hist WHERE stablecoin_id = '_total') h`
			rows, err := cx.S.Query(q, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()

			type r struct {
				chain               string
				current, periodAgo  float64
				change, changePct   float64
			}
			all := []r{}
			for rows.Next() {
				var chain string
				var cur, prev sql.NullFloat64
				if err := rows.Scan(&chain, &cur, &prev); err != nil {
					return err
				}
				if !cur.Valid || cur.Float64 == 0 {
					continue
				}
				row := r{chain: chain, current: cur.Float64, periodAgo: prev.Float64}
				row.change = row.current - row.periodAgo
				if row.periodAgo > 0 {
					row.changePct = (row.change / row.periodAgo) * 100
				}
				all = append(all, row)
			}

			// Sort.
			switch sortBy {
			case "", "change":
				sort.Slice(all, func(i, j int) bool { return abs(all[i].change) > abs(all[j].change) })
			case "change_pct":
				sort.Slice(all, func(i, j int) bool { return abs(all[i].changePct) > abs(all[j].changePct) })
			case "current":
				sort.Slice(all, func(i, j int) bool { return all[i].current > all[j].current })
			default:
				return fmt.Errorf("unknown --sort %q (change|change_pct|current)", sortBy)
			}

			lim := G.Limit
			if lim <= 0 {
				lim = 30
			}
			if lim < len(all) {
				all = all[:lim]
			}

			rec := format.NewRecorder([]string{"CHAIN", "CURRENT", "PERIOD_AGO", "CHANGE", "CHANGE_PCT"})
			for _, x := range all {
				rec.Append([]format.Cell{
					format.Str(x.chain),
					format.USD(x.current),
					format.USD(x.periodAgo),
					format.USD(x.change),
					format.Pct(x.changePct),
				})
			}
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1, 2, 3, 4})})
		}),
	}
	cmd.Flags().StringVar(&period, "period", "30d", "lookback window (7d|30d|90d|180d|1y|all)")
	cmd.Flags().StringVar(&sortBy, "sort", "change", "sort: change|change_pct|current")
	return cmd
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func ensureStablecoinHistory(cx *Ctx) error {
	if G.NoSync {
		return nil
	}
	stale, err := cx.S.StaleBefore("stablecoin_hist", cx.Cfg.StaleHistoricalDur())
	if err != nil {
		return err
	}
	if !stale {
		return nil
	}
	fmt.Fprintln(os.Stderr, "syncing stablecoin chain history (this hits many chains, ~30 calls)...")
	cx.Eng.Report = func(d, msg string) { fmt.Fprintf(os.Stderr, "  [%s] %s\n", d, msg) }
	return cx.Eng.SyncStablecoinChainHistory(context.Background())
}

func runStableDetail(cx *Ctx, sym string) error {
	q := `SELECT id, symbol, name, peg_type, peg_mechanism, price, circulating, chains FROM stablecoins WHERE LOWER(symbol) = LOWER(?) OR LOWER(name) = LOWER(?) LIMIT 1`
	var id, symbol, name, pegType, mech, chains string
	var price, circ float64
	if err := cx.S.QueryRow(q, sym, sym).Scan(&id, &symbol, &name, &pegType, &mech, &price, &circ, &chains); err != nil {
		return fmt.Errorf("stablecoin %q not found", sym)
	}
	rec := format.NewRecorder([]string{"FIELD", "VALUE"})
	rec.Append([]format.Cell{format.Str("SYMBOL"), format.Str(symbol)})
	rec.Append([]format.Cell{format.Str("NAME"), format.Str(name)})
	rec.Append([]format.Cell{format.Str("PEG"), format.Str(pegType)})
	rec.Append([]format.Cell{format.Str("MECH"), format.Str(mech)})
	rec.Append([]format.Cell{format.Str("PRICE"), format.USD(price)})
	rec.Append([]format.Cell{format.Str("CIRCULATING"), format.USD(circ)})
	rows, err := cx.S.Query(`SELECT chain, circulating FROM stablecoin_chains WHERE stablecoin_id = ? AND circulating > 0 ORDER BY circulating DESC`, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var chain string
			var c float64
			if err := rows.Scan(&chain, &c); err != nil {
				return err
			}
			rec.Append([]format.Cell{format.Str("  " + chain), format.USD(c)})
		}
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1})})
}

func runStablesByChain(cx *Ctx, chain, sortBy, with string, lim int) error {
	sortCol := "sc.circulating DESC"
	if sortBy == "symbol" {
		sortCol = "s.symbol ASC"
	}
	q := fmt.Sprintf(`SELECT s.symbol, s.name, sc.circulating, s.circulating
		FROM stablecoin_chains sc JOIN stablecoins s ON s.id = sc.stablecoin_id
		WHERE LOWER(sc.chain) = LOWER(?) AND sc.circulating > 0
		ORDER BY %s LIMIT ?`, sortCol)
	rows, err := cx.S.Query(q, chain, lim)
	if err != nil {
		return err
	}
	defer rows.Close()
	wantDom := containsAny(with, "dominance")
	headers := []string{"SYMBOL", "NAME", "ON_CHAIN", "TOTAL"}
	right := []int{2, 3}
	if wantDom {
		headers = append(headers, "DOMINANCE")
		right = append(right, len(headers)-1)
	}

	type row struct {
		sym, name string
		onChain   float64
		total     float64
	}
	all := []row{}
	chainTotal := 0.0
	for rows.Next() {
		r := row{}
		if err := rows.Scan(&r.sym, &r.name, &r.onChain, &r.total); err != nil {
			return err
		}
		all = append(all, r)
		chainTotal += r.onChain
	}
	if len(all) == 0 {
		return fmt.Errorf("no stablecoin data for chain %q", chain)
	}
	rec := format.NewRecorder(headers)
	for _, r := range all {
		cells := []format.Cell{
			format.Str(r.sym), format.Str(r.name),
			format.USD(r.onChain), format.USD(r.total),
		}
		if wantDom {
			dom := 0.0
			if chainTotal > 0 {
				dom = (r.onChain / chainTotal) * 100
			}
			cells = append(cells, format.Pct(dom))
		}
		rec.Append(cells)
	}
	if !G.NoHeader && mode() == format.ModeTable {
		fmt.Fprintf(os.Stderr, "%s total stablecoin supply: %s\n", chain, format.USDString(chainTotal))
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet(right)})
}

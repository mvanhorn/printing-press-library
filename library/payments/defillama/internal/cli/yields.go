package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newYieldsCmd() *cobra.Command {
	var chain, project, sortBy, poolID, period string
	var minTVL float64
	var stableOnly, singleOnly, history bool

	cmd := &cobra.Command{
		Use:   "yields",
		Short: "Search yield pools across chains and projects",
		RunE:  yieldsRun(&chain, &project, &sortBy, &minTVL, &stableOnly, &singleOnly, &poolID, &history, &period),
	}
	cmd.Flags().StringVar(&chain, "chain", "", "filter by chain")
	cmd.Flags().StringVar(&project, "project", "", "filter by project (e.g. aave, curve-dex)")
	cmd.Flags().StringVar(&sortBy, "sort", "tvl", "sort key: tvl|apy|apy_base")
	cmd.Flags().Float64Var(&minTVL, "min-tvl", 0, "minimum TVL in USD")
	cmd.Flags().BoolVar(&stableOnly, "stablecoin-only", false, "only stablecoin pools")
	cmd.Flags().BoolVar(&singleOnly, "single-only", false, "only single-asset exposure pools")
	cmd.Flags().StringVar(&poolID, "pool", "", "show history for a specific pool id (requires --history)")
	cmd.Flags().BoolVar(&history, "history", false, "show historical chart for the --pool")
	cmd.Flags().StringVar(&period, "period", "30d", "period: 7d|30d|90d|180d|1y|all")

	top := &cobra.Command{
		Use:   "top",
		Short: "Top yield pools matching criteria",
		RunE:  yieldsRun(&chain, &project, &sortBy, &minTVL, &stableOnly, &singleOnly, &poolID, &history, &period),
	}
	top.Flags().StringVar(&chain, "chain", "", "filter by chain")
	top.Flags().StringVar(&project, "project", "", "filter by project (e.g. aave, curve-dex)")
	top.Flags().StringVar(&sortBy, "sort", "tvl", "sort key: tvl|apy|apy_base")
	top.Flags().Float64Var(&minTVL, "min-tvl", 0, "minimum TVL in USD")
	top.Flags().BoolVar(&stableOnly, "stablecoin-only", false, "only stablecoin pools")
	top.Flags().BoolVar(&singleOnly, "single-only", false, "only single-asset exposure pools")
	top.Flags().StringVar(&poolID, "pool", "", "show history for a specific pool id (requires --history)")
	top.Flags().BoolVar(&history, "history", false, "show historical chart for the --pool")
	top.Flags().StringVar(&period, "period", "30d", "period: 7d|30d|90d|180d|1y|all")
	cmd.AddCommand(top)
	return cmd
}

func yieldsRun(chain, project, sortBy *string, minTVL *float64, stableOnly, singleOnly *bool, poolID *string, history *bool, period *string) func(*cobra.Command, []string) error {
	return withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
		if *history {
			if *poolID == "" {
				return fmt.Errorf("--history requires --pool <pool_id>")
			}
			return runPoolHistory(cx, *poolID, *period)
		}
		if err := ensureFresh(context.Background(), cx, []string{"pools"}); err != nil {
			return err
		}
		lim := G.Limit
		if lim <= 0 {
			lim = 50
		}

		where := []string{"tvl_usd > 0"}
		qargs := []any{}
		if *chain != "" {
			where = append(where, "LOWER(chain) = LOWER(?)")
			qargs = append(qargs, *chain)
		}
		if *project != "" {
			where = append(where, "LOWER(project) = LOWER(?)")
			qargs = append(qargs, *project)
		}
		if *minTVL > 0 {
			where = append(where, "tvl_usd >= ?")
			qargs = append(qargs, *minTVL)
		}
		if *stableOnly {
			where = append(where, "stablecoin = 1")
		}
		if *singleOnly {
			where = append(where, "LOWER(exposure) = 'single'")
		}

		sortCol := "tvl_usd DESC"
		switch *sortBy {
		case "tvl":
			sortCol = "tvl_usd DESC"
		case "apy":
			sortCol = "apy DESC"
		case "apy_base":
			sortCol = "apy_base DESC"
		default:
			return fmt.Errorf("unknown --sort %q", *sortBy)
		}

		q := fmt.Sprintf(
			"SELECT pool_id, project, chain, symbol, tvl_usd, apy, apy_base, apy_reward, il_risk FROM pools WHERE %s ORDER BY %s LIMIT ?",
			strings.Join(where, " AND "), sortCol,
		)
		qargs = append(qargs, lim)
		rows, err := cx.S.Query(q, qargs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		rec := format.NewRecorder([]string{"POOL_ID", "POOL", "PROJECT", "CHAIN", "TVL", "APY", "APY_BASE", "APY_REW", "IL_RISK"})
		right := rightSet([]int{4, 5, 6, 7})
		for rows.Next() {
			var id, project, chain, symbol, ilRisk string
			var tvl, apy, apyBase, apyRew float64
			if err := rows.Scan(&id, &project, &chain, &symbol, &tvl, &apy, &apyBase, &apyRew, &ilRisk); err != nil {
				return err
			}
			rec.Append([]format.Cell{
				format.Str(id),
				format.Str(symbol),
				format.Str(project),
				format.Str(chain),
				format.USD(tvl),
				format.Pct(apy),
				format.Pct(apyBase),
				format.Pct(apyRew),
				format.Str(ilRisk),
			})
		}
		return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: right})
	})
}

// runPoolHistory fetches yields.llama.fi /chart/{pool} on demand and renders
// the TVL+APY time series.
func runPoolHistory(cx *Ctx, poolID, period string) error {
	days, err := parsePeriod(period)
	if err != nil {
		return err
	}
	stale, _ := cx.S.StaleBefore("pool_hist:"+poolID, cx.Cfg.StaleHistoricalDur())
	if stale && !G.NoSync {
		fmt.Fprintf(os.Stderr, "syncing pool history %s...\n", poolID)
		if err := cx.Eng.SyncPoolChart(context.Background(), poolID); err != nil {
			return err
		}
	}
	q := `SELECT date, tvl, apy FROM pool_hist WHERE pool_id = ?`
	args := []any{poolID}
	if days > 0 {
		q += ` AND date >= date('now', ?)`
		args = append(args, fmt.Sprintf("-%d days", days))
	}
	q += ` ORDER BY date ASC`
	rows, err := cx.S.Query(q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	rec := format.NewRecorder([]string{"DATE", "TVL", "APY"})
	for rows.Next() {
		var date string
		var tvl, apy float64
		if err := rows.Scan(&date, &tvl, &apy); err != nil {
			return err
		}
		rec.Append([]format.Cell{format.Str(date), format.USD(tvl), format.Pct(apy)})
	}
	if len(rec.Rows) == 0 {
		return fmt.Errorf("no history for pool %q", poolID)
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1, 2})})
}

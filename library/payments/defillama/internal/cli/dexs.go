package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newDexsCmd() *cobra.Command {
	var chain, sortBy, with, period string
	var history bool
	cmd := &cobra.Command{
		Use:   "dexs [protocol]",
		Short: "DEX volume overview, by protocol or chain",
		Args:  cobra.MaximumNArgs(1),
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if len(args) == 1 && history {
				if err := ensureFresh(context.Background(), cx, []string{"protocols"}); err != nil {
					return err
				}
				slug, _, err := resolveProtocol(cx.S, args[0])
				if err != nil {
					return err
				}
				return runDexHistory(cx, slug, period)
			}
			if err := ensureFresh(context.Background(), cx, []string{"dexs"}); err != nil {
				return err
			}
			lim := G.Limit
			if lim <= 0 {
				lim = 30
			}

			// Chain-filtered queries join dex_chain_volume to use chain-specific
			// totals instead of each DEX's global numbers.
			perChain := chain != ""
			where := []string{}
			qargs := []any{}
			joins := ""
			volExpr := "d.total_24h"
			vol7Expr := "d.total_7d"
			vol30Expr := "d.total_30d"
			if perChain {
				joins = " JOIN dex_chain_volume dcv ON dcv.protocol = d.protocol AND LOWER(dcv.chain) = LOWER(?)"
				qargs = append(qargs, chain)
				where = append(where, "dcv.total_24h > 0")
				volExpr, vol7Expr, vol30Expr = "dcv.total_24h", "dcv.total_7d", "dcv.total_30d"
			} else {
				where = append(where, "d.total_24h > 0")
			}
			if len(args) == 1 {
				slug, _, err := resolveProtocol(cx.S, args[0])
				if err != nil {
					return err
				}
				where = append(where, "d.protocol = ?")
				qargs = append(qargs, slug)
			}
			sortCol := volExpr + " DESC"
			switch sortBy {
			case "", "volume_24h", "volume":
			case "volume_7d":
				sortCol = vol7Expr + " DESC"
			case "volume_30d":
				sortCol = vol30Expr + " DESC"
			default:
				return fmt.Errorf("unknown --sort %q", sortBy)
			}
			q := fmt.Sprintf(`SELECT d.display_name, d.protocol, %s, %s, %s, d.change_1d
			      FROM dex_overview d%s WHERE %s ORDER BY %s LIMIT ?`,
				volExpr, vol7Expr, vol30Expr, joins,
				strings.Join(where, " AND "), sortCol)
			qargs = append(qargs, lim)
			rows, err := cx.S.Query(q, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()
			headers := []string{"DEX", "VOL_24H", "VOL_7D", "VOL_30D", "CHANGE_1D"}
			right := []int{1, 2, 3, 4}

			withShare := containsAny(with, "market-share")
			var totalVol float64
			if withShare {
				_ = cx.S.QueryRow(`SELECT COALESCE(SUM(total_24h),0) FROM dex_overview WHERE total_24h > 0`).Scan(&totalVol)
				headers = append(headers, "MARKET_SHARE")
				right = append(right, len(headers)-1)
			}

			rec := format.NewRecorder(headers)
			for rows.Next() {
				var display, slug string
				var v24, v7, v30, ch1 float64
				if err := rows.Scan(&display, &slug, &v24, &v7, &v30, &ch1); err != nil {
					return err
				}
				if display == "" {
					display = slug
				}
				row := []format.Cell{
					format.Str(display),
					format.USD(v24), format.USD(v7), format.USD(v30),
					format.Pct(ch1),
				}
				if withShare {
					share := 0.0
					if totalVol > 0 {
						share = (v24 / totalVol) * 100
					}
					row = append(row, format.Pct(share))
				}
				rec.Append(row)
			}
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet(right)})
		}),
	}
	cmd.Flags().StringVar(&chain, "chain", "", "filter by chain")
	cmd.Flags().StringVar(&sortBy, "sort", "volume_24h", "sort: volume_24h|volume_7d|volume_30d")
	cmd.Flags().StringVar(&with, "with", "", "extras: market-share")
	cmd.Flags().BoolVar(&history, "history", false, "(with a protocol arg) show historical volume")
	cmd.Flags().StringVar(&period, "period", "30d", "period: 7d|30d|90d|180d|1y|all")
	return cmd
}

func runDexHistory(cx *Ctx, slug, period string) error {
	days, err := parsePeriod(period)
	if err != nil {
		return err
	}
	if err := ensureProtocolHistory(cx, slug); err != nil {
		return err
	}
	q := `SELECT date, volume FROM dex_hist WHERE protocol = ?`
	args := []any{slug}
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
	rec := format.NewRecorder([]string{"DATE", "VOLUME"})
	for rows.Next() {
		var date string
		var vol float64
		if err := rows.Scan(&date, &vol); err != nil {
			return err
		}
		rec.Append([]format.Cell{format.Str(date), format.USD(vol)})
	}
	if len(rec.Rows) == 0 {
		return fmt.Errorf("no DEX history for %q", slug)
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1})})
}

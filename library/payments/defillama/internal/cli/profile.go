package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newProfileCmd() *cobra.Command {
	var period string
	cmd := &cobra.Command{
		Use:   "profile <protocol>",
		Short: "Deep dive on a single protocol (TVL + fees + revenue + volume)",
		Args:  cobra.ExactArgs(1),
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if err := ensureFresh(context.Background(), cx, []string{"protocols", "fees", "dexs"}); err != nil {
				return err
			}
			slug, name, err := resolveProtocol(cx.S, args[0])
			if err != nil {
				return err
			}

			var category, chains, symbol, mainChain, url, desc string
			var tvl, mcap, c1h, c1d, c7d sql.NullFloat64
			if err := cx.S.QueryRow(`SELECT category, chains, symbol, chain, url, description, tvl, mcap, change_1h, change_1d, change_7d FROM protocols WHERE slug = ?`, slug).
				Scan(&category, &chains, &symbol, &mainChain, &url, &desc, &tvl, &mcap, &c1h, &c1d, &c7d); err != nil {
				return err
			}

			var fees24, fees7, fees30, rev24, rev7, rev30 sql.NullFloat64
			_ = cx.S.QueryRow(`SELECT total_24h_fees, total_7d_fees, total_30d_fees, total_24h_rev, total_7d_rev, total_30d_rev FROM fees_overview WHERE protocol = ?`, slug).
				Scan(&fees24, &fees7, &fees30, &rev24, &rev7, &rev30)

			var vol24, vol7, vol30 sql.NullFloat64
			_ = cx.S.QueryRow(`SELECT total_24h, total_7d, total_30d FROM dex_overview WHERE protocol = ?`, slug).
				Scan(&vol24, &vol7, &vol30)

			rec := format.NewRecorder([]string{"FIELD", "VALUE"})
			str := func(k, v string) {
				rec.Append([]format.Cell{format.Str(k), format.Str(v)})
			}
			usd := func(k string, v float64) {
				rec.Append([]format.Cell{format.Str(k), format.USD(v)})
			}
			pct := func(k string, v float64) {
				rec.Append([]format.Cell{format.Str(k), format.Pct(v)})
			}

			str("PROTOCOL", name)
			str("SLUG", slug)
			str("SYMBOL", symbol)
			str("CATEGORY", category)
			str("MAIN_CHAIN", mainChain)
			str("CHAINS", format.Truncate(chains, 80))
			usd("TVL", tvl.Float64)
			usd("MCAP", mcap.Float64)
			pct("CHANGE_1H", c1h.Float64)
			pct("CHANGE_1D", c1d.Float64)
			pct("CHANGE_7D", c7d.Float64)
			if fees24.Valid || fees7.Valid || fees30.Valid {
				usd("FEES_24H", fees24.Float64)
				usd("FEES_7D", fees7.Float64)
				usd("FEES_30D", fees30.Float64)
				usd("REV_24H", rev24.Float64)
				usd("REV_7D", rev7.Float64)
				usd("REV_30D", rev30.Float64)
			}
			if vol24.Valid || vol7.Valid || vol30.Valid {
				usd("VOL_24H", vol24.Float64)
				usd("VOL_7D", vol7.Float64)
				usd("VOL_30D", vol30.Float64)
			}
			if url != "" {
				str("URL", url)
			}
			if err := format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1})}); err != nil {
				return err
			}
			if period != "" {
				return runProfileHistory(cx, slug, period)
			}
			return nil
		}),
	}
	cmd.Flags().StringVar(&period, "period", "", "include historical TVL/fees/revenue over period (7d|30d|90d|180d|1y|all)")
	return cmd
}

// runProfileHistory prints a historical TVL/fees/revenue section after the
// summary row, triggering a per-protocol sync if data is stale.
func runProfileHistory(cx *Ctx, slug, period string) error {
	days, err := parsePeriod(period)
	if err != nil {
		return err
	}
	if err := ensureProtocolHistory(cx, slug); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "history:")
	q := `SELECT pt.date,
	             pt.tvl,
	             COALESCE(fh.fees,0),
	             COALESCE(fh.revenue,0),
	             COALESCE(dh.volume,0)
	      FROM protocol_tvl_hist pt
	      LEFT JOIN fees_hist fh ON fh.protocol = pt.protocol_slug AND fh.date = pt.date
	      LEFT JOIN dex_hist  dh ON dh.protocol = pt.protocol_slug AND dh.date = pt.date
	      WHERE pt.protocol_slug = ? AND pt.chain = ''`
	args := []any{slug}
	if days > 0 {
		q += ` AND pt.date >= date('now', ?)`
		args = append(args, fmt.Sprintf("-%d days", days))
	}
	q += ` ORDER BY pt.date ASC`
	rows, err := cx.S.Query(q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	rec := format.NewRecorder([]string{"DATE", "TVL", "FEES", "REV", "VOL"})
	for rows.Next() {
		var date string
		var tvl, fees, rev, vol float64
		if err := rows.Scan(&date, &tvl, &fees, &rev, &vol); err != nil {
			return err
		}
		rec.Append([]format.Cell{
			format.Str(date), format.USD(tvl), format.USD(fees), format.USD(rev), format.USD(vol),
		})
	}
	if len(rec.Rows) == 0 {
		fmt.Fprintln(os.Stdout, "(no history available)")
		return nil
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1, 2, 3, 4})})
}

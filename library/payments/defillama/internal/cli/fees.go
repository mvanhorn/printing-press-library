package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newFeesCmd() *cobra.Command {
	var chain, sortBy, period string
	var history bool
	cmd := &cobra.Command{
		Use:   "fees [protocol]",
		Short: "Protocol or chain fees and revenue",
		Args:  cobra.MaximumNArgs(1),
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if chain != "" {
				return runFeesChain(cx, chain, sortBy)
			}
			if len(args) == 0 {
				return fmt.Errorf("fees requires a protocol name or --chain")
			}
			if err := ensureFresh(context.Background(), cx, []string{"protocols", "fees"}); err != nil {
				return err
			}
			slug, name, err := resolveProtocol(cx.S, args[0])
			if err != nil {
				return err
			}
			if history {
				return runFeesHistory(cx, slug, period)
			}
			var f24, f7, f30, r24, r7, r30 sql.NullFloat64
			_ = cx.S.QueryRow(`SELECT total_24h_fees, total_7d_fees, total_30d_fees,
			                          total_24h_rev,  total_7d_rev,  total_30d_rev
			                   FROM fees_overview WHERE protocol = ?`, slug).
				Scan(&f24, &f7, &f30, &r24, &r7, &r30)
			rec := format.NewRecorder([]string{"PROTOCOL", "FEES_24H", "FEES_7D", "FEES_30D", "REV_24H", "REV_7D", "REV_30D"})
			rec.Append([]format.Cell{
				format.Str(name),
				format.USD(f24.Float64), format.USD(f7.Float64), format.USD(f30.Float64),
				format.USD(r24.Float64), format.USD(r7.Float64), format.USD(r30.Float64),
			})
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1, 2, 3, 4, 5, 6})})
		}),
	}
	cmd.Flags().StringVar(&chain, "chain", "", "filter to a chain")
	cmd.Flags().StringVar(&sortBy, "sort", "fees", "sort: fees|revenue (--chain mode)")
	cmd.Flags().BoolVar(&history, "history", false, "show historical fees + revenue")
	cmd.Flags().StringVar(&period, "period", "30d", "period: 7d|30d|90d|180d|1y|all")
	return cmd
}

func runFeesChain(cx *Ctx, chain, sortBy string) error {
	if err := ensureFresh(context.Background(), cx, []string{"protocols", "fees"}); err != nil {
		return err
	}
	lim := G.Limit
	if lim <= 0 {
		lim = 30
	}
	sortCol := "f.total_24h_fees DESC"
	if sortBy == "revenue" {
		sortCol = "f.total_24h_rev DESC"
	}
	q := fmt.Sprintf(`SELECT p.name, f.total_24h_fees, f.total_24h_rev, COALESCE(p.category,'')
	      FROM fees_overview f JOIN protocols p ON p.slug = f.protocol
	      JOIN protocol_chain_tvl pct ON pct.protocol_slug = p.slug
	      WHERE LOWER(pct.chain) = LOWER(?) AND COALESCE(p.category,'') != 'CEX'
	      AND (f.total_24h_fees > 0 OR f.total_24h_rev > 0)
	      GROUP BY p.slug ORDER BY %s LIMIT ?`, sortCol)
	rows, err := cx.S.Query(q, chain, lim)
	if err != nil {
		return err
	}
	defer rows.Close()
	rec := format.NewRecorder([]string{"PROTOCOL", "FEES_24H", "REV_24H", "CATEGORY"})
	for rows.Next() {
		var name, cat string
		var fees, rev float64
		if err := rows.Scan(&name, &fees, &rev, &cat); err != nil {
			return err
		}
		rec.Append([]format.Cell{format.Str(name), format.USD(fees), format.USD(rev), format.Str(cat)})
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1, 2})})
}

func runFeesHistory(cx *Ctx, slug, period string) error {
	days, err := parsePeriod(period)
	if err != nil {
		return err
	}
	if err := ensureProtocolHistory(cx, slug); err != nil {
		return err
	}
	q := `SELECT date, fees, revenue FROM fees_hist WHERE protocol = ?`
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
	rec := format.NewRecorder([]string{"DATE", "FEES", "REVENUE"})
	for rows.Next() {
		var date string
		var fees, rev float64
		if err := rows.Scan(&date, &fees, &rev); err != nil {
			return err
		}
		rec.Append([]format.Cell{format.Str(date), format.USD(fees), format.USD(rev)})
	}
	if len(rec.Rows) == 0 {
		return fmt.Errorf("no fees history for %q (try `sync --protocol %s`)", slug, slug)
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1, 2})})
}

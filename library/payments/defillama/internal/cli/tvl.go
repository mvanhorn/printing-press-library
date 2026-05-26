package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newTVLCmd() *cobra.Command {
	var chain, period string
	var history bool
	cmd := &cobra.Command{
		Use:   "tvl [protocol]",
		Short: "Protocol or chain TVL (current or historical)",
		Args:  cobra.MaximumNArgs(1),
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if chain != "" {
				return runTVLChain(cx, chain)
			}
			if len(args) == 0 {
				return fmt.Errorf("tvl requires a protocol name or --chain")
			}
			if err := ensureFresh(context.Background(), cx, []string{"protocols"}); err != nil {
				return err
			}
			slug, name, err := resolveProtocol(cx.S, args[0])
			if err != nil {
				return err
			}
			if history {
				return runTVLHistory(cx, slug, period)
			}
			var tvl sql.NullFloat64
			if err := cx.S.QueryRow(`SELECT tvl FROM protocols WHERE slug = ?`, slug).Scan(&tvl); err != nil {
				return err
			}
			rec := format.NewRecorder([]string{"PROTOCOL", "TVL"})
			rec.Append([]format.Cell{format.Str(name), format.USD(tvl.Float64)})
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1})})
		}),
	}
	cmd.Flags().StringVar(&chain, "chain", "", "show all protocols on chain")
	cmd.Flags().BoolVar(&history, "history", false, "show historical TVL (requires sync --protocol first)")
	cmd.Flags().StringVar(&period, "period", "30d", "period: 7d|30d|90d|180d|1y|all")
	return cmd
}

func runTVLChain(cx *Ctx, chain string) error {
	if err := ensureFresh(context.Background(), cx, []string{"protocols"}); err != nil {
		return err
	}
	lim := G.Limit
	if lim <= 0 {
		lim = 30
	}
	q := `SELECT p.name, pct.tvl, COALESCE(p.category,'')
	      FROM protocol_chain_tvl pct JOIN protocols p ON p.slug = pct.protocol_slug
	      WHERE LOWER(pct.chain) = LOWER(?) AND pct.tvl > 0 AND COALESCE(p.category,'') != 'CEX'
	      ORDER BY pct.tvl DESC LIMIT ?`
	rows, err := cx.S.Query(q, chain, lim)
	if err != nil {
		return err
	}
	defer rows.Close()
	rec := format.NewRecorder([]string{"PROTOCOL", "TVL", "CATEGORY"})
	for rows.Next() {
		var name, cat string
		var tvl float64
		if err := rows.Scan(&name, &tvl, &cat); err != nil {
			return err
		}
		rec.Append([]format.Cell{format.Str(name), format.USD(tvl), format.Str(cat)})
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1})})
}

func runTVLHistory(cx *Ctx, slug, period string) error {
	days, err := parsePeriod(period)
	if err != nil {
		return err
	}
	if err := ensureProtocolHistory(cx, slug); err != nil {
		return err
	}
	q := `SELECT date, tvl FROM protocol_tvl_hist WHERE protocol_slug = ? AND chain = ''`
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
	rec := format.NewRecorder([]string{"DATE", "TVL"})
	for rows.Next() {
		var date string
		var tvl float64
		if err := rows.Scan(&date, &tvl); err != nil {
			return err
		}
		rec.Append([]format.Cell{format.Str(date), format.USD(tvl)})
	}
	if len(rec.Rows) == 0 {
		return fmt.Errorf("no TVL history for %q (try `sync --protocol %s`)", slug, slug)
	}
	return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1})})
}

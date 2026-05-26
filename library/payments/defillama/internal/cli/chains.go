package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newChainsCmd() *cobra.Command {
	var sortBy, with string
	cmd := &cobra.Command{
		Use:   "chains",
		Short: "Chain leaderboard by TVL",
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			deps := []string{"chains"}
			needStable := containsAny(with, "stablecoin-mcap")
			needDex := containsAny(with, "dex-volume")
			if needStable {
				deps = append(deps, "stablecoins")
			}
			if needDex {
				deps = append(deps, "dexs")
			}
			if err := ensureFresh(context.Background(), cx, deps); err != nil {
				return err
			}
			lim := G.Limit
			if lim <= 0 {
				lim = 30
			}
			sortCol := "tvl DESC"
			if sortBy == "name" {
				sortCol = "name ASC"
			}
			rows, err := cx.S.Query(fmt.Sprintf(`SELECT name, tvl, token_symbol FROM chains WHERE tvl > 0 ORDER BY %s LIMIT ?`, sortCol), lim)
			if err != nil {
				return err
			}
			defer rows.Close()
			headers := []string{"CHAIN", "TVL", "TOKEN"}
			right := []int{1}
			if needStable {
				headers = append(headers, "STABLE_MCAP")
				right = append(right, len(headers)-1)
			}
			if needDex {
				headers = append(headers, "DEX_VOL_24H")
				right = append(right, len(headers)-1)
			}
			rec := format.NewRecorder(headers)
			for rows.Next() {
				var name, token string
				var tvl float64
				if err := rows.Scan(&name, &tvl, &token); err != nil {
					return err
				}
				row := []format.Cell{format.Str(name), format.USD(tvl), format.Str(token)}
				if needStable {
					var s float64
					_ = cx.S.QueryRow(`SELECT COALESCE(SUM(circulating),0) FROM stablecoin_chains WHERE LOWER(chain) = LOWER(?)`, name).Scan(&s)
					row = append(row, format.USD(s))
				}
				if needDex {
					var v float64
					_ = cx.S.QueryRow(`SELECT COALESCE(SUM(total_24h),0) FROM dex_overview WHERE chains LIKE ?`, "%"+strings.ToLower(name)+"%").Scan(&v)
					row = append(row, format.USD(v))
				}
				rec.Append(row)
			}
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet(right)})
		}),
	}
	cmd.Flags().StringVar(&sortBy, "sort", "tvl", "sort key: tvl|name")
	cmd.Flags().StringVar(&with, "with", "", "extras: stablecoin-mcap,dex-volume")
	return cmd
}

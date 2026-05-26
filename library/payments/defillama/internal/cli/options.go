package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newOptionsCmd() *cobra.Command {
	var chain string
	cmd := &cobra.Command{
		Use:   "options",
		Short: "Options DEX volume overview",
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			if err := ensureFresh(context.Background(), cx, []string{"options"}); err != nil {
				return err
			}
			lim := G.Limit
			if lim <= 0 {
				lim = 30
			}
			where := []string{"total_24h >= 0"}
			qargs := []any{}
			if chain != "" {
				where = append(where, `LOWER(chains) LIKE LOWER(?)`)
				qargs = append(qargs, "%"+strings.ToLower(chain)+"%")
			}
			q := fmt.Sprintf(`SELECT display_name, protocol, total_24h, total_7d FROM options_overview WHERE %s ORDER BY total_24h DESC LIMIT ?`,
				strings.Join(where, " AND "))
			qargs = append(qargs, lim)
			rows, err := cx.S.Query(q, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()
			rec := format.NewRecorder([]string{"PROTOCOL", "VOL_24H", "VOL_7D"})
			for rows.Next() {
				var display, slug string
				var v24, v7 float64
				if err := rows.Scan(&display, &slug, &v24, &v7); err != nil {
					return err
				}
				if display == "" {
					display = slug
				}
				rec.Append([]format.Cell{format.Str(display), format.USD(v24), format.USD(v7)})
			}
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet([]int{1, 2})})
		}),
	}
	cmd.Flags().StringVar(&chain, "chain", "", "filter by chain")
	return cmd
}

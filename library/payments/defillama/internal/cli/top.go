package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

func newTopCmd() *cobra.Command {
	var chain string
	var category string
	var sortBy string
	var with string
	var includeCEX bool
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Top protocols by TVL (and optionally fees/revenue/volume)",
		RunE: withCtx(func(cmd *cobra.Command, args []string, cx *Ctx) error {
			needFees := containsAny(with, "fees", "revenue")
			needVol := containsAny(with, "volume")

			deps := []string{"protocols"}
			if needFees {
				deps = append(deps, "fees")
			}
			if needVol {
				deps = append(deps, "dexs")
			}
			if err := ensureFresh(context.Background(), cx, deps); err != nil {
				return err
			}

			lim := G.Limit
			if lim <= 0 {
				lim = 20
			}

			// Column selection differs depending on whether we're filtered to a chain.
			perChain := chain != ""

			selects := []string{"p.name"}
			headers := []string{"PROTOCOL"}
			right := []int{}
			joins := ""
			where := []string{}
			qargs := []any{}

			if perChain {
				joins += " JOIN protocol_chain_tvl pct ON pct.protocol_slug = p.slug"
				where = append(where, "LOWER(pct.chain) = LOWER(?)", "pct.tvl > 0")
				qargs = append(qargs, chain)
				selects = append(selects, "pct.tvl")
				headers = append(headers, "TVL")
				right = append(right, len(headers)-1)
				if !includeCEX {
					where = append(where, "(p.category IS NULL OR p.category != 'CEX')")
				}
			} else {
				selects = append(selects, "p.tvl")
				headers = append(headers, "TVL")
				right = append(right, len(headers)-1)
				where = append(where, "p.tvl > 0")
			}

			if category != "" {
				where = append(where, "LOWER(p.category) = LOWER(?)")
				qargs = append(qargs, category)
			}

			if needFees {
				joins += " LEFT JOIN fees_overview f ON f.protocol = p.slug"
				selects = append(selects, "COALESCE(f.total_24h_fees,0)", "COALESCE(f.total_24h_rev,0)")
				headers = append(headers, "FEES_24H", "REV_24H")
				right = append(right, len(headers)-2, len(headers)-1)
			}
			if needVol {
				joins += " LEFT JOIN dex_overview d ON d.protocol = p.slug"
				selects = append(selects, "COALESCE(d.total_24h,0)")
				headers = append(headers, "VOL_24H")
				right = append(right, len(headers)-1)
			}
			selects = append(selects, "COALESCE(p.category,'')")
			headers = append(headers, "CATEGORY")

			sortCol := ""
			switch sortBy {
			case "", "tvl":
				if perChain {
					sortCol = "pct.tvl DESC"
				} else {
					sortCol = "p.tvl DESC"
				}
			case "fees":
				if !needFees {
					return fmt.Errorf("--sort fees requires --with fees")
				}
				sortCol = "f.total_24h_fees DESC"
			case "revenue":
				if !needFees {
					return fmt.Errorf("--sort revenue requires --with revenue")
				}
				sortCol = "f.total_24h_rev DESC"
			case "volume":
				if !needVol {
					return fmt.Errorf("--sort volume requires --with volume")
				}
				sortCol = "d.total_24h DESC"
			case "change_1d":
				sortCol = "p.change_1d DESC"
			case "change_7d":
				sortCol = "p.change_7d DESC"
			default:
				return fmt.Errorf("unknown --sort %q", sortBy)
			}

			query := fmt.Sprintf(
				"SELECT %s FROM protocols p%s WHERE %s ORDER BY %s LIMIT ?",
				strings.Join(selects, ", "), joins, strings.Join(where, " AND "), sortCol,
			)
			qargs = append(qargs, lim)

			rows, err := cx.S.Query(query, qargs...)
			if err != nil {
				return err
			}
			defer rows.Close()

			rec := format.NewRecorder(headers)
			for rows.Next() {
				var name string
				var tvl float64
				var feesV, revV, volV float64
				var cat string
				dest := []any{&name, &tvl}
				if needFees {
					dest = append(dest, &feesV, &revV)
				}
				if needVol {
					dest = append(dest, &volV)
				}
				dest = append(dest, &cat)
				if err := rows.Scan(dest...); err != nil {
					return err
				}
				vals := []format.Cell{
					format.Str(name),
					format.USD(tvl),
				}
				if needFees {
					vals = append(vals, format.USD(feesV), format.USD(revV))
				}
				if needVol {
					vals = append(vals, format.USD(volV))
				}
				vals = append(vals, format.Str(cat))
				rec.Append(vals)
			}
			return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader, RightAlign: rightSet(right)})
		}),
	}
	cmd.Flags().StringVar(&chain, "chain", "", "filter by chain (uses per-chain TVL; excludes CEX by default)")
	cmd.Flags().StringVar(&category, "category", "", "filter by category (e.g. Lending, DEX)")
	cmd.Flags().StringVar(&sortBy, "sort", "tvl", "sort key: tvl|fees|revenue|volume|change_1d|change_7d")
	cmd.Flags().StringVar(&with, "with", "", "extra columns: comma list of fees,revenue,volume")
	cmd.Flags().BoolVar(&includeCEX, "include-cex", false, "include CEX protocols when filtering by chain")
	return cmd
}

func containsAny(csv string, needles ...string) bool {
	if csv == "" {
		return false
	}
	parts := strings.Split(csv, ",")
	set := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		set[strings.ToLower(strings.TrimSpace(p))] = struct{}{}
	}
	for _, n := range needles {
		if _, ok := set[n]; ok {
			return true
		}
	}
	return false
}

func rightSet(idxs []int) map[int]bool {
	m := make(map[int]bool, len(idxs))
	for _, i := range idxs {
		m[i] = true
	}
	return m
}

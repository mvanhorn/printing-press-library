// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

func newNovelCompareMarketsCmd(flags *rootFlags) *cobra.Command {
	var markets []string
	var zip string
	var top int
	cmd := &cobra.Command{
		Use:   "markets <words>",
		Short: "The same search on several EDEKA markets side by side.",
		Long: "Use this command to compare one product across markets. Do NOT use it to search one market; use 'products search'.\n\n" +
			"Pass the markets with --market (repeatable) or --zip to compare every online market for a postal code.\n" +
			"Without either, the selected market is used. Works without login.",
		Example:     "  edeka-pp-cli compare markets milch --market doerflinger-ulm --json\n  edeka-pp-cli compare markets 'weide milch' --zip 89081",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "words=milch;--zip=89081"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare markets")
			}
			q := joinArgs(args)
			hosts := []string{}
			for _, m := range markets {
				for _, part := range strings.Split(m, ",") {
					if strings.TrimSpace(part) == "" {
						continue
					}
					h, err := edeka.NormalizeMarket(part)
					if err != nil {
						return usageErr(err)
					}
					hosts = append(hosts, h)
				}
			}
			if zip != "" {
				portal, err := openPortal(flags)
				if err != nil {
					return err
				}
				ms, err := portal.SearchMarkets(cmd.Context(), zip)
				if err != nil {
					return edekaErr(err)
				}
				for _, m := range ms {
					hosts = append(hosts, m.Host)
				}
			}
			if len(hosts) == 0 {
				sh, err := openPublicShop(flags, "")
				if err != nil {
					return err
				}
				hosts = append(hosts, sh.Host)
			}
			if len(hosts) > 8 {
				hosts = hosts[:8]
			}
			type row struct {
				Market   string          `json:"market"`
				Best     *edeka.Product  `json:"best_match,omitempty"`
				Products []edeka.Product `json:"products"`
				Error    string          `json:"error,omitempty"`
			}
			var rows []row
			for _, h := range hosts {
				sh, err := edeka.OpenWith(&edeka.Session{}, "", h)
				if err != nil {
					return err
				}
				g, err := sh.Search(cmd.Context(), q, 0)
				r := row{Market: edeka.MarketSlug(h), Products: []edeka.Product{}}
				if err != nil {
					r.Error = err.Error()
					rows = append(rows, r)
					continue
				}
				recordPrices(cmd.Context(), g.Products, h, "search")
				if b, ok := edeka.BestMatch(q, g.Products); ok {
					r.Best = &b
				}
				for i, p := range g.Products {
					if i >= top {
						break
					}
					r.Products = append(r.Products, p)
				}
				rows = append(rows, r)
			}
			found := false
			for _, r := range rows {
				if r.Best != nil {
					found = true
				}
			}
			if !found {
				return emitThen(cmd, flags, map[string]any{"query": q, "markets": rows}, notFoundErr(fmt.Errorf("no market has a product matching %q", q)))
			}
			return emit(cmd, flags, map[string]any{"query": q, "markets": rows}, func(w io.Writer) {
				for _, r := range rows {
					if r.Error != "" {
						fmt.Fprintf(w, "%-28s error: %s\n", r.Market, r.Error)
						continue
					}
					if r.Best == nil {
						fmt.Fprintf(w, "%-28s no match\n", r.Market)
						continue
					}
					fmt.Fprintf(w, "%-28s %-44s %9s  %s\n", r.Market, truncate(r.Best.Title, 44), r.Best.Price, r.Best.UnitPrice)
				}
			})
		},
	}
	cmd.Flags().StringSliceVar(&markets, "market", nil, "Market subdomain to compare (repeatable or comma-separated)")
	cmd.Flags().StringVar(&zip, "zip", "", "Compare every online market for this postal code")
	cmd.Flags().IntVar(&top, "top", 3, "Products listed per market")
	return cmd
}

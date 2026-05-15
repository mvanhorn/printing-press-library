// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/store"
)

type walletGroup struct {
	Wallet       string   `json:"wallet"`
	Count        int      `json:"count"`
	TotalBalance float64  `json:"total_balance_usdc"`
	Categories   []string `json:"categories"`
}

func newCampaignGroupByWalletCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "group-by-wallet",
		Short: "Group locally mirrored campaigns by their x402 payer wallet.",
		Long: `Bucket the locally mirrored campaigns by x402_payer_wallet and emit a
per-wallet summary: count, total balance, distinct categories. Useful for
spotting one advertiser running multiple campaigns from the same wallet
(common when an agency operates a stable of brand pages).

Reads from the local store only.`,
		Example: strings.Trim(`
  operon-pp-cli campaign group-by-wallet
  operon-pp-cli campaign group-by-wallet --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := dbPath
			if path == "" {
				path = store.DefaultPath("operon-pp-cli")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query store: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "would group campaigns_local by x402_payer_wallet\n")
				return nil
			}

			ctx := context.Background()
			st, err := store.Open(ctx, path)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer st.Close()

			grouped, err := st.GroupByWallet(ctx)
			if err != nil {
				return apiErr(err)
			}

			if flags.asJSON && flags.selectFields == "" && !flags.compact && !flags.csv {
				// Spec calls for "object with wallet -> []campaign" — we
				// emit a stable shape with wallets as keys.
				out := map[string][]map[string]any{}
				for wallet, campaigns := range grouped {
					rows := make([]map[string]any, 0, len(campaigns))
					for _, c := range campaigns {
						rows = append(rows, map[string]any{
							"campaign_id":  c.CampaignID,
							"service":      c.Service,
							"category":     c.Category,
							"status":       c.Status,
							"balance_usdc": c.BalanceUSDC,
							"trust_score":  c.TrustScore,
						})
					}
					out[wallet] = rows
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			// Summary rows for table or aggregated JSON.
			summaries := make([]walletGroup, 0, len(grouped))
			for wallet, campaigns := range grouped {
				wg := walletGroup{Wallet: wallet, Count: len(campaigns)}
				catSet := map[string]bool{}
				for _, c := range campaigns {
					if c.BalanceUSDC != nil {
						wg.TotalBalance += *c.BalanceUSDC
					}
					if c.Category != "" {
						catSet[c.Category] = true
					}
				}
				for c := range catSet {
					wg.Categories = append(wg.Categories, c)
				}
				sort.Strings(wg.Categories)
				summaries = append(summaries, wg)
			}
			sort.Slice(summaries, func(i, j int) bool {
				if summaries[i].Count != summaries[j].Count {
					return summaries[i].Count > summaries[j].Count
				}
				return summaries[i].Wallet < summaries[j].Wallet
			})

			if flags.csv || flags.compact || flags.selectFields != "" {
				return printJSONFiltered(cmd.OutOrStdout(), summaries, flags)
			}

			if len(summaries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No campaigns mirrored locally. Use `operon-pp-cli sync` after creating campaigns to populate the mirror.")
				return nil
			}
			headers := []string{"wallet", "count", "total_balance_usdc", "categories"}
			rows := make([][]string, 0, len(summaries))
			for _, s := range summaries {
				wallet := s.Wallet
				if wallet == "" {
					wallet = "(unknown)"
				}
				rows = append(rows, []string{
					wallet,
					fmt.Sprintf("%d", s.Count),
					fmt.Sprintf("%.2f", s.TotalBalance),
					strings.Join(s.Categories, ","),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the default store path")
	return cmd
}

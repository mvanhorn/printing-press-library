// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: structured lookup of a single credit card.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCardsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <slug>",
		Short: "Get a credit card's structured terms (fees, APRs, welcome bonus, rewards)",
		Long: strings.TrimSpace(`
Fetch one credit card by slug and return its structured terms: annual fee,
welcome bonus, ongoing APRs, rewards multipliers, and TPG rating. Find slugs
with 'cards list'. This is the clean structured view; the bare 'cards <slug>'
form returns the full raw page data.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli cards get chase-sapphire-preferred-card
  thepointsguy-pp-cli cards get american-express-gold-card --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "chase-sapphire-preferred-card"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch a card")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a card slug is required (see 'cards list')"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)
			card, err := c.CardDetail(ctx, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, card)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s\n", card.Name)
			if card.Superlative != "" {
				fmt.Fprintf(out, "  %s\n", card.Superlative)
			}
			fmt.Fprintf(out, "  Annual fee:     %s\n", dash(card.AnnualFee))
			if card.TPGRating > 0 {
				fmt.Fprintf(out, "  TPG rating:     %.1f/5\n", card.TPGRating)
			}
			fmt.Fprintf(out, "  Rec. credit:    %s\n", dash(card.RecommendedCredit))
			fmt.Fprintf(out, "  Welcome bonus:  %s\n", dash(card.WelcomeBonus))
			if len(card.APRs) > 0 {
				fmt.Fprintln(out, "  APRs:")
				for _, a := range card.APRs {
					fmt.Fprintf(out, "    - %s: %s\n", a.Name, a.DisplayText)
				}
			}
			if len(card.Rewards) > 0 {
				fmt.Fprintln(out, "  Rewards:")
				for _, r := range card.Rewards {
					fmt.Fprintf(out, "    - %s\n", r)
				}
			}
			fmt.Fprintf(out, "  URL:            %s\n", card.URL)
			return nil
		},
	}
	return cmd
}

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

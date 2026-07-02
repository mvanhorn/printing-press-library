// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: side-by-side comparison of two or more credit cards.
package cli

import (
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

// pp:data-source live
func newNovelCardsCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <slug> <slug> [slug...]",
		Short: "Compare two or more credit cards side by side",
		Long: strings.TrimSpace(`
Fetch two or more cards by slug and compare annual fee, welcome bonus, ongoing
APR range, rewards, and TPG rating side by side. Find slugs with 'cards list'.
Cards that fail to load are reported and excluded from the comparison.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli cards compare chase-sapphire-preferred-card chase-sapphire-reserve
  thepointsguy-pp-cli cards compare american-express-gold-card capital-one-venture --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "chase-sapphire-preferred-card;chase-sapphire-reserve"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare cards")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide at least two card slugs to compare (see 'cards list')"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)

			type result struct {
				idx  int
				card *tpg.Card
				err  error
			}
			results := make([]result, len(args))
			var wg sync.WaitGroup
			for i, slug := range args {
				wg.Add(1)
				go func(i int, slug string) {
					defer wg.Done()
					card, err := c.CardDetail(ctx, slug)
					results[i] = result{idx: i, card: card, err: err}
				}(i, slug)
			}
			wg.Wait()

			cards := make([]*tpg.Card, 0, len(args))
			failures := make([]cardFailure, 0)
			for i, r := range results {
				if r.err != nil || r.card == nil {
					msg := "no data"
					if r.err != nil {
						msg = r.err.Error()
					}
					failures = append(failures, cardFailure{Slug: args[i], Error: msg})
					continue
				}
				cards = append(cards, r.card)
			}
			if len(failures) > 0 {
				var slugs []string
				for _, f := range failures {
					slugs = append(slugs, f.Slug)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d card(s) could not be loaded and were excluded: %s\n",
					len(failures), strings.Join(slugs, ", "))
			}
			if len(cards) < 2 {
				if flags.asJSON || flags.agent {
					return emitJSON(cmd, flags, map[string]any{"error": "fewer than two cards loaded", "cards": cards, "fetch_failures": failures})
				}
				return notFoundErr(fmt.Errorf("could not load at least two cards to compare"))
			}

			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, struct {
					Cards         []*tpg.Card   `json:"cards"`
					FetchFailures []cardFailure `json:"fetch_failures"`
				}{cards, failures})
			}
			// Human table: one row per attribute, one column per card.
			w := newTabWriter(cmd.OutOrStdout())
			header := "ATTRIBUTE"
			for _, c := range cards {
				header += "\t" + truncRunes(c.Name, 28)
			}
			fmt.Fprintln(w, header)
			row := func(label string, val func(*tpg.Card) string) {
				line := label
				for _, c := range cards {
					line += "\t" + truncRunes(val(c), 28)
				}
				fmt.Fprintln(w, line)
			}
			row("Annual fee", func(c *tpg.Card) string { return c.AnnualFee })
			row("TPG rating", func(c *tpg.Card) string {
				if c.TPGRating == 0 {
					return "-"
				}
				return fmt.Sprintf("%.1f", c.TPGRating)
			})
			row("Rec. credit", func(c *tpg.Card) string { return c.RecommendedCredit })
			row("Welcome bonus", func(c *tpg.Card) string { return c.WelcomeBonus })
			row("Ongoing APR", func(c *tpg.Card) string { return ongoingAPR(c) })
			row("Top reward", func(c *tpg.Card) string {
				if len(c.Rewards) == 0 {
					return "-"
				}
				return c.Rewards[0]
			})
			return w.Flush()
		},
	}
	return cmd
}

// cardFailure records a card slug that could not be loaded.
type cardFailure struct {
	Slug  string `json:"slug"`
	Error string `json:"error"`
}

func ongoingAPR(c *tpg.Card) string {
	for _, a := range c.APRs {
		if strings.Contains(strings.ToLower(a.Name), "purchase") {
			return a.DisplayText
		}
	}
	if len(c.APRs) > 0 {
		return c.APRs[0].DisplayText
	}
	return "-"
}

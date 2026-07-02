// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: cards recommended on a TPG best-of / category page.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

func newCardsBestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "best <category>",
		Short: "List the cards The Points Guy recommends for a category",
		Long: strings.TrimSpace(`
List the card slugs featured on a TPG best-of or category page, e.g. "travel",
"airline", "no-annual-fee", "airport-lounge-access", "best". Use the slugs with
'cards get' or 'cards compare'.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli cards best travel
  thepointsguy-pp-cli cards best airport-lounge-access --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "travel"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list recommended cards")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a category is required, e.g. travel, airline, no-annual-fee"))
			}
			category := strings.ToLower(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)
			slugs, err := c.CategoryCardSlugs(ctx, category)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type cardRef struct {
				Slug string `json:"slug"`
				URL  string `json:"url"`
			}
			refs := make([]cardRef, 0, len(slugs))
			for _, s := range slugs {
				refs = append(refs, cardRef{Slug: s, URL: tpg.BaseURL + "/credit-cards/" + s + "/"})
			}
			if len(refs) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no cards found for category %q; try 'cards best travel' or 'cards list --categories'\n", category)
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, struct {
					Category string    `json:"category"`
					Count    int       `json:"count"`
					Cards    []cardRef `json:"cards"`
				}{category, len(refs), refs})
			}
			for _, r := range refs {
				fmt.Fprintln(cmd.OutOrStdout(), r.Slug)
			}
			return nil
		},
	}
	return cmd
}

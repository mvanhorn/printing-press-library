// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: list credit-card slugs from the card sitemap.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

func newCardsListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var includeCategories bool
	var filter string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List credit-card slugs from The Points Guy's card database",
		Long: strings.TrimSpace(`
List credit-card page slugs from the card sitemap. Use these slugs with
'cards get' and 'cards compare'. --filter narrows by substring; --categories
also includes category index pages (best-of, issuer, network).`),
		Example: strings.Trim(`
  thepointsguy-pp-cli cards list --filter sapphire
  thepointsguy-pp-cli cards list --limit 20 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list card slugs")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)
			slugs, err := c.CardSlugs(ctx, includeCategories)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			type cardRef struct {
				Slug string `json:"slug"`
				URL  string `json:"url"`
			}
			refs := make([]cardRef, 0, len(slugs))
			for _, s := range slugs {
				if filter != "" && !strings.Contains(s, strings.ToLower(filter)) {
					continue
				}
				refs = append(refs, cardRef{Slug: s, URL: tpg.BaseURL + "/credit-cards/" + s + "/"})
				if limit > 0 && len(refs) >= limit {
					break
				}
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, struct {
					Count int       `json:"count"`
					Cards []cardRef `json:"cards"`
				}{len(refs), refs})
			}
			for _, r := range refs {
				fmt.Fprintln(cmd.OutOrStdout(), r.Slug)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d cards\n", len(refs))
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum slugs to return (0 = all)")
	cmd.Flags().StringVar(&filter, "filter", "", "Only show slugs containing this substring")
	cmd.Flags().BoolVar(&includeCategories, "categories", false, "Include category index pages (best-of, issuer, network)")
	return cmd
}

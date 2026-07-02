// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed command: look up a points-and-miles glossary term.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

func newGlossaryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "glossary <term>",
		Short: "Look up a points-and-miles term in The Points Guy glossary",
		Long: strings.TrimSpace(`
Look up a term in The Points Guy glossary and return its definition. The term
is slugified (spaces become hyphens), e.g. "points redemption" resolves to
/glossary/what-is-points-redemption/ when a direct match is not found.`),
		Example: strings.Trim(`
  thepointsguy-pp-cli glossary "positioning flight"
  thepointsguy-pp-cli glossary award-chart --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "award-chart"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would look up a glossary term")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a term is required, e.g. \"award chart\""))
			}
			term := strings.Join(args, " ")
			slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(term), " ", "-"))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newTPGClient(flags)

			// TPG glossary URLs are phrased as questions and the article varies
			// (a/an/are), so try the common slug shapes in order.
			candidates := []string{
				"/glossary/" + slug + "/",
				"/glossary/what-is-" + slug + "/",
				"/glossary/what-is-a-" + slug + "/",
				"/glossary/what-is-an-" + slug + "/",
				"/glossary/what-are-" + slug + "/",
			}
			for _, path := range candidates {
				meta, err := c.PageMetadata(ctx, path)
				if err == nil && meta.Title != "" {
					def := cleanDefinition(firstNonEmptyStr(meta.Body, meta.Description))
					view := struct {
						Term       string `json:"term"`
						Title      string `json:"title"`
						Definition string `json:"definition"`
						URL        string `json:"url"`
						Exact      bool   `json:"exact_glossary_match"`
					}{term, meta.Title, def, meta.URL, true}
					if flags.asJSON || flags.agent {
						return emitJSON(cmd, flags, view)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Title)
					if def != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", def)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\nFull definition: %s\n", view.URL)
					return nil
				}
			}

			// No exact glossary page: fall back to a content search so the user
			// still gets a relevant, honest pointer instead of an error.
			res, serr := c.Search(ctx, tpg.IndexContent, term, 3)
			if serr != nil || len(res.Hits) == 0 {
				return notFoundErr(fmt.Errorf("no glossary entry or article found for %q", term))
			}
			top := res.Hits[0]
			meta, _ := c.PageMetadata(ctx, top.URL)
			def := ""
			if meta != nil {
				def = firstNonEmptyStr(meta.Description, meta.Body)
			}
			view := struct {
				Term       string `json:"term"`
				Title      string `json:"title"`
				Definition string `json:"definition"`
				URL        string `json:"url"`
				Exact      bool   `json:"exact_glossary_match"`
				Note       string `json:"note"`
			}{term, top.Title, def, top.URL, false, "no exact glossary page; showing the closest article"}
			fmt.Fprintf(cmd.ErrOrStderr(), "no exact glossary entry for %q; closest article:\n", term)
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, view)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n%s\n\n%s\n", top.Title, def, top.URL)
			return nil
		},
	}
	return cmd
}

// cleanDefinition drops the generic site tagline so glossary never presents it
// as a real definition.
func cleanDefinition(s string) string {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "maximize your travel.") || strings.EqualFold(s, "maximize your travel") {
		return ""
	}
	return s
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

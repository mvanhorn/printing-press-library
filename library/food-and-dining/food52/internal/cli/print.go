// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newPrintCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print <slug-or-url>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Print a clean cooking-mode view of a Food52 recipe (no ads, no nav, no images)",
		Long: strings.TrimSpace(`
Renders a recipe to stdout as a fixed-width view: title, yield, total time,
numbered ingredient list, numbered steps. Strip-down designed for piping to
'lp' or pasting into notes. No metadata noise (no author bio, no related
products, no comments).
`),
		Example: strings.Trim(`
  food52-pp-cli print sarah-fennel-s-best-lunch-lady-brownie-recipe
  food52-pp-cli print sarah-fennel-s-best-lunch-lady-brownie-recipe | lp
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := recipeSlugFromArg(args[0])
			if slug == "" {
				return fmt.Errorf("recipe slug or URL is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			html, err := fetchHTML(c, "/recipes/"+slug, nil)
			if err != nil {
				return classifyAPIError(err)
			}
			if c.DryRun {
				// fetchHTML printed the would-be URL to stderr and returned a
				// stub body. There's no recipe to render.
				return nil
			}
			r, err := food52.ExtractRecipe(html, canonicalRecipeURL(slug))
			if err != nil {
				return err
			}
			renderPrintView(os.Stdout, r)
			return nil
		},
	}
	return cmd
}

func renderPrintView(w *os.File, r *food52.Recipe) {
	if r == nil {
		return
	}
	fmt.Fprintln(w, r.Title)
	fmt.Fprintln(w, strings.Repeat("=", min(len(r.Title), 78)))
	if r.AuthorName != "" {
		fmt.Fprintf(w, "by %s\n", r.AuthorName)
	}
	if r.Yield != "" {
		fmt.Fprintf(w, "Yield: %s\n", r.Yield)
	}
	timeLine := []string{}
	if r.PrepTime != "" {
		timeLine = append(timeLine, "prep "+r.PrepTime)
	}
	if r.CookTime != "" {
		timeLine = append(timeLine, "cook "+r.CookTime)
	}
	if r.TotalTime != "" {
		timeLine = append(timeLine, "total "+r.TotalTime)
	}
	if len(timeLine) > 0 {
		fmt.Fprintln(w, "Time: "+strings.Join(timeLine, " · "))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "INGREDIENTS")
	fmt.Fprintln(w, "-----------")
	for _, ing := range r.Ingredients {
		fmt.Fprintf(w, "  %s\n", ing)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "STEPS")
	fmt.Fprintln(w, "-----")
	for i, step := range r.Instructions {
		fmt.Fprintf(w, "%d. %s\n\n", i+1, step)
	}
	fmt.Fprintln(w, r.URL)
}

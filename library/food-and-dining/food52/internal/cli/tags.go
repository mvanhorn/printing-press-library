// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/food52/internal/food52"
)

func newTagsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Discover Food52 recipe tags and filter the curated taxonomy",
	}
	cmd.AddCommand(newTagsListCmd(flags))
	return cmd
}

func newTagsListCmd(flags *rootFlags) *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List Food52 recipe tags discovered from the homepage navigation, optionally filtered by kind",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: strings.TrimSpace(`
List the curated Food52 tag taxonomy. Tags are grouped by kind:
meal, course, ingredient, cuisine, lifestyle, preparation, convenience.

Pass --kind to narrow the listing (e.g., --kind cuisine). The slugs returned
here are the values that 'recipes browse <tag>' and 'recipes search --tag'
expect.
`),
		Example: strings.Trim(`
  food52-pp-cli tags list
  food52-pp-cli tags list --kind cuisine
  food52-pp-cli tags list --kind meal --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			tags := food52.FilterTagsByKind(kind)
			payload := map[string]any{
				"kind":  kind,
				"kinds": food52.AllTagKinds(),
				"count": len(tags),
				"tags":  tags,
			}
			return emitFromFlags(flags, payload, func() {
				if kind == "" {
					fmt.Printf("All curated tags (%d)\n", len(tags))
				} else {
					fmt.Printf("Tags in %s (%d)\n", kind, len(tags))
				}
				for _, t := range tags {
					fmt.Printf("  %-30s  %s  (%s)\n", t.Slug, t.Title, t.Kind)
				}
			})
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Filter to one kind: "+strings.Join(food52.AllTagKinds(), ", "))
	return cmd
}

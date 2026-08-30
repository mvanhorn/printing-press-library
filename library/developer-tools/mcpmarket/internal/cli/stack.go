// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed
// This command computes results from locally stored history (resource_snapshots)
// built up as the user browses; it does not read a single upstream resource type.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/mcpmarket/internal/client"
)

type stackEntry struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Hop         int    `json:"hop"`
}

// serverCategorySlug fetches a server's detail page and pulls its category
// slug out of the "Related MCP Servers" isRelatedTo entry, since the JSON-LD
// applicationCategory field is schema.org's generic type, not the MCP Market
// category slug the similar-tools endpoint requires.
func serverCategorySlug(ctx context.Context, c *client.Client, slug string) (string, error) {
	raw, err := c.Get(ctx, "/server/"+slug, nil)
	if err != nil {
		return "", err
	}
	data, err := extractHTMLResponse(raw, htmlExtractionOptions{
		Mode:           "embedded-json",
		ScriptSelector: `script[type="application/ld+json"]`,
	})
	if err != nil {
		return "", err
	}
	var entity struct {
		IsRelatedTo []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"isRelatedTo"`
	}
	if err := json.Unmarshal(data, &entity); err != nil {
		return "", err
	}
	for _, rel := range entity.IsRelatedTo {
		if strings.Contains(strings.ToLower(rel.Name), "related mcp servers") {
			return slugFromMCPMarketURL(rel.URL), nil
		}
	}
	return "", fmt.Errorf("could not determine category for server %q", slug)
}

func newNovelStackCmd(flags *rootFlags) *cobra.Command {
	var flagDepth int

	cmd := &cobra.Command{
		Use:         "stack <server>",
		Short:       "Chain the similar-tools recommendation graph multiple hops to build a ranked shortlist around a server you already use.",
		Example:     "  mcpmarket-pp-cli stack firecrawl --depth 2 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3", "pp:happy-args": "server=firecrawl"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "stack")
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("server slug is required"))
			}
			depth := flagDepth
			if depth < 1 {
				depth = 1
			}
			if depth > 3 {
				depth = 3
			}
			root := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if _, err := serverCategorySlug(ctx, c, root); err != nil {
				return notFoundErr(fmt.Errorf("server %q not found: %w", root, err))
			}

			visited := map[string]bool{root: true}
			frontier := []string{root}
			results := make([]stackEntry, 0)

			for hop := 1; hop <= depth && len(frontier) > 0; hop++ {
				next := make([]string, 0)
				for _, slug := range frontier {
					category, err := serverCategorySlug(ctx, c, slug)
					if err != nil {
						continue
					}
					raw, err := c.Get(ctx, "/api/similar-tools", map[string]string{
						"slug": slug, "category": category, "type": "server",
					})
					if err != nil {
						continue
					}
					var related []struct {
						Slug        string `json:"slug"`
						Name        string `json:"name"`
						Description string `json:"description"`
					}
					if err := json.Unmarshal(raw, &related); err != nil {
						continue
					}
					for _, r := range related {
						if visited[r.Slug] {
							continue
						}
						visited[r.Slug] = true
						results = append(results, stackEntry{
							Slug: r.Slug, Name: r.Name, Description: r.Description, Hop: hop,
						})
						next = append(next, r.Slug)
					}
				}
				frontier = next
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"root": root, "depth": depth, "count": len(results), "stack": results,
			}, flags)
		},
	}
	cmd.Flags().IntVar(&flagDepth, "depth", 1, "how many hops to walk the similar-tools graph (1-3)")
	return cmd
}

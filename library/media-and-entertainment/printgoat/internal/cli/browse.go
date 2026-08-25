// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Absorbed-feature command family: `browse category|user`. Consolidates the
// per-source category/user browsing every competing single-site tool offers
// into one unified command per site.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/client"
	"github.com/spf13/cobra"
)

func newBrowseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "browse",
		Short:       "browse subcommands: category, user",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newBrowseCategoryCmd(flags))
	cmd.AddCommand(newBrowseUserCmd(flags))
	return cmd
}

func newBrowseCategoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "category <source> [category]",
		Short:   "Browse a source's categories, or list things/creations within one",
		Example: "  printgoat-pp-cli browse category thingiverse\n  printgoat-pp-cli browse category thingiverse 3d-printer-parts --agent",
		Long: `With just a source, lists that source's top-level categories.
With a category slug/name added, lists items within that category.

Printables does not expose a confirmed category-browse query from research
for this build; requesting it returns a clear "not supported" result rather
than a guess.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("browse category requires <source>\nUsage: %s <source> [category]", cmd.CommandPath()))
			}
			source := args[0]
			if !validSource(source) {
				return usageErr(fmt.Errorf("invalid source %q: expected printables, thingiverse, or cults3d", source))
			}
			var category string
			if len(args) > 1 {
				category = args[1]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			switch source {
			case "thingiverse":
				if category == "" {
					data, err := c.Get(ctx, "https://api.thingiverse.com/categories", nil)
					if err != nil {
						return classifyAPIError(err, flags)
					}
					return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
				}
				data, err := c.Get(ctx, "https://api.thingiverse.com/categories/"+url.PathEscape(category)+"/things", nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
			case "cults3d":
				return browseCults3DCategories(ctx, c, category, cmd, flags)
			case "printables":
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"source":  "printables",
					"message": "Printables category browsing has no confirmed query from research for this build; use 'search' with a keyword instead.",
				}, flags)
			default:
				return usageErr(fmt.Errorf("invalid source %q", source))
			}
		},
	}
	return cmd
}

func browseCults3DCategories(ctx context.Context, c *client.Client, category string, cmd *cobra.Command, flags *rootFlags) error {
	body := map[string]any{
		"query": `query Categories { categories { id name } }`,
	}
	data, _, err := c.Post(ctx, "https://cults3d.com/graphql", body)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	var envelope struct {
		Data struct {
			Categories json.RawMessage `json:"categories"`
		} `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("parsing Cults3D categories response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("cults3d graphql error: %v", envelope.Errors)
	}
	if category != "" {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"source":  "cults3d",
			"message": "Cults3D category-scoped item listing has no confirmed query from research for this build; showing the full category list instead.",
			"categories": json.RawMessage(func() []byte {
				if len(envelope.Data.Categories) == 0 {
					return []byte("[]")
				}
				return envelope.Data.Categories
			}()),
		}, flags)
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), envelope.Data.Categories, flags, map[string]any{"source": "live"})
}

func newBrowseUserCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "user <source> <handle>",
		Short:       "Browse a designer/user's published things or creations on a given source",
		Example:     "  printgoat-pp-cli browse user thingiverse PrintedSolid --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("browse user requires <source> and <handle>\nUsage: %s <source> <handle>", cmd.CommandPath()))
			}
			source, handle := args[0], args[1]
			if !validSource(source) {
				return usageErr(fmt.Errorf("invalid source %q: expected printables, thingiverse, or cults3d", source))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			switch source {
			case "thingiverse":
				data, err := c.Get(ctx, "https://api.thingiverse.com/users/"+url.PathEscape(handle)+"/things", nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), data, flags, map[string]any{"source": "live"})
			case "cults3d":
				// User.creationsBatch does not exist (confirmed via live
				// GraphQL introspection against cults3d.com/graphql): the
				// real field is `creations`, which returns a bare array
				// directly since it lacks the "Batch" suffix that marks a
				// {results, total} envelope wrapper elsewhere in this
				// schema (see novel_shared.go's cults3DSearchQuery comment).
				body := map[string]any{
					"query": `query UserCreations($nick: String!) { user(nick: $nick) { nick shortUrl bio creationsCount creations(limit: 20) { identifier name shortUrl likesCount downloadsCount } } }`,
					"variables": map[string]any{
						"nick": handle,
					},
				}
				data, _, err := c.Post(ctx, "https://cults3d.com/graphql", body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var envelope struct {
					Data struct {
						User json.RawMessage `json:"user"`
					} `json:"data"`
					Errors []map[string]any `json:"errors"`
				}
				if err := json.Unmarshal(data, &envelope); err != nil {
					return fmt.Errorf("parsing Cults3D user response: %w", err)
				}
				if len(envelope.Errors) > 0 {
					return fmt.Errorf("cults3d graphql error: %v", envelope.Errors)
				}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), envelope.Data.User, flags, map[string]any{"source": "live"})
			case "printables":
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"source":  "printables",
					"message": "Printables user-scoped print listing has no confirmed query from research for this build; use 'search' and check the 'designer' field on results instead.",
				}, flags)
			default:
				return usageErr(fmt.Errorf("invalid source %q", source))
			}
		},
	}
	return cmd
}

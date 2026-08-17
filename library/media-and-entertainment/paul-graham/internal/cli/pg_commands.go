// Copyright 2026 Deb Mukherjee and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: Paul Graham static-site commands layered on the generated articles.html wrapper.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/paul-graham/internal/pg"
	"github.com/spf13/cobra"
)

func newLatestCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "latest",
		Short: "List the newest Paul Graham essays from articles.html",
		Example: `  paul-graham-pp-cli latest --limit 10
  paul-graham-pp-cli latest --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := loadPGIndex(cmd.Context(), flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printPGEssays(cmd, flags, pg.Filter(index.Essays, "", limit))
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum essays to return")
	return cmd
}

func newListCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Paul Graham essays from the canonical essay index",
		Example: `  paul-graham-pp-cli list --query startup --limit 20
  paul-graham-pp-cli list --json --select title,url`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := loadPGIndex(cmd.Context(), flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printPGEssays(cmd, flags, pg.Filter(index.Essays, query, limit))
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Filter by title or slug")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum essays to return (0 for all)")
	return cmd
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var fullText bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Paul Graham essay titles, slugs, and optionally full essay text",
		Args:  cobra.ExactArgs(1),
		Example: `  paul-graham-pp-cli search startup --limit 10
  paul-graham-pp-cli search "default alive" --full-text --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := loadPGIndex(cmd.Context(), flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if fullText {
				results, err := pg.SearchFullText(cmd.Context(), index.Essays, args[0], flags.timeout, limit)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return printPGJSONOrText(cmd, flags, results, func() {
					w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
					fmt.Fprintln(w, "ORDER\tWORDS\tTITLE\tURL\tEXCERPT")
					for _, essay := range results {
						fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\n", essay.Order, essay.WordCount, essay.Title, essay.URL, essay.Excerpt)
					}
					w.Flush()
				})
			}
			return printPGEssays(cmd, flags, pg.Filter(index.Essays, args[0], limit))
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum essays to return")
	cmd.Flags().BoolVar(&fullText, "full-text", false, "Fetch matching essay pages and search body text too")
	return cmd
}

func newReadEssayCmd(flags *rootFlags) *cobra.Command {
	var maxChars int
	cmd := &cobra.Command{
		Use:   "read <slug|url|title>",
		Short: "Read a Paul Graham essay by slug, URL, or title",
		Args:  cobra.ExactArgs(1),
		Example: `  paul-graham-pp-cli read greatwork
  paul-graham-pp-cli read "Founder Mode" --max-chars 2000 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := loadPGIndex(cmd.Context(), flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			essay, ok := pg.Find(index.Essays, args[0])
			if !ok {
				return notFoundErr(fmt.Errorf("essay not found in articles.html: %s", args[0]))
			}
			text, err := pg.Read(cmd.Context(), essay, flags.timeout)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if maxChars > 0 && len(text.Text) > maxChars {
				runes := []rune(text.Text)
				if len(runes) > maxChars {
					text.Text = strings.TrimSpace(string(runes[:maxChars])) + "..."
				}
			}
			return printPGJSONOrText(cmd, flags, text, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n%d words\n\n%s\n", text.Title, text.URL, text.WordCount, text.Text)
			})
		},
	}
	cmd.Flags().IntVar(&maxChars, "max-chars", 0, "Maximum essay text characters to print (0 for full text)")
	return cmd
}

func newLinksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "links <slug|url|title>",
		Short: "Extract links from a Paul Graham essay page",
		Args:  cobra.ExactArgs(1),
		Example: `  paul-graham-pp-cli links greatwork
  paul-graham-pp-cli links "Founder Mode" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := loadPGIndex(cmd.Context(), flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			essay, ok := pg.Find(index.Essays, args[0])
			if !ok {
				return notFoundErr(fmt.Errorf("essay not found in articles.html: %s", args[0]))
			}
			links, err := pg.Links(cmd.Context(), essay, flags.timeout)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printPGJSONOrText(cmd, flags, links, func() {
				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "TEXT\tURL")
				for _, link := range links {
					fmt.Fprintf(w, "%s\t%s\n", link.Text, link.URL)
				}
				w.Flush()
			})
		},
	}
	return cmd
}

func newRandomEssayCmd(flags *rootFlags) *cobra.Command {
	var seed int64
	cmd := &cobra.Command{
		Use:   "random",
		Short: "Pick a random Paul Graham essay from the index",
		Example: `  paul-graham-pp-cli random
  paul-graham-pp-cli random --seed 42 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := loadPGIndex(cmd.Context(), flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			essay, ok := pg.Random(index.Essays, seed)
			if !ok {
				return notFoundErr(fmt.Errorf("no essays found in articles.html"))
			}
			return printPGEssays(cmd, flags, []pg.Essay{essay})
		},
	}
	cmd.Flags().Int64Var(&seed, "seed", 0, "Deterministic random seed (0 uses current time)")
	return cmd
}

func loadPGIndex(ctx context.Context, flags *rootFlags) (pg.Index, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return pg.FetchIndex(ctx, flags.timeout)
}

func printPGEssays(cmd *cobra.Command, flags *rootFlags, essays []pg.Essay) error {
	return printPGJSONOrText(cmd, flags, essays, func() {
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ORDER\tSLUG\tTITLE\tURL")
		for _, essay := range essays {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", essay.Order, essay.Slug, essay.Title, essay.URL)
		}
		w.Flush()
	})
}

func printPGJSONOrText(cmd *cobra.Command, flags *rootFlags, value any, printText func()) error {
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		} else if flags.compact {
			data = compactFields(data)
		}
		return printOutput(cmd.OutOrStdout(), data, true)
	}
	printText()
	return nil
}

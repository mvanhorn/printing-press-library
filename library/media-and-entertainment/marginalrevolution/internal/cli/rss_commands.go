// Copyright 2026 hinuri. Licensed under Apache-2.0. See LICENSE.
// PATCH: RSS-native commands layered on the v4.2.1 generated Marginal Revolution feed wrapper.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/marginalrevolution/internal/mr"
	"github.com/spf13/cobra"
)

func newLatestCmd(flags *rootFlags) *cobra.Command {
	var author, category string
	var limit int
	cmd := &cobra.Command{
		Use:   "latest",
		Short: "List recent Marginal Revolution posts",
		Example: `  marginalrevolution-pp-cli latest --limit 5
  marginalrevolution-pp-cli latest --author Tyler --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadMRFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			return printMRItems(cmd, flags, mr.Filter(feed.Items, "", author, category, limit))
		},
	}
	cmd.Flags().StringVar(&author, "author", "", "Filter by author name")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum posts to return")
	return cmd
}

func newSearchFeedCmd(flags *rootFlags) *cobra.Command {
	var author, category string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search titles and body text in the current RSS feed",
		Args:  cobra.ExactArgs(1),
		Example: `  marginalrevolution-pp-cli search ai --agent
  marginalrevolution-pp-cli search economics --category Economics --limit 5`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadMRFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			return printMRItems(cmd, flags, mr.Filter(feed.Items, args[0], author, category, limit))
		},
	}
	cmd.Flags().StringVar(&author, "author", "", "Filter by author name")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum posts to return")
	return cmd
}

func newReadPostCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "read <url|guid|title>",
		Short: "Read a post currently present in the RSS feed",
		Args:  cobra.ExactArgs(1),
		Example: `  marginalrevolution-pp-cli read "self-fulfilling-misalignment"
  marginalrevolution-pp-cli read https://marginalrevolution.com/marginalrevolution/2026/05/self-fulfilling-misalignment.html --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadMRFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			item, ok := mr.Find(feed.Items, args[0])
			if !ok {
				return notFoundErr(fmt.Errorf("post not found in current feed: %s", args[0]))
			}
			return printMRJSONOrText(cmd, flags, item, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n%s\n\n%s\n", item.Title, item.Author, item.Link, item.ContentText)
			})
		},
	}
}

func newCategoriesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "categories",
		Short: "Show category counts in the current feed",
		Example: `  marginalrevolution-pp-cli categories
  marginalrevolution-pp-cli categories --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadMRFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			return printMRCounts(cmd, flags, mr.SortedCounts(mr.CategoryCounts(feed.Items)))
		},
	}
}

func newAuthorsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "authors",
		Short: "Show author counts in the current feed",
		Example: `  marginalrevolution-pp-cli authors
  marginalrevolution-pp-cli authors --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadMRFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			return printMRCounts(cmd, flags, mr.SortedCounts(mr.AuthorCounts(feed.Items)))
		},
	}
}

func newLinksCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	cmd := &cobra.Command{
		Use:   "links",
		Short: "Extract outbound links from recent posts",
		Example: `  marginalrevolution-pp-cli links --limit 3
  marginalrevolution-pp-cli links --query ai --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadMRFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			items := mr.Filter(feed.Items, query, "", "", limit)
			type row struct {
				PostTitle string `json:"post_title"`
				Text      string `json:"text,omitempty"`
				URL       string `json:"url"`
			}
			var rows []row
			for _, item := range items {
				for _, link := range item.Links {
					if strings.Contains(link.URL, "marginalrevolution.com") {
						continue
					}
					rows = append(rows, row{PostTitle: item.Title, Text: link.Text, URL: link.URL})
				}
			}
			return printMRJSONOrText(cmd, flags, rows, func() {
				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "POST\tTEXT\tURL")
				for _, row := range rows {
					fmt.Fprintf(w, "%s\t%s\t%s\n", row.PostTitle, row.Text, row.URL)
				}
				w.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Only extract links from posts matching this query")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum posts to inspect")
	return cmd
}

func loadMRFeed(ctx context.Context, flags *rootFlags) (mr.Feed, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return mr.Fetch(ctx, flags.timeout)
}

func printMRItems(cmd *cobra.Command, flags *rootFlags, items []mr.Item) error {
	return printMRJSONOrText(cmd, flags, items, func() {
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "DATE\tAUTHOR\tCOMMENTS\tTITLE\tURL")
		for _, item := range items {
			date := ""
			if !item.Published.IsZero() {
				date = item.Published.Format("2006-01-02")
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", date, item.Author, item.CommentCount, item.Title, item.Link)
		}
		w.Flush()
	})
}

func printMRCounts(cmd *cobra.Command, flags *rootFlags, rows []struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}) error {
	return printMRJSONOrText(cmd, flags, rows, func() {
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tCOUNT")
		for _, row := range rows {
			fmt.Fprintf(w, "%s\t%d\n", row.Name, row.Count)
		}
		w.Flush()
	})
}

func printMRJSONOrText(cmd *cobra.Command, flags *rootFlags, value any, printText func()) error {
	if flags.asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	printText()
	return nil
}

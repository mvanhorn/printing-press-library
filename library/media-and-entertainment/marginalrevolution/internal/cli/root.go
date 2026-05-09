package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/marginalrevolution/internal/mr"
	"github.com/spf13/cobra"
)

var version = "1.0.0"

type rootFlags struct {
	json    bool
	plain   bool
	agent   bool
	timeout time.Duration
	limit   int
}

func Execute() error {
	return RootCmd().Execute()
}

func RootCmd() *cobra.Command {
	flags := &rootFlags{}
	cmd := &cobra.Command{
		Use:   "marginalrevolution-pp-cli",
		Short: "Read and filter Marginal Revolution from its public RSS feed",
		Long: `Read and filter Marginal Revolution from its public RSS feed.

The feed endpoint is stable and does not require authentication. The normal
site search and WordPress JSON endpoints may present Cloudflare challenges to
non-browser clients, so this CLI keeps searches bounded to the current feed.`,
		SilenceUsage: true,
		Version:      version,
	}
	cmd.SetVersionTemplate("marginalrevolution-pp-cli {{ .Version }}\n")
	cmd.PersistentFlags().BoolVar(&flags.json, "json", false, "Output JSON")
	cmd.PersistentFlags().BoolVar(&flags.plain, "plain", false, "Output tab-separated plain text")
	cmd.PersistentFlags().BoolVar(&flags.agent, "agent", false, "Agent defaults: --json and non-interactive behavior")
	cmd.PersistentFlags().DurationVar(&flags.timeout, "timeout", 20*time.Second, "HTTP request timeout")
	cmd.PersistentFlags().IntVar(&flags.limit, "limit", 10, "Maximum posts to return")
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if flags.agent {
			flags.json = true
		}
	}

	cmd.AddCommand(newLatestCmd(flags))
	cmd.AddCommand(newSearchCmd(flags))
	cmd.AddCommand(newReadCmd(flags))
	cmd.AddCommand(newCategoriesCmd(flags))
	cmd.AddCommand(newAuthorsCmd(flags))
	cmd.AddCommand(newLinksCmd(flags))
	cmd.AddCommand(newDoctorCmd(flags))
	cmd.AddCommand(newWhichCmd())
	return cmd
}

func newLatestCmd(flags *rootFlags) *cobra.Command {
	var author, category string
	cmd := &cobra.Command{
		Use:   "latest",
		Short: "List recent Marginal Revolution posts",
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			return printItems(cmd, flags, mr.Filter(feed.Items, "", author, category, flags.limit))
		},
	}
	cmd.Flags().StringVar(&author, "author", "", "Filter by author name")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category")
	return cmd
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var author, category string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search titles and body text in the current RSS feed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			return printItems(cmd, flags, mr.Filter(feed.Items, args[0], author, category, flags.limit))
		},
	}
	cmd.Flags().StringVar(&author, "author", "", "Filter by author name")
	cmd.Flags().StringVar(&category, "category", "", "Filter by category")
	return cmd
}

func newReadCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "read <url|guid|title>",
		Short: "Read a post currently present in the RSS feed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			item, ok := mr.Find(feed.Items, args[0])
			if !ok {
				return fmt.Errorf("post not found in current feed: %s", args[0])
			}
			return printJSONOrText(cmd, flags, item, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n%s\n\n%s\n", item.Title, item.Author, item.Link, item.ContentText)
			})
		},
	}
}

func newCategoriesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "categories",
		Short: "Show category counts in the current feed",
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			return printJSONOrTable(cmd, flags, mr.SortedCounts(mr.CategoryCounts(feed.Items)))
		},
	}
}

func newAuthorsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "authors",
		Short: "Show author counts in the current feed",
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			return printJSONOrTable(cmd, flags, mr.SortedCounts(mr.AuthorCounts(feed.Items)))
		},
	}
}

func newLinksCmd(flags *rootFlags) *cobra.Command {
	var query string
	return &cobra.Command{
		Use:   "links",
		Short: "Extract outbound links from recent posts",
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			items := mr.Filter(feed.Items, query, "", "", flags.limit)
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
			return printJSONOrText(cmd, flags, rows, func() {
				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "POST\tTEXT\tURL")
				for _, row := range rows {
					fmt.Fprintf(w, "%s\t%s\t%s\n", row.PostTitle, row.Text, row.URL)
				}
				w.Flush()
			})
		},
	}
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check RSS feed reachability",
		RunE: func(cmd *cobra.Command, args []string) error {
			feed, err := loadFeed(cmd.Context(), flags)
			if err != nil {
				return err
			}
			result := map[string]any{
				"ok":         true,
				"feed_url":   mr.FeedURL,
				"title":      feed.Title,
				"item_count": len(feed.Items),
				"last_build": feed.LastBuild,
			}
			return printJSONOrText(cmd, flags, result, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "ok: %d items from %s\n", len(feed.Items), mr.FeedURL)
			})
		},
	}
}

func newWhichCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "which <capability>",
		Short: "Suggest the best Marginal Revolution command for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.ToLower(args[0])
			switch {
			case strings.Contains(query, "link"):
				fmt.Fprintln(cmd.OutOrStdout(), "links")
			case strings.Contains(query, "category"):
				fmt.Fprintln(cmd.OutOrStdout(), "categories")
			case strings.Contains(query, "author") || strings.Contains(query, "tyler") || strings.Contains(query, "alex"):
				fmt.Fprintln(cmd.OutOrStdout(), "authors or latest --author <name>")
			case strings.Contains(query, "search") || strings.Contains(query, "find"):
				fmt.Fprintln(cmd.OutOrStdout(), "search <query>")
			case strings.Contains(query, "read"):
				fmt.Fprintln(cmd.OutOrStdout(), "read <url|guid|title>")
			default:
				fmt.Fprintln(cmd.OutOrStdout(), "latest")
			}
			return nil
		},
	}
}

func loadFeed(ctx context.Context, flags *rootFlags) (mr.Feed, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return mr.Fetch(ctx, flags.timeout)
}

func printItems(cmd *cobra.Command, flags *rootFlags, items []mr.Item) error {
	return printJSONOrText(cmd, flags, items, func() {
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

func printJSONOrTable(cmd *cobra.Command, flags *rootFlags, rows []struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}) error {
	return printJSONOrText(cmd, flags, rows, func() {
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tCOUNT")
		for _, row := range rows {
			fmt.Fprintf(w, "%s\t%d\n", row.Name, row.Count)
		}
		w.Flush()
	})
}

func printJSONOrText(cmd *cobra.Command, flags *rootFlags, value any, printText func()) error {
	if flags.json {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	printText()
	return nil
}

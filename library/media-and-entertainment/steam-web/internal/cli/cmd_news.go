package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newNewsCmd(flags *rootFlags) *cobra.Command {
	var count int

	cmd := &cobra.Command{
		Use:   "news <appid>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show news articles for a game",
		Long:  `Fetch recent news and patch notes for a game by its App ID.`,
		Example: `  # Show news for CS2
  steam-web-pp-cli news 730

  # Show only 5 articles
  steam-web-pp-cli news 730 --count 5

  # Output as JSON
  steam-web-pp-cli news 730 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/ISteamNews/GetNewsForApp/v2"
			params := map[string]string{
				"appid": args[0],
			}
			if count > 0 {
				params["count"] = fmt.Sprintf("%d", count)
			}
			data, prov, err := resolveRead(c, flags, "isteam-news", false, path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Extract appnews.newsitems array
			var wrapper map[string]json.RawMessage
			if err := json.Unmarshal(data, &wrapper); err == nil {
				if appnews, ok := wrapper["appnews"]; ok {
					var inner map[string]json.RawMessage
					if err := json.Unmarshal(appnews, &inner); err == nil {
						if items, ok := inner["newsitems"]; ok {
							data = items
						}
					}
				}
			}

			{
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := data
				if flags.compact {
					filtered = compactFields(filtered)
				}
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}

			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().IntVar(&count, "count", 0, "Number of news articles to return (default: API default)")

	return cmd
}

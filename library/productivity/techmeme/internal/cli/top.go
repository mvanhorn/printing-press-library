// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/xml"
	"fmt"

	"github.com/spf13/cobra"
)

func newTopCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show top stories on Techmeme right now",
		Long: `Display the top stories currently on Techmeme's front page.
Fetches the RSS feed and shows headlines in a clean table format.
This is an alias for the 'headlines' command with a more intuitive name.`,
		Example: `  # Show top stories
  techmeme-pp-cli top

  # Top stories as JSON
  techmeme-pp-cli top --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/feed.xml", map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var rss rssResponse
			if err := xml.Unmarshal(data, &rss); err != nil {
				return fmt.Errorf("parsing RSS feed: %w", err)
			}

			type headline struct {
				Num      int    `json:"num"`
				Time     string `json:"time"`
				Source   string `json:"source"`
				Headline string `json:"headline"`
				Link     string `json:"link"`
			}

			headlines := make([]headline, 0, len(rss.Channel.Items))
			for i, item := range rss.Channel.Items {
				t := parseRSSDate(item.PubDate)
				timeStr := ""
				if !t.IsZero() {
					timeStr = t.Local().Format("15:04")
				}

				headlines = append(headlines, headline{
					Num:      i + 1,
					Time:     timeStr,
					Source:   extractSource(item.Description),
					Headline: item.Title,
					Link:     item.Link,
				})
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), headlines, flags)
			}

			headers := []string{"#", "TIME", "SOURCE", "HEADLINE"}
			rows := make([][]string, 0, len(headlines))
			for _, h := range headlines {
				rows = append(rows, []string{
					fmt.Sprintf("%d", h.Num),
					h.Time,
					truncate(h.Source, 25),
					truncate(h.Headline, 70),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	return cmd
}

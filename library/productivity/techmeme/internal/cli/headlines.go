// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type rssResponse struct {
	XMLName xml.Name `xml:"rss"`
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// stripHTML removes HTML tags from a string.
func stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}

// extractSource tries to extract the source name from an RSS description.
func extractSource(description string) string {
	text := stripHTML(description)
	// The description often starts with the source name
	if idx := strings.Index(text, " - "); idx > 0 && idx < 60 {
		return strings.TrimSpace(text[:idx])
	}
	if idx := strings.Index(text, ": "); idx > 0 && idx < 60 {
		return strings.TrimSpace(text[:idx])
	}
	if len(text) > 40 {
		return text[:40]
	}
	return text
}

// parseRSSDate parses common RSS date formats.
func parseRSSDate(s string) time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func newHeadlinesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "headlines",
		Short: "Show top Techmeme headlines from the RSS feed",
		Long: `Fetch and display the top 15 headlines currently on Techmeme.
Parses the RSS feed at /feed.xml and displays headlines in a table format.`,
		Example: `  # Show current headlines
  techmeme-pp-cli headlines

  # Show headlines as JSON
  techmeme-pp-cli headlines --json

  # Show compact output for agents
  techmeme-pp-cli headlines --agent`,
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

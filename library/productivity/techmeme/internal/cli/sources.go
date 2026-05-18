// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"

	"github.com/spf13/cobra"
)

type opmlResponse struct {
	XMLName xml.Name `xml:"opml"`
	Body    struct {
		Outlines []opmlOutline `xml:"outline"`
	} `xml:"body"`
}

type opmlOutline struct {
	Text    string `xml:"text,attr"`
	Type    string `xml:"type,attr"`
	HTMLUrl string `xml:"htmlUrl,attr"`
	XMLUrl  string `xml:"xmlUrl,attr"`
	URL     string `xml:"url,attr"`
}

func newSourcesCmd(flags *rootFlags) *cobra.Command {
	var topN int

	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Show Techmeme's top sources from the leaderboard",
		Long: `Fetch and display Techmeme's top sources from the OPML leaderboard.
Shows source name, website URL, and whether an RSS feed is available.`,
		Example: `  # Show top 20 sources (default)
  techmeme-pp-cli sources

  # Show all sources
  techmeme-pp-cli sources --top 51

  # Show top 10 as JSON
  techmeme-pp-cli sources --top 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/lb.opml", map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var opml opmlResponse
			decoder := xml.NewDecoder(bytes.NewReader(data))
			decoder.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
				if strings.EqualFold(label, "ISO-8859-1") || strings.EqualFold(label, "latin1") {
					return charset.NewReaderLabel(label, input)
				}
				return input, nil
			}
			if err := decoder.Decode(&opml); err != nil {
				return fmt.Errorf("parsing OPML: %w", err)
			}

			type sourceEntry struct {
				Rank    int    `json:"rank"`
				Source  string `json:"source"`
				Website string `json:"website"`
				FeedURL string `json:"feed_url,omitempty"`
				HasFeed bool   `json:"has_feed"`
			}

			var sources []sourceEntry
			for i, outline := range opml.Body.Outlines {
				if topN > 0 && i+1 > topN {
					break
				}
				feedURL := outline.XMLUrl
				website := outline.HTMLUrl
				if website == "" {
					website = outline.URL
				}
				sources = append(sources, sourceEntry{
					Rank:    i + 1,
					Source:  outline.Text,
					Website: website,
					FeedURL: feedURL,
					HasFeed: feedURL != "",
				})
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), sources, flags)
			}

			headers := []string{"RANK", "SOURCE", "WEBSITE", "HAS FEED"}
			rows := make([][]string, 0, len(sources))
			for _, s := range sources {
				hasFeed := "No"
				if s.HasFeed {
					hasFeed = "Yes"
				}
				rows = append(rows, []string{
					fmt.Sprintf("%d", s.Rank),
					truncate(s.Source, 30),
					truncate(s.Website, 40),
					hasFeed,
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&topN, "top", 20, "Number of top sources to show")

	return cmd
}

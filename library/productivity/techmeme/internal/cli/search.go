// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var searchLinkRE = regexp.MustCompile(`<A HREF="(https?://[^"]+)"[^>]*>([^<]+)</A>`)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Techmeme headlines",
		Long: `Search Techmeme for headlines matching a query.
Supports quoted phrases, wildcards, +/-, AND/OR/NOT, sourcename:X.`,
		Example: `  # Search for a topic
  techmeme-pp-cli search "artificial intelligence"

  # Search with source filter
  techmeme-pp-cli search "AI sourcename:Bloomberg"

  # Search as JSON
  techmeme-pp-cli search "Apple" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			query := strings.Join(args, " ")

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"q":  query,
				"wm": "false",
			}

			data, err := c.Get("/search/d3results.jsp", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			type searchResult struct {
				Num      int    `json:"num"`
				Source   string `json:"source"`
				Headline string `json:"headline"`
				Link     string `json:"link"`
			}

			html := string(data)
			matches := searchLinkRE.FindAllStringSubmatch(html, -1)

			var results []searchResult
			for _, m := range matches {
				href := m[1]
				title := stripHTML(m[2])
				title = strings.ReplaceAll(title, "&nbsp;", " ")
				title = strings.ReplaceAll(title, "&mdash;", "—")
				title = strings.ReplaceAll(title, "&amp;", "&")

				if strings.Contains(href, "techmeme.com/") && strings.Contains(title, "context") {
					continue
				}
				if strings.Contains(href, "techmeme.com/r2/") {
					continue
				}
				if len(title) < 10 {
					continue
				}

				source := extractDomain(href)
				results = append(results, searchResult{
					Num:      len(results) + 1,
					Source:   source,
					Headline: title,
					Link:     href,
				})
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No results for %q\n", query)
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "%d results for %q\n", len(results), query)

			headers := []string{"#", "SOURCE", "HEADLINE"}
			rows := make([][]string, 0, len(results))
			for _, r := range results {
				rows = append(rows, []string{
					fmt.Sprintf("%d", r.Num),
					truncate(r.Source, 25),
					truncate(r.Headline, 70),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	return cmd
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	host := u.Hostname()
	host = strings.TrimPrefix(host, "www.")
	return host
}

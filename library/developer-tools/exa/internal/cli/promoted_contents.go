// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-coded typed-flag wrapper for the /contents endpoint (spec uses oneOf/anyOf).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newContentsPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagURLs []string
	var flagText bool
	var flagMaxChars int
	var flagHighlights bool
	var flagHighlightsQuery string
	var flagSummary string
	var flagSubpages int
	var flagLivecrawl string
	var flagMaxAgeHours int
	var flagExtractLinks int
	var flagBodyJSON string

	cmd := &cobra.Command{
		Use:   "contents [url...]",
		Short: "Retrieve clean text, highlights, and summaries from URLs",
		Long: `Retrieve page text, AI highlights, and summaries from one or more URLs.

URLs can be passed as positional arguments or via --urls. Reads from stdin when
neither is set (one URL per line).`,
		Example: `  # Get text from a URL
  exa-pp-cli contents https://arxiv.org/abs/2307.06435 --text

  # Multiple URLs with summary
  exa-pp-cli contents https://a.com https://b.com --summary "What do they offer?"

  # Highlights with a focus query
  exa-pp-cli contents https://example.com --highlights --highlights-query "key findings"

  # Full extraction pipeline
  exa-pp-cli contents https://arxiv.org/abs/2307.06435 \
    --text --max-chars 2000 --highlights --summary "main contribution" \
    --livecrawl preferred --subpages 2 --json`,
		Annotations: map[string]string{"pp:endpoint": "contents.get", "pp:method": "POST", "pp:path": "/contents", "mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --body-json is set, pass through directly (escape hatch).
			if cmd.Flags().Changed("body-json") {
				return contentsBodyJSONPassthrough(cmd, flags, flagBodyJSON)
			}

			// Collect URLs from args, --urls flag, and stdin.
			urls := append(flagURLs, args...)
			if len(urls) == 0 {
				// Check if stdin has data.
				fi, _ := os.Stdin.Stat()
				if fi != nil && (fi.Mode()&os.ModeCharDevice) == 0 {
					var line string
					for {
						n, _ := fmt.Fscanln(os.Stdin, &line)
						if n == 0 {
							break
						}
						line = strings.TrimSpace(line)
						if line != "" {
							urls = append(urls, line)
						}
					}
				}
			}
			if len(urls) == 0 && !flags.dryRun {
				return cmd.Help()
			}

			body := map[string]any{
				"urls": urls,
			}

			// Build ContentsOptions sub-object.
			opts := map[string]any{}
			if cmd.Flags().Changed("text") || cmd.Flags().Changed("max-chars") {
				if flagMaxChars > 0 {
					opts["text"] = map[string]any{"maxCharacters": flagMaxChars}
				} else {
					opts["text"] = true
				}
			}
			if cmd.Flags().Changed("highlights") || cmd.Flags().Changed("highlights-query") {
				if flagHighlightsQuery != "" {
					opts["highlights"] = map[string]any{"query": flagHighlightsQuery}
				} else {
					opts["highlights"] = true
				}
			}
			if flagSummary != "" {
				opts["summary"] = map[string]any{"query": flagSummary}
			}
			if flagSubpages > 0 {
				opts["subpages"] = flagSubpages
			}
			if flagLivecrawl != "" {
				opts["livecrawl"] = flagLivecrawl
			}
			if flagMaxAgeHours > 0 {
				opts["maxAgeHours"] = flagMaxAgeHours
			}
			if flagExtractLinks > 0 {
				opts["extras"] = map[string]any{"links": flagExtractLinks}
			}
			for k, v := range opts {
				body[k] = v
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.PostQueryWithParams(cmd.Context(), "/contents", nil, body)
			prov := attachFreshness(DataProvenance{Source: "live"}, flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					printProvenance(cmd, len(items), prov)
					return printAutoTable(cmd.OutOrStdout(), items)
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringArrayVar(&flagURLs, "urls", nil, "URLs to extract content from (repeatable; also accepts positional args)")
	cmd.Flags().BoolVar(&flagText, "text", false, "Return full page text")
	cmd.Flags().IntVar(&flagMaxChars, "max-chars", 0, "Max characters of text to return (implies --text)")
	cmd.Flags().BoolVar(&flagHighlights, "highlights", false, "Return AI-extracted highlights")
	cmd.Flags().StringVar(&flagHighlightsQuery, "highlights-query", "", "Focus highlights on this question (implies --highlights)")
	cmd.Flags().StringVar(&flagSummary, "summary", "", "Return an AI summary answering this question")
	cmd.Flags().IntVar(&flagSubpages, "subpages", 0, "Number of subpages to extract (0 = none)")
	cmd.Flags().StringVar(&flagLivecrawl, "livecrawl", "", "Live-crawl preference: preferred|always|never|fallback")
	cmd.Flags().IntVar(&flagMaxAgeHours, "max-age-hours", 0, "Maximum content age in hours before re-crawl")
	cmd.Flags().IntVar(&flagExtractLinks, "extract-links", 0, "Number of links to extract from each page")
	cmd.Flags().StringVar(&flagBodyJSON, "body-json", "", "Full request body as JSON (escape hatch for advanced payloads)")

	return cmd
}

func contentsBodyJSONPassthrough(cmd *cobra.Command, flags *rootFlags, bodyJSON string) error {
	var body map[string]any
	if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
		return fmt.Errorf("parsing --body-json: %w", err)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, _, err := c.PostQueryWithParams(cmd.Context(), "/contents", nil, body)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
}

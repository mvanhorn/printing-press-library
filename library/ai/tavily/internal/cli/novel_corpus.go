// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel corpus commands — corpus build and corpus gaps.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/tavily/internal/store"
)

// newCorpusCmd creates the parent `corpus` command group.
func newCorpusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "corpus",
		Short: "Build and manage local RAG corpora",
		Long: `Build a RAG corpus via fan-out search, extract, and crawl with URL
deduplication. Also provides tools for finding gaps in your corpus.`,
		Annotations: map[string]string{"pp:novel": "true"},
	}
	cmd.AddCommand(newCorpusBuildCmd(flags))
	cmd.AddCommand(newCorpusGapsCmd(flags))
	return cmd
}

func newCorpusBuildCmd(flags *rootFlags) *cobra.Command {
	var queries []string
	var queryFile string
	var maxResults int
	var crawlDepth int
	var outputFile string
	var session string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Fan-out search+extract+crawl to build a JSONL corpus",
		Long: `Fan out search, extract, and crawl calls with automatic URL deduplication
to build a JSONL corpus ready for embedding. Each line is a JSON object
with url, title, content, and source fields.

Pass multiple --query flags or a file of queries (one per line) with --query-file.`,
		Example: `  tavily-pp-cli corpus build --query "Tavily search API" --query "LLM tool use"
  tavily-pp-cli corpus build --query-file queries.txt --output corpus.jsonl
  tavily-pp-cli corpus build --query "RAG systems" --crawl-depth 2 --session my-rag`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if maxResults <= 0 {
				maxResults = 5
			}

			allQueries := queries
			if queryFile != "" {
				fileQueries, ferr := readQueryFile(queryFile)
				if ferr != nil {
					return fmt.Errorf("reading --query-file %q: %w", queryFile, ferr)
				}
				allQueries = append(allQueries, fileQueries...)
			}
			if len(allQueries) == 0 && len(args) > 0 {
				allQueries = args
			}
			if len(allQueries) == 0 {
				return fmt.Errorf("required: at least one --query or non-empty --query-file")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			st, sterr := store.Open()
			if sterr != nil {
				return fmt.Errorf("opening store: %w", sterr)
			}
			defer st.Close()

			type CorpusEntry struct {
				URL     string `json:"url"`
				Title   string `json:"title"`
				Content string `json:"content"`
				Source  string `json:"source"`
				Query   string `json:"query"`
			}

			seenURLs := make(map[string]bool)
			var entries []CorpusEntry

			for _, q := range allQueries {
				if !dryRun {
					body := map[string]any{
						"query":               q,
						"max_results":         maxResults,
						"include_raw_content": "markdown",
					}
					data, _, serr := c.Post("/search", body)
					if serr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: search %q failed: %v\n", q, serr)
						continue
					}
					bodyJSON, _ := json.Marshal(body)
					st.InsertSearch(q, string(bodyJSON), string(data), session)
					st.InsertCredit("search", 1.0, session)

					var resp struct {
						Results []struct {
							URL        string `json:"url"`
							Title      string `json:"title"`
							Content    string `json:"content"`
							RawContent string `json:"raw_content"`
						} `json:"results"`
					}
					if err := json.Unmarshal(data, &resp); err != nil {
						continue
					}
					for _, r := range resp.Results {
						if seenURLs[r.URL] {
							continue
						}
						seenURLs[r.URL] = true
						content := r.Content
						if r.RawContent != "" {
							content = r.RawContent
						}
						entries = append(entries, CorpusEntry{
							URL:     r.URL,
							Title:   r.Title,
							Content: content,
							Source:  "search",
							Query:   q,
						})
					}

					// If crawl-depth requested, crawl each result URL
					if crawlDepth > 0 {
						for _, r := range resp.Results {
							if seenURLs[r.URL+":crawled"] {
								continue
							}
							seenURLs[r.URL+":crawled"] = true
							crawlBody := map[string]any{
								"url":       r.URL,
								"max_depth": crawlDepth,
								"limit":     10,
							}
							cdata, _, cerr := c.Post("/crawl", crawlBody)
							if cerr != nil {
								continue
							}
							paramsJSON, _ := json.Marshal(crawlBody)
							// PATCH: surface InsertCrawl failures and skip
							// the matching InsertCredit / UpdateCrawlCheckpoint
							// so cost-report doesn't over-count crawls that
							// have no row to resume from.
							crawlID, cierr := st.InsertCrawl(r.URL, string(paramsJSON), session)
							if cierr != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "warning: store InsertCrawl %q failed: %v\n", r.URL, cierr)
							} else {
								if cerr := st.InsertCredit("crawl", 2.0, session); cerr != nil {
									fmt.Fprintf(cmd.ErrOrStderr(), "warning: store InsertCredit crawl failed: %v\n", cerr)
								}
							}

							var cresp struct {
								Results []struct {
									URL     string `json:"url"`
									Content string `json:"content"`
									Title   string `json:"title"`
								} `json:"results"`
							}
							if err := json.Unmarshal(cdata, &cresp); err == nil {
								for _, cr := range cresp.Results {
									if seenURLs[cr.URL] {
										continue
									}
									seenURLs[cr.URL] = true
									entries = append(entries, CorpusEntry{
										URL:     cr.URL,
										Title:   cr.Title,
										Content: cr.Content,
										Source:  "crawl",
										Query:   q,
									})
								}
							}
							if cierr == nil {
								if uerr := st.UpdateCrawlCheckpoint(crawlID, len(cresp.Results), "{}", "complete"); uerr != nil {
									fmt.Fprintf(cmd.ErrOrStderr(), "warning: store UpdateCrawlCheckpoint id=%d failed: %v\n", crawlID, uerr)
								}
							}
						}
					}
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would search: %q\n", q)
				}
			}

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would write %d entries to %s\n",
					len(allQueries)*maxResults, outputFile)
				return nil
			}

			// Output
			var out strings.Builder
			for _, e := range entries {
				line, _ := json.Marshal(e)
				out.Write(line)
				out.WriteByte('\n')
			}

			if outputFile != "" && outputFile != "-" {
				if err := writeFile(outputFile, out.String()); err != nil {
					return fmt.Errorf("writing output: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d entries to %s\n", len(entries), outputFile)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), out.String())
			}
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&queries, "query", nil, "Search query (repeatable)")
	cmd.Flags().StringVar(&queryFile, "query-file", "", "File with one query per line")
	cmd.Flags().IntVar(&maxResults, "max-results", 5, "Search results per query")
	cmd.Flags().IntVar(&crawlDepth, "crawl-depth", 0, "BFS crawl depth for each result URL (0=skip)")
	cmd.Flags().StringVar(&outputFile, "output", "-", "Output JSONL file path (- for stdout)")
	cmd.Flags().StringVar(&session, "session", "", "Session label for tracking")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be fetched without making API calls")
	return cmd
}

func newCorpusGapsCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "gaps",
		Short: "Find URLs in cached search results that have not been extracted yet",
		Long: `Scan all stored search results and find URLs that appear in the
result sets but have never been extracted. Returns a prioritized list
sorted by mention frequency — high-frequency URLs are most worth extracting.`,
		Example: `  tavily-pp-cli corpus gaps
  tavily-pp-cli corpus gaps --limit 20 --json`,
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit <= 0 {
				limit = 50
			}
			st, err := store.Open()
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer st.Close()

			mentioned, err := st.URLsMentionedInSearches()
			if err != nil {
				return fmt.Errorf("reading search results: %w", err)
			}
			extracted, err := st.ExtractedURLs()
			if err != nil {
				return fmt.Errorf("reading extracted URLs: %w", err)
			}

			type gap struct {
				URL     string `json:"url"`
				Mentions int   `json:"mentions"`
			}
			var gaps []gap
			for url, count := range mentioned {
				if !extracted[url] {
					gaps = append(gaps, gap{URL: url, Mentions: count})
				}
			}
			sort.Slice(gaps, func(i, j int) bool {
				return gaps[i].Mentions > gaps[j].Mentions
			})
			if len(gaps) > limit {
				gaps = gaps[:limit]
			}

			if flags.asJSON {
				data, _ := json.MarshalIndent(gaps, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			if len(gaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No gaps found — all cached URLs have been extracted.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d unextracted URLs (sorted by mention frequency):\n\n", len(gaps))
			for i, g := range gaps {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%d] mentions=%d  %s\n", i+1, g.Mentions, g.URL)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Max URLs to return")
	return cmd
}

// writeFile is a small helper to write a string to a file.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// readQueryFile loads queries from a file, one per line. Blank lines and
// lines whose first non-space character is '#' (comment) are skipped so
// users can annotate their query lists.
func readQueryFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var queries []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		queries = append(queries, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return queries, nil
}

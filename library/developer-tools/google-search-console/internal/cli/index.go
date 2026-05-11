// Copyright 2026 james-frewin. Licensed under Apache-2.0. See LICENSE.

// Indexing API integration. Three commands that ask Google to crawl/recrawl
// individual URLs (URL_UPDATED) or note their removal (URL_DELETED), and one
// to check the status of a prior submission.
//
// Google's Indexing API lives on indexing.googleapis.com, separate from the
// Search Console v3 API at webmasters.googleapis.com. The two share OAuth
// (we request both scopes at sign-in) but not the base URL, so these calls
// hit absolute URLs through the shared client.
//
// Quota note: free-tier projects get 200 URL notifications per day per
// project. The bulk-submit command exposes --rate and --max to keep agent
// loops well inside that budget.
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const indexingPublishURL = "https://indexing.googleapis.com/v3/urlNotifications:publish"
const indexingMetadataURL = "https://indexing.googleapis.com/v3/urlNotifications/metadata"

func newIndexSubmitCmd(flags *rootFlags) *cobra.Command {
	var (
		notifyType string
	)
	cmd := &cobra.Command{
		Use:   "index-submit <url>",
		Short: "Ask Google to crawl (or note the deletion of) a URL via the Indexing API",
		Long: strings.TrimSpace(`
Sends a URL_UPDATED or URL_DELETED notification to Google's Indexing API
for a single URL. Counts against the 200/day project quota.

Use --type URL_UPDATED (default) when a page has been published or
substantively changed and you want Google to recrawl. Use
--type URL_DELETED when a page has been removed and you want Google to
drop it from the index faster than passive crawl discovery would.
`),
		Example: strings.Join([]string{
			"  google-search-console-pp-cli index-submit https://example.com/new-post",
			"  google-search-console-pp-cli index-submit https://example.com/gone --type URL_DELETED",
		}, "\n"),
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			target := args[0]
			if notifyType != "URL_UPDATED" && notifyType != "URL_DELETED" {
				return usageErr(fmt.Errorf("--type must be URL_UPDATED or URL_DELETED, got %q", notifyType))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"url": target, "type": notifyType}
			raw, status, err := c.Post(indexingPublishURL, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status >= 400 {
				return apiErr(fmt.Errorf("indexing API returned %d: %s", status, string(raw)))
			}
			var resp map[string]any
			_ = json.Unmarshal(raw, &resp)
			return emit(cmd, flags, map[string]any{
				"url":      target,
				"type":     notifyType,
				"status":   "submitted",
				"response": resp,
			})
		},
	}
	cmd.Flags().StringVar(&notifyType, "type", "URL_UPDATED", "Notification type: URL_UPDATED or URL_DELETED.")
	return cmd
}

func newIndexStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index-status <url>",
		Short: "Read the latest Indexing API notification status for a URL",
		Long: strings.TrimSpace(`
Returns metadata about the most recent URL_UPDATED and URL_DELETED
notifications previously sent for the given URL — the type, the
notification timestamp, and the URL Google has on file. Useful for
confirming a submission was received and to dedupe before re-submitting.
`),
		Example:     "  google-search-console-pp-cli index-status https://example.com/page",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			target := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Indexing API accepts ?url=<encoded> as the lookup key.
			fullURL := indexingMetadataURL + "?url=" + url.QueryEscape(target)
			raw, err := c.Get(fullURL, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp map[string]any
			if err := json.Unmarshal(raw, &resp); err != nil {
				return apiErr(fmt.Errorf("parsing indexing metadata: %w", err))
			}
			return emit(cmd, flags, resp)
		},
	}
	return cmd
}

func newIndexBulkSubmitCmd(flags *rootFlags) *cobra.Command {
	var (
		fromFile   string
		notifyType string
		rate       int
		max        int
	)
	cmd := &cobra.Command{
		Use:   "index-bulk-submit",
		Short: "Submit many URLs to the Indexing API with rate limiting",
		Long: strings.TrimSpace(`
Reads URLs from a file (one per line; blank lines and # comments
ignored) and submits each via the Indexing API. Throttles to --rate
submissions per second and stops after --max submissions to stay within
the free-tier 200/day project quota.

The summary output records every URL with its outcome (submitted,
error, skipped-over-cap) so callers can re-run with --skip-already on a
filtered file to retry only the failures.
`),
		Example: strings.Join([]string{
			"  google-search-console-pp-cli index-bulk-submit --from-file urls.txt --max 150",
			"  google-search-console-pp-cli index-bulk-submit --from-file removed.txt --type URL_DELETED",
		}, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if fromFile == "" {
				return usageErr(fmt.Errorf("--from-file is required"))
			}
			if notifyType != "URL_UPDATED" && notifyType != "URL_DELETED" {
				return usageErr(fmt.Errorf("--type must be URL_UPDATED or URL_DELETED, got %q", notifyType))
			}
			f, err := os.Open(fromFile)
			if err != nil {
				return configErr(err)
			}
			defer f.Close()

			urls := []string{}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				urls = append(urls, line)
			}
			if err := scanner.Err(); err != nil {
				return configErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			interval := time.Second
			if rate > 1 {
				interval = time.Second / time.Duration(rate)
			}

			results := make([]map[string]any, 0, len(urls))
			submitted := 0
			for _, u := range urls {
				if max > 0 && submitted >= max {
					results = append(results, map[string]any{"url": u, "status": "skipped-over-cap"})
					continue
				}
				body := map[string]any{"url": u, "type": notifyType}
				raw, status, err := c.Post(indexingPublishURL, body)
				if err != nil {
					results = append(results, map[string]any{"url": u, "status": "error", "error": err.Error()})
					submitted++
					time.Sleep(interval)
					continue
				}
				if status >= 400 {
					results = append(results, map[string]any{"url": u, "status": "error", "http_status": status, "body": string(raw)})
				} else {
					results = append(results, map[string]any{"url": u, "status": "submitted"})
				}
				submitted++
				time.Sleep(interval)
			}

			return emit(cmd, flags, map[string]any{
				"file":       fromFile,
				"type":       notifyType,
				"total":      len(urls),
				"submitted":  submitted,
				"results":    results,
			})
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Path to a file of URLs, one per line. Required.")
	cmd.Flags().StringVar(&notifyType, "type", "URL_UPDATED", "Notification type: URL_UPDATED or URL_DELETED.")
	cmd.Flags().IntVar(&rate, "rate", 1, "Submissions per second (default 1).")
	cmd.Flags().IntVar(&max, "max", 0, "Stop after N submissions in this run (0 = no cap; honor 200/day project quota externally).")
	return cmd
}

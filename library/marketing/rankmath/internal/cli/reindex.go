// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil"
	"github.com/spf13/cobra"
)

type reindexItem struct {
	URL      string
	Modified time.Time
}
type indexLogEntry struct {
	URL  string `json:"url"`
	Time int64  `json:"time"`
}
type reindexBatch struct {
	Count   int    `json:"count"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}
type reindexResult struct {
	Since          string         `json:"since"`
	Candidates     int            `json:"candidates"`
	Skipped        int            `json:"skipped_already_submitted"`
	ToSubmit       []string       `json:"to_submit"`
	Submitted      int            `json:"submitted"`
	Confirmed      bool           `json:"confirmed"`
	Batches        []reindexBatch `json:"batches"`
	LogUnavailable bool           `json:"log_unavailable,omitempty"`
}

func parseReindexSinceDuration(s string) (time.Duration, error) {
	d, err := cliutil.ParseDurationLoose(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("--since must be a positive duration such as 24h or 7d")
	}
	return d, nil
}

func dedupeAgainstLog(items []reindexItem, log []indexLogEntry) ([]reindexItem, int) {
	latest := map[string]int64{}
	for _, entry := range log {
		for _, u := range strings.Fields(entry.URL) {
			if previous, ok := latest[u]; !ok || entry.Time > previous {
				latest[u] = entry.Time
			}
		}
	}
	out := make([]reindexItem, 0)
	skipped := 0
	for _, item := range items {
		if stamp, ok := latest[item.URL]; ok && !time.Unix(stamp, 0).Before(item.Modified) {
			skipped++
			continue
		}
		out = append(out, item)
	}
	return out, skipped
}

func newNovelReindexCmd(flags *rootFlags) *cobra.Command {
	var since, kinds string
	var confirm bool
	var batchSize, maxPages int
	cmd := &cobra.Command{
		Use:         "reindex",
		Short:       "Submit recently modified posts to IndexNow",
		Long:        "Use this command to submit recently modified posts to IndexNow. Do NOT use this command to find unindexed posts; use 'coverage' instead.",
		Example:     "  rankmath-pp-cli reindex --since 7d\n  rankmath-pp-cli reindex --since 24h --confirm",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "--since=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reindex")
			}
			if since == "" {
				return usageErr(fmt.Errorf("--since is required (e.g. --since 7d)"))
			}
			duration, err := parseReindexSinceDuration(since)
			if err != nil {
				return usageErr(err)
			}
			if batchSize < 1 || maxPages < 1 {
				return usageErr(fmt.Errorf("--batch and --max-scan-pages must be positive"))
			}
			types := make([]string, 0)
			seenTypes := map[string]bool{}
			for _, kind := range cliutil.SplitCSV(kinds) {
				if kind != "posts" && kind != "pages" {
					return usageErr(fmt.Errorf("--type must contain posts,pages"))
				}
				if !seenTypes[kind] {
					types = append(types, kind)
					seenTypes[kind] = true
				}
			}
			if confirm && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "reindex")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			after := time.Now().UTC().Add(-duration).Format(time.RFC3339)
			maxPages = scanCap(maxPages, 1)
			items := make([]reindexItem, 0)
			seenURLs := map[string]bool{}
			for _, kind := range types {
				for page := 1; page <= maxPages; page++ {
					raw, err := c.Get(ctx, "/wp/v2/"+kind, map[string]string{"per_page": "100", "page": strconv.Itoa(page), "modified_after": after, "_fields": "id,link,modified,modified_gmt,status"})
					if err != nil {
						var apiErr *client.APIError
						if errors.As(err, &apiErr) && apiErr.StatusCode == 400 && strings.Contains(string(apiErr.Body), "rest_post_invalid_page_number") {
							break
						}
						return err
					}
					var records []struct {
						Link        string `json:"link"`
						Modified    string `json:"modified"`
						ModifiedGMT string `json:"modified_gmt"`
						Status      string `json:"status"`
					}
					if err = json.Unmarshal(raw, &records); err != nil {
						return err
					}
					for _, r := range records {
						if r.Status != "publish" {
							continue
						}
						u, err := url.Parse(r.Link)
						if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
							return fmt.Errorf("inventory contains an invalid absolute URL: %q", r.Link)
						}
						modifiedRaw := r.ModifiedGMT
						if modifiedRaw == "" {
							modifiedRaw = r.Modified
						}
						modified, err := time.Parse(time.RFC3339, modifiedRaw)
						if err != nil {
							modified, err = time.ParseInLocation("2006-01-02T15:04:05", modifiedRaw, time.UTC)
						}
						if err != nil {
							return fmt.Errorf("invalid modified time for %s: %w", r.Link, err)
						}
						if !seenURLs[r.Link] {
							items = append(items, reindexItem{URL: r.Link, Modified: modified.UTC()})
							seenURLs[r.Link] = true
						}
					}
					if len(records) < 100 {
						break
					}
				}
			}
			result := reindexResult{Since: since, Candidates: len(items), ToSubmit: make([]string, 0), Batches: make([]reindexBatch, 0), Confirmed: confirm}
			raw, status, err := c.Post(ctx, "/rankmath/v1/in/getLog", map[string]string{"filter": "all"})
			var log struct {
				Data []indexLogEntry `json:"data"`
			}
			if err != nil {
				if status != 401 {
					return err
				}
				result.LogUnavailable = true
			} else if err = json.Unmarshal(raw, &log); err != nil {
				return err
			}
			remaining, skipped := dedupeAgainstLog(items, log.Data)
			result.Skipped = skipped
			for _, item := range remaining {
				result.ToSubmit = append(result.ToSubmit, item.URL)
			}
			emit := func() error {
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "Candidates: %d; skipped: %d; submitted: %d\n", result.Candidates, result.Skipped, result.Submitted)
					for _, u := range result.ToSubmit {
						fmt.Fprintln(cmd.OutOrStdout(), u)
					}
					return nil
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if confirm {
				for start := 0; start < len(result.ToSubmit); start += batchSize {
					end := start + batchSize
					if end > len(result.ToSubmit) {
						end = len(result.ToSubmit)
					}
					raw, _, err := c.Post(ctx, "/rankmath/v1/in/submitUrls", map[string]string{"urls": strings.Join(result.ToSubmit[start:end], "\n")})
					if err != nil {
						return err
					}
					var response struct {
						Success bool   `json:"success"`
						Message string `json:"message"`
						Code    string `json:"code"`
					}
					if err = json.Unmarshal(raw, &response); err != nil {
						return err
					}
					result.Batches = append(result.Batches, reindexBatch{Count: end - start, Success: response.Success, Message: response.Message})
					if !response.Success || response.Code != "" {
						if err := emit(); err != nil {
							return err
						}
						return fmt.Errorf("IndexNow submission failed: %s %s", response.Code, response.Message)
					}
					result.Submitted += end - start
				}
			}
			return emit()
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Modified within this duration (24h, 7d)")
	cmd.Flags().StringVar(&kinds, "type", "posts,pages", "WordPress types: posts,pages")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Submit URLs (default: preview)")
	cmd.Flags().IntVar(&batchSize, "batch", 100, "URLs per submission")
	cmd.Flags().IntVar(&maxPages, "max-scan-pages", 10, "Maximum inventory pages per type")
	return cmd
}

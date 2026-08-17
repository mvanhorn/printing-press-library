// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source auto

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

type batchFetchResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
	Fetched    bool   `json:"fetched"`
	Skipped    bool   `json:"skipped,omitempty"`
}

type batchFetchView struct {
	Items        []batchFetchResult `json:"items"`
	Total        int                `json:"total"`
	FetchedCount int                `json:"fetched_count"`
	FailedCount  int                `json:"failed_count"`
	SkippedCount int                `json:"skipped_count"`
	Checkpoint   string             `json:"checkpoint,omitempty"`
	Note         string             `json:"note,omitempty"`
}

func newNovelFetchBatchCmd(flags *rootFlags) *cobra.Command {
	var flagFile string
	var flagFormat string
	var flagResume bool
	var flagPace string

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Fetch a list of URLs with rate-limit pacing and a resumable checkpoint, so large scrape jobs survive interruptions.",
		Long: `Use this command when you have a list of URLs to fetch right now with rate-limit pacing and resumable progress.
Do NOT use it to look back at what was already fetched; use 'web history' instead.`,
		Example: "  browserbase-pp-cli fetch batch --file testdata/fetch-batch-urls.txt --format markdown --resume --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--file=testdata/fetch-batch-urls.txt;--format=markdown",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch batch")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if flagFile == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file is required (path to a file with one URL per line)"))
			}
			format := flagFormat
			if format == "" {
				format = "raw"
			}
			if format != "raw" && format != "markdown" && format != "json" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format %q is invalid; use raw, markdown, or json", format))
			}
			pace := 250 * time.Millisecond
			if flagPace != "" {
				parsed, err := time.ParseDuration(flagPace)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--pace %q is invalid: %w", flagPace, err))
				}
				pace = parsed
			}
			if pace <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--pace must be greater than zero (got %s)", pace))
			}

			// Read URLs (after dry-run so verify probes don't need the file).
			f, err := os.Open(flagFile)
			if err != nil {
				return fmt.Errorf("opening --file: %w", err)
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			urls := make([]string, 0)
			for scanner.Scan() {
				u := strings.TrimSpace(scanner.Text())
				if u == "" || strings.HasPrefix(u, "#") {
					continue
				}
				urls = append(urls, u)
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading --file: %w", err)
			}
			if len(urls) == 0 {
				return usageErr(fmt.Errorf("--file %q contains no URLs", flagFile))
			}

			// Resumable checkpoint: a JSON file of completed URLs next to the
			// CLI data dir.
			checkpoint := ""
			if flagResume {
				checkpoint = filepath.Join(filepath.Dir(defaultDBPath("browserbase-pp-cli")), "fetch-batch-checkpoint.json")
			}

			done := map[string]bool{}
			if flagResume {
				if cp, err := os.ReadFile(checkpoint); err == nil {
					var prev []string
					if json.Unmarshal(cp, &prev) == nil {
						for _, u := range prev {
							done[u] = true
						}
					}
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			results := make([]batchFetchResult, len(urls))
			completed := make([]string, 0, len(urls))
			var mu sync.Mutex
			var wg sync.WaitGroup
			var sem = make(chan struct{}, 3) // bounded concurrency

			// Global pacing: a shared ticker enforces the documented ~5
			// req/sec aggregate (one slot per pace interval), regardless of
			// how many workers are in flight. Per-goroutine sleeps would
			// multiply the rate by the worker count.
			ticker := time.NewTicker(pace)
			defer ticker.Stop()

			for idx, u := range urls {
				u := u
				idx := idx
				if done[u] {
					results[idx] = batchFetchResult{URL: u, Fetched: true, Skipped: true}
					continue
				}
				<-ticker.C // one request per pace interval globally
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					body := map[string]any{"url": u, "format": format}
					_, statusCode, err := c.PostWithParams(ctx, "/v1/fetch", nil, body)
					res := batchFetchResult{URL: u}
					if err != nil {
						res.Error = err.Error()
					} else {
						res.StatusCode = statusCode
						res.Fetched = statusCode >= 200 && statusCode < 300
						if statusCode < 200 || statusCode >= 300 {
							res.Error = fmt.Sprintf("HTTP %d", statusCode)
						}
					}
					mu.Lock()
					results[idx] = res
					if res.Fetched && res.Error == "" {
						completed = append(completed, u)
					}
					mu.Unlock()
				}()
			}
			wg.Wait()

			// Persist checkpoint for resume.
			if flagResume && len(completed) > 0 {
				prev := make([]string, 0, len(done)+len(completed))
				for u := range done {
					prev = append(prev, u)
				}
				prev = append(prev, completed...)
				if b, err := json.Marshal(prev); err == nil {
					_ = os.MkdirAll(filepath.Dir(checkpoint), 0o700)
					_ = os.WriteFile(checkpoint, b, 0o600)
				}
			}

			view := batchFetchView{
				Items:      results,
				Total:      len(urls),
				Checkpoint: checkpoint,
			}
			for _, r := range results {
				switch {
				case r.Skipped:
					view.SkippedCount++
				case r.Error != "":
					view.FailedCount++
				case r.Fetched:
					view.FetchedCount++
				}
			}
			// The checkpoint path is machine noise; only surface it in human output.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				view.Checkpoint = ""
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, r := range results {
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "FAIL\t%s\t%s\n", r.URL, r.Error)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "OK\t%s\tHTTP %d\n", r.URL, r.StatusCode)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d URLs: %d fetched, %d failed, %d resumed\n", view.Total, view.FetchedCount, view.FailedCount, view.SkippedCount)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Path to a file with one URL per line")
	cmd.Flags().StringVar(&flagFormat, "format", "raw", "Output format: raw, markdown, or json")
	cmd.Flags().BoolVar(&flagResume, "resume", false, "Resume from the local checkpoint, skipping already-fetched URLs")
	cmd.Flags().StringVar(&flagPace, "pace", "250ms", "Delay between fetches (the API allows ~5 req/sec)")
	return cmd
}

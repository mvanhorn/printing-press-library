// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source auto

package cli

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/cliutil"
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

func fetchBatchClientScope(c *client.Client, flags *rootFlags) string {
	hash := sha256.New()
	if c != nil {
		_, _ = fmt.Fprintf(hash, "base_url=%s\n", strings.TrimRight(c.RequestBaseURL(), "/"))
	}
	if flags != nil && flags.platformSession != nil {
		session := flags.platformSession
		_, _ = fmt.Fprintf(hash, "profile=%s\nsource=%s\ncredential_fingerprint=%s\n",
			session.ProfileName, session.Source, session.CredentialFingerprint)
	} else if c != nil && c.Config != nil {
		_, _ = fmt.Fprintf(hash, "config_path=%s\nauth_source=%s\n", c.Config.Path, c.Config.AuthSource)
		if authHeader := c.Config.AuthHeader(); authHeader != "" {
			credentialHash := sha256.Sum256([]byte(authHeader))
			_, _ = fmt.Fprintf(hash, "credential_fingerprint=%x\n", credentialHash)
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func fetchBatchCheckpointPath(dataPath, clientScope, format string, urls []string) string {
	unique, _ := indexBatchURLs(urls)
	sort.Strings(unique)
	return fetchBatchCheckpointName(dataPath, clientScope, format, unique, ".done", 16)
}

// This is the earlier single-file format, used only to import progress from
// private installs when their URL order still matches the current input.
func fetchBatchLegacyCheckpointPath(dataPath, clientScope, format string, urls []string) string {
	return fetchBatchCheckpointName(dataPath, clientScope, format, urls, ".json", 8)
}

func fetchBatchCheckpointName(dataPath, clientScope, format string, urls []string, suffix string, digestBytes int) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "client_scope=%s\n", clientScope)
	_, _ = fmt.Fprintf(hash, "format=%s\n", format)
	for _, url := range urls {
		_, _ = fmt.Fprintf(hash, "url=%s\n", url)
	}
	digest := hash.Sum(nil)
	return filepath.Join(filepath.Dir(dataPath), fmt.Sprintf("fetch-batch-checkpoint-%x%s", digest[:digestBytes], suffix))
}

func fetchBatchMarkerPath(path, url string) string {
	digest := sha256.Sum256([]byte(url))
	return filepath.Join(path, fmt.Sprintf("%x.done", digest))
}

func loadFetchBatchCheckpoint(path, legacyPath string, urls []string) (map[string]bool, error) {
	done := map[string]bool{}
	if legacyPath != "" {
		data, err := os.ReadFile(filepath.Clean(legacyPath)) // #nosec G304 -- app-derived data path.
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading legacy checkpoint: %w", err)
		}
		if err == nil {
			if trimmed := bytes.TrimSpace(data); len(trimmed) == 0 || trimmed[0] != '[' {
				return nil, fmt.Errorf("legacy checkpoint is malformed: expected a JSON array")
			}
			var completed []string
			if err := json.Unmarshal(data, &completed); err != nil {
				return nil, fmt.Errorf("legacy checkpoint is malformed: %w", err)
			}
			for _, url := range completed {
				if url != "" {
					done[url] = true
				}
			}
		}
	}
	for _, url := range urls {
		marker := fetchBatchMarkerPath(path, url)
		data, err := os.ReadFile(marker) // #nosec G304 -- app-derived hashed marker path.
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading checkpoint marker: %w", err)
		}
		if string(data) != "done\n" {
			return nil, fmt.Errorf("checkpoint marker is malformed: %s", marker)
		}
		done[url] = true
	}
	return done, nil
}

// One atomic marker per URL means parallel CLI processes never overwrite one
// another's completed work. URL bytes stay out of filenames and marker data.
func saveFetchBatchCheckpoint(path, url string) error {
	if err := cliutil.AtomicWritePrivateFile(fetchBatchMarkerPath(path, url), []byte("done\n"), 0o600, 0o700); err != nil {
		return fmt.Errorf("writing checkpoint: %w", err)
	}
	return nil
}

func indexBatchURLs(urls []string) ([]string, map[string][]int) {
	unique := make([]string, 0, len(urls))
	indexes := make(map[string][]int, len(urls))
	for idx, url := range urls {
		if _, exists := indexes[url]; !exists {
			unique = append(unique, url)
		}
		indexes[url] = append(indexes[url], idx)
	}
	return unique, indexes
}

func applyBatchResult(results []batchFetchResult, indexes map[string][]int, result batchFetchResult) {
	for _, idx := range indexes[result.URL] {
		results[idx] = result
	}
}

func waitForBatchSlot(ctx context.Context, ticks <-chan time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticks:
		return ctx.Err()
	}
}

func acquireBatchWorker(ctx context.Context, sem chan struct{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case sem <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-sem
			return err
		}
		return nil
	}
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
			workURLs, indexesByURL := indexBatchURLs(urls)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resumable checkpoint: a JSON file of completed URLs next to the
			// CLI data dir, scoped to the non-secret client identity so work for
			// one endpoint or tenant cannot suppress requests for another.
			checkpoint := ""
			legacyCheckpoint := ""
			if flagResume {
				dataPath := defaultDBPath("browserbase-pp-cli")
				scope := fetchBatchClientScope(c, flags)
				checkpoint = fetchBatchCheckpointPath(
					dataPath,
					scope,
					format,
					urls,
				)
				legacyCheckpoint = fetchBatchLegacyCheckpointPath(dataPath, scope, format, urls)
			}

			done := map[string]bool{}
			if flagResume {
				done, err = loadFetchBatchCheckpoint(checkpoint, legacyCheckpoint, workURLs)
				if err != nil {
					return fmt.Errorf("loading resume checkpoint: %w", err)
				}
			}

			results := make([]batchFetchResult, len(urls))
			var mu sync.Mutex
			var wg sync.WaitGroup
			var sem = make(chan struct{}, 3) // bounded concurrency

			// Global pacing: a shared ticker enforces the documented ~5
			// req/sec aggregate (one slot per pace interval), regardless of
			// how many workers are in flight. Per-goroutine sleeps would
			// multiply the rate by the worker count.
			ticker := time.NewTicker(pace)
			defer ticker.Stop()

			markRemaining := func(start int, cause error) {
				mu.Lock()
				defer mu.Unlock()
				for _, u := range workURLs[start:] {
					if done[u] {
						applyBatchResult(results, indexesByURL, batchFetchResult{URL: u, Fetched: true, Skipped: true})
						continue
					}
					applyBatchResult(results, indexesByURL, batchFetchResult{URL: u, Error: cause.Error()})
				}
			}

		scheduleLoop:
			for workIdx, u := range workURLs {
				u := u
				mu.Lock()
				alreadyDone := done[u]
				if alreadyDone {
					applyBatchResult(results, indexesByURL, batchFetchResult{URL: u, Fetched: true, Skipped: true})
					mu.Unlock()
					continue
				}
				mu.Unlock()
				if err := waitForBatchSlot(ctx, ticker.C); err != nil {
					markRemaining(workIdx, err)
					break scheduleLoop
				}
				if err := acquireBatchWorker(ctx, sem); err != nil {
					markRemaining(workIdx, err)
					break scheduleLoop
				}
				wg.Add(1)
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
					if res.Fetched && res.Error == "" {
						done[u] = true
						if flagResume {
							if err := saveFetchBatchCheckpoint(checkpoint, u); err != nil {
								delete(done, u)
								res.Error = err.Error()
							}
						}
					}
					applyBatchResult(results, indexesByURL, res)
					mu.Unlock()
				}()
			}
			wg.Wait()

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
			var completionErr error
			if view.FailedCount > 0 {
				completionErr = ctx.Err()
			}
			// The checkpoint path is machine noise; only surface it in human output.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				view.Checkpoint = ""
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				return completionErr
			}
			for _, r := range results {
				if r.Error != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "FAIL\t%s\t%s\n", r.URL, r.Error)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "OK\t%s\tHTTP %d\n", r.URL, r.StatusCode)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d URLs: %d fetched, %d failed, %d resumed\n", view.Total, view.FetchedCount, view.FailedCount, view.SkippedCount)
			return completionErr
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Path to a file with one URL per line")
	cmd.Flags().StringVar(&flagFormat, "format", "raw", "Output format: raw, markdown, or json")
	cmd.Flags().BoolVar(&flagResume, "resume", false, "Resume from the local checkpoint, skipping already-fetched URLs")
	cmd.Flags().StringVar(&flagPace, "pace", "250ms", "Delay between fetches (the API allows ~5 req/sec)")
	return cmd
}

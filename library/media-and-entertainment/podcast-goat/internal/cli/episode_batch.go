// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// episode batch — parallel multi-URL fetch with progress + summary. Closes
// Lan's "Monday morning 5-tab" workflow from the brief.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

// batchResult is one row in the batch summary. Carried over channels.
type batchResult struct {
	URL         string  `json:"url"`
	Status      string  `json:"status"` // pass | fail | skip
	Source      string  `json:"source,omitempty"`
	Segments    int     `json:"segments,omitempty"`
	DurationMs  int64   `json:"duration_ms"`
	Error       string  `json:"error,omitempty"`
	OutPath     string  `json:"out_path,omitempty"`
	CostCredits float64 `json:"cost_credits,omitempty"`
}

const (
	batchDefaultConcurrency = 3
	batchMaxConcurrency     = 5
)

func newEpisodeBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagConcurrency     int
		flagOutDir          string
		flagPaid            bool
		flagProvider        string
		flagAutoPaid        bool
		flagContinueOnError bool
		flagFormat          string // md | json | jsonl (per-file shape; default md)
	)

	cmd := &cobra.Command{
		Use:   "batch [url1 url2 ...]",
		Short: "Fetch multiple episodes in parallel with progress + summary",
		Long: `Pulls multiple episode URLs concurrently via the standard cookie -> free -> paid
dispatcher. Each URL gets its own dispatch trace; failures don't abort the
batch (use --continue-on-error=false to make them abort). Cached episodes
re-hit cache; nothing duplicates.

Concurrency defaults to 3 and caps at 5 — higher values run into upstream
per-source rate limits (spoken.md, Spotify) and just queue up at the server.

Use --out-dir to write each transcript to <dir>/<sha256(url)>.md|json|jsonl
instead of stdout; a summary table still prints. Use --json for a machine
output of the summary itself (agents prefer this).`,
		Example: `  podcast-goat-pp-cli episode batch <url1> <url2> <url3>
  podcast-goat-pp-cli episode batch <url1> <url2> --out-dir ~/transcripts --format md
  podcast-goat-pp-cli episode batch <url1> <url2> <url3> <url4> <url5> --concurrency 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "false"}, // writes cache, files, spend log
		Args:        cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			urls := args

			// Concurrency: clamp to [1, batchMaxConcurrency].
			conc := flagConcurrency
			if conc <= 0 {
				conc = batchDefaultConcurrency
			}
			if conc > batchMaxConcurrency {
				conc = batchMaxConcurrency
			}
			if conc > len(urls) {
				conc = len(urls)
			}

			// Dispatcher options shared by every URL.
			providers := []string{}
			if flagProvider != "" {
				for _, p := range strings.Split(flagProvider, ",") {
					if t := strings.TrimSpace(p); t != "" {
						providers = append(providers, t)
					}
				}
			}
			opts := dispatch.Options{
				AllowPaid:        flagPaid || flagAutoPaid,
				AllowedProviders: providers,
				DryRun:           flags.dryRun,
				Explain:          flags.dryRun,
			}

			if cliutil.IsVerifyEnv() && !flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would batch-fetch %d URLs (verify mode short-circuit)\n", len(urls))
				return nil
			}

			// Out-dir setup.
			if flagOutDir != "" {
				if err := os.MkdirAll(flagOutDir, 0o755); err != nil {
					return fmt.Errorf("create out-dir: %w", err)
				}
			}
			if flagFormat == "" {
				flagFormat = "md"
			}

			// Workerpool: N goroutines pull from urlsCh, push to resultsCh.
			// Each result also carries its input index so we can render the
			// summary in input order regardless of completion order.
			type indexedURL struct {
				idx int
				url string
			}
			type indexedResult struct {
				idx int
				res batchResult
			}
			urlsCh := make(chan indexedURL, len(urls))
			resultsCh := make(chan indexedResult, len(urls))
			var wg sync.WaitGroup

			for w := 0; w < conc; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for item := range urlsCh {
						resultsCh <- indexedResult{idx: item.idx, res: batchFetchOne(cmd.Context(), item.url, opts, flagOutDir, flagFormat)}
					}
				}()
			}
			for i, u := range urls {
				urlsCh <- indexedURL{idx: i, url: u}
			}
			close(urlsCh)

			// Progress + collection. Progress prints to stderr so stdout
			// remains parseable (--json case).
			go func() { wg.Wait(); close(resultsCh) }()

			results := make([]batchResult, len(urls))
			done := 0
			var firstErr error
			for r := range resultsCh {
				results[r.idx] = r.res
				done++
				progressLine(cmd.ErrOrStderr(), done, len(urls), r.res)
				if r.res.Status == "fail" && firstErr == nil {
					firstErr = fmt.Errorf("first failure: %s -- %s", r.res.URL, r.res.Error)
				}
				// --continue-on-error=false note: we already dispatched
				// every worker, so we drain in-flight results then use
				// firstErr for the exit code below. We don't try to
				// cancel running fetches mid-flight — that would orphan
				// partial state.
			}

			// Summary output.
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			renderBatchTable(cmd.OutOrStdout(), results)

			// Exit code: success unless any failed AND --continue-on-error is false.
			if !flagContinueOnError && firstErr != nil {
				return firstErr
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&flagConcurrency, "concurrency", batchDefaultConcurrency,
		fmt.Sprintf("Parallel workers (default %d, max %d)", batchDefaultConcurrency, batchMaxConcurrency))
	cmd.Flags().StringVar(&flagOutDir, "out-dir", "",
		"Write each transcript to <dir>/<sha256(url)>.<format> instead of stdout")
	cmd.Flags().StringVar(&flagFormat, "format", "md", "Per-file output format when --out-dir is set: md|json|jsonl")
	cmd.Flags().BoolVar(&flagPaid, "paid", false, "Allow paid-tier adapters to fire")
	cmd.Flags().BoolVar(&flagAutoPaid, "auto-paid", false, "Implies --paid and --yes")
	cmd.Flags().StringVar(&flagProvider, "provider", "",
		"Restrict to specific adapter(s), comma-separated (e.g. spoken or spoken,taddy)")
	cmd.Flags().BoolVar(&flagContinueOnError, "continue-on-error", true,
		"Continue processing remaining URLs after a failure (default true)")
	return cmd
}

// batchFetchOne runs the dispatcher for a single URL and writes output as
// configured. Captures duration. Never panics — every error becomes a
// batchResult with status=fail.
func batchFetchOne(ctx context.Context, url string, opts dispatch.Options, outDir, format string) batchResult {
	start := time.Now()
	res := batchResult{URL: url, Status: "fail"}

	dispatchRes, err := dispatch.Dispatch(ctx, url, opts)
	res.DurationMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if dispatchRes.Transcript == nil {
		res.Error = "no transcript returned"
		return res
	}
	tr := dispatchRes.Transcript
	res.Status = "pass"
	res.Source = tr.Source
	res.Segments = len(tr.Segments)
	res.CostCredits = tr.CostCredits

	// Persist transcript to local store (cache write) regardless of out-dir;
	// out-dir is an additional output, not a replacement.
	if ps, perr := openPodcastStore(ctx); perr == nil {
		_ = ps.UpsertTranscript(ctx, tr)
		if tr.Tier == transcript.TierPaid && tr.CostCredits > 0 {
			_ = ps.RecordSpend(ctx, tr.Provider, tr.URL, tr.CostCredits, estimateUSD(tr.Provider, tr.CostCredits))
		}
	}

	if outDir != "" {
		body := batchFormatBody(tr, format)
		filename := batchOutFilename(url, format)
		path := filepath.Join(outDir, filename)
		if err := os.WriteFile(path, []byte(body), 0o644); err == nil {
			res.OutPath = path
		} else {
			// File write failure doesn't fail the fetch — surface in error
			// but keep status pass since the transcript IS cached.
			res.Error = "out-dir write failed: " + err.Error()
		}
	}
	return res
}

func batchFormatBody(tr *transcript.Transcript, format string) string {
	switch format {
	case "json":
		data, _ := json.MarshalIndent(tr, "", "  ")
		return string(data)
	case "jsonl":
		return tr.JSONL()
	default:
		return tr.CanonicalMarkdown()
	}
}

func batchOutFilename(url, format string) string {
	h := sha256.Sum256([]byte(url))
	short := hex.EncodeToString(h[:])[:16]
	ext := format
	if ext == "" {
		ext = "md"
	}
	return short + "." + ext
}

func progressLine(w io.Writer, done, total int, r batchResult) {
	marker := "ok"
	if r.Status != "pass" {
		marker = "FAIL"
	}
	src := r.Source
	if src == "" {
		src = "-"
	}
	fmt.Fprintf(w, "  [%d/%d] %s  %s  (%dms, %s)  %s\n",
		done, total, marker, r.URL, r.DurationMs, src,
		ellipsize(r.Error, 80))
}

func ellipsize(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func renderBatchTable(w io.Writer, results []batchResult) {
	pass, fail := 0, 0
	for _, r := range results {
		if r.Status == "pass" {
			pass++
		} else {
			fail++
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Batch summary: %d/%d ok, %d failed\n", pass, len(results), fail)
	for _, r := range results {
		src := r.Source
		if src == "" {
			src = "-"
		}
		urlShort := r.URL
		if len(urlShort) > 60 {
			urlShort = urlShort[:57] + "..."
		}
		if r.Status == "pass" {
			fmt.Fprintf(w, "  ok    %-60s  %-12s segs:%d  %dms\n", urlShort, src, r.Segments, r.DurationMs)
		} else {
			fmt.Fprintf(w, "  fail  %-60s  %-12s %s\n", urlShort, src, ellipsize(r.Error, 60))
		}
	}
}

// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: render a whole script to a directory of numbered files.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/store"
	"github.com/spf13/cobra"
)

// batchFailure records one line the batch could not render. Failed lines stay
// out of the cost and byte totals so a partial run never reports a number that
// includes work that did not happen.
type batchFailure struct {
	LineNo int    `json:"line_no"`
	Text   string `json:"text"`
	Error  string `json:"error"`
}

// batchSummary is the JSON contract `tts batch` prints.
//
// Count and the cost fields report the API calls actually made, not the number
// of input lines: identical lines are rendered once and the audio is copied to
// each output file. Deduped is how many lines reused another line's render, and
// Files is how many audio files were written. Billing a duplicate line twice
// would overstate spend by exactly the amount that was never charged.
type batchSummary struct {
	Count            int              `json:"count"`
	Deduped          int              `json:"deduped"`
	Files            int              `json:"files"`
	BytesIn          int64            `json:"bytes_in"`
	BytesOut         int64            `json:"bytes_out"`
	CostUSD          float64          `json:"cost_usd"`
	CostUSDPaidEquiv float64          `json:"cost_usd_paid_equiv"`
	OutDir           string           `json:"out_dir"`
	Renders          []renderManifest `json:"renders"`
	Failed           []batchFailure   `json:"failed"`
	Note             string           `json:"note,omitempty"`
}

// walletBalance is the subset of GET /wallet/self/api-credit the budget guard
// reads. The vendor returns the credit as a decimal string, so the field stays
// raw and parseCreditValue accepts either a string or a number.
type walletBalance struct {
	Credit json.RawMessage `json:"credit"`
}

// parseCreditValue reads a credit balance that may arrive as "12.34" or 12.34.
func parseCreditValue(raw json.RawMessage) (float64, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0, fmt.Errorf("the response carried no credit value")
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strconv.ParseFloat(strings.TrimSpace(asString), 64)
	}
	var asNumber float64
	if err := json.Unmarshal(raw, &asNumber); err == nil {
		return asNumber, nil
	}
	return 0, fmt.Errorf("credit value %q is neither a string nor a number", trimmed)
}

// apiCreditPath is the dev-API credit ledger. It is a different ledger from
// the subscription package: a full package balance does not pay for API bytes.
const apiCreditPath = "/wallet/self/api-credit"

func newNovelTtsBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagInput       string
		flagLines       []string
		flagVoice       string
		flagOutDir      string
		flagConcurrency int
		flagBudgetGuard bool
		flagDialogue    bool
		flagSpeakerMap  []string
		flagDB          string
		opts            renderOptions
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Estimate a batch's cost against your live API credit and refuse to start if it would overdraw.",
		Long: `Renders every line of an input file to a numbered file in --out-dir, records
each one in the local render log, and prints an aggregate summary.

Input shapes, decided per line:
  plain      the whole line is the text
  JSONL      {"text": "...", "voice": "<model_id>"} overrides the batch voice
  dialogue   Alice: Hello there   (only with --dialogue)

--dialogue folds the whole script into ONE multi-speaker request. Speakers are
numbered in order of first appearance, tagged as <|speaker:N|>, and matched to
the reference_id array built from --speaker-map. Multi-speaker synthesis is an
s2-family capability; --model s1 is rejected before any call is made.

--budget-guard totals the estimated cost, reads GET /wallet/self/api-credit,
and exits 5 without rendering anything when the estimate exceeds the balance.

Failed lines are reported in "failed" and excluded from the byte and cost
totals, so a partial run never overstates what it produced.`,
		Example: strings.Trim(`
  fish-audio-pp-cli tts batch --line "Your table is ready." --line "Thanks for calling." --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --model s2.1-pro-free --out-dir ./out
  fish-audio-pp-cli tts batch --input script.jsonl --voice 7f92f8afb8ec43bf81429cc1c9199cb1 --out-dir ./out --concurrency 5 --budget-guard --json
  fish-audio-pp-cli tts batch --input dialogue.txt --out-dir ./out --dialogue --model s2.1-pro --speaker-map Alice=7f92f8afb8ec43bf81429cc1c9199cb1 --speaker-map Bob=2c9ab1e0f4d2426b9d3ef5a71b0c8d44
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"mcp:local-write":     "true",
			"pp:happy-args":       "--line=Your table is ready.;--voice=7f92f8afb8ec43bf81429cc1c9199cb1;--model=s2.1-pro-free;--out-dir=out",
			"pp:typed-exit-codes": "0,2,4,5,6,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tts batch")
			}
			if strings.TrimSpace(flagInput) == "" && len(flagLines) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--input or --line is required: pass the file holding one render per line, or one or more --line values"))
			}
			if strings.TrimSpace(flagOutDir) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--out-dir is required: pass the directory to write the numbered audio files to"))
			}
			model, format, latency, warning, err := resolveRenderOptions(&opts)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if warning != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
			}
			// The s2-family gate runs before the input file is read: an
			// unsupported model is a usage error regardless of what the script
			// says, and failing here costs nothing.
			if flagDialogue && !fishaudio.IsS2Family(model) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--dialogue requires an s2-family model (s2-pro, s2.1-pro, s2.1-pro-free); %q does not support multi-speaker <|speaker:N|> tags", model))
			}
			speakerMap, err := fishaudio.ParseSpeakerMap(flagSpeakerMap)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if !flagDialogue && strings.TrimSpace(flagVoice) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--voice is required: pass the model_id every line renders with, or use JSONL lines with a per-line voice"))
			}
			if flagConcurrency < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--concurrency must be at least 1"))
			}

			var content string
			if strings.TrimSpace(flagInput) != "" {
				// #nosec G304 -- flagInput is the operator's own --input value; reading it
				// is the flag's purpose.
				raw, err := os.ReadFile(flagInput)
				if err != nil {
					return usageErr(fmt.Errorf("reading --input %s: %w", flagInput, err))
				}
				content = string(raw)
			}
			if len(flagLines) > 0 {
				if content != "" && !strings.HasSuffix(content, "\n") {
					content += "\n"
				}
				content += strings.Join(flagLines, "\n") + "\n"
			}
			lines, err := fishaudio.ParseBatchInput(content, flagDialogue)
			if err != nil {
				return usageErr(err)
			}
			if len(lines) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--input %s has no renderable lines", flagInput))
			}

			summary := batchSummary{
				OutDir:  flagOutDir,
				Renders: make([]renderManifest, 0, len(lines)),
				Failed:  make([]batchFailure, 0),
			}
			// The live-dogfood matrix runs under a flat per-command timeout, so
			// a real batch would blow through it. One line still exercises the
			// whole path against the real API.
			if cliutil.IsDogfoodEnv() && len(lines) > 1 {
				lines = lines[:1]
				summary.Note = "curtailed to 1 line under the live-dogfood harness"
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Build the unit list. Dialogue collapses every line into a single
			// multi-speaker request; everything else is one request per line.
			var units []batchUnit
			if flagDialogue {
				dialogue, buildErr := fishaudio.BuildDialogue(lines, speakerMap)
				if buildErr != nil {
					_ = cmd.Usage()
					return usageErr(buildErr)
				}
				req := buildRenderRequest(dialogue.Text, "", model, format, latency, &opts)
				req.SpeakerVoiceIDs = dialogue.ReferenceIDs
				units = append(units, batchUnit{
					index:  1,
					lineNo: lines[0].LineNo,
					req:    req,
					voice:  strings.Join(dialogue.ReferenceIDs, ","),
					path:   filepath.Join(flagOutDir, fishaudio.BatchOutputName(1, format)),
				})
			} else {
				for i, line := range lines {
					voice := line.Voice
					if voice == "" {
						voice = flagVoice
					}
					req := buildRenderRequest(line.Text, voice, model, format, latency, &opts)
					units = append(units, batchUnit{
						index:  i + 1,
						lineNo: line.LineNo,
						req:    req,
						voice:  voice,
						path:   filepath.Join(flagOutDir, fishaudio.BatchOutputName(i+1, format)),
					})
				}
			}

			if flagBudgetGuard {
				if guardErr := enforceBudgetGuard(ctx, cmd, c, dedupeBatchUnits(units), model); guardErr != nil {
					return guardErr
				}
			}

			if err := os.MkdirAll(flagOutDir, 0o700); err != nil {
				return fmt.Errorf("creating --out-dir %s: %w", flagOutDir, err)
			}
			dbPath := fishRenderDBPath(flagDB)
			db, err := openRenderStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			jobs := dedupeBatchUnits(units)
			summary.Deduped = len(units) - len(jobs)
			results := runBatch(ctx, c, db, jobs, model, format, flagConcurrency)

			var sawRateLimit bool
			for _, r := range results {
				if r.err != nil {
					if ExitCode(r.err) == 7 {
						sawRateLimit = true
					}
					for _, unit := range r.job.units {
						summary.Failed = append(summary.Failed, batchFailure{LineNo: unit.lineNo, Text: truncate(unit.req.Text, 120), Error: r.err.Error()})
					}
					continue
				}
				// One API call, one billed render, however many files it wrote.
				summary.Count++
				summary.BytesIn += r.job.units[0].req.BytesIn64()
				summary.CostUSD += r.costUSD
				summary.CostUSDPaidEquiv += r.paidEquivUSD
				for _, manifest := range r.manifests {
					summary.BytesOut += manifest.BytesOut
					summary.Files++
					summary.Renders = append(summary.Renders, manifest)
				}
			}
			if summary.Deduped > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "%d duplicate line(s) reused an earlier render; %d API call(s) produced %d file(s)\n",
					summary.Deduped, summary.Count, summary.Files)
			}
			if len(summary.Failed) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d lines failed; totals cover the %d render(s) that succeeded\n",
					len(summary.Failed), len(units), summary.Count)
			}

			if outErr := emitBatchSummary(cmd, flags, summary); outErr != nil {
				return outErr
			}
			switch {
			case sawRateLimit:
				return rateLimitErr(fmt.Errorf("%d of %d lines hit the API rate limit; lower --concurrency and retry the failed lines", len(summary.Failed), len(units)))
			case len(summary.Failed) > 0:
				return partialFailureErr(fmt.Errorf("%d of %d lines failed", len(summary.Failed), len(units)))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagInput, "input", "", "File with one render per line: plain text, JSONL, or a dialogue script")
	cmd.Flags().StringArrayVar(&flagLines, "line", nil, "Inline line to render (repeatable); appended after --input when both are given")
	cmd.Flags().StringVar(&flagVoice, "voice", "", "Voice model_id every line renders with unless the line names its own")
	cmd.Flags().StringVar(&flagOutDir, "out-dir", "", "Directory to write the numbered audio files to")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 3, "Renders in flight at once, matching the vendor concurrency-slot tier")
	cmd.Flags().BoolVar(&flagBudgetGuard, "budget-guard", false, "Check the estimated cost against the API credit ledger and refuse to overdraw")
	cmd.Flags().BoolVar(&flagDialogue, "dialogue", false, "Read the input as a Speaker: text script and render it as one multi-speaker request")
	cmd.Flags().StringArrayVar(&flagSpeakerMap, "speaker-map", nil, "Map a dialogue speaker to a voice, as name=model_id; repeatable")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	registerRenderFlags(cmd, &opts)
	return cmd
}

// batchUnit is one input line and the file it must produce.
type batchUnit struct {
	index  int
	lineNo int
	voice  string
	path   string
	req    fishaudio.RenderRequest
}

// batchJob is one API call plus every output file it feeds. Two identical
// lines share a job: the audio is fetched once and written to both paths.
type batchJob struct {
	units []batchUnit
}

// batchResult pairs a job with what happened to it. The error is preserved per
// job rather than aborting the pool, so one bad line cannot discard the renders
// that already succeeded.
type batchResult struct {
	job          batchJob
	manifests    []renderManifest
	costUSD      float64
	paidEquivUSD float64
	err          error
}

// dedupeBatchUnits groups units by request hash, preserving input order.
//
// Rendering the same line twice is a double charge for one piece of audio.
// Grouping also keeps the render log honest: the store's request_hash is
// UNIQUE, so two identical units would otherwise collapse into a single row
// and `render spend` would under-report the batch.
func dedupeBatchUnits(units []batchUnit) []batchJob {
	jobs := make([]batchJob, 0, len(units))
	index := make(map[string]int, len(units))
	for _, unit := range units {
		hash := unit.req.Hash()
		if at, seen := index[hash]; seen {
			jobs[at].units = append(jobs[at].units, unit)
			continue
		}
		index[hash] = len(jobs)
		jobs = append(jobs, batchJob{units: []batchUnit{unit}})
	}
	return jobs
}

// runBatch renders every job through a bounded worker pool and returns the
// results in input order.
func runBatch(ctx context.Context, c *client.Client, db *store.Store, jobs []batchJob, model, format string, concurrency int) []batchResult {
	if concurrency > len(jobs) {
		concurrency = len(jobs)
	}
	results := make([]batchResult, len(jobs))
	for i := range jobs {
		results[i] = batchResult{job: jobs[i], err: errBatchNotStarted}
	}
	queue := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range queue {
				results[i] = renderBatchJob(ctx, c, db, jobs[i], model, format)
			}
		}()
	}
	for i := range jobs {
		select {
		case <-ctx.Done():
		case queue <- i:
			continue
		}
		break
	}
	close(queue)
	wg.Wait()
	return results
}

// errBatchNotStarted marks a job the pool never reached, which happens when the
// command context is cancelled mid-batch. Seeding every slot with it means a
// cancelled job is reported as a failure instead of silently counting as a
// zero-byte success.
var errBatchNotStarted = errors.New("render was cancelled before it started")

// renderBatchJob makes one API call and writes every output file the job owns.
func renderBatchJob(ctx context.Context, c *client.Client, db *store.Store, job batchJob, model, format string) batchResult {
	res := batchResult{job: job, manifests: make([]renderManifest, 0, len(job.units))}
	primary := job.units[0]
	audio, repaired, err := synthesize(ctx, c, primary.req)
	if err != nil {
		res.err = classifyRawAPIError(err)
		return res
	}
	if len(audio) == 0 {
		res.err = fmt.Errorf("line %d: POST %s returned no audio bytes", primary.lineNo, fishTTSPath)
		return res
	}
	cost, paidEquiv := fishaudio.TTSCost(primary.req.BytesIn(), model)
	res.costUSD = cost
	res.paidEquivUSD = paidEquiv

	requestHash := primary.req.Hash()
	for i, unit := range job.units {
		sum, writeErr := writeAudioFile(unit.path, audio)
		if writeErr != nil {
			res.err = writeErr
			return res
		}
		// Only the first file carries the cost: the rest are copies of audio
		// that was paid for once.
		rowCost, rowPaid := cost, paidEquiv
		if i > 0 {
			rowCost, rowPaid = 0, 0
		}
		id, insertErr := db.InsertRenderRow(ctx, store.RenderRow{
			RequestHash:      fishaudio.BatchRowHash(requestHash, unit.path),
			Text:             unit.req.Text,
			Model:            model,
			VoiceID:          unit.voice,
			Format:           format,
			BytesIn:          unit.req.BytesIn64(),
			BytesOut:         int64(len(audio)),
			CostUSD:          rowCost,
			CostUSDPaidEquiv: rowPaid,
			FilePath:         unit.path,
			FileSHA256:       sum,
			Source:           "tts batch",
		})
		if insertErr != nil {
			res.err = insertErr
			return res
		}
		res.manifests = append(res.manifests, renderManifest{
			ID:                id,
			File:              unit.path,
			BytesIn:           unit.req.BytesIn64(),
			BytesOut:          int64(len(audio)),
			SHA256:            sum,
			Model:             model,
			Voice:             unit.voice,
			Format:            format,
			CostUSD:           rowCost,
			CostUSDPaidEquiv:  rowPaid,
			Skipped:           i > 0,
			WAVHeaderRepaired: repaired,
		})
	}
	return res
}

// enforceBudgetGuard totals the estimate and refuses to start when the API
// credit ledger cannot cover it. The check reads the dev-API credit ledger,
// not the subscription package: they are separate balances and a full package
// does not pay for API bytes.
func enforceBudgetGuard(ctx context.Context, cmd *cobra.Command, c *client.Client, jobs []batchJob, model string) error {
	// Price the deduplicated jobs, not the raw lines: a batch that repeats a
	// line pays for it once, and an estimate that double-counts would refuse a
	// batch the balance can actually cover.
	var estimate float64
	for _, job := range jobs {
		cost, _ := fishaudio.TTSCost(job.units[0].req.BytesIn(), model)
		estimate += cost
	}
	data, err := c.Get(ctx, apiCreditPath, nil)
	if err != nil {
		return apiErr(fmt.Errorf("budget guard could not read %s: %w", apiCreditPath, err))
	}
	var balance walletBalance
	if err := json.Unmarshal(data, &balance); err != nil {
		return apiErr(fmt.Errorf("budget guard could not parse %s: %w", apiCreditPath, err))
	}
	available, err := parseCreditValue(balance.Credit)
	if err != nil {
		return apiErr(fmt.Errorf("budget guard could not read the credit balance: %w", err))
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "budget guard: estimate $%.6f against $%.6f of API credit\n", estimate, available)
	if estimate > available {
		return apiErr(fmt.Errorf("budget guard refused the batch: estimated $%.6f exceeds the $%.6f API credit balance; top up or drop --budget-guard to run anyway", estimate, available))
	}
	return nil
}

// emitBatchSummary prints the summary through the shared output helpers.
func emitBatchSummary(cmd *cobra.Command, flags *rootFlags, summary batchSummary) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "wrote %d file(s) into %s from %d API call(s)\n", summary.Files, summary.OutDir, summary.Count)
	if summary.Deduped > 0 {
		fmt.Fprintf(out, "  %d duplicate line(s) reused an earlier render\n", summary.Deduped)
	}
	fmt.Fprintf(out, "  %d bytes in, %d bytes out, $%.6f (paid equivalent $%.6f)\n",
		summary.BytesIn, summary.BytesOut, summary.CostUSD, summary.CostUSDPaidEquiv)
	for _, f := range summary.Failed {
		fmt.Fprintf(out, "  failed line %d: %s\n", f.LineNo, f.Error)
	}
	if summary.Note != "" {
		fmt.Fprintf(out, "  note: %s\n", summary.Note)
	}
	return nil
}

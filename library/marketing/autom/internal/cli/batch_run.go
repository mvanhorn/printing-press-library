// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

// batchItem is one keyword to check.
type batchItem struct {
	Query   string `json:"query"`
	Domain  string `json:"domain,omitempty"`
	Engine  string `json:"engine"`
	Country string `json:"country,omitempty"`
	Lang    string `json:"lang,omitempty"`
}

type batchRow struct {
	batchItem
	Position   int    `json:"position"`
	URL        string `json:"url,omitempty"`
	Results    int    `json:"result_count"`
	SnapshotID int64  `json:"snapshot_id,omitempty"`
	Error      string `json:"error,omitempty"`
}

type batchView struct {
	Total         int        `json:"total"`
	Succeeded     int        `json:"succeeded"`
	Failed        int        `json:"failed"`
	CreditsUsed   int        `json:"credits_used"`
	BalanceBefore *int       `json:"balance_before,omitempty"`
	Rows          []batchRow `json:"results"`
}

// maxBatchInput caps a keyword file or stdin (1 MiB is tens of thousands of keywords).
const maxBatchInput = 1 << 20

// key identifies one search; rows that share it share one billed call.
func (it batchItem) key() string {
	return asciiLower(it.Query) + " | " + it.Engine + " | " + asciiLower(it.Country) + " | " + asciiLower(it.Lang)
}

// asciiLower folds only ASCII letters, like SQLite's lower(), so a batch key
// and a later store lookup agree on which spellings are the same keyword.
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// maxBatchConcurrency caps parallel searches; Autom rate-limits per key.
const maxBatchConcurrency = 16

// mcpDefaultBudget caps the searches of one batch run started through MCP
// when the caller sets no --budget.
const mcpDefaultBudget = 50

// isBatchCSVHeader reports a CSV header: a line with a cell that is exactly
// "keyword" or "query" (a plain keyword such as "best keyword tool, free" is not).
func isBatchCSVHeader(line string) bool {
	if !strings.Contains(line, ",") {
		return false
	}
	for _, cell := range strings.Split(line, ",") {
		switch strings.ToLower(strings.Trim(strings.TrimSpace(cell), `"`)) {
		case "keyword", "query":
			return true
		}
	}
	return false
}

// parseBatchInput reads one keyword per line, or a CSV whose header has a
// "keyword" (or "query") column and optional domain, engine, country, lang
// columns. Blank lines and lines starting with # are skipped.
func parseBatchInput(r io.Reader, def batchItem) ([]batchItem, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBatchInput+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBatchInput {
		return nil, fmt.Errorf("keyword input is larger than %d bytes; split it into several runs", maxBatchInput)
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return make([]batchItem, 0), nil
	}
	if isBatchCSVHeader(strings.SplitN(text, "\n", 2)[0]) {
		rd := csv.NewReader(strings.NewReader(text))
		rd.FieldsPerRecord = -1
		rd.TrimLeadingSpace = true
		recs, err := rd.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("reading CSV: %w", err)
		}
		col := map[string]int{}
		for i, h := range recs[0] {
			col[strings.ToLower(strings.TrimSpace(h))] = i
		}
		get := func(rec []string, names ...string) string {
			for _, n := range names {
				if i, ok := col[n]; ok && i < len(rec) {
					return strings.TrimSpace(rec[i])
				}
			}
			return ""
		}
		out := make([]batchItem, 0, len(recs)-1)
		for _, rec := range recs[1:] {
			it := def
			it.Query = get(rec, "keyword", "query")
			if it.Query == "" || strings.HasPrefix(it.Query, "#") {
				continue
			}
			if v := get(rec, "domain", "target_domain"); v != "" {
				it.Domain = v
			}
			if v := get(rec, "engine"); v != "" {
				it.Engine = v
			}
			if v := get(rec, "country", "geo"); v != "" {
				it.Country = v
			}
			if v := get(rec, "lang", "language", "locale"); v != "" {
				it.Lang = v
			}
			out = append(out, it)
		}
		return out, nil
	}
	out := make([]batchItem, 0)
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		it := def
		it.Query = line
		out = append(out, it)
	}
	return out, sc.Err()
}

func newBatchRunCmd(flags *rootFlags) *cobra.Command {
	var keywords, engine, country, lang, domain, delay, dbPath string
	var concurrency, budget int
	var noBudgetCheck, noSave bool
	cmd := &cobra.Command{
		Use:   "run [file]",
		Short: "Check a list of keywords in one run, within a credit budget, and save every results page",
		Long: `Run rank checks for many keywords. Give a file (one keyword per line, or a CSV with a
keyword column and optional domain, engine, country, lang columns), "-" for stdin, or
--keywords. Before spending anything, the remaining balance is read from the free usage
endpoint and the run stops if it cannot cover every search. Costs 1 credit per distinct
keyword/engine/country/lang (rows differing only by domain share one search). Through MCP,
a run is capped at 50 searches unless --budget says otherwise.`,
		Example: strings.Trim(`
  autom-pp-cli batch run keywords.txt --domain autom.dev --country fr
  autom-pp-cli batch run --keywords "serp api pricing,google search scraper" --domain autom.dev --agent`, "\n"),
		Annotations: map[string]string{
			"pp:data-source": "live",
			"pp:happy-args":  "--keywords=serp api pricing;--domain=autom.dev",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "batch run")
			}
			def := batchItem{Domain: strings.TrimSpace(domain), Engine: strings.TrimSpace(engine), Country: strings.TrimSpace(country), Lang: strings.TrimSpace(lang)}
			items := make([]batchItem, 0)
			for _, k := range splitCSV(keywords) {
				it := def
				it.Query = k
				items = append(items, it)
			}
			if len(args) > 0 {
				if os.Getenv(cliutil.MCPChildEnvVar) != "" {
					// An MCP caller must not make the CLI read arbitrary local files.
					return usageErr(fmt.Errorf("file and stdin input are not available through MCP; pass the keywords with --keywords"))
				}
				var r io.Reader
				if args[0] == "-" {
					r = cmd.InOrStdin()
				} else {
					f, err := os.Open(args[0])
					if err != nil {
						return usageErr(fmt.Errorf("opening keyword file: %w", err))
					}
					defer f.Close()
					if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
						return usageErr(fmt.Errorf("%s is not a regular file", args[0]))
					}
					r = f
				}
				fromFile, err := parseBatchInput(r, def)
				if err != nil {
					return usageErr(err)
				}
				items = append(items, fromFile...)
			}
			if len(items) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no keywords: give a file, - for stdin, or --keywords"))
			}
			for i := range items {
				e, err := serp.LookupEngine(items[i].Engine)
				if err != nil {
					return usageErr(fmt.Errorf("keyword %q: %w", items[i].Query, err))
				}
				items[i].Engine = e.Name
				if items[i].Domain != "" {
					d, derr := domainArg(items[i].Domain, false)
					if derr != nil {
						return usageErr(fmt.Errorf("keyword %q: %w", items[i].Query, derr))
					}
					items[i].Domain = d
				}
			}
			if cliutil.IsDogfoodEnv() && len(items) > 1 {
				items = items[:1]
			}
			// One billed search per distinct (query, engine, country, lang); rows that
			// differ only by domain share it.
			searches := make([]batchItem, 0, len(items))
			seenSearch := map[string]bool{}
			for _, it := range items {
				if !seenSearch[it.key()] {
					seenSearch[it.key()] = true
					searches = append(searches, it)
				}
			}
			if budget < 0 {
				return usageErr(fmt.Errorf("--budget must be 0 (no cap) or a positive number of searches"))
			}
			if budget == 0 && os.Getenv(cliutil.MCPChildEnvVar) != "" {
				budget = mcpDefaultBudget
			}
			if budget > 0 && len(searches) > budget {
				return usageErr(fmt.Errorf("%d searches exceed --budget %d; raise the budget or trim the list", len(searches), budget))
			}
			pause, err := cliutil.ParseDurationLoose(delay)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --delay %q: %w", delay, err))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view := batchView{Total: len(items), Rows: make([]batchRow, 0, len(items))}
			if !noBudgetCheck {
				raw, err := c.GetNoCache(ctx, "/v1/usage", fullResponse(nil))
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var u struct {
					Remaining *int `json:"remaining"`
				}
				if json.Unmarshal(raw, &u) == nil && u.Remaining != nil {
					view.BalanceBefore = u.Remaining
					if *u.Remaining < len(searches) {
						return rateLimitErr(fmt.Errorf("the account has %d calls left but this run needs %d; top up or trim the list (skip this check with --no-budget-check)", *u.Remaining, len(searches)))
					}
				}
			}
			concurrency = min(max(concurrency, 1), maxBatchConcurrency)
			results, failures := cliutil.FanoutRun(ctx, searches,
				func(it batchItem) string { return it.key() },
				func(ctx context.Context, it batchItem) ([]serp.Result, error) {
					e, _ := serp.LookupEngine(it.Engine)
					res, err := fetchResults(ctx, c, e, it.Query, searchOpts{Country: it.Country, Lang: it.Lang})
					if pause > 0 {
						select {
						case <-time.After(pause):
						case <-ctx.Done():
						}
					}
					return res, err
				}, cliutil.WithConcurrency(concurrency))
			byKey := map[string][]serp.Result{}
			for _, r := range results {
				byKey[r.Source] = r.Value
			}
			fails, errs := fanoutFailures(failures)
			errByKey := map[string]string{}
			for _, f := range fails {
				errByKey[f.Source] = f.Error
			}
			var saver func(it batchItem, res []serp.Result) int64
			if !noSave {
				db, err := openSerpStore(ctx, dbPath)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
				} else {
					defer db.Close()
					saver = func(it batchItem, res []serp.Result) int64 {
						return saveSnapshot(ctx, cmd, db, it.Query, it.Engine, searchOpts{Country: it.Country, Lang: it.Lang}, res)
					}
				}
			}
			savedID := map[string]int64{}
			for _, it := range items {
				key := it.key()
				row := batchRow{batchItem: it}
				if msg, failed := errByKey[key]; failed {
					row.Error = msg
					view.Failed++
				} else {
					res := byKey[key]
					row.Results = len(res)
					if it.Domain != "" {
						row.Position, row.URL = serp.RankOf(res, it.Domain)
					}
					if saver != nil {
						if _, done := savedID[key]; !done {
							savedID[key] = saver(it, res)
						}
						row.SnapshotID = savedID[key]
					}
					view.Succeeded++
				}
				view.Rows = append(view.Rows, row)
			}
			view.CreditsUsed = len(searches) - len(failures)
			if view.Failed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d keywords failed (failed calls are not billed)\n", view.Failed, view.Total)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				rows := make([]map[string]any, 0, len(view.Rows))
				for _, r := range view.Rows {
					status := posLabel(r.Position)
					if r.Domain == "" {
						status = fmt.Sprintf("%d results", r.Results)
					}
					if r.Error != "" {
						status = "error: " + truncateText(r.Error, 50)
					}
					rows = append(rows, map[string]any{"keyword": truncateText(r.Query, 40), "engine": r.Engine, "country": r.Country, "result": status})
				}
				if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d checked, %d failed, %d credit(s) used\n", view.Succeeded, view.Failed, view.CreditsUsed)
			}
			if view.Succeeded == 0 {
				return allFailedErr("keyword", fails, errs)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&keywords, "keywords", "", "Keywords to check, comma-separated (in addition to the file)")
	cmd.Flags().StringVar(&engine, "engine", "google", "Default engine for rows that do not set one")
	cmd.Flags().StringVar(&country, "country", "", "Default country code for rows that do not set one")
	cmd.Flags().StringVar(&lang, "lang", "", "Default language code for rows that do not set one")
	cmd.Flags().StringVar(&domain, "domain", "", "Default domain to locate for rows that do not set one")
	cmd.Flags().IntVar(&concurrency, "concurrency", 2, "Parallel searches, 1 to 16 (Autom rate-limits per key)")
	cmd.Flags().StringVar(&delay, "delay", "0s", "Pause after each search, for example 1s")
	cmd.Flags().IntVar(&budget, "budget", 0, "Refuse to run more than this many distinct searches (rows differing only by domain share one; 0 = no cap, or 50 when run through MCP)")
	cmd.Flags().BoolVar(&noBudgetCheck, "no-budget-check", false, "Skip the balance check against the usage endpoint")
	cmd.Flags().BoolVar(&noSave, "no-save", false, "Do not save the results pages locally")
	addDBFlag(cmd, &dbPath)
	return cmd
}

func newBatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "batch",
		Short:       "Run many keyword checks in one go",
		Annotations: map[string]string{"pp:data-source": "live", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newBatchRunCmd(flags))
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newBatchCmd(flags))
	})
}

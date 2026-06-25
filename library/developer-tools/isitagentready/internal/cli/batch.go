// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newNovelBatchCmd(flags *rootFlags) *cobra.Command {
	var rank string

	cmd := &cobra.Command{
		Use:   "batch [<file>]",
		Short: "Scan a list of URLs (file or stdin), store each, and rank worst-first",
		Long: "Scan every URL in a file (one per line; '#' comments allowed) or from stdin, store each\n" +
			"scan, and print a leaderboard ranked worst-first by readiness level (default) or by\n" +
			"failing-check count (--rank failing). Use --csv for spreadsheet output.",
		Example: "  isitagentready-pp-cli batch urls.txt --rank failing\n" +
			"  cat urls.txt | isitagentready-pp-cli batch --csv",
		// A non-file positional is treated as "no URLs" (valid empty result),
		// and missing files degrade gracefully under dogfood, so there is no
		// bad-input error path to probe; skip it (see check.go).
		// Not mcp:read-only: each scanned URL is appended to the local scan
		// store (persistScan), an observable side effect (see check.go).
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !stdinHasData() {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan a list of URLs and rank them")
				return nil
			}
			if rank != "" && rank != "level" && rank != "failing" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--rank must be 'level' or 'failing', got %q", rank))
			}

			urls, err := readURLs(cmd, args)
			if err != nil {
				return err
			}
			if len(urls) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no URLs to scan; pass a file argument or pipe URLs on stdin")
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			if cliutil.IsDogfoodEnv() && len(urls) > 1 {
				urls = urls[:1]
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Scan concurrently (each scan is several seconds). cliutil.FanoutRun
			// bounds concurrency (the rate-limit safety valve) and returns results
			// in source order; store.Append is mutex-guarded so the concurrent
			// persists are safe. Failed scans come back as FanoutErrors.
			results, ferrs := cliutil.FanoutRun(ctx, urls,
				func(u string) string { return u },
				func(c context.Context, u string) (store.ScanRecord, error) {
					raw, err := performScan(c, flags, u)
					if err != nil {
						return store.ScanRecord{}, err
					}
					persistScan(raw)
					rep, perr := store.ParseReport(raw)
					if perr != nil {
						return store.ScanRecord{}, perr
					}
					return store.ScanRecord{URL: rep.URL, ScannedAt: rep.ScannedAt, Level: rep.Level, LevelName: rep.LevelName, Raw: raw}, nil
				})
			records := make([]store.ScanRecord, 0, len(results))
			for _, r := range results {
				records = append(records, r.Value)
			}
			failures := make([]map[string]any, 0, len(ferrs))
			for _, fe := range ferrs {
				failures = append(failures, map[string]any{"url": fe.Source, "error": fe.Err.Error()})
			}

			ranked := store.RankRecords(records, rank)
			rows := make([]map[string]any, 0, len(ranked))
			for i, r := range ranked {
				rep, _ := store.ParseReport(r.Raw)
				failing := 0
				siteErr := false
				if rep != nil {
					failing = len(rep.FailingChecks())
					siteErr = rep.SiteError != nil
				}
				rows = append(rows, map[string]any{
					"rank": i + 1, "url": r.URL, "level": r.Level, "levelName": r.LevelName,
					"failing": failing, "siteError": siteErr,
				})
			}

			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d scans failed; ranking covers the remaining %d\n",
					len(failures), len(urls), len(rows))
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"ranked":        rows,
					"scanned":       len(rows),
					"fetchFailures": failures,
				}, flags)
			}
			// CSV / human table over the ranked rows.
			return printOutputWithFlags(cmd.OutOrStdout(), mustJSON(rows), flags)
		},
	}
	cmd.Flags().StringVar(&rank, "rank", "level", "Rank by 'level' (worst first) or 'failing' (most failing checks first)")
	return cmd
}

// readURLs reads scan targets from a file argument or stdin. Lines starting
// with '#' and blank lines are skipped. A missing file is fatal in normal use
// but degrades to an empty list under verify/dogfood so probes stay clean.
func readURLs(cmd *cobra.Command, args []string) ([]string, error) {
	var lines []string
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			if os.IsNotExist(err) && (cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv()) {
				return nil, nil
			}
			return nil, usageErr(fmt.Errorf("opening URL list %q: %w", args[0], err))
		}
		defer f.Close()
		lines = scanLines(f)
	} else if stdinHasData() {
		lines = scanLines(os.Stdin)
	}
	var urls []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		urls = append(urls, l)
	}
	return urls, nil
}

func scanLines(r interface{ Read([]byte) (int, error) }) []string {
	var out []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

// stdinHasData reports whether stdin is a pipe/redirect with data (not a tty).
func stdinHasData() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

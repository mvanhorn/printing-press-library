// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newCrossrefCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crossref <url>",
		Short: "Show both readiness scanners' native verdicts for one site, side by side",
		Long: "Fetch (or read from local history) BOTH the isitagentready.com and is-agentic.com\n" +
			"reports for one site and show them side by side, plus the verdicts for the few\n" +
			"checks the two scanners genuinely measure the same way. crossref deliberately ignores\n" +
			"--source: it is by definition both scanners. The two scales are never merged; the\n" +
			"overlap table is the only place an agree/disagree verdict is possible.\n\n" +
			"If one scanner has no data (a never-scanned is-agentic domain 404s immediately), crossref\n" +
			"still prints the scanner that worked and marks the other unavailable with its reason.\n" +
			"Exit code is 0 unless BOTH scanners failed.",
		Example: "  isitagentready-pp-cli crossref https://example.com\n" +
			"  isitagentready-pp-cli crossref https://example.com --agent",
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show both scanners' verdicts for a URL")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a URL argument is required, e.g. crossref https://example.com"))
			}
			url := args[0]

			// Fan out both scanner fetches concurrently. The is-agentic client's
			// limiter already caps at 2 req/s, so no extra throttle here.
			type crossJob struct{ source, url string }
			jobs := []crossJob{
				{store.SourceIsItAgentReady, url},
				{store.SourceIsAgentic, url},
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			results, ferrs := cliutil.FanoutRun(ctx, jobs,
				func(j crossJob) string { return j.source },
				func(c context.Context, j crossJob) (json.RawMessage, error) {
					return resolveReportCtxForSource(c, flags, j.source, j.url)
				})

			// Capture each fetch outcome keyed by source.
			rawBySource := map[string]json.RawMessage{}
			errBySource := map[string]error{}
			for _, r := range results {
				rawBySource[r.Source] = r.Value
			}
			for _, fe := range ferrs {
				errBySource[fe.Source] = fe.Err
			}

			// Parse each available report; on either 404 (report_not_found is
			// the expected common case) we still render the other scanner.
			var iar *store.Report
			if raw, ok := rawBySource[store.SourceIsItAgentReady]; ok {
				if rep, err := store.ParseReport(raw); err == nil {
					iar = rep
				} else {
					errBySource[store.SourceIsItAgentReady] = err
					delete(rawBySource, store.SourceIsItAgentReady)
				}
			}
			var ag *store.AgenticReport
			if raw, ok := rawBySource[store.SourceIsAgentic]; ok {
				if rep, err := store.ParseAgenticReport(raw); err == nil {
					ag = rep
				} else {
					errBySource[store.SourceIsAgentic] = err
					delete(rawBySource, store.SourceIsAgentic)
				}
			}

			// Exit non-zero only when BOTH scanners failed; otherwise the
			// working one is enough to render a useful crossref.
			if iar == nil && ag == nil {
				iarErr := "no data"
				if errBySource[store.SourceIsItAgentReady] != nil {
					iarErr = errBySource[store.SourceIsItAgentReady].Error()
				}
				agErr := "no data"
				if errBySource[store.SourceIsAgentic] != nil {
					agErr = errBySource[store.SourceIsAgentic].Error()
				}
				return apiErr(fmt.Errorf("both scanners failed for %s: isitagentready (%s); is-agentic (%s)", url, iarErr, agErr))
			}

			res := store.BuildCrossRef(url, iar, ag)

			// Attach an "unavailable" note per missing scanner so the reason
			// (including is-agentic's verbatim resolution) survives in JSON too.
			if _, ok := rawBySource[store.SourceIsItAgentReady]; !ok && errBySource[store.SourceIsItAgentReady] != nil {
				res.Notes = append(res.Notes, "isitagentready unavailable: "+errBySource[store.SourceIsItAgentReady].Error())
			}
			if _, ok := rawBySource[store.SourceIsAgentic]; !ok && errBySource[store.SourceIsAgentic] != nil {
				res.Notes = append(res.Notes, "is-agentic unavailable: "+errBySource[store.SourceIsAgentic].Error())
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			return renderCrossRef(cmd, res, errBySource)
		},
	}
	return cmd
}

// renderCrossRef prints the two native verdict blocks, the overlap table, and
// the notes for a terminal.
func renderCrossRef(cmd *cobra.Command, res store.CrossRefResult, errBySource map[string]error) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, bold(res.URL))

	if res.IsItAgentReady != nil {
		fmt.Fprintln(out, "\n"+"isitagentready.com (native):")
		fmt.Fprintf(out, "  level %d — %s\n", res.IsItAgentReady.Level, res.IsItAgentReady.LevelName)
		fmt.Fprintf(out, "  checks: %d pass, %d fail, %d neutral (of %d)\n", res.IsItAgentReady.Pass, res.IsItAgentReady.Fail, res.IsItAgentReady.Neutral, res.IsItAgentReady.Total)
		if res.IsItAgentReady.ScannedAt != "" {
			fmt.Fprintf(out, "  scanned %s\n", res.IsItAgentReady.ScannedAt)
		}
		if res.IsItAgentReady.SiteError {
			fmt.Fprintln(out, "  (site error — the scanner could not fetch the target)")
		}
	} else {
		fmt.Fprintln(out, "\n"+"isitagentready.com (native): unavailable")
		if e, ok := errBySource[store.SourceIsItAgentReady]; ok && e != nil {
			fmt.Fprintf(out, "  %s\n", e.Error())
		}
	}

	if res.IsAgentic != nil {
		fmt.Fprintln(out, "\n"+"is-agentic.com (native):")
		fmt.Fprintf(out, "  score %d — %s\n", res.IsAgentic.Score, res.IsAgentic.ScoreLabel)
		fmt.Fprintf(out, "  eligible checks: %d\n", res.IsAgentic.EligibleChecks)
		fmt.Fprintf(out, "  essential: %d/%d passing\n", res.IsAgentic.EssentialPassing, res.IsAgentic.EssentialTotal)
		fmt.Fprintf(out, "  recommended: %d/%d passing\n", res.IsAgentic.RecommendedPassing, res.IsAgentic.RecommendedTotal)
		fmt.Fprintf(out, "  bonus: %.1f points\n", res.IsAgentic.BonusPoints)
		if res.IsAgentic.ScannedAt != "" {
			fmt.Fprintf(out, "  scanned %s\n", res.IsAgentic.ScannedAt)
		}
	} else {
		fmt.Fprintln(out, "\n"+"is-agentic.com (native): unavailable")
		if e, ok := errBySource[store.SourceIsAgentic]; ok && e != nil {
			fmt.Fprintf(out, "  %s\n", e.Error())
		}
	}

	fmt.Fprintln(out, "\nOverlap (checks both scanners measure the same way):")
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "  LABEL\tISITAGENTREADY\tIS-AGENTIC\tVERDICT")
	for _, o := range res.Overlap {
		fmt.Fprintf(tw, "  %s\t%s:%s\t%s:%s\t%s\n",
			o.Label, o.IsItAgentReadyCheck, o.IsItAgentReadyStatus,
			o.IsAgenticCheck, o.IsAgenticStatus, o.Verdict)
	}
	_ = tw.Flush()

	if len(res.Notes) > 0 {
		fmt.Fprintln(out, "\nNotes:")
		for _, n := range res.Notes {
			fmt.Fprintf(out, "  - %s\n", n)
		}
	}
	return nil
}

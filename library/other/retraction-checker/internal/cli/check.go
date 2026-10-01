// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for retraction-checker-pp-cli.

package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/retraction-checker/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelCheckCmd(flags *rootFlags) *cobra.Command {
	var mailto string
	cmd := &cobra.Command{
		Use:   "check <doi|pmid>",
		Short: "Tell whether a paper (by DOI or PMID) has been retracted, when, why, and where the notice is.",
		Long: "Check whether a single scientific paper has been retracted. Accepts a DOI or a PMID\n" +
			"(PMIDs are resolved to a DOI via the NCBI ID Converter first). Reports retraction\n" +
			"status, update type, date, source, and the retraction notice reference. Keyless.",
		Example:     "  retraction-checker-pp-cli check 10.1016/j.micpro.2020.103768 --json",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a DOI or PMID argument is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			limiter := cliutil.NewAdaptiveLimiter(flags.rateLimit)
			v := resolveAndCheck(ctx, c, mailto, args[0], limiter)
			// JSON/agent output serializes the full verdict, including any
			// error field, so structured consumers already see failures.
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), v, flags); err != nil {
					return err
				}
				if v.Error != "" {
					flags.deliverOnError = true
					return fmt.Errorf("%s", v.Error)
				}
				return nil
			}
			// Human-readable path: surface any check failure instead of
			// silently printing "NOT retracted". checkDOI sets v.DOI before
			// the network call, so a transient error or 404 leaves both
			// v.Error and v.DOI populated — the old v.DOI == "" gate swallowed it.
			if v.Error != "" {
				return fmt.Errorf("%s", v.Error)
			}
			writeHumanCheckResult(cmd.OutOrStdout(), v)
			return nil
		},
	}
	cmd.Flags().StringVar(&mailto, "mailto", "", "Contact email for the Crossref polite pool (better rate limits)")
	return cmd
}

func writeHumanCheckResult(w io.Writer, v retractionVerdict) {
	status := "NOT retracted"
	switch {
	case v.Retracted && v.ExpressionOfConcern:
		status = "RETRACTED + EDITORIAL CONCERN"
	case v.Retracted:
		status = "RETRACTED"
	case v.ExpressionOfConcern:
		status = "EDITORIAL CONCERN"
	}
	fmt.Fprintf(w, "%s\n", status)
	fmt.Fprintf(w, "  DOI:    %s\n", v.DOI)
	if v.Title != "" {
		fmt.Fprintf(w, "  Title:  %s\n", v.Title)
	}
	if v.Retracted || v.ExpressionOfConcern {
		if v.UpdateType != "" {
			fmt.Fprintf(w, "  Type:   %s\n", v.UpdateType)
		}
		if v.Date != "" {
			fmt.Fprintf(w, "  Date:   %s\n", v.Date)
		}
		if v.Source != "" {
			fmt.Fprintf(w, "  Source: %s\n", v.Source)
		}
		if v.NoticeURL != "" {
			fmt.Fprintf(w, "  Notice: %s\n", v.NoticeURL)
		}
	}
	if v.Retracted && v.ExpressionOfConcern {
		if v.ConcernDate != "" {
			fmt.Fprintf(w, "  Concern date:   %s\n", v.ConcernDate)
		}
		if v.ConcernNoticeURL != "" {
			fmt.Fprintf(w, "  Concern notice: %s\n", v.ConcernNoticeURL)
		}
	}
}

// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/agentic"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/is-agentic/internal/store"
	"github.com/spf13/cobra"
)

func openAgenticStore(ctx context.Context, flags *rootFlags) (*store.Store, string, error) {
	path := defaultDBPath("is-agentic-pp-cli")
	if flags != nil && flags.dataSource == "live" {
		return nil, path, nil
	}
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, path, err
	}
	return s, path, nil
}

func reportClient() *agentic.Client { return agentic.New() }

func fetchAndSave(ctx context.Context, s *store.Store, target string) (*agentic.Report, store.AgenticSnapshot, error) {
	report, err := reportClient().Fetch(ctx, target)
	if err != nil {
		return nil, store.AgenticSnapshot{}, err
	}
	if s == nil {
		return report, store.AgenticSnapshot{}, nil
	}
	snapshot, err := s.SaveAgenticSnapshot(ctx, report.Raw, report.FetchedAt)
	if err != nil {
		return nil, store.AgenticSnapshot{}, err
	}
	return report, snapshot, nil
}

func renderReport(w io.Writer, report *agentic.Report) error {
	if report == nil {
		return nil
	}
	fmt.Fprintf(w, "Is Agentic  %s\n\n", report.Parsed.DisplayTarget)
	if report.Parsed.Score == nil {
		fmt.Fprintln(w, "Score      unavailable")
	} else {
		fmt.Fprintf(w, "Score      %.1f / 100\n", *report.Parsed.Score)
	}
	if report.Parsed.ScoreLabel != "" {
		fmt.Fprintf(w, "Label      %s\n", report.Parsed.ScoreLabel)
	}
	fmt.Fprintf(w, "Scanned    %s\n", report.Parsed.ScannedAt)
	fmt.Fprintf(w, "Report     %s\n", report.Parsed.ReportUrl)
	fmt.Fprintln(w, "\nScore breakdown")
	for _, name := range []string{"essential", "recommended", "bonus"} {
		if bucket, ok := report.ScoreBreakdown[name]; ok {
			if name == "bonus" {
				fmt.Fprintf(w, "  %-12s +%.1f (%d positive signals)\n", name, bucket.Points, bucket.PositiveSignals)
			} else {
				fmt.Fprintf(w, "  %-12s %.1f / %.1f (%d/%d passed)\n", name, bucket.Earned, bucket.Available, bucket.Passing, bucket.Total)
			}
		}
	}
	if len(report.Issues) == 0 {
		fmt.Fprintln(w, "\nNo failed or partial checks.")
		return nil
	}
	fmt.Fprintf(w, "\nFindings (%d)\n", len(report.Issues))
	for i, issue := range report.Issues {
		fmt.Fprintf(w, "\n%d. %s · %s  %s\n", i+1, strings.ToUpper(issue.Result), strings.ToUpper(issue.Tier), issue.Name)
		if issue.Details != "" {
			fmt.Fprintf(w, "   Evidence  %s\n", issue.Details)
		}
		if issue.Recommendation != "" {
			fmt.Fprintf(w, "   Fix       %s\n", issue.Recommendation)
		}
	}
	return nil
}

func emitReport(cmd *cobra.Command, flags *rootFlags, report *agentic.Report) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) && !flags.asJSON && !flags.agent && flags.selectFields == "" && !flags.compact && !flags.csv && !flags.plain && isatty(cmd.OutOrStdout()) {
		return renderReport(cmd.OutOrStdout(), report)
	}
	return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(report.Raw), flags)
}

func emitSnapshots(cmd *cobra.Command, flags *rootFlags, items []store.AgenticSnapshot) error {
	if items == nil {
		items = make([]store.AgenticSnapshot, 0)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) && isatty(cmd.OutOrStdout()) && !flags.asJSON && !flags.agent && !flags.csv && !flags.plain && !flags.quiet {
		if len(items) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No local snapshots. Fetch a report first.")
			return nil
		}
		for _, item := range items {
			score := "null"
			if item.Score != nil {
				score = fmt.Sprintf("%.1f", *item.Score)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\t%s\n", item.ID, item.Target, score, item.ScannedAt, item.FetchedAt)
		}
		return nil
	}
	return printJSONFiltered(cmd.OutOrStdout(), items, flags)
}

func missingStore(cmd *cobra.Command, flags *rootFlags, path string) error {
	fmt.Fprintf(cmd.ErrOrStderr(), "no local report mirror at %s\nrun a report lookup or portfolio refresh first\n", path)
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), make([]store.AgenticSnapshot, 0), flags)
	}
	return nil
}

func parseReportSnapshot(item store.AgenticSnapshot) (*agentic.Report, error) {
	return agentic.ParseReport(item.Raw, parseTime(item.FetchedAt))
}
func parseTime(v string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, v)
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

// pp:data-source auto
// Supported strategies: auto, local, live, or computed.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type policyResult struct {
	Target           string   `json:"target"`
	Score            *float64 `json:"score,omitempty"`
	MinimumScore     float64  `json:"minimum_score"`
	EssentialPassing bool     `json:"essential_passing"`
	NewIssues        []string `json:"new_issues"`
	Passed           bool     `json:"passed"`
	Reasons          []string `json:"reasons"`
}

func newNovelCheckCmd(flags *rootFlags) *cobra.Command {
	var target string
	var minScore float64
	var requireEssential, noNewIssues bool
	cmd := &cobra.Command{Use: "check", Short: "Fail CI with an explicit, machine-readable readiness policy decision.", Example: "  is-agentic-pp-cli check --target https://is-agentic.com --min-score 80 --json", Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && !hasChangedLocalFlags(cmd) && !flags.dryRun {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "evaluate readiness policy")
		}
		if target == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--target is required"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		s, path, err := openAgenticStore(ctx, flags)
		if err != nil {
			return err
		}
		if s == nil {
			return missingStore(cmd, flags, path)
		}
		defer s.Close()
		report, snap, err := fetchAndSave(ctx, s, target)
		if err != nil {
			return fmt.Errorf("fetching policy target: %w", err)
		}
		result := policyResult{Target: report.Parsed.Target, Score: report.Parsed.Score, MinimumScore: minScore, EssentialPassing: true, NewIssues: make([]string, 0), Reasons: make([]string, 0)}
		if report.Parsed.Score == nil {
			result.Reasons = append(result.Reasons, "report score is unavailable")
		} else if *report.Parsed.Score < minScore {
			result.Reasons = append(result.Reasons, fmt.Sprintf("score %.1f is below minimum %.1f", *report.Parsed.Score, minScore))
		}
		if requireEssential {
			if b, ok := report.ScoreBreakdown["essential"]; ok {
				result.EssentialPassing = b.Passing >= b.Total
				if !result.EssentialPassing {
					result.Reasons = append(result.Reasons, "not every applicable Essential check passed")
				}
			}
		}
		if noNewIssues {
			if priorItems, e := s.ListAgenticSnapshots(ctx, target, 2); e == nil && len(priorItems) >= 2 {
				prior, _ := parseReportSnapshot(priorItems[1])
				old := map[string]bool{}
				for _, i := range prior.Issues {
					old[i.Id] = true
				}
				for _, i := range report.Issues {
					if !old[i.Id] {
						result.NewIssues = append(result.NewIssues, i.Id)
					}
				}
				if len(result.NewIssues) > 0 {
					result.Reasons = append(result.Reasons, fmt.Sprintf("%d new issue(s) appeared since the previous snapshot", len(result.NewIssues)))
				}
			}
		}
		result.Passed = len(result.Reasons) == 0
		_ = snap
		if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
			return err
		}
		if !result.Passed {
			return notFoundErr(fmt.Errorf("readiness policy failed for %s", target))
		}
		return nil
	}}
	cmd.Flags().StringVar(&target, "target", "", "public URL to evaluate")
	cmd.Flags().Float64Var(&minScore, "min-score", 0, "minimum score required (0 disables)")
	cmd.Flags().BoolVar(&requireEssential, "require-essential", false, "require every applicable Essential check to pass")
	cmd.Flags().BoolVar(&noNewIssues, "no-new-issues", false, "fail when new issue IDs appear after the baseline")
	return cmd
}

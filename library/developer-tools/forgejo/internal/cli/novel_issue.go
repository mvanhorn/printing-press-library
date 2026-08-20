// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newNovelIssueDashboardCmd(flags *rootFlags) *cobra.Command {
	var assignee, label, state string
	var allHosts, allRepos bool

	cmd := &cobra.Command{
		Use:     "dashboard",
		Aliases: []string{"dash"},
		Short:   "See all your open issues across repos in one sorted table",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would fetch issues from /repos/issues/search")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"state": state,
				"type":  "issues",
				"limit": "50",
			}
			if assignee == "" || assignee == "@me" {
				params["assigned"] = "true"
			}
			if label != "" {
				params["labels"] = label
			}

			data, prov, err := resolvePaginatedRead(cmd.Context(), c, flags, "repos-issues", "/repos/issues/search", params, nil, true, "page", "", "", cmd.ErrOrStderr())
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			var issues []map[string]any
			if json.Unmarshal(data, &issues) != nil || len(issues) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No issues found.")
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "REPO\t#\tTITLE\tLABELS\tUPDATED")
			for _, issue := range issues {
				repo := ""
				if r, ok := issue["repository"].(map[string]any); ok {
					repo = fmt.Sprintf("%v", r["full_name"])
				}
				num := fmt.Sprintf("%.0f", issue["number"])
				title := truncate(fmt.Sprintf("%v", issue["title"]), 50)
				labels := ""
				if labArr, ok := issue["labels"].([]any); ok {
					var names []string
					for _, l := range labArr {
						if lm, ok := l.(map[string]any); ok {
							names = append(names, fmt.Sprintf("%v", lm["name"]))
						}
					}
					labels = strings.Join(names, ",")
				}
				updated := ""
				if u, ok := issue["updated"].(string); ok && len(u) >= 10 {
					updated = u[:10]
				} else if u, ok := issue["updated_at"].(string); ok && len(u) >= 10 {
					updated = u[:10]
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", repo, num, title, labels, updated)
			}
			_ = tw.Flush()

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(issues), prov)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&assignee, "assignee", "@me", `Filter by assignee; use "@me" for self`)
	cmd.Flags().StringVar(&label, "label", "", "Filter by label name")
	cmd.Flags().StringVar(&state, "state", "open", "Issue state: open, closed, or all")
	cmd.Flags().BoolVar(&allHosts, "all-hosts", false, "Aggregate issues across all configured host profiles (not yet implemented)")
	cmd.Flags().BoolVar(&allRepos, "all-repos", false, "Show issues across all accessible repos (default when --assignee is set)")
	_ = allHosts // multi-host aggregation requires SQLite sync; single-host API used otherwise
	_ = allRepos // cross-repo search uses /repos/issues/search regardless

	return cmd
}

func newNovelIssueSweepCmd(flags *rootFlags) *cobra.Command {
	var owner, repo, staleAfter, label, comment string
	var closeIssue bool

	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Find stale issues, label them, optionally close — with --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would scan issues for staleness and apply labels/comments")
			}
			if owner == "" || repo == "" {
				return fmt.Errorf("--owner and --repo are required; e.g. fj issue sweep --owner acme --repo myrepo --stale-after 90d")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Parse stale-after duration
			staleDuration, err := parseStaleDuration(staleAfter)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --stale-after %q: %w", staleAfter, err))
			}
			threshold := time.Now().Add(-staleDuration)

			// Fetch open issues
			path := fmt.Sprintf("/repos/%s/%s/issues", owner, repo)
			data, err := c.Get(cmd.Context(), path, map[string]string{
				"state": "open",
				"type":  "issues",
				"limit": "50",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var issues []map[string]any
			if err := json.Unmarshal(data, &issues); err != nil {
				return fmt.Errorf("parsing issues: %w", err)
			}

			// Filter stale issues
			var stale []map[string]any
			for _, issue := range issues {
				updatedStr, _ := issue["updated_at"].(string)
				if updatedStr == "" {
					continue
				}
				updated, err := time.Parse(time.RFC3339, updatedStr)
				if err != nil {
					continue
				}
				if updated.Before(threshold) {
					stale = append(stale, issue)
				}
			}

			if len(stale) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stale issues found (threshold: %s).\n", threshold.Format("2006-01-02"))
				return nil
			}

			// Print table
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "#\tTITLE\tUPDATED\tACTION")
			action := "label"
			if closeIssue {
				action = "label+close"
			}
			if flags.dryRun {
				action = "[dry-run] " + action
			}
			for _, issue := range stale {
				num := fmt.Sprintf("%.0f", issue["number"])
				title := truncate(fmt.Sprintf("%v", issue["title"]), 50)
				updated := ""
				if u, ok := issue["updated_at"].(string); ok && len(u) >= 10 {
					updated = u[:10]
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", num, title, updated, action)
			}
			_ = tw.Flush()

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d stale issue(s) found. Re-run without --dry-run to apply changes.\n", len(stale))
				return nil
			}

			if !flags.yes && !flags.noInput {
				fmt.Fprintf(os.Stderr, "\nApply changes to %d issue(s)? [y/N] ", len(stale))
				var resp string
				_, _ = fmt.Scanln(&resp)
				if !strings.EqualFold(resp, "y") {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			// Apply changes
			for _, issue := range stale {
				num := fmt.Sprintf("%.0f", issue["number"])

				if label != "" {
					labelPath := fmt.Sprintf("/repos/%s/%s/issues/%s/labels", owner, repo, num)
					_, _, _ = c.Post(cmd.Context(), labelPath, map[string]any{"labels": []string{label}})
				}

				if comment != "" {
					commentPath := fmt.Sprintf("/repos/%s/%s/issues/%s/comments", owner, repo, num)
					_, _, _ = c.Post(cmd.Context(), commentPath, map[string]any{"body": comment})
				}

				if closeIssue {
					editPath := fmt.Sprintf("/repos/%s/%s/issues/%s", owner, repo, num)
					_, _, _ = c.Post(cmd.Context(), editPath, map[string]any{"state": "closed"})
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Applied changes to %d stale issue(s).\n", len(stale))
			return nil
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner (required)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name (required)")
	cmd.Flags().StringVar(&staleAfter, "stale-after", "90d", "Mark issues with no activity older than this as stale (e.g. 30d, 6m)")
	cmd.Flags().StringVar(&label, "label", "stale", "Label to apply to stale issues")
	cmd.Flags().StringVar(&comment, "comment", "", "Comment to post on stale issues")
	cmd.Flags().BoolVar(&closeIssue, "close", false, "Also close stale issues")

	return cmd
}

// parseStaleDuration parses durations like "90d", "6m", "2w", or standard Go durations.
func parseStaleDuration(s string) (time.Duration, error) {
	if s == "" {
		return 90 * 24 * time.Hour, nil
	}
	// Try standard Go duration first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	// Try custom suffixes: d=days, w=weeks, m=months (approx)
	if len(s) < 2 {
		return 0, fmt.Errorf("unrecognized duration format")
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	var n int
	if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
		return 0, fmt.Errorf("unrecognized duration format")
	}
	switch unit {
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'm':
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	case 'y':
		return time.Duration(n) * 365 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unrecognized unit %q (use d, w, m, y)", string(unit))
	}
}

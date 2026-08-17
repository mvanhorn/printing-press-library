// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newNovelPRQueueCmd(flags *rootFlags) *cobra.Command {
	var owner, repo, reviewRequested string
	var limit int
	var allRepos bool

	cmd := &cobra.Command{
		Use:   "queue",
		Short: "See PRs awaiting your review across repos",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would fetch PR review queue")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			params := map[string]string{
				"type":  "pulls",
				"state": "open",
				"limit": fmt.Sprintf("%d", limit),
			}
			// Apply review-requested filter: Forgejo's search API uses "review_requested"
			// to filter PRs where the given user is a requested reviewer.
			if reviewRequested != "" && reviewRequested != "@me" {
				params["review_requested"] = reviewRequested
			} else {
				// Default: show PRs assigned to the current user (proxy for review queue).
				params["assigned"] = "true"
			}

			var data json.RawMessage
			var prov DataProvenance

			if owner != "" && repo != "" {
				// Specific repo
				path := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)
				data, prov, err = resolvePaginatedRead(cmd.Context(), c, flags, "pulls", path, map[string]string{
					"state": "open",
					"limit": fmt.Sprintf("%d", limit),
				}, nil, false, "page", "", "", cmd.ErrOrStderr())
			} else {
				// Cross-repo search
				data, prov, err = resolvePaginatedRead(cmd.Context(), c, flags, "pulls", "/repos/issues/search", params, nil, false, "page", "", "", cmd.ErrOrStderr())
			}
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

			var prs []map[string]any
			if json.Unmarshal(data, &prs) != nil || len(prs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No PRs awaiting review found.")
				return nil
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "REPO\t#\tTITLE\tAUTHOR\tAPPROVALS\tUPDATED")
			for _, pr := range prs {
				repoName := ""
				if r, ok := pr["repository"].(map[string]any); ok {
					repoName = fmt.Sprintf("%v", r["full_name"])
				} else if owner != "" && repo != "" {
					repoName = owner + "/" + repo
				}
				num := fmt.Sprintf("%.0f", pr["number"])
				title := truncate(fmt.Sprintf("%v", pr["title"]), 45)
				author := ""
				if u, ok := pr["user"].(map[string]any); ok {
					author = fmt.Sprintf("%v", u["login"])
				}
				approvals := extractApprovalCount(pr)
				updated := ""
				if u, ok := pr["updated_at"].(string); ok && len(u) >= 10 {
					updated = u[:10]
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", repoName, num, title, author, approvals, updated)
			}
			_ = tw.Flush()

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(prs), prov)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner (optional, searches all repos if omitted)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name (optional, requires --owner)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of PRs to show")
	cmd.Flags().BoolVar(&allRepos, "all-repos", false, "Search all accessible repos (default when --owner is not set)")
	cmd.Flags().StringVar(&reviewRequested, "review-requested", "", "Filter by reviewer (use @me for yourself)")
	_ = allRepos

	return cmd
}

func extractApprovalCount(pr map[string]any) string {
	// Try various fields where approval count might be
	if reviews, ok := pr["reviews"].([]any); ok {
		count := 0
		for _, r := range reviews {
			if rm, ok := r.(map[string]any); ok {
				if fmt.Sprintf("%v", rm["state"]) == "APPROVED" {
					count++
				}
			}
		}
		return fmt.Sprintf("%d", count)
	}
	// Check for review_comments count as a proxy
	if rc, ok := pr["review_comments"].(float64); ok {
		_ = rc
	}
	// Check requested reviewers for "N reviewers pending"
	if rr, ok := pr["requested_reviewers"].([]any); ok {
		pending := len(rr)
		if rrt, ok := pr["requested_teams"].([]any); ok {
			pending += len(rrt)
		}
		if pending > 0 {
			return fmt.Sprintf("0 (pending %d)", pending)
		}
	}
	return "?"
}

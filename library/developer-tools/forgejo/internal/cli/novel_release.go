// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelReleaseChangelogCmd(flags *rootFlags) *cobra.Command {
	var owner, repo, format, section string

	cmd := &cobra.Command{
		Use:   "changelog <from-tag> <to-tag>",
		Short: "Generate a changelog between two tags from closed issues and merged PRs",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would generate changelog from closed issues and merged PRs between two tags")
			}
			if owner == "" || repo == "" {
				return fmt.Errorf("--owner and --repo are required; e.g. fj release changelog v1.0 v1.1 --owner acme --repo myrepo")
			}
			fromTag := args[0]
			toTag := args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Get from-tag release to find its timestamp
			fromData, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s/releases/tags/%s", owner, repo, fromTag), nil)
			if err != nil {
				return classifyAPIError(fmt.Errorf("getting from-tag release %q: %w", fromTag, err), flags)
			}
			var fromRelease map[string]any
			if err := json.Unmarshal(fromData, &fromRelease); err != nil {
				return fmt.Errorf("parsing from-tag release: %w", err)
			}
			fromDate, _ := fromRelease["published_at"].(string)
			if fromDate == "" {
				fromDate, _ = fromRelease["created_at"].(string)
			}

			// Fetch closed issues since fromDate
			issueParams := map[string]string{
				"state": "closed",
				"type":  "issues",
				"limit": "50",
			}
			if fromDate != "" {
				issueParams["since"] = fromDate
			}
			issueData, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s/issues", owner, repo), issueParams)
			if err != nil {
				return classifyAPIError(fmt.Errorf("fetching issues: %w", err), flags)
			}
			var issues []map[string]any
			_ = json.Unmarshal(issueData, &issues)

			// Fetch merged PRs
			prData, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s/pulls", owner, repo), map[string]string{
				"state": "closed",
				"limit": "50",
			})
			if err != nil {
				return classifyAPIError(fmt.Errorf("fetching PRs: %w", err), flags)
			}
			var prs []map[string]any
			_ = json.Unmarshal(prData, &prs)

			// Filter PRs by merged_at in window
			var mergedPRs []map[string]any
			for _, pr := range prs {
				mergedAt, _ := pr["merged_at"].(string)
				if mergedAt == "" {
					continue
				}
				if fromDate != "" {
					t, err := time.Parse(time.RFC3339, mergedAt)
					from, fromErr := time.Parse(time.RFC3339, fromDate)
					if err == nil && fromErr == nil && t.Before(from) {
						continue
					}
				}
				mergedPRs = append(mergedPRs, pr)
			}

			// Group by label sections if requested
			sectionLabels := []string{}
			if section != "" {
				sectionLabels = strings.Split(section, ",")
			}

			out := cmd.OutOrStdout()
			switch format {
			case "json":
				result := map[string]any{
					"from":   fromTag,
					"to":     toTag,
					"issues": issues,
					"prs":    mergedPRs,
				}
				return printJSONFiltered(out, result, flags)
			case "md":
				return printChangelogMarkdown(out, fromTag, toTag, issues, mergedPRs, sectionLabels)
			default: // "text"
				return printChangelogText(out, fromTag, toTag, issues, mergedPRs, sectionLabels)
			}
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner (required)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name (required)")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text, md, json")
	cmd.Flags().StringVar(&section, "section", "", "Comma-separated label names to group by (e.g. bug,feature,enhancement)")

	return cmd
}

func issueHasLabel(issue map[string]any, labelName string) bool {
	labArr, ok := issue["labels"].([]any)
	if !ok {
		return false
	}
	for _, l := range labArr {
		if lm, ok := l.(map[string]any); ok {
			if fmt.Sprintf("%v", lm["name"]) == labelName {
				return true
			}
		}
	}
	return false
}

func printChangelogText(w io.Writer, fromTag, toTag string, issues, prs []map[string]any, sections []string) error {
	fmt.Fprintf(w, "Changelog: %s → %s\n", fromTag, toTag)
	fmt.Fprintln(w)

	if len(sections) > 0 {
		for _, sec := range sections {
			fmt.Fprintf(w, "[%s]\n", sec)
			for _, issue := range issues {
				if issueHasLabel(issue, sec) {
					fmt.Fprintf(w, "  #%.0f %s\n", issue["number"], issue["title"])
				}
			}
			for _, pr := range prs {
				if issueHasLabel(pr, sec) {
					fmt.Fprintf(w, "  PR #%.0f %s\n", pr["number"], pr["title"])
				}
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "[Other]\n")
		for _, issue := range issues {
			hasSection := false
			for _, sec := range sections {
				if issueHasLabel(issue, sec) {
					hasSection = true
					break
				}
			}
			if !hasSection {
				fmt.Fprintf(w, "  #%.0f %s\n", issue["number"], issue["title"])
			}
		}
		for _, pr := range prs {
			hasSection := false
			for _, sec := range sections {
				if issueHasLabel(pr, sec) {
					hasSection = true
					break
				}
			}
			if !hasSection {
				fmt.Fprintf(w, "  PR #%.0f %s\n", pr["number"], pr["title"])
			}
		}
		return nil
	}

	if len(issues) > 0 {
		fmt.Fprintln(w, "Issues:")
		for _, issue := range issues {
			fmt.Fprintf(w, "  #%.0f %s\n", issue["number"], issue["title"])
		}
		fmt.Fprintln(w)
	}
	if len(prs) > 0 {
		fmt.Fprintln(w, "Pull Requests:")
		for _, pr := range prs {
			fmt.Fprintf(w, "  PR #%.0f %s\n", pr["number"], pr["title"])
		}
	}
	return nil
}

func printChangelogMarkdown(w io.Writer, fromTag, toTag string, issues, prs []map[string]any, sections []string) error {
	fmt.Fprintf(w, "## Changelog: %s → %s\n\n", fromTag, toTag)

	if len(sections) > 0 {
		for _, sec := range sections {
			fmt.Fprintf(w, "### %s\n\n", sec)
			for _, issue := range issues {
				if issueHasLabel(issue, sec) {
					fmt.Fprintf(w, "- #%.0f %s\n", issue["number"], issue["title"])
				}
			}
			for _, pr := range prs {
				if issueHasLabel(pr, sec) {
					fmt.Fprintf(w, "- PR #%.0f %s\n", pr["number"], pr["title"])
				}
			}
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "### Other\n\n")
		for _, issue := range issues {
			hasSection := false
			for _, sec := range sections {
				if issueHasLabel(issue, sec) {
					hasSection = true
					break
				}
			}
			if !hasSection {
				fmt.Fprintf(w, "- #%.0f %s\n", issue["number"], issue["title"])
			}
		}
		for _, pr := range prs {
			hasSection := false
			for _, sec := range sections {
				if issueHasLabel(pr, sec) {
					hasSection = true
					break
				}
			}
			if !hasSection {
				fmt.Fprintf(w, "- PR #%.0f %s\n", pr["number"], pr["title"])
			}
		}
		return nil
	}

	if len(issues) > 0 {
		fmt.Fprintf(w, "### Issues\n\n")
		for _, issue := range issues {
			fmt.Fprintf(w, "- #%.0f %s\n", issue["number"], issue["title"])
		}
		fmt.Fprintln(w)
	}
	if len(prs) > 0 {
		fmt.Fprintf(w, "### Pull Requests\n\n")
		for _, pr := range prs {
			fmt.Fprintf(w, "- PR #%.0f %s\n", pr["number"], pr["title"])
		}
	}
	return nil
}

func newNovelReleaseUploadCmd(flags *rootFlags) *cobra.Command {
	var owner, repo string
	var retry int
	var progress bool

	cmd := &cobra.Command{
		Use:   "upload <tag> <file>...",
		Short: "Upload release assets with progress and retry",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeNoop(flags, "dry-run", "dry-run: would upload release assets with progress and retry")
			}
			if owner == "" || repo == "" {
				return fmt.Errorf("--owner and --repo are required; e.g. fj release upload v1.0 file.tar.gz --owner acme --repo myrepo")
			}
			tag := args[0]
			filePatterns := args[1:]

			// Expand globs
			var files []string
			for _, pattern := range filePatterns {
				matches, err := filepath.Glob(pattern)
				if err != nil {
					return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
				}
				if len(matches) == 0 {
					// If no glob match, try as literal path
					files = append(files, pattern)
				} else {
					files = append(files, matches...)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Get release ID
			releaseData, err := c.Get(ctx, fmt.Sprintf("/repos/%s/%s/releases/tags/%s", owner, repo, tag), nil)
			if err != nil {
				return classifyAPIError(fmt.Errorf("getting release for tag %q: %w", tag, err), flags)
			}
			var release map[string]any
			if err := json.Unmarshal(releaseData, &release); err != nil {
				return fmt.Errorf("parsing release: %w", err)
			}
			releaseID := fmt.Sprintf("%.0f", release["id"])

			// Upload each file
			var uploadErrs []string
			for _, filePath := range files {
				baseName := filepath.Base(filePath)

				// Get local file size
				stat, err := os.Stat(filePath)
				if err != nil {
					uploadErrs = append(uploadErrs, fmt.Sprintf("%s: %v", baseName, err))
					continue
				}
				localSize := stat.Size()

				if progress {
					fmt.Fprintf(cmd.ErrOrStderr(), "Uploading %s (%d bytes)...", baseName, localSize)
				}

				// Upload with retry
				var lastErr error
				var uploaded bool
				for attempt := 0; attempt <= retry; attempt++ {
					if attempt > 0 {
						backoff := time.Duration(attempt) * time.Second
						time.Sleep(backoff)
						if progress {
							fmt.Fprintf(cmd.ErrOrStderr(), " retry %d...", attempt)
						}
					}

					uploadPath := fmt.Sprintf("/repos/%s/%s/releases/%s/assets", owner, repo, releaseID)
					fields := map[string]string{"name": baseName}
					fileFields := map[string]string{"attachment": filePath}

					respData, _, err := c.PostMultipart(ctx, uploadPath, fields, fileFields)
					if err != nil {
						lastErr = err
						continue
					}

					// Verify size
					var asset map[string]any
					if json.Unmarshal(respData, &asset) == nil {
						if sz, ok := asset["size"].(float64); ok {
							if int64(sz) != localSize {
								lastErr = fmt.Errorf("size mismatch: uploaded %d bytes, expected %d", int64(sz), localSize)
								continue
							}
						}
					}

					uploaded = true
					lastErr = nil
					break
				}

				if !uploaded {
					uploadErrs = append(uploadErrs, fmt.Sprintf("%s: %v", baseName, lastErr))
					if progress {
						fmt.Fprintf(cmd.ErrOrStderr(), " FAILED\n")
					}
				} else {
					if progress {
						fmt.Fprintf(cmd.ErrOrStderr(), " done (%.1f KB)\n", float64(localSize)/1024)
					}
				}
			}

			if len(uploadErrs) > 0 {
				return fmt.Errorf("upload errors:\n  %s", strings.Join(uploadErrs, "\n  "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&owner, "owner", "", "Repository owner (required)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name (required)")
	cmd.Flags().IntVar(&retry, "retry", 3, "Number of retry attempts on failure")
	cmd.Flags().BoolVar(&progress, "progress", true, "Show upload progress")

	return cmd
}

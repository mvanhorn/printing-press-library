// Feature 6: Pre-submission Diff Validation
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/jules/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		diffCmd := newDiffValidateCmd(flags)
		addNovelCommandIfAbsent(root, diffCmd)
	})
}

func newDiffValidateCmd(flags *rootFlags) *cobra.Command {
	var sessionID string
	var strict bool

	cmd := &cobra.Command{
		Use:   "diff-validate",
		Short: "Validate session diffs before submission",
		Long: `Validate that session outputs produce non-empty diffs.

Prevents empty or no-op PRs from being submitted by validating:
- Diff is not empty
- Diff contains actual code changes (not just whitespace)
- Diff doesn't exceed reasonable size limits
- Commit message is meaningful`,
		Example: `  # Validate diff for a session
  jules-pp-cli diff-validate --session-id abc123

  # Strict validation (fail on any warnings)
  jules-pp-cli diff-validate --session-id abc123 --strict`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			return validateSessionDiff(cmd.Context(), c, sessionID, strict, cmd.OutOrStdout(), flags.asJSON)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID to validate (required)")
	cmd.Flags().BoolVar(&strict, "strict", false, "Fail on any warnings, not just critical issues")
	_ = cmd.MarkFlagRequired("session-id")

	return cmd
}

func validateSessionDiff(ctx context.Context, c *client.Client, sessionID string, strict bool, out io.Writer, asJSON bool) error {
	// Fetch session
	sessionPath := fmt.Sprintf("/sessions/%s", sessionID)
	sessionData, err := c.Get(ctx, sessionPath, map[string]string{})
	if err != nil {
		return err
	}

	var session map[string]any
	if err := json.Unmarshal(sessionData, &session); err != nil {
		return err
	}

	state, _ := session["state"].(string)
	if !asJSON {
		fmt.Fprintf(out, "Validating session %s (state=%s)...\n", sessionID, state)
	}

	// Fetch activities to get the actual diffs
	activitiesPath := fmt.Sprintf("/sessions/%s/activities", sessionID)
	activitiesData, err := c.Get(ctx, activitiesPath, map[string]string{"pageSize": "100"})
	if err != nil {
		return err
	}

	activities := decodeJSONList(activitiesData, "activities")

	var issues []string
	var totalDiffSize int
	var changeCount int

	// Analyze artifacts from activities
	for _, a := range activities {
		activity, ok := a.(map[string]any)
		if !ok {
			continue
		}

		artifacts, ok := activity["artifacts"].([]any)
		if !ok {
			continue
		}

		for _, art := range artifacts {
			artifact, ok := art.(map[string]any)
			if !ok {
				continue
			}

			// Check for changeSet artifacts
			changeSet, ok := artifact["changeSet"].(map[string]any)
			if !ok {
				continue
			}

			gitPatch, ok := changeSet["unidiffPatch"].(string)
			if !ok || gitPatch == "" {
				issues = append(issues, "No diff content found in artifact")
				continue
			}

			totalDiffSize += len(gitPatch)
			changeCount++

			// Validate diff is not empty
			lines := strings.Split(gitPatch, "\n")
			if len(lines) < 3 {
				issues = append(issues, "Diff appears to be empty or too small")
				continue
			}

			// Check for actual content changes (not just metadata)
			hasContentChange := false
			for _, line := range lines {
				if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
					if strings.TrimSpace(line) != "+" {
						hasContentChange = true
						break
					}
				}
				if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
					if strings.TrimSpace(line) != "-" {
						hasContentChange = true
						break
					}
				}
			}

			if !hasContentChange {
				issues = append(issues, "Diff contains no actual content changes (only whitespace/metadata)")
			}

			// Check size limits
			if totalDiffSize > 1000000 { // 1MB limit
				issues = append(issues, fmt.Sprintf("Diff is too large: %d bytes (limit: 1MB)", totalDiffSize))
			}

			// Validate commit message
			commitMsg, ok := changeSet["suggestedCommitMessage"].(string)
			if !ok || strings.TrimSpace(commitMsg) == "" {
				if strict {
					issues = append(issues, "Warning: No commit message provided")
				}
			} else if len(commitMsg) < 10 {
				if strict {
					issues = append(issues, "Warning: Commit message is too short")
				}
			}
		}
	}

	if asJSON {
		if issues == nil {
			issues = []string{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(map[string]any{
			"session_id":      sessionID,
			"state":           state,
			"change_count":    changeCount,
			"total_diff_size": totalDiffSize,
			"valid":           len(issues) == 0,
			"issues":          issues,
		}); err != nil {
			return err
		}
		if len(issues) > 0 {
			return fmt.Errorf("diff validation failed: %d issues found", len(issues))
		}
		return nil
	}

	// Report results
	fmt.Fprintf(out, "\nValidation Results:\n")
	fmt.Fprintf(out, "  Total changes: %d\n", changeCount)
	fmt.Fprintf(out, "  Total diff size: %d bytes\n", totalDiffSize)

	if len(issues) == 0 {
		fmt.Fprintf(out, "  Status: ✓ VALID\n")
		return nil
	}

	fmt.Fprintf(out, "  Status: ✗ INVALID\n")
	fmt.Fprintf(out, "\nIssues:\n")
	for _, issue := range issues {
		fmt.Fprintf(out, "  - %s\n", issue)
	}

	return fmt.Errorf("diff validation failed: %d issues found", len(issues))
}

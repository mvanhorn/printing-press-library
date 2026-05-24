package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newLibraryHealthScoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health-score",
		Short: "Compute a 0-100 library health score from check_library output",
		Long: `Runs calibredb check_library across all report dimensions and computes
a weighted health score. Each report category deducts points:
  - malformed_formats: -15 per issue
  - missing_formats:   -5 per issue
  - missing_covers:    -3 per issue
  - extra_files:       -2 per issue
  - invalid_titles:    -2 per issue
  - malformed_paths:   -3 per issue
  - failed_folders:    -10 per issue
  - all others:        -1 per issue

Score is clamped to 0-100. Returns structured JSON with per-category breakdown.`,
		Example:     "  calibre-ebook-pp-cli library health-score --agent",
		Annotations: map[string]string{"pp:endpoint": "library.health-score", "pp:method": "GET", "pp:path": "/library/health-score", "mcp:read-only": "true", "pp:insight": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/library/check", map[string]string{"csv": "false"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var checkResult map[string]any
			if err := json.Unmarshal(data, &checkResult); err != nil {
				return fmt.Errorf("parsing check_library output: %w", err)
			}

			reportText, _ := checkResult["report"].(string)
			categories := parseCheckLibraryReport(reportText)

			weights := map[string]int{
				"malformed_formats": 15,
				"missing_formats":   5,
				"missing_covers":    3,
				"malformed_paths":   3,
				"failed_folders":    10,
				"extra_files":       2,
				"invalid_titles":    2,
				"extra_titles":      1,
				"invalid_authors":   1,
				"extra_authors":     1,
				"extra_formats":     1,
				"extra_covers":      1,
			}

			score := 100
			deductions := map[string]any{}
			totalIssues := 0
			for cat, items := range categories {
				w := 1
				if wt, ok := weights[cat]; ok {
					w = wt
				}
				deduction := len(items) * w
				if deduction > 0 {
					deductions[cat] = map[string]any{
						"count":     len(items),
						"weight":    w,
						"deduction": deduction,
						"items":     items,
					}
					score -= deduction
					totalIssues += len(items)
				}
			}
			if score < 0 {
				score = 0
			}

			var grade string
			switch {
			case score >= 90:
				grade = "A"
			case score >= 75:
				grade = "B"
			case score >= 50:
				grade = "C"
			case score >= 25:
				grade = "D"
			default:
				grade = "F"
			}

			result := map[string]any{
				"score":        score,
				"grade":        grade,
				"total_issues": totalIssues,
				"categories":   len(categories),
				"deductions":   deductions,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, _ := wrapWithProvenance(mustMarshal(result), DataProvenance{Source: "live"})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			gradeColor := green
			if score < 75 {
				gradeColor = yellow
			}
			if score < 50 {
				gradeColor = red
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Library Health: %s (%d/100)\n", gradeColor(grade), score)
			fmt.Fprintf(cmd.OutOrStdout(), "  Total issues:   %d across %d categories\n", totalIssues, len(categories))
			if totalIssues > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Top issues:\n")
				for cat, d := range deductions {
					if dm, ok := d.(map[string]any); ok {
						fmt.Fprintf(cmd.OutOrStdout(), "    %-25s %d issues (-%d points)\n", cat, dm["count"], dm["deduction"])
					}
				}
			}
			return nil
		},
	}
	return cmd
}

func parseCheckLibraryReport(raw string) map[string][]string {
	categories := map[string][]string{}
	var currentCat string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "(") && strings.Contains(line, ":") || (!strings.Contains(line, "(") && !strings.HasPrefix(line, "/")) {
			if isCategoryHeader(line) {
				currentCat = strings.TrimSuffix(strings.TrimSpace(line), ":")
				continue
			}
		}
		if currentCat != "" && line != "" {
			categories[currentCat] = append(categories[currentCat], line)
		}
	}
	return categories
}

func isCategoryHeader(line string) bool {
	headers := []string{"invalid_titles", "extra_titles", "invalid_authors", "extra_authors",
		"missing_formats", "extra_formats", "extra_files", "missing_covers", "extra_covers",
		"malformed_formats", "malformed_paths", "failed_folders"}
	lower := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(line, ":")))
	for _, h := range headers {
		if lower == h {
			return true
		}
	}
	return false
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(data)
}

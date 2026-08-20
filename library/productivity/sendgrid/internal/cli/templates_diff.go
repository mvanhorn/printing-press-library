// Hand-coded — do not add "DO NOT EDIT" header. Persists across regenerations.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type templateDiffResult struct {
	SubjectDiff string `json:"subject_diff,omitempty"`
	PlainDiff   string `json:"plain_diff,omitempty"`
	HTMLDiff    string `json:"html_diff,omitempty"`
}

func newTemplatesDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <template-id> <version-a> <version-b>",
		Short: "Unified diff between two template versions",
		Long: `Fetches two versions of a transactional template and produces a unified diff
of subject, plain_content, and html_content. HTML is normalized (whitespace
collapsed, attribute order sorted) before diffing to reduce noise. By default
outputs unified diff text; with --json returns {subject_diff, plain_diff, html_diff}.`,
		Example:     "  sendgrid-pp-cli templates diff d-abc123 version-a-id version-b-id",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			templateID := args[0]
			versionA := args[1]
			versionB := args[2]

			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			pathA := fmt.Sprintf("/v3/templates/%s/versions/%s", templateID, versionA)

			pathB := fmt.Sprintf("/v3/templates/%s/versions/%s", templateID, versionB)

			dataA, err := c.Get(pathA, nil)
			if err != nil {
				return fmt.Errorf("fetching version %s: %w", versionA, classifyAPIError(err, flags))
			}
			dataB, err := c.Get(pathB, nil)
			if err != nil {
				return fmt.Errorf("fetching version %s: %w", versionB, classifyAPIError(err, flags))
			}

			extractField := func(data json.RawMessage, field string) string {
				var m map[string]json.RawMessage
				if err := json.Unmarshal(data, &m); err != nil {
					return ""
				}
				raw, ok := m[field]
				if !ok {
					return ""
				}
				var s string
				_ = json.Unmarshal(raw, &s)
				return s
			}

			subjectA := extractField(dataA, "subject")
			subjectB := extractField(dataB, "subject")
			plainA := extractField(dataA, "plain_content")
			plainB := extractField(dataB, "plain_content")
			htmlA := normalizeHTML(extractField(dataA, "html_content"))
			htmlB := normalizeHTML(extractField(dataB, "html_content"))

			result := templateDiffResult{
				SubjectDiff: unifiedDiff(subjectA, subjectB, "subject/a", "subject/b"),
				PlainDiff:   unifiedDiff(plainA, plainB, "plain/a", "plain/b"),
				HTMLDiff:    unifiedDiff(htmlA, htmlB, "html/a", "html/b"),
			}

			if flags.asJSON {
				raw, _ := json.Marshal(result)
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			// Human output: print non-empty diffs
			printed := false
			for _, section := range []struct {
				label string
				diff  string
			}{
				{"Subject", result.SubjectDiff},
				{"Plain Text", result.PlainDiff},
				{"HTML", result.HTMLDiff},
			} {
				if section.diff != "" {
					if printed {
						_, _ = fmt.Fprintln(cmd.OutOrStdout())
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "=== %s ===\n%s", section.label, section.diff)
					printed = true
				}
			}
			if !printed {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No differences found between the two versions.")
			}
			return nil
		},
	}

	return cmd
}

// normalizeHTML collapses whitespace and sorts tag attributes to reduce noisy diffs.
func normalizeHTML(html string) string {
	// Collapse whitespace between tags
	wsRE := regexp.MustCompile(`>\s+<`)
	html = wsRE.ReplaceAllString(html, "><")

	// Collapse runs of whitespace in text nodes
	wsRunRE := regexp.MustCompile(`\s{2,}`)
	html = wsRunRE.ReplaceAllString(html, " ")

	// Sort attributes inside opening tags
	attrTagRE := regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)(\s[^>]*?)?>`)
	html = attrTagRE.ReplaceAllStringFunc(html, func(tag string) string {
		// Extract tag name and attribute string
		inner := tag[1 : len(tag)-1]
		spaceIdx := strings.IndexAny(inner, " \t\n")
		if spaceIdx < 0 {
			return tag
		}
		tagName := inner[:spaceIdx]
		attrStr := inner[spaceIdx:]

		// Split on attribute boundaries (naive but good enough for v1)
		attrRE := regexp.MustCompile(`\s+[a-zA-Z:_][a-zA-Z0-9:_.-]*(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]*))?`)
		attrs := attrRE.FindAllString(attrStr, -1)
		sort.Strings(attrs)
		return "<" + tagName + strings.Join(attrs, "") + ">"
	})

	return strings.TrimSpace(html)
}

// unifiedDiff produces a simple unified diff of two strings.
func unifiedDiff(a, b, labelA, labelB string) string {
	if a == b {
		return ""
	}

	linesA := splitLines(a)
	linesB := splitLines(b)

	edits := computeDiff(linesA, linesB)

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "--- %s\n+++ %s\n", labelA, labelB)

	// Group into hunks (context: 3 lines)
	const ctx = 3
	type hunk struct {
		lines []diffEdit
	}

	var hunks []hunk
	var current *hunk
	for i, e := range edits {
		if e.op != ' ' {
			if current == nil {
				startAt := i - ctx
				if startAt < 0 {
					startAt = 0
				}
				current = &hunk{lines: make([]diffEdit, 0)}
				current.lines = append(current.lines, edits[startAt:i]...)
			}
			current.lines = append(current.lines, e)
		} else if current != nil {
			current.lines = append(current.lines, e)
			// Check if we're ctx lines past the last change
			lastChange := -1
			for j := len(current.lines) - 1; j >= 0; j-- {
				if current.lines[j].op != ' ' {
					lastChange = j
					break
				}
			}
			if lastChange >= 0 && len(current.lines)-lastChange-1 >= ctx {
				// Trim trailing context
				current.lines = current.lines[:lastChange+ctx+1]
				hunks = append(hunks, *current)
				current = nil
			}
		}
	}
	if current != nil {
		hunks = append(hunks, *current)
	}

	aLine, bLine := 1, 1
	for _, h := range hunks {
		aCount, bCount := 0, 0
		for _, e := range h.lines {
			if e.op != '+' {
				aCount++
			}
			if e.op != '-' {
				bCount++
			}
		}
		_, _ = fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", aLine, aCount, bLine, bCount)
		for _, e := range h.lines {
			_, _ = fmt.Fprintf(&sb, "%c%s\n", e.op, e.line)
		}
		aLine += aCount
		bLine += bCount
	}

	return sb.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

type diffEdit struct {
	op   byte
	line string
}

func computeDiff(a, b []string) []diffEdit {
	// Myers diff algorithm (simplified O(ND))
	n, m := len(a), len(b)
	if n == 0 {
		edits := make([]diffEdit, m)
		for i, l := range b {
			edits[i] = diffEdit{op: '+', line: l}
		}
		return edits
	}
	if m == 0 {
		edits := make([]diffEdit, n)
		for i, l := range a {
			edits[i] = diffEdit{op: '-', line: l}
		}
		return edits
	}

	// Fall back to simple patience-like diff for v1
	lcsLen := lcs(a, b)
	_ = lcsLen

	// Reconstruct with DP
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				if dp[i+1][j] > dp[i][j+1] {
					dp[i][j] = dp[i+1][j]
				} else {
					dp[i][j] = dp[i][j+1]
				}
			}
		}
	}

	var edits []diffEdit
	i, j := 0, 0
	for i < n || j < m {
		if i < n && j < m && a[i] == b[j] {
			edits = append(edits, diffEdit{op: ' ', line: a[i]})
			i++
			j++
		} else if j < m && (i >= n || dp[i][j+1] >= dp[i+1][j]) {
			edits = append(edits, diffEdit{op: '+', line: b[j]})
			j++
		} else {
			edits = append(edits, diffEdit{op: '-', line: a[i]})
			i++
		}
	}
	return edits
}

func lcs(a, b []string) int {
	n, m := len(a), len(b)
	prev := make([]int, m+1)
	for i := 0; i < n; i++ {
		curr := make([]int, m+1)
		for j := 0; j < m; j++ {
			if a[i] == b[j] {
				curr[j+1] = prev[j] + 1
			} else if prev[j+1] > curr[j] {
				curr[j+1] = prev[j+1]
			} else {
				curr[j+1] = curr[j]
			}
		}
		prev = curr
	}
	return prev[m]
}

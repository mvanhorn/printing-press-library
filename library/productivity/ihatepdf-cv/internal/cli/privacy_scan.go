// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"regexp"
	"strings"
)

var privacyPatterns = map[string]*regexp.Regexp{
	"email":   regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	"phone":   regexp.MustCompile(`(?:\+?\d[\s().-]?){8,}\d`),
	"card":    regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`),
	"aadhaar": regexp.MustCompile(`\b\d{4}[ -]?\d{4}[ -]?\d{4}\b`),
	"pan":     regexp.MustCompile(`\b[A-Z]{5}\d{4}[A-Z]\b`),
}

func newNovelPrivacyScanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "privacy-scan [input.pdf]", Short: "Scan local PDF content and metadata; returns classified risk findings and hints.", Example: "  ihatepdf-cv-pp-cli privacy-scan report.pdf --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "input=testdata/fixture.pdf"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "scan PDF privacy risks")
		}
		if len(args) < 1 {
			return usageErr(fmt.Errorf("input PDF path is required"))
		}
		b, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("read %s: %w", args[0], err)
		}
		text := extractLiteralText(b)
		if text == "" {
			text = string(b)
		}
		findings := make([]riskFinding, 0)
		total := 0
		for kind, re := range privacyPatterns {
			matches := re.FindAllString(text, 8)
			if len(matches) > 0 {
				for i := range matches {
					matches[i] = strings.TrimSpace(matches[i])
				}
				findings = append(findings, riskFinding{Kind: kind, Count: len(re.FindAllString(text, -1)), Examples: matches})
				total += len(re.FindAllString(text, -1))
			}
		}
		sortRisk(findings)
		hints := make([]string, 0)
		raw := string(b)
		for _, k := range []string{"/Author", "/Creator", "/Producer", "/Title", "/Subject"} {
			if strings.Contains(raw, k) {
				hints = append(hints, k)
			}
		}
		return emitLocal(cmd, flags, privacyResult{Path: args[0], Findings: findings, MetadataHints: hints, RiskCount: total})
	}}
	return cmd
}
func sortRisk(v []riskFinding) {
	for i := 0; i < len(v); i++ {
		for j := i + 1; j < len(v); j++ {
			if v[j].Count > v[i].Count {
				v[i], v[j] = v[j], v[i]
			}
		}
	}
}

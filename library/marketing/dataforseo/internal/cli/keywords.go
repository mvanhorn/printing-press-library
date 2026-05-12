// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"bufio"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/cliutil"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// newKeywordsCmd is the parent group for keyword utilities that are
// transcendence-only (pre-cleaning, delta tracking). The endpoint-mirror
// group lives under `keywords-data`, not here.
func newKeywordsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keywords",
		Short: "Keyword utilities (pre-clean, delta tracking)",
		Long: `Keyword utilities that complement the keywords-data endpoint group.

  keywords clean   Strip junk keywords (>10 words, special chars) before sending to Google Ads volume so one bad keyword does not poison the whole batch.
  keywords delta   Diff current vs last-known search volume per keyword from the local SQLite store.
  keywords volume  Fetch Google Ads search volume with auto-mode routing (Live <-> Standard).`,
	}
	cmd.AddCommand(newKeywordsCleanCmd(flags))
	cmd.AddCommand(newKeywordsDeltaCmd(flags))
	cmd.AddCommand(newKeywordsVolumeCmd(flags))
	return cmd
}

// keywordsCleanAllowed keeps only letters, digits, spaces, and hyphens.
// Mirrors seo_engine/scoring/serp.py:_clean_for_dfs.
var keywordsCleanAllowed = regexp.MustCompile(`[^a-z0-9\s\-]`)
var keywordsCleanWhitespace = regexp.MustCompile(`\s+`)

type keywordCleanReject struct {
	Keyword string `json:"keyword"`
	Reason  string `json:"reason"`
}

type keywordCleanResult struct {
	Kept     []string             `json:"kept"`
	Rejected []keywordCleanReject `json:"rejected"`
}

func cleanOne(raw string) (string, string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "empty"
	}
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", " ")
	s = keywordsCleanAllowed.ReplaceAllString(s, "")
	s = keywordsCleanWhitespace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "stripped_to_empty"
	}
	if len(strings.Fields(s)) > 10 {
		return "", "too_long"
	}
	return s, ""
}

func newKeywordsCleanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean [file]",
		Short: "Pre-clean keywords before sending to Google Ads volume",
		Long: `Reads keywords (one per line) from a file argument or stdin, applies the same
cleaner DataForSEO's keywords_data/google_ads endpoint requires, and prints the
survivors. One malformed keyword (>10 words, unicode, punctuation) fails the
whole batch with cost=0 on the live endpoint, so cleaning is mandatory.

Rules:
  - lowercase
  - underscores -> spaces
  - drop anything outside [a-z0-9 -]
  - collapse whitespace
  - reject anything with more than 10 words`,
		Example: strings.Trim(`
  dataforseo-pp-cli keywords clean /tmp/raw-keywords.txt
  printf "tree service daytona\nSTUMP_GRINDING\n!!!" | dataforseo-pp-cli keywords clean
  dataforseo-pp-cli keywords clean keywords.txt --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			var reader io.Reader
			switch {
			case len(args) > 0 && args[0] != "-":
				f, err := os.Open(args[0])
				if err != nil {
					return usageErr(fmt.Errorf("open %s: %w", args[0], err))
				}
				defer f.Close()
				reader = f
			default:
				reader = cmd.InOrStdin()
			}

			result := keywordCleanResult{Kept: []string{}, Rejected: []keywordCleanReject{}}
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.TrimSpace(line) == "" {
					continue
				}
				cleaned, reason := cleanOne(line)
				if reason != "" {
					result.Rejected = append(result.Rejected, keywordCleanReject{Keyword: line, Reason: reason})
					continue
				}
				result.Kept = append(result.Kept, cleaned)
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}

			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
			} else {
				out := cmd.OutOrStdout()
				for _, k := range result.Kept {
					fmt.Fprintln(out, k)
				}
				if len(result.Rejected) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "rejected %d keyword(s)\n", len(result.Rejected))
				}
			}

			if len(result.Kept) == 0 {
				return usageErr(fmt.Errorf("no keywords survived cleaning (%d rejected)", len(result.Rejected)))
			}
			_ = cliutil.IsVerifyEnv // helper available for future hooks
			return nil
		},
	}
	return cmd
}

// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for retraction-checker-pp-cli.

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/other/retraction-checker/internal/cliutil"
	"github.com/spf13/cobra"
)

// doiInBibRe extracts DOIs from a .bib doi = {...} / doi = "..." field or bare text.
var doiInBibRe = regexp.MustCompile(`(?i)10\.\d{4,9}/[-._;()/:a-z0-9]+`)

type scanResult struct {
	File           string              `json:"file"`
	Total          int                 `json:"total"`
	RetractedCount int                 `json:"retracted_count"`
	ConcernCount   int                 `json:"concern_count"`
	FailureCount   int                 `json:"failure_count"`
	Entries        []retractionVerdict `json:"entries"`
}

func summarizeScan(path string, verdicts []retractionVerdict) scanResult {
	res := scanResult{File: path, Total: len(verdicts), Entries: verdicts}
	for _, verdict := range verdicts {
		if verdict.Retracted {
			res.RetractedCount++
		}
		if verdict.ExpressionOfConcern {
			res.ConcernCount++
		}
		if verdict.Error != "" {
			res.FailureCount++
		}
	}
	return res
}

func writeHumanScanResult(w io.Writer, res scanResult) {
	fmt.Fprintf(w, "Scanned %d entries from %s: %d retracted, %d editorial concerns, %d errors\n\n", res.Total, res.File, res.RetractedCount, res.ConcernCount, res.FailureCount)
	for _, verdict := range res.Entries {
		switch {
		case verdict.Error != "":
			fmt.Fprintf(w, "  ?  %s (%s)\n", verdict.Input, verdict.Error)
		case verdict.Retracted && verdict.ExpressionOfConcern:
			fmt.Fprintf(w, "  X! RETRACTED + EDITORIAL CONCERN  %s  %s\n", verdict.DOI, verdict.Date)
		case verdict.Retracted:
			fmt.Fprintf(w, "  X  RETRACTED  %s  %s\n", verdict.DOI, verdict.Date)
		case verdict.ExpressionOfConcern:
			fmt.Fprintf(w, "  !  EDITORIAL CONCERN  %s  %s\n", verdict.DOI, verdict.Date)
		default:
			fmt.Fprintf(w, "  ok            %s\n", verdict.DOI)
		}
	}
}

// parseIdentifiers extracts one DOI/PMID per meaningful line. For .bib content
// it also pulls DOIs out of doi = {...} fields. Blank lines and lines starting
// with '#' or '%' are skipped.
func parseIdentifiers(content string) []string {
	var ids []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		ids = append(ids, s)
	}
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "%") {
			continue
		}
		// .bib field line, e.g. `doi = {10.1/x},` or `doi = "10.1/x"`.
		if m := doiInBibRe.FindString(line); m != "" {
			add(strings.TrimRight(m, ".,;)}\""))
			continue
		}
		// Plain DOI or PMID line.
		if looksLikePMID(line) {
			add(line)
			continue
		}
		cleaned := cleanDOI(line)
		if strings.HasPrefix(cleaned, "10.") {
			add(cleaned)
		}
	}
	return ids
}

func newNovelScanCmd(flags *rootFlags) *cobra.Command {
	var mailto string
	cmd := &cobra.Command{
		Use:   "scan <file>",
		Short: "Batch-check a reading list or .bib file and flag every retracted entry.",
		Long: "Scan a bibliography or reading list for retracted papers. The file may contain one\n" +
			"DOI or PMID per line (blank lines and lines starting with # or % are skipped), or\n" +
			"be a BibTeX (.bib) file whose doi fields are extracted automatically. Each entry is\n" +
			"checked against Crossref; retracted entries are flagged. Keyless.",
		Example:     "  retraction-checker-pp-cli scan refs.bib --json",
		Args:        cobra.ArbitraryArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a file path argument is required"))
			}
			path := args[0]
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			ids := parseIdentifiers(string(content))
			if len(ids) == 0 {
				return fmt.Errorf("no DOIs or PMIDs found in %s", path)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			verdicts := make([]retractionVerdict, len(ids))
			// One shared limiter across all goroutines: NCBI's unauthenticated
			// PMID-resolution endpoint allows only 3 req/s, well below the
			// concurrency of 6 in-flight resolveAndCheck calls below.
			limiter := cliutil.NewAdaptiveLimiter(flags.rateLimit)
			sem := make(chan struct{}, 6)
			var wg sync.WaitGroup
			for i, id := range ids {
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					verdicts[i] = resolveAndCheck(ctx, c, mailto, id, limiter)
				}()
			}
			wg.Wait()

			res := summarizeScan(path, verdicts)
			if res.FailureCount > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d entries could not be checked\n", res.FailureCount, res.Total)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			writeHumanScanResult(cmd.OutOrStdout(), res)
			return nil
		},
	}
	cmd.Flags().StringVar(&mailto, "mailto", "", "Contact email for the Crossref polite pool (better rate limits)")
	return cmd
}

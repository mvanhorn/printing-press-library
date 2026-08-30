// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/ihatepdf-cv/internal/store"
	"github.com/spf13/cobra"
)

type pdfSearchMatch struct {
	Path    string `json:"path"`
	Page    int    `json:"page,omitempty"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var ignoreCase bool
	var contextLines int
	var catalogPath string
	var stdin bool
	cmd := &cobra.Command{
		Use:   "search [query] [files...]",
		Short: "Search extracted text across local PDFs; returns matching snippets and counts.",
		Long:  "Searches literal PDF text locally without uploading files. Provide one or more PDF paths; scanned or compressed text may require OCR before it can be found.",
		Example: `  ihatepdf-cv-pp-cli search "invoice" reports/*.pdf --agent
  ihatepdf-cv-pp-cli search "customer id" report.pdf --context 2 --json
  echo invoice | ihatepdf-cv-pp-cli search --stdin report.pdf --agent`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "query=fixture;file=testdata/fixture.pdf"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if stdin {
				queryBytes, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read query from stdin: %w", err)
				}
				args = append([]string{strings.TrimSpace(string(queryBytes))}, args...)
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("provide a query and at least one PDF path, or use --catalog"))
			}
			if contextLines < 0 || contextLines > 10 {
				return usageErr(fmt.Errorf("--context must be between 0 and 10"))
			}
			query := args[0]
			if query == "" {
				return usageErr(fmt.Errorf("query must not be empty"))
			}
			if strings.TrimSpace(catalogPath) != "" {
				db, err := store.OpenReadOnlyContext(cmd.Context(), catalogPath)
				if err != nil {
					return fmt.Errorf("open catalog: %w", err)
				}
				defer db.Close()
				results, err := db.SearchPDF(args[0], 50)
				if err != nil {
					return fmt.Errorf("search catalog: %w", err)
				}
				return emitLocal(cmd, flags, map[string]any{"query": args[0], "results": results, "count": len(results), "source": "local-catalog"})
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("provide a query and at least one PDF path"))
			}
			if ignoreCase {
				query = "(?i)" + regexp.QuoteMeta(query)
			} else {
				query = regexp.QuoteMeta(query)
			}
			re, err := regexp.Compile(query)
			if err != nil {
				return usageErr(fmt.Errorf("compile query: %w", err))
			}
			matches := make([]pdfSearchMatch, 0)
			for _, path := range args[1:] {
				b, readErr := readFile(path)
				if readErr != nil {
					return readErr
				}
				text := extractLiteralText(b)
				if text == "" {
					text = string(b)
				}
				lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
				for i, line := range lines {
					if !re.MatchString(line) {
						continue
					}
					start, end := i-contextLines, i+contextLines
					if start < 0 {
						start = 0
					}
					if end >= len(lines) {
						end = len(lines) - 1
					}
					matches = append(matches, pdfSearchMatch{Path: path, Line: i + 1, Snippet: strings.Join(lines[start:end+1], " ")})
				}
			}
			return emitLocal(cmd, flags, map[string]any{"query": args[0], "matches": matches, "count": len(matches)})
		},
	}
	cmd.Flags().BoolVar(&ignoreCase, "ignore-case", false, "match query without case sensitivity")
	cmd.Flags().IntVar(&contextLines, "context", 0, "include this many neighboring text lines in each snippet")
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "search an indexed SQLite catalog instead of direct PDF paths")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "read the search query from stdin")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { addNovelCommandIfAbsent(root, newSearchCmd(flags)) })
}

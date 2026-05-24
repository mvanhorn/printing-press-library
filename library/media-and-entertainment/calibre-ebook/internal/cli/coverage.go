package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Report metadata coverage percentages across the library",
		Long: `Scans every book in the library and reports what percentage have
common metadata fields populated (title, authors, tags, series, publisher,
language, ISBN, covers, formats). Identifies fields with low coverage
so you can prioritize bulk metadata improvements.`,
		Example:     "  calibre-ebook-pp-cli coverage --agent",
		Annotations: map[string]string{"pp:endpoint": "library.coverage", "pp:method": "GET", "pp:path": "/library/coverage", "mcp:read-only": "true", "pp:insight": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/books", map[string]string{
				"fields": "id,title,authors,tags,series,publisher,language,isbn,formats,cover",
				"limit":  "0",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var books []map[string]any
			if err := json.Unmarshal(data, &books); err != nil {
				return fmt.Errorf("parsing book list: %w", err)
			}

			total := len(books)
			if total == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), `{"total_books":0,"coverage":{}}`)
				return nil
			}

			fields := []string{"title", "authors", "tags", "series", "publisher", "language", "isbn", "formats", "cover"}
			counts := map[string]int{}
			for _, book := range books {
				for _, f := range fields {
					v := fmt.Sprintf("%v", book[f])
					if v != "" && v != "<nil>" && v != "[]" && v != "map[]" {
						counts[f]++
					}
				}
			}

			coverage := map[string]any{}
			var gaps []map[string]any
			for _, f := range fields {
				pctVal := pct(counts[f], total)
				coverage[f] = map[string]any{
					"filled":     counts[f],
					"total":      total,
					"percentage": pctVal,
				}
				if pctVal < 80 {
					gaps = append(gaps, map[string]any{
						"field":      f,
						"percentage": pctVal,
						"missing":    total - counts[f],
					})
				}
			}

			overall := 0.0
			for _, f := range fields {
				overall += float64(counts[f])
			}
			overallPct := pct(int(overall), total*len(fields))

			result := map[string]any{
				"total_books":     total,
				"overall_pct":     overallPct,
				"field_coverage":  coverage,
				"low_coverage":    gaps,
				"completion":      overallPct,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, _ := wrapWithProvenance(mustMarshal(result), DataProvenance{Source: "live"})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "  Metadata Coverage: %.0f%% overall (%d books)\n\n", overallPct, total)
			for _, f := range fields {
				pctVal := pct(counts[f], total)
				bar := strings.Repeat("#", int(pctVal/5)) + strings.Repeat(".", 20-int(pctVal/5))
				fmt.Fprintf(cmd.OutOrStdout(), "    %-12s %s %5.1f%% (%d/%d)\n", f, bar, pctVal, counts[f], total)
			}
			if len(gaps) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Low coverage fields (< 80%%):\n")
				for _, g := range gaps {
					fmt.Fprintf(cmd.OutOrStdout(), "    - %s: %.0f%% (%d missing)\n", g["field"], g["percentage"], g["missing"])
				}
			}
			return nil
		},
	}
	return cmd
}

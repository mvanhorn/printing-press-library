package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newWorkflowHealthAuditCmd(flags *rootFlags) *cobra.Command {
	var fix bool

	cmd := &cobra.Command{
		Use:   "health-audit",
		Short: "Run a full library health audit with actionable recommendations",
		Long: `Compound workflow that runs check_library, duplicates detection,
series gap analysis, and format coverage — then synthesizes
a single report with prioritized fix recommendations.

With --fix, automatically embeds metadata for books that need it.`,
		Example:     "  calibre-ebook-pp-cli workflow health-audit --agent",
		Annotations: map[string]string{"pp:endpoint": "workflow.health-audit", "pp:method": "POST", "pp:path": "/workflow/health-audit", "pp:insight": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			start := time.Now()

			checkData, checkErr := c.Get("/library/check", nil)
			checkResult := map[string]any{"error": "skipped"}
			if checkErr == nil {
				json.Unmarshal(checkData, &checkResult)
			}

			booksData, booksErr := c.Get("/books", map[string]string{
				"fields": "id,title,authors,formats,series,series_index,size",
				"limit":  "0",
			})
			var books []map[string]any
			if booksErr == nil {
				json.Unmarshal(booksData, &books)
			}

			formatCounts := map[string]int{}
			singleFormat := 0
			noSeries := 0
			for _, book := range books {
				formats := parseFormats(fmt.Sprintf("%v", book["formats"]))
				if len(formats) <= 1 {
					singleFormat++
				}
				for _, f := range formats {
					formatCounts[f]++
				}
				series := fmt.Sprintf("%v", book["series"])
				if series == "" || series == "<nil>" {
					noSeries++
				}
			}

			var recommendations []map[string]any
			severity := "healthy"

			if singleFormat > len(books)/2 {
				recommendations = append(recommendations, map[string]any{
					"action":  "Convert single-format books to a second format for backup",
					"count":   singleFormat,
					"pct":     pct(singleFormat, len(books)),
					"payload": fmt.Sprintf("%d books have only one format", singleFormat),
				})
				severity = "warning"
			}

			if noSeries > len(books)/3 {
				recommendations = append(recommendations, map[string]any{
					"action":  "Set series metadata for books that belong to a series",
					"count":   noSeries,
					"payload": fmt.Sprintf("%d books have no series metadata", noSeries),
				})
				if severity == "healthy" {
					severity = "info"
				}
			}

			if checkErr == nil {
				if reportText, ok := checkResult["report"].(string); ok && len(reportText) > 100 {
					recommendations = append(recommendations, map[string]any{
						"action":  "Fix issues reported by check_library",
						"payload": "Run 'library check --csv' for detailed CSV output",
					})
					if severity == "healthy" {
						severity = "info"
					}
				}
			}

			if fix {
				_, _, embedErr := c.Post("/library/backup-metadata", nil)
				if embedErr != nil {
					recommendations = append(recommendations, map[string]any{
						"action": "backup_metadata failed",
						"error":  embedErr.Error(),
					})
				}
			}

			result := map[string]any{
				"severity":       severity,
				"total_books":    len(books),
				"duration_ms":    time.Since(start).Milliseconds(),
				"format_coverage": formatCounts,
				"recommendations": recommendations,
				"checks": map[string]any{
					"library_check":  checkResult,
					"format_audit":   fmt.Sprintf("%d single-format, %d multi-format", singleFormat, len(books)-singleFormat),
					"series_coverage": fmt.Sprintf("%d without series metadata", noSeries),
				},
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, _ := wrapWithProvenance(mustMarshal(result), DataProvenance{Source: "live"})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			colorFn := green
			if severity == "warning" {
				colorFn = yellow
			}
			if severity == "critical" {
				colorFn = red
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n  Library Health Audit: %s (%s)\n", colorFn(severity), time.Since(start).Round(time.Millisecond))
			fmt.Fprintf(cmd.OutOrStdout(), "  Books: %d\n", len(books))
			if len(recommendations) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  Recommendations:\n")
				for _, r := range recommendations {
					fmt.Fprintf(cmd.OutOrStdout(), "    - %v\n", r["payload"])
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\n  No issues found.\n")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "Auto-fix: run backup-metadata after audit")
	return cmd
}

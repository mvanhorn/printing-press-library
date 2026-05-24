package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newLibrarySeriesGapsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "series-gaps",
		Short: "Find series with missing books (gaps in series_index)",
		Long: `Scans all books with series metadata, groups by series name,
and detects gaps in the series_index sequence. Reports series
that are missing entries (e.g. books 1,2,4 → gap at 3) and
series with only a single book.`,
		Example:     "  calibre-ebook-pp-cli library series-gaps --agent",
		Annotations: map[string]string{"pp:endpoint": "library.series-gaps", "pp:method": "GET", "pp:path": "/library/series-gaps", "mcp:read-only": "true", "pp:insight": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/books", map[string]string{
				"fields": "id,title,series,series_index",
				"search": "series:True",
				"limit":  "0",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var books []map[string]any
			if err := json.Unmarshal(data, &books); err != nil {
				return fmt.Errorf("parsing book list: %w", err)
			}

			seriesMap := map[string][]map[string]any{}
			for _, book := range books {
				seriesName := fmt.Sprintf("%v", book["series"])
				if seriesName == "" || seriesName == "<nil>" {
					continue
				}
				seriesMap[seriesName] = append(seriesMap[seriesName], book)
			}

			type gapReport struct {
				Series      string              `json:"series"`
				BookCount   int                 `json:"book_count"`
				MaxIndex    float64             `json:"max_index"`
				Missing     []float64           `json:"missing,omitempty"`
				Indices     []float64           `json:"indices"`
				Books       []map[string]any    `json:"books"`
				IsSingleton bool                `json:"is_singleton"`
			}

			var reports []gapReport
			for name, books := range seriesMap {
				var indices []float64
				for _, b := range books {
					idx, _ := strconv.ParseFloat(fmt.Sprintf("%v", b["series_index"]), 64)
					if idx == 0 {
						continue
					}
					indices = append(indices, idx)
				}
				if len(indices) == 0 {
					continue
				}
				sort.Float64s(indices)

				maxIdx := indices[len(indices)-1]
				var missing []float64
				if maxIdx > 1 && len(indices) > 1 {
					indexSet := map[float64]bool{}
					for _, i := range indices {
						indexSet[i] = true
					}
					for i := 1.0; i <= maxIdx; i += 1.0 {
						if !indexSet[i] {
							missing = append(missing, i)
						}
					}
				}

				reports = append(reports, gapReport{
					Series:      name,
					BookCount:   len(books),
					MaxIndex:    maxIdx,
					Missing:     missing,
					Indices:     indices,
					Books:       books,
					IsSingleton: len(books) == 1,
				})
			}

			sort.Slice(reports, func(i, j int) bool {
				return len(reports[i].Missing) > len(reports[j].Missing)
			})

			totalGaps := 0
			singletons := 0
			for _, r := range reports {
				totalGaps += len(r.Missing)
				if r.IsSingleton {
					singletons++
				}
			}

			result := map[string]any{
				"total_series":   len(reports),
				"total_gaps":     totalGaps,
				"singletons":     singletons,
				"series_with_gaps": func() int {
					n := 0
					for _, r := range reports {
						if len(r.Missing) > 0 {
							n++
						}
					}
					return n
				}(),
				"reports": reports,
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, _ := wrapWithProvenance(mustMarshal(result), DataProvenance{Source: "live"})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "  Series: %d total, %d with gaps, %d singletons\n\n", len(reports), result["series_with_gaps"], singletons)
			for _, r := range reports {
				if len(r.Missing) > 0 || r.IsSingleton {
					tag := ""
					if r.IsSingleton {
						tag = " (singleton)"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    %-40s books:%d max_idx:%.0f missing:%v%s\n", r.Series, r.BookCount, r.MaxIndex, r.Missing, tag)
				}
			}
			return nil
		},
	}
	return cmd
}

var _ = os.Stdout
var _ = strconv.Atoi
var _ = strings.TrimSpace

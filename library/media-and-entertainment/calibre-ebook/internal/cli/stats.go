package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newLibraryStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Aggregate library statistics (formats, authors, series, sizes)",
		Long: `Computes aggregate statistics across the entire library:
  - Books per format (EPUB, MOBI, PDF, etc.)
  - Top authors by book count
  - Books with/without series metadata
  - Books with/without covers
  - Format distribution (single-format vs multi-format)
  - Size distribution
Derived from calibredb list — not available from calibredb directly.`,
		Example:     "  calibre-ebook-pp-cli library stats --agent",
		Annotations: map[string]string{"pp:endpoint": "library.stats", "pp:method": "GET", "pp:path": "/library/stats", "mcp:read-only": "true", "pp:insight": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/books", map[string]string{
				"fields": "id,title,authors,formats,series,size",
				"limit":  "0",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var books []map[string]any
			if err := json.Unmarshal(data, &books); err != nil {
				return fmt.Errorf("parsing book list: %w", err)
			}

			formatCounts := map[string]int{}
			authorCounts := map[string]int{}
			var totalSize float64
			withSeries := 0
			withoutSeries := 0
			singleFormat := 0
			multiFormat := 0
			formatCombinations := map[string]int{}

			for _, book := range books {
				formats := parseFormats(fmt.Sprintf("%v", book["formats"]))
				if len(formats) == 1 {
					singleFormat++
				} else if len(formats) > 1 {
					multiFormat++
				}
				for _, f := range formats {
					formatCounts[strings.ToUpper(f)]++
				}
				sort.Strings(formats)
				formatCombinations[strings.Join(formats, "+")]++

				authorsStr := fmt.Sprintf("%v", book["authors"])
				for _, a := range splitAuthors(authorsStr) {
					a = strings.TrimSpace(a)
					if a != "" {
						authorCounts[a]++
					}
				}

				series := fmt.Sprintf("%v", book["series"])
				if series == "" || series == "<nil>" {
					withoutSeries++
				} else {
					withSeries++
				}

				size, _ := book["size"].(float64)
				totalSize += size
			}

			topAuthors := rankMap(authorCounts, 10)
			topFormats := rankMap(formatCounts, 20)
			topCombos := rankMap(formatCombinations, 10)

			result := map[string]any{
				"total_books":   len(books),
				"total_size_mb": int(totalSize / 1024 / 1024),
				"formats": map[string]any{
					"distribution":      topFormats,
					"single_format":     singleFormat,
					"multi_format":      multiFormat,
					"combinations":      topCombos,
					"unique_formats":    len(formatCounts),
				},
				"authors": map[string]any{
					"unique":    len(authorCounts),
					"top_10":    topAuthors,
				},
				"series": map[string]any{
					"with":    withSeries,
					"without": withoutSeries,
					"pct_with": pct(withSeries, len(books)),
				},
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, _ := wrapWithProvenance(mustMarshal(result), DataProvenance{Source: "live"})
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "  Library Stats: %d books, %d MB\n\n", len(books), int(totalSize/1024/1024))
			fmt.Fprintf(cmd.OutOrStdout(), "  Formats:\n")
			for _, kv := range topFormats {
				fmt.Fprintf(cmd.OutOrStdout(), "    %-10s %d books\n", kv.Key, kv.Value)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Top Authors:\n")
			for _, kv := range topAuthors {
				fmt.Fprintf(cmd.OutOrStdout(), "    %-30s %d books\n", kv.Key, kv.Value)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n  Series: %d with (%.0f%%), %d without\n", withSeries, pct(withSeries, len(books)), withoutSeries)
			return nil
		},
	}
	return cmd
}

func parseFormats(raw string) []string {
	raw = strings.Trim(raw, "[]")
	if raw == "" || raw == "<nil>" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"")
		if ext := extractExt(p); ext != "" {
			result = append(result, ext)
		}
	}
	return result
}

func extractExt(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		ext := path[idx+1:]
		ext = strings.TrimRight(ext, "\"")
		if ext != "" && len(ext) <= 10 {
			return ext
		}
	}
	return ""
}

func splitAuthors(raw string) []string {
	raw = strings.ReplaceAll(raw, " & ", ",")
	raw = strings.ReplaceAll(raw, ";", ",")
	return strings.Split(raw, ",")
}

type kv struct {
	Key   string `json:"key"`
	Value int    `json:"value"`
}

func rankMap(m map[string]int, limit int) []kv {
	var entries []kv
	for k, v := range m {
		entries = append(entries, kv{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Value > entries[j].Value
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	return entries
}

func pct(num, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(num) / float64(total) * 100
}

var _ = os.Stdout

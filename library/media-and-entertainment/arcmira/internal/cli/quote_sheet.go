package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

type sheetPassage struct {
	ID      string   `json:"id"`
	Title   string   `json:"video_title"`
	Channel string   `json:"channel_name"`
	Start   *float64 `json:"start_seconds"`
	Text    string   `json:"text"`
	URL     string   `json:"watch_url"`
}

type sheetRow struct {
	sheetPassage
	Queries []string `json:"queries"`
}

type searchForSheet struct {
	Query  string         `json:"query"`
	Chunks []sheetPassage `json:"chunks"`
}

// pp:data-source computed
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		cmd := &cobra.Command{
			Use:         "quote-sheet <search.json> [more.json ...]",
			Short:       "Merge saved transcript searches into a deduplicated, timestamp-linked quote sheet",
			Long:        "Keep one row per passage, with its exact quote, source link, and every query that found it. Files are read in order. Use - to read one search from stdin.",
			Example:     "  arcmira-pp-cli transcript-search --q 'vibe coding' --json > search.json\n  arcmira-pp-cli quote-sheet search.json --json",
			Args:        cobra.MinimumNArgs(1),
			Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "search.json=examples/quote-sheet.json"},
			RunE: func(cmd *cobra.Command, args []string) error {
				if flags.dryRun {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "merge saved transcript searches", "inputs": args}, flags)
				}
				rows := []sheetRow{}
				byID := map[string]int{}
				for _, path := range args {
					var b []byte
					var err error
					if path == "-" {
						b, err = io.ReadAll(cmd.InOrStdin())
					} else {
						b, err = os.ReadFile(path)
					}
					if err != nil {
						return err
					}
					var input struct {
						searchForSheet
						Results *searchForSheet `json:"results"`
					}
					if err := json.Unmarshal(b, &input); err != nil {
						return fmt.Errorf("%s: %w", path, err)
					}
					s := &input.searchForSheet
					if input.Results != nil {
						s = input.Results
					}
					if strings.TrimSpace(s.Query) == "" || s.Chunks == nil {
						return fmt.Errorf("%s: expected transcript-search JSON with query and chunks; save full --json output", path)
					}
					for i, p := range s.Chunks {
						if p.ID == "" || p.URL == "" || p.Start == nil || *p.Start < 0 || strings.TrimSpace(p.Text) == "" {
							return fmt.Errorf("%s: passage %d is missing a quote, ID, source link or valid timestamp", path, i+1)
						}
						if pos, ok := byID[p.ID]; ok {
							row := &rows[pos]
							if p.Text != row.Text || p.URL != row.URL || *p.Start != *row.Start {
								return fmt.Errorf("%s: conflicting saved versions of passage %s", path, p.ID)
							}
							if !slices.Contains(row.Queries, s.Query) {
								row.Queries = append(row.Queries, s.Query)
							}
							continue
						}
						byID[p.ID] = len(rows)
						rows = append(rows, sheetRow{p, []string{s.Query}})
					}
				}
				data, err := json.Marshal(rows)
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			},
		}
		root.AddCommand(cmd)
	})
}

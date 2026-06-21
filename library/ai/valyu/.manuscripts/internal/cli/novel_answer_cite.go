// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/valyu/internal/store"
)

func newCitedAnswerCmd(flags *rootFlags) *cobra.Command {
	var query string
	var citeOutput string
	var format string
	var searchType string
	var startDate string
	var endDate string

	cmd := &cobra.Command{
		Use:   "cited-answer",
		Short: "Get an answer with cited sources from Valyu.",
		Long: `Submit a query to the Valyu answer API and receive a cited answer.

The answer text is printed to stdout. Citations can be saved to a separate
file with --cite-output in JSON, RIS, or BibTeX format.`,
		Example: `  valyu-pp-cli cited-answer --query "What is CRISPR?"
  valyu-pp-cli cited-answer --query "Inflation 2024" --cite-output refs.bib --format bibtex
  valyu-pp-cli cited-answer --query "RNA folding" --format ris --cite-output refs.ris`,
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 && !flags.dryRun {
				return cmd.Help()
			}
			if query == "" && !flags.dryRun {
				return fmt.Errorf("required flag \"query\" not set")
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "dry-run: POST /v1/answer {\"query\": %q}\n", query)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{"query": query}
			if searchType != "" {
				body["search_type"] = searchType
			}
			if startDate != "" {
				body["start_date"] = startDate
			}
			if endDate != "" {
				body["end_date"] = endDate
			}

			data, _, apiErr := c.PostQueryWithParams(cmd.Context(), "/v1/answer", map[string]string{}, body)
			if apiErr != nil {
				return classifyAPIError(apiErr, flags)
			}

			cost := extractCostFromResponse(data)
			db, dbErr := store.OpenWithContext(cmd.Context(), defaultDBPath("valyu-pp-cli"))
			if dbErr == nil {
				defer db.Close()
				recordCost(db, "cited-answer", cost)
			}

			if flags.asJSON {
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			// Print answer text
			var resp struct {
				Answer  string `json:"answer"`
				Sources []struct {
					URL     string `json:"url"`
					Title   string `json:"title"`
					Snippet string `json:"snippet"`
				} `json:"sources"`
				Cost float64 `json:"cost"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				// Fallback: print raw
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			fmt.Fprintln(cmd.OutOrStdout(), resp.Answer)

			if citeOutput != "" && len(resp.Sources) > 0 {
				var content string
				switch strings.ToLower(format) {
				case "ris":
					content = formatRIS(resp.Sources)
				case "bibtex":
					content = formatBibTeX(resp.Sources)
				default:
					srcJSON, _ := json.MarshalIndent(resp.Sources, "", "  ")
					content = string(srcJSON)
				}
				if err := os.WriteFile(citeOutput, []byte(content), 0o644); err != nil {
					return fmt.Errorf("writing citations to %s: %w", citeOutput, err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "citations written to %s (%d sources)\n", citeOutput, len(resp.Sources))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "The question to answer")
	cmd.Flags().StringVar(&citeOutput, "cite-output", "", "File path to write citations to")
	cmd.Flags().StringVar(&format, "format", "json", "Citation format: json, ris, or bibtex")
	cmd.Flags().StringVar(&searchType, "search-type", "", "Search type for context retrieval (web, paper, all, etc.)")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Filter sources published on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDate, "end-date", "", "Filter sources published on or before this date (YYYY-MM-DD)")
	return cmd
}

func formatRIS(sources []struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}) string {
	var sb strings.Builder
	for _, s := range sources {
		sb.WriteString("TY  - ELEC\n")
		if s.Title != "" {
			sb.WriteString("TI  - " + s.Title + "\n")
		}
		if s.URL != "" {
			sb.WriteString("UR  - " + s.URL + "\n")
		}
		if s.Snippet != "" {
			sb.WriteString("AB  - " + s.Snippet + "\n")
		}
		sb.WriteString("ER  - \n\n")
	}
	return sb.String()
}

func formatBibTeX(sources []struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}) string {
	var sb strings.Builder
	for i, s := range sources {
		key := fmt.Sprintf("source%d", i+1)
		sb.WriteString(fmt.Sprintf("@misc{%s,\n", key))
		if s.Title != "" {
			sb.WriteString(fmt.Sprintf("  title = {%s},\n", s.Title))
		}
		if s.URL != "" {
			sb.WriteString(fmt.Sprintf("  url = {%s},\n", s.URL))
		}
		if s.Snippet != "" {
			sb.WriteString(fmt.Sprintf("  note = {%s},\n", s.Snippet))
		}
		sb.WriteString("}\n\n")
	}
	return sb.String()
}

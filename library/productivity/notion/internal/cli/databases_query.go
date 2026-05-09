// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/cliutil"

	"github.com/spf13/cobra"
)

func newDatabasesQueryCmd(flags *rootFlags) *cobra.Command {
	var filterJSON string
	var sortJSON string
	var limit int
	var startCursor string

	cmd := &cobra.Command{
		Use:   "query <database_id>",
		Short: "Query records in a database",
		Long:  "Query all records in a Notion database with optional filters and sorts. Uses POST /v1/databases/{id}/query.",
		Example: strings.Trim(`
  notion-pp-cli databases query 21ef2ab1-3bb8-80d3-86e2-fa66a1a7938a
  notion-pp-cli databases query 21ef2ab1-3bb8-80d3-86e2-fa66a1a7938a --json
  notion-pp-cli databases query 21ef2ab1-3bb8-80d3-86e2-fa66a1a7938a --sort '[{"timestamp":"last_edited_time","direction":"descending"}]'
  notion-pp-cli databases query 21ef2ab1-3bb8-80d3-86e2-fa66a1a7938a --limit 10 --json --select results`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would POST /v1/databases/%s/query\n", args[0])
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"results":[],"has_more":false}`)
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			authHeader := cfg.AuthHeader()
			if authHeader == "" {
				return fmt.Errorf("not authenticated — run 'notion-pp-cli auth set-token' or set NOTION_TOKEN")
			}

			body := map[string]any{}
			if filterJSON != "" {
				var f any
				if err := json.Unmarshal([]byte(filterJSON), &f); err != nil {
					return fmt.Errorf("invalid --filter JSON: %w", err)
				}
				body["filter"] = f
			}
			if sortJSON != "" {
				var s any
				if err := json.Unmarshal([]byte(sortJSON), &s); err != nil {
					return fmt.Errorf("invalid --sort JSON: %w", err)
				}
				body["sorts"] = s
			}
			if limit > 0 {
				body["page_size"] = limit
			}
			if startCursor != "" {
				body["start_cursor"] = startCursor
			}

			bodyBytes, _ := json.Marshal(body)
			url := fmt.Sprintf("%s/v1/databases/%s/query", cfg.BaseURL, args[0])

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, url, bytes.NewReader(bodyBytes))
			if err != nil {
				return fmt.Errorf("building request: %w", err)
			}
			req.Header.Set("Authorization", authHeader)
			req.Header.Set("Notion-Version", "2022-06-28")
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("reading response: %w", err)
			}

			if resp.StatusCode >= 400 {
				var apiErr map[string]any
				if json.Unmarshal(data, &apiErr) == nil {
					return fmt.Errorf("API error %d: %s", resp.StatusCode, apiErr["message"])
				}
				return fmt.Errorf("API error %d", resp.StatusCode)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Human-readable: extract title from each result
			var result struct {
				Results []map[string]any `json:"results"`
				HasMore bool             `json:"has_more"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%d records\n\n", len(result.Results))
			for _, r := range result.Results {
				title := extractRecordTitle(r)
				edited, _ := r["last_edited_time"].(string)
				if len(edited) >= 10 {
					edited = edited[:10]
				}
				id, _ := r["id"].(string)
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-40s  %s\n", edited, truncate(title, 40), id)
			}
			if result.HasMore {
				fmt.Fprintf(cmd.OutOrStdout(), "\n(more results available — use --start-cursor)\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&filterJSON, "filter", "", "Filter as JSON (Notion filter object)")
	cmd.Flags().StringVar(&sortJSON, "sort", "", "Sorts as JSON array (e.g. '[{\"timestamp\":\"last_edited_time\",\"direction\":\"descending\"}]')")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max records to return (default: Notion's page size of 100)")
	cmd.Flags().StringVar(&startCursor, "start-cursor", "", "Pagination cursor from a previous response")

	return cmd
}

// extractRecordTitle pulls the plain-text title from a database record's properties.
func extractRecordTitle(r map[string]any) string {
	props, ok := r["properties"].(map[string]any)
	if !ok {
		return "(untitled)"
	}
	for _, v := range props {
		prop, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if prop["type"] == "title" {
			if titleArr, ok := prop["title"].([]any); ok && len(titleArr) > 0 {
				if rt, ok := titleArr[0].(map[string]any); ok {
					if pt, ok := rt["plain_text"].(string); ok && pt != "" {
						return pt
					}
				}
			}
		}
	}
	return "(untitled)"
}

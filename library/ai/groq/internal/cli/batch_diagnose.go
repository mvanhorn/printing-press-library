// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type batchLineStatus struct {
	CustomID string `json:"custom_id,omitempty"`
	Status   int    `json:"status_code"`
	Error    string `json:"error,omitempty"`
}

type batchDiagnoseReport struct {
	BatchID      string            `json:"batch_id"`
	Status       string            `json:"status"`
	InputFileID  string            `json:"input_file_id,omitempty"`
	OutputFileID string            `json:"output_file_id,omitempty"`
	TotalLines   int               `json:"total_lines"`
	StatusCounts map[string]int    `json:"status_counts"`
	ErrorSummary []errorCount      `json:"error_summary,omitempty"`
	RetryWorthy  int               `json:"retry_worthy_lines"`
	Rows         []batchLineStatus `json:"rows,omitempty"`
}

type errorCount struct {
	Error string `json:"error"`
	Count int    `json:"count"`
}

func newNovelBatchDiagnoseCmd(flags *rootFlags) *cobra.Command {
	var flagFile string

	cmd := &cobra.Command{
		Use:         "diagnose <batch_id>",
		Short:       "Tabulate a completed batch's per-line status codes and errors, highlighting retry-worthy failures.",
		Long:        "Fetch a batch's metadata and result file (or read a local results .jsonl) and tabulate per-line status codes and errors, highlighting retry-worthy failures (429/5xx/timeouts). Replaces grep-based error tabulation over thousands of result lines. Batch API access requires a Developer plan.",
		Example:     "  groq-pp-cli batch diagnose batch_abc123 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "batch_id=batch_abc123;--file=testdata/sample-results.jsonl"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "batch diagnose")
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a batch ID is required"))
			}
			batchID := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// With --file, analyze a local results file entirely offline:
			// no batch-metadata API call is needed. Otherwise fetch the
			// batch and download its output file.
			var batchObj struct {
				ID           string `json:"id"`
				Status       string `json:"status"`
				InputFileID  string `json:"input_file_id"`
				OutputFileID string `json:"output_file_id"`
			}
			var results []byte
			if flagFile != "" {
				results, err = os.ReadFile(flagFile)
				if err != nil {
					return fmt.Errorf("reading results file %q: %w", flagFile, err)
				}
				batchObj.ID = batchID
				batchObj.Status = "completed"
			} else {
				batchPath := "/openai/v1/batches/" + batchID
				batchData, err := c.Get(ctx, batchPath, nil)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				if err := json.Unmarshal(batchData, &batchObj); err != nil {
					return fmt.Errorf("parsing batch metadata: %w", err)
				}
				if batchObj.OutputFileID != "" {
					outPath := "/openai/v1/files/" + batchObj.OutputFileID + "/content"
					data, err := c.Get(ctx, outPath, nil)
					if err != nil {
						return classifyAPIError(cmd.OutOrStdout(), err, flags)
					}
					results = data
				} else {
					return fmt.Errorf("batch %q has no output file yet (status %q); pass --file to analyze a local results file", batchID, orDash(batchObj.Status))
				}
			}

			report, err := tabulateBatchResults(batchObj, results)
			if err != nil {
				return err
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Batch %s (status: %s): %d result lines\n", report.BatchID, report.Status, report.TotalLines)
			statusKeys := make([]string, 0, len(report.StatusCounts))
			for k := range report.StatusCounts {
				statusKeys = append(statusKeys, k)
			}
			sort.Strings(statusKeys)
			for _, k := range statusKeys {
				fmt.Fprintf(cmd.OutOrStdout(), "  HTTP %s: %d\n", k, report.StatusCounts[k])
			}
			if len(report.ErrorSummary) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  errors:\n")
				for _, e := range report.ErrorSummary {
					fmt.Fprintf(cmd.OutOrStdout(), "    %-12s x%d\n", truncateString(e.Error, 40), e.Count)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  retry-worthy failures: %d\n", report.RetryWorthy)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "Path to a local batch results .jsonl file (skips the API results download)")
	return cmd
}

func tabulateBatchResults(batchObj struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	InputFileID  string `json:"input_file_id"`
	OutputFileID string `json:"output_file_id"`
}, results []byte) (*batchDiagnoseReport, error) {
	report := &batchDiagnoseReport{
		BatchID:      batchObj.ID,
		Status:       batchObj.Status,
		InputFileID:  batchObj.InputFileID,
		OutputFileID: batchObj.OutputFileID,
		StatusCounts: map[string]int{},
		Rows:         make([]batchLineStatus, 0),
	}

	scanner := bufio.NewScanner(bytes.NewReader(results))
	scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj struct {
			ID       string `json:"id"`
			CustomID string `json:"custom_id"`
			Response struct {
				StatusCode int             `json:"status_code"`
				Body       json.RawMessage `json:"body"`
			} `json:"response"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}
		status := obj.Response.StatusCode
		entry := batchLineStatus{CustomID: obj.CustomID, Status: status}
		hasError := len(obj.Error) > 0 && string(obj.Error) != "null"
		if hasError {
			entry.Error = errorText(obj.Error)
		}
		if entry.Error == "" && status >= 400 {
			entry.Error = extractBodyError(obj.Response.Body)
		}
		// OpenAI-style batch failures return "response": null with a top-level
		// error; force a non-200 status so failed lines are never counted as
		// HTTP 200 successes.
		if status == 0 {
			if hasError {
				status = 500
				entry.Status = status
				if entry.Error == "" {
					entry.Error = "batch request failed (no response body)"
				}
			} else {
				status = 200
				entry.Status = status
			}
		}
		report.TotalLines++
		report.StatusCounts[fmt.Sprintf("%d", status)]++
		if isRetryWorthy(status) {
			report.RetryWorthy++
		}
		report.Rows = append(report.Rows, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning results: %w", err)
	}

	counts := map[string]int{}
	for _, r := range report.Rows {
		if r.Error != "" {
			counts[r.Error]++
		}
	}
	for e, n := range counts {
		report.ErrorSummary = append(report.ErrorSummary, errorCount{Error: e, Count: n})
	}
	sort.Slice(report.ErrorSummary, func(i, j int) bool {
		return report.ErrorSummary[i].Count > report.ErrorSummary[j].Count
	})
	if len(report.ErrorSummary) > 5 {
		report.ErrorSummary = report.ErrorSummary[:5]
	}
	return report, nil
}

func errorText(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if obj.Message != "" {
			return obj.Message
		}
		if obj.Type != "" {
			return obj.Type
		}
		return obj.Code
	}
	return string(raw)
}

func extractBodyError(body json.RawMessage) string {
	if len(body) == 0 {
		return ""
	}
	var obj struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &obj); err == nil && obj.Error.Message != "" {
		return obj.Error.Message
	}
	return ""
}

func isRetryWorthy(status int) bool {
	return status == 429 || status == 500 || status == 502 || status == 503 || status == 504 || status == 408
}

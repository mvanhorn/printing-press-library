// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type batchLineResult struct {
	Line             int      `json:"line"`
	CustomID         string   `json:"custom_id,omitempty"`
	Endpoint         string   `json:"endpoint,omitempty"`
	Valid            bool     `json:"valid"`
	Issues           []string `json:"issues,omitempty"`
	EstimatedTokens  int64    `json:"estimated_tokens"`
	EstimatedCost    float64  `json:"estimated_cost_usd"`
}

type batchValidateReport struct {
	File                 string            `json:"file"`
	TotalLines           int               `json:"total_lines"`
	ValidLines           int               `json:"valid_lines"`
	InvalidLines         int               `json:"invalid_lines"`
	EstimatedPromptTokens int64            `json:"estimated_prompt_tokens"`
	EstimatedCostUSD     float64           `json:"estimated_cost_usd"`
	PerLine              []batchLineResult `json:"per_line"`
	Note                 string            `json:"note,omitempty"`
}

func newNovelBatchValidateCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "validate <file.jsonl>",
		Short:       "Validate every line of a .jsonl batch request file and estimate tokens/cost before uploading.",
		Long:        "Validate each line of a Groq batch request file (.jsonl) against the expected request shape and estimate per-line tokens and cost before submitting to the Batch API, which rejects a whole file on one malformed line. Pure local computation — no API call.",
		Example:     "  groq-pp-cli batch validate testdata/sample-batch.jsonl --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "testdata/sample-batch.jsonl"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "batch validate")
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a .jsonl batch request file path is required"))
			}
			path := args[0]
			report, err := validateBatchFile(path)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Batch file: %s\n", report.File)
			fmt.Fprintf(cmd.OutOrStdout(), "  lines: %d total, %d valid, %d invalid\n", report.TotalLines, report.ValidLines, report.InvalidLines)
			fmt.Fprintf(cmd.OutOrStdout(), "  estimated prompt tokens: %d\n", report.EstimatedPromptTokens)
			fmt.Fprintf(cmd.OutOrStdout(), "  estimated cost: $%.6f\n", report.EstimatedCostUSD)
			for _, line := range report.PerLine {
				if !line.Valid {
					fmt.Fprintf(cmd.OutOrStdout(), "  invalid line %d: %s\n", line.Line, strings.Join(line.Issues, "; "))
				}
			}
			if report.InvalidLines > 0 {
				return usageErr(fmt.Errorf("%d line(s) failed validation; fix them before uploading the batch", report.InvalidLines))
			}
			return nil
		},
	}
	return cmd
}

// validateBatchFile parses a .jsonl batch request file line by line and
// validates each against the shapes Groq's Batch API accepts (OpenAI-style:
// chat completions, responses, embeddings). No API calls are made.
func validateBatchFile(path string) (*batchValidateReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening batch file %q: %w", path, err)
	}
	defer f.Close()

	report := &batchValidateReport{
		File:    filepath.Base(path),
		PerLine: make([]batchLineResult, 0),
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<20), 8<<20)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		res := validateBatchLine(lineNo, line)
		report.TotalLines++
		report.PerLine = append(report.PerLine, res)
		if res.Valid {
			report.ValidLines++
			report.EstimatedPromptTokens += res.EstimatedTokens
			report.EstimatedCostUSD += res.EstimatedCost
		} else {
			report.InvalidLines++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading batch file %q: %w", path, err)
	}
	if report.TotalLines == 0 {
		report.Note = "file is empty or contains only blank lines"
	}
	return report, nil
}

func validateBatchLine(lineNo int, line string) batchLineResult {
	res := batchLineResult{Line: lineNo, Valid: true}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		res.Valid = false
		res.Issues = append(res.Issues, "not valid JSON: "+err.Error())
		return res
	}

	res.CustomID = stringAt(obj, "custom_id")
	if res.CustomID == "" {
		res.CustomID = stringAt(obj, "id")
	}
	if res.CustomID == "" {
		res.Valid = false
		res.Issues = append(res.Issues, "missing custom_id (or id)")
	}

	// Detect endpoint by which request body keys are present.
	var endpoint string
	if _, ok := obj["messages"]; ok {
		endpoint = "/v1/chat/completions"
	} else if _, ok := obj["input"]; ok {
		if _, isText := obj["input_text"]; isText {
			endpoint = "/v1/chat/completions"
		} else {
			endpoint = "/v1/embeddings"
		}
	} else if _, ok := obj["text_format"]; ok {
		endpoint = "/v1/responses"
	}
	if endpoint == "" && hasModel(obj) {
		endpoint = "/v1/responses"
	}
	res.Endpoint = endpoint
	if endpoint == "" {
		res.Valid = false
		res.Issues = append(res.Issues, "cannot determine endpoint: expected messages (chat completions) or input (embeddings/responses)")
	}

	if !hasModel(obj) {
		res.Valid = false
		res.Issues = append(res.Issues, "missing model")
	}
	if endpoint == "/v1/chat/completions" {
		if _, ok := obj["messages"]; !ok {
			res.Valid = false
			res.Issues = append(res.Issues, "chat completion request missing messages")
		}
	} else if endpoint == "/v1/embeddings" {
		if _, ok := obj["input"]; !ok {
			res.Valid = false
			res.Issues = append(res.Issues, "embedding request missing input")
		}
	}

	res.EstimatedTokens = estimatePromptTokens(obj)
	res.EstimatedCost = estimateLineCost(stringAt(obj, "model"), res.EstimatedTokens)
	return res
}

func hasModel(obj map[string]json.RawMessage) bool {
	_, ok := obj["model"]
	return ok
}

func stringAt(obj map[string]json.RawMessage, key string) string {
	raw, ok := obj[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

func estimatePromptTokens(obj map[string]json.RawMessage) int64 {
	// Rough token estimate: ~4 chars per token on average. Counts string
	// values in messages/input plus the system prompt.
	var total int64
	var count func(v any)
	count = func(v any) {
		switch t := v.(type) {
		case string:
			total += int64(len(t)) / 4
		case []any:
			for _, item := range t {
				count(item)
			}
		case map[string]any:
			for _, item := range t {
				count(item)
			}
		}
	}
	for _, key := range []string{"messages", "input", "instructions", "prompt"} {
		if raw, ok := obj[key]; ok {
			var v any
			if err := json.Unmarshal(raw, &v); err == nil {
				count(v)
			}
		}
	}
	if total == 0 {
		total = 10
	}
	return total
}

func estimateLineCost(model string, promptTokens int64) float64 {
	mp := modelPrice{}.orBuiltin(model)
	if mp.Prompt == 0 && mp.Completion == 0 {
		return 0
	}
	return float64(promptTokens) * mp.Prompt
}

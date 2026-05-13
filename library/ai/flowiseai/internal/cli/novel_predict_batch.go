// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func newPredictBatchCmd(flags *rootFlags) *cobra.Command {
	var inputFile string
	var outFile string
	var concurrency int
	var overrideConfigJSON string
	var questionField string
	var format string // csv, ndjson, txt (auto by extension)

	cmd := &cobra.Command{
		Use:   "batch [chatflowId]",
		Short: "Run a chatflow against a CSV or NDJSON of questions concurrently",
		Long: `Read questions from a CSV (one column or specified column), NDJSON (one JSON
object per line with a "question" key, or override via --question-field), or
plain text (one question per line) file. Fire N concurrent predictions and
stream results as NDJSON to --out (default: stdout).

Each result contains the chatId so the run can be replayed or audited later.`,
		Example: "  flowiseai-pp-cli predict batch abc-chatflow --input listings.csv --concurrency 4 --out results.ndjson",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if inputFile == "" {
				return usageErr(fmt.Errorf("--input is required"))
			}
			if concurrency < 1 {
				concurrency = 1
			}
			chatflowID := args[0]

			questions, err := loadBatchQuestions(inputFile, questionField, format)
			if err != nil {
				return err
			}
			if len(questions) == 0 {
				return apiErr(fmt.Errorf("no questions found in %s", inputFile))
			}

			var overrideConfig map[string]any
			if overrideConfigJSON != "" {
				if err := json.Unmarshal([]byte(overrideConfigJSON), &overrideConfig); err != nil {
					return usageErr(fmt.Errorf("parsing --override-config JSON: %w", err))
				}
			}

			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{
					"chatflowId":    chatflowID,
					"input":         inputFile,
					"questionCount": len(questions),
					"concurrency":   concurrency,
					"dryRun":        true,
				})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var out io.Writer = cmd.OutOrStdout()
			if outFile != "" {
				f, err := os.Create(outFile)
				if err != nil {
					return fmt.Errorf("opening --out %s: %w", outFile, err)
				}
				defer f.Close()
				out = f
			}
			outEnc := json.NewEncoder(out)

			type batchResult struct {
				Index      int    `json:"index"`
				Question   string `json:"question"`
				ChatID     string `json:"chatId,omitempty"`
				Text       string `json:"text,omitempty"`
				DurationMs int64  `json:"durationMs"`
				Error      string `json:"error,omitempty"`
				HTTPStatus int    `json:"httpStatus,omitempty"`
			}

			results := make([]batchResult, len(questions))
			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup
			var writeMu sync.Mutex

			for i, q := range questions {
				wg.Add(1)
				go func(idx int, question string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					started := time.Now()
					body := map[string]any{"question": question}
					if overrideConfig != nil {
						body["overrideConfig"] = overrideConfig
					}
					resp, statusCode, postErr := c.Post("/prediction/"+chatflowID, body)
					dur := time.Since(started).Milliseconds()
					r := batchResult{Index: idx, Question: question, DurationMs: dur, HTTPStatus: statusCode}
					if postErr != nil {
						r.Error = postErr.Error()
					} else {
						var blob map[string]any
						if json.Unmarshal(resp, &blob) == nil {
							if cid, ok := blob["chatId"].(string); ok {
								r.ChatID = cid
							}
							if t, ok := blob["text"].(string); ok {
								r.Text = t
							}
						}
					}
					results[idx] = r

					// Stream NDJSON result immediately so the consumer can pipeline
					writeMu.Lock()
					_ = outEnc.Encode(r)
					writeMu.Unlock()
				}(i, q)
			}
			wg.Wait()

			if outFile != "" {
				// emit summary to stderr so the file stays pure NDJSON
				successes := 0
				for _, r := range results {
					if r.Error == "" && r.HTTPStatus < 400 {
						successes++
					}
				}
				fmt.Fprintf(os.Stderr, "batch complete: %d/%d successful, wrote NDJSON to %s\n", successes, len(results), outFile)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFile, "input", "", "Input file (CSV, NDJSON, or plain text — one question per line/object)")
	cmd.Flags().StringVar(&outFile, "out", "", "Output NDJSON file (default: stdout)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Number of concurrent predictions")
	cmd.Flags().StringVar(&overrideConfigJSON, "override-config", "", "JSON overrideConfig applied to every prediction in the batch")
	cmd.Flags().StringVar(&questionField, "question-field", "question", "For NDJSON input: which key holds the question (default 'question')")
	cmd.Flags().StringVar(&format, "format", "", "Force input format: csv, ndjson, txt (auto-detected by extension when omitted)")
	return cmd
}

// loadBatchQuestions reads the input file and extracts the per-row questions.
func loadBatchQuestions(path, ndjsonField, formatOverride string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	format := strings.ToLower(formatOverride)
	if format == "" {
		switch strings.ToLower(getExt(path)) {
		case ".csv":
			format = "csv"
		case ".ndjson", ".jsonl":
			format = "ndjson"
		default:
			format = "txt"
		}
	}
	var questions []string
	switch format {
	case "csv":
		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		rows, err := r.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("reading CSV: %w", err)
		}
		if len(rows) == 0 {
			return nil, nil
		}
		// Detect header row: if first row matches a named header, skip it.
		header := rows[0]
		colIdx := 0
		for i, h := range header {
			if strings.EqualFold(strings.TrimSpace(h), "question") || strings.EqualFold(strings.TrimSpace(h), ndjsonField) {
				colIdx = i
				rows = rows[1:]
				break
			}
		}
		for _, row := range rows {
			if colIdx < len(row) {
				q := strings.TrimSpace(row[colIdx])
				if q != "" {
					questions = append(questions, q)
				}
			}
		}
	case "ndjson":
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(line), &obj); err != nil {
				continue
			}
			if v, ok := obj[ndjsonField].(string); ok && v != "" {
				questions = append(questions, v)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("scanning NDJSON: %w", err)
		}
	default: // txt
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				questions = append(questions, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("scanning text: %w", err)
		}
	}
	return questions, nil
}

func getExt(p string) string {
	i := strings.LastIndex(p, ".")
	if i < 0 {
		return ""
	}
	return p[i:]
}

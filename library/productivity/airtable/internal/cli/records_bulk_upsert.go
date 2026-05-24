// Copyright 2026 joelsephus. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newRecordsBulkUpsertCmd is a hand-built wrapper that adds --merge-on and
// --batch-progress on top of the generated records upsert path. It batches the
// input into 10-record chunks (the Airtable cap), prints a stderr progress
// line per batch when --batch-progress is set, and forwards each batch to the
// generated client via the same body shape the generator emits.
func newRecordsBulkUpsertCmd(flags *rootFlags) *cobra.Command {
	var recordsArg string
	var recordsFile string
	var mergeOn []string
	var batchProgress bool
	var typecast bool
	var stdinBody bool

	cmd := &cobra.Command{
		Use:   "bulk-upsert <baseId> <tableIdOrName>",
		Short: "Bulk upsert records with --merge-on, automatic 10-record batching, and progress",
		Annotations: map[string]string{
			"pp:endpoint": "records.upsert",
			"pp:method":   "PATCH",
			"pp:path":     "/{baseId}/{tableIdOrName}",
		},
		Long: `Bulk variant of 'records upsert' built for ingest-from-file pipelines.

Reads a JSON array of {"fields": {...}} records from --records (literal JSON or
@path), --records-file <path>, or stdin (--stdin). Splits into 10-record
batches (Airtable's per-request cap), and PATCHes each batch with
performUpsert.fieldsToMergeOn = <--merge-on values>. When --batch-progress is
set, emits one JSON progress line per batch on stderr.`,
		Example: strings.Trim(`
  # Upsert from file, merge on Email
  airtable-pp-cli records bulk-upsert appXXX tblYYY --records-file ./customers.json --merge-on Email --batch-progress

  # Upsert from stdin
  cat customers.json | airtable-pp-cli records bulk-upsert appXXX tblYYY --stdin --merge-on Email
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("baseId and tableIdOrName are required\nUsage: %s <baseId> <tableIdOrName>", cmd.CommandPath()))
			}
			if len(mergeOn) == 0 {
				return usageErr(fmt.Errorf("--merge-on is required (one or more field names)"))
			}

			rawJSON, err := readBulkUpsertInput(recordsArg, recordsFile, stdinBody)
			if err != nil {
				return usageErr(err)
			}
			var records []map[string]any
			if err := json.Unmarshal(rawJSON, &records); err != nil {
				return usageErr(fmt.Errorf("parsing records JSON (expected array): %w", err))
			}
			if len(records) == 0 {
				return usageErr(fmt.Errorf("no records to upsert"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/{baseId}/{tableIdOrName}"
			path = replacePathParam(path, "baseId", args[0])
			path = replacePathParam(path, "tableIdOrName", args[1])

			const batchSize = 10
			total := len(records)
			batches := (total + batchSize - 1) / batchSize

			type batchResult struct {
				Batch   int             `json:"batch"`
				Size    int             `json:"size"`
				Status  int             `json:"status"`
				Records json.RawMessage `json:"records,omitempty"`
				Error   string          `json:"error,omitempty"`
			}
			results := make([]batchResult, 0, batches)

			for i := 0; i < total; i += batchSize {
				end := i + batchSize
				if end > total {
					end = total
				}
				chunk := records[i:end]
				body := map[string]any{
					"records": chunk,
					"performUpsert": map[string]any{
						"fieldsToMergeOn": mergeOn,
					},
				}
				if typecast {
					body["typecast"] = true
				}

				batchNum := (i / batchSize) + 1
				if batchProgress {
					fmt.Fprintf(cmd.ErrOrStderr(),
						`{"event":"batch","batch":%d,"of":%d,"size":%d}`+"\n",
						batchNum, batches, len(chunk))
				}

				data, statusCode, err := c.PatchWithParams(cmd.Context(), path, map[string]string{}, body)
				if err != nil {
					results = append(results, batchResult{
						Batch:  batchNum,
						Size:   len(chunk),
						Status: statusCode,
						Error:  err.Error(),
					})
					return classifyAPIError(err, flags)
				}
				results = append(results, batchResult{
					Batch:   batchNum,
					Size:    len(chunk),
					Status:  statusCode,
					Records: data,
				})
			}

			if batchProgress {
				fmt.Fprintf(cmd.ErrOrStderr(),
					`{"event":"complete","batches":%d,"total":%d}`+"\n",
					batches, total)
			}
			return flags.printJSON(cmd, results)
		},
	}
	cmd.Flags().StringVar(&recordsArg, "records", "", "Records as JSON array, or @path to a JSON file")
	cmd.Flags().StringVar(&recordsFile, "records-file", "", "Path to a file containing the JSON array of records")
	cmd.Flags().StringSliceVar(&mergeOn, "merge-on", nil, "Field name(s) Airtable uses to match existing records (repeatable or comma-separated)")
	cmd.Flags().BoolVar(&batchProgress, "batch-progress", false, "Emit one JSON progress line per batch on stderr")
	cmd.Flags().BoolVar(&typecast, "typecast", false, "Allow Airtable to coerce values to the field type")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read the JSON records array from stdin")
	return cmd
}

// readBulkUpsertInput resolves the records payload from the three accepted
// sources (literal --records, @path, --records-file, --stdin). Exactly one
// must be set; the caller validates that the parsed shape is a JSON array.
func readBulkUpsertInput(recordsArg, recordsFile string, fromStdin bool) ([]byte, error) {
	sources := 0
	if recordsArg != "" {
		sources++
	}
	if recordsFile != "" {
		sources++
	}
	if fromStdin {
		sources++
	}
	if sources == 0 {
		return nil, fmt.Errorf("provide --records, --records-file <path>, or --stdin")
	}
	if sources > 1 {
		return nil, fmt.Errorf("--records, --records-file, and --stdin are mutually exclusive")
	}
	switch {
	case fromStdin:
		return readAllStdin()
	case recordsFile != "":
		return os.ReadFile(recordsFile)
	case strings.HasPrefix(recordsArg, "@"):
		return os.ReadFile(strings.TrimPrefix(recordsArg, "@"))
	default:
		return []byte(recordsArg), nil
	}
}

func readAllStdin() ([]byte, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return data, nil
}

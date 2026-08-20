// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newImportCmd(flags *rootFlags) *cobra.Command {
	var inputFile string
	var dryRun bool
	var batchSize int

	cmd := &cobra.Command{
		Use:   "import <resource>",
		Short: "Import data from JSONL file via API create/upsert calls",
		Example: `  # Import from a JSONL file
  gorgias-pp-cli import <resource> --input data.jsonl

  # Dry-run to preview without sending
  gorgias-pp-cli import <resource> --input data.jsonl --dry-run

  # Import from stdin
  cat data.jsonl | gorgias-pp-cli import <resource> --input -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.DryRun = dryRun

			resource := args[0]
			path := "/" + resource

			var reader io.Reader
			if inputFile == "-" || inputFile == "" {
				reader = os.Stdin
			} else {
				f, err := os.Open(inputFile)
				if err != nil {
					return fmt.Errorf("opening input file: %w", err)
				}
				defer f.Close()
				reader = f
			}

			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer

			var success, failed, skipped int
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || line[0] == '#' {
					skipped++
					continue
				}

				var body map[string]any
				if err := json.Unmarshal([]byte(line), &body); err != nil {
					fmt.Fprintf(os.Stderr, "warning: skipping invalid JSON line: %v\n", err)
					failed++
					continue
				}

				_, _, err := c.Post(path, body)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to import record: %v\n", err)
					failed++
					continue
				}
				success++
			}

			if err := scanner.Err(); err != nil {
				return fmt.Errorf("reading input: %w", err)
			}

			// JSON envelope: {succeeded, failed, skipped}.
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"succeeded": success,
					"failed":    failed,
					"skipped":   skipped,
				}, flags)
			}
			fmt.Fprintf(os.Stderr, "Import complete: %d succeeded, %d failed, %d skipped\n", success, failed, skipped)
			return nil
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input JSONL file path (use - for stdin)")
	_ = cmd.MarkFlagRequired("input")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview import without sending requests")
	cmd.Flags().IntVar(&batchSize, "batch-size", 1, "Records per batch (future: batch API support)")

	return cmd
}

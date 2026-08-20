// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/store"
	"github.com/spf13/cobra"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var format, outputFile string
	var limit int
	cmd := &cobra.Command{
		Use:     "export observations [id]",
		Short:   "Export locally persisted observations without contacting a remote HTTP origin",
		Example: "  bambu-pp-cli export observations --format jsonl --limit 1000\n  bambu-pp-cli export observations --format json --output ./observations.json",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "observations" {
				return usageErr(fmt.Errorf("unknown resource %q; valid: observations", args[0]))
			}
			if format != "json" && format != "jsonl" {
				return usageErr(fmt.Errorf("--format must be json or jsonl"))
			}
			if limit < 1 || limit > 10000 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 10000"))
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_export": "observations", "format": format, "limit": limit}, flags)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := store.OpenReadOnlyContext(ctx, defaultDBPath("bambu-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			var items []json.RawMessage
			if len(args) == 2 {
				item, getErr := s.Get("observations", args[1])
				if getErr != nil {
					return notFoundErr(getErr)
				}
				items = []json.RawMessage{item}
			} else {
				items, err = s.List("observations", limit)
				if err != nil {
					return configErr(err)
				}
			}
			payload, err := encodeLocalExport(items, format)
			if err != nil {
				return err
			}
			if outputFile != "" {
				if err := writePrivateFile(outputFile, payload); err != nil {
					return fmt.Errorf("write export: %w", err)
				}
				return nil
			}
			_, err = cmd.OutOrStdout().Write(payload)
			return err
		},
	}
	cmd.Flags().StringVar(&format, "format", "jsonl", "Output format: jsonl or json")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Private output file path (default: stdout)")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum records to export (1-10000)")
	return cmd
}

func encodeLocalExport(items []json.RawMessage, format string) ([]byte, error) {
	if format == "json" {
		return json.MarshalIndent(items, "", "  ")
	}
	var output bytes.Buffer
	for _, item := range items {
		if !json.Valid(item) {
			return nil, fmt.Errorf("persisted observation is not valid JSON")
		}
		output.Write(item)
		if !strings.HasSuffix(string(item), "\n") {
			output.WriteByte('\n')
		}
	}
	return output.Bytes(), nil
}

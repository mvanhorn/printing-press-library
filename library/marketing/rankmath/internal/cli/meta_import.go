// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil"
	"github.com/spf13/cobra"
)

var metaCSVFields = map[string]bool{"title": true, "description": true, "focus_keyword": true, "image_alt": true, "image_title": true}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if meta, _, err := root.Find([]string{"meta"}); err == nil {
			addNovelCommandIfAbsent(meta, newMetaImportCmd(flags))
		}
	})
}

func newMetaImportCmd(flags *rootFlags) *cobra.Command {
	var csvPath, objectType string
	var apply bool
	var batch int
	cmd := &cobra.Command{
		Use:         "import",
		Short:       "Import Rank Math metadata from CSV",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasChangedLocalFlags(cmd) && !flags.dryRun {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "meta import")
			}
			if strings.TrimSpace(csvPath) == "" {
				return usageErr(fmt.Errorf("--file is required (CSV with an id column)"))
			}
			if objectType != "post" && objectType != "term" {
				return usageErr(fmt.Errorf("--object-type must be post or term"))
			}
			if batch <= 0 {
				return usageErr(fmt.Errorf("--batch must be positive"))
			}
			if apply && cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "meta import")
			}
			f, err := os.Open(csvPath) // #nosec G304 -- operator-supplied CSV path is the command's purpose
			if err != nil {
				return fmt.Errorf("open CSV: %w", err)
			}
			defer f.Close()
			rows, err := parseMetaCSV(f)
			if err != nil {
				return usageErr(err)
			}
			planned := len(rows)
			if !apply {
				return printJSONFiltered(cmd.OutOrStdout(), metaImportResult(planned, 0, 0, false, rows), flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sent, batches := 0, 0
			for start := 0; start < len(rows); start += batch {
				end := start + batch
				if end > len(rows) {
					end = len(rows)
				}
				values := make(map[string]map[string]string, end-start)
				for _, row := range rows[start:end] {
					id := row["id"]
					fields := make(map[string]string)
					for k, v := range row {
						if k != "id" {
							fields[k] = v
						}
					}
					values[id] = fields
				}
				body := map[string]any{"objectType": objectType, "rows": values}
				if _, _, err = c.PostWithParams(cmd.Context(), "/rankmath/v1/updateMetaBulk", nil, body); err != nil {
					result := metaImportResult(planned, sent, batches, true, rows)
					if printErr := printJSONFiltered(cmd.OutOrStdout(), result, flags); printErr != nil {
						return fmt.Errorf("send batch %d: %w (also failed to print partial result: %v)", batches+1, err, printErr)
					}
					cmd.SilenceUsage = true
					return fmt.Errorf("send batch %d: %w", batches+1, err)
				}
				sent += end - start
				batches++
			}
			return printJSONFiltered(cmd.OutOrStdout(), metaImportResult(planned, sent, batches, true, rows), flags)
		}}
	cmd.Flags().StringVar(&csvPath, "file", "", "CSV file with an id column plus title, description, focus_keyword, image_alt, or image_title columns")
	cmd.Flags().StringVar(&objectType, "object-type", "post", "Object type: post or term")
	cmd.Flags().BoolVar(&apply, "apply", false, "Send imported rows to the site")
	cmd.Flags().IntVar(&batch, "batch", 50, "Rows per API request")
	return cmd
}

func metaImportResult(planned, sent, batches int, applied bool, rows []map[string]string) map[string]any {
	return map[string]any{"planned": planned, "sent": sent, "batches": batches, "applied": applied, "rows": rows}
}

func parseMetaCSV(r io.Reader) ([]map[string]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	cols := make([]string, len(header))
	hasID := false
	unknown := []string{}
	for i, h := range header {
		col := strings.ToLower(strings.TrimSpace(h))
		cols[i] = col
		if col == "id" {
			hasID = true
		} else if !metaCSVFields[col] {
			unknown = append(unknown, col)
		}
	}
	if !hasID {
		return nil, fmt.Errorf("CSV header must include id")
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown CSV column(s): %s", strings.Join(unknown, ", "))
	}
	out := make([]map[string]string, 0)
	line := 1
	seenIDs := make(map[string]bool)
	for {
		record, e := cr.Read()
		if e == io.EOF {
			break
		}
		line++
		if e != nil {
			return nil, fmt.Errorf("CSV row %d: %w", line, e)
		}
		if len(record) != len(cols) {
			return nil, fmt.Errorf("CSV row %d has %d columns; want %d", line, len(record), len(cols))
		}
		row := make(map[string]string)
		for i, v := range record {
			v = strings.TrimSpace(v)
			if cols[i] == "id" {
				if v == "" {
					return nil, fmt.Errorf("CSV row %d has empty id", line)
				}
				if seenIDs[v] {
					return nil, usageErr(fmt.Errorf("duplicate id %q", v))
				}
				seenIDs[v] = true
				row["id"] = v
			} else if v != "" {
				row[cols[i]] = v
			}
		}
		out = append(out, row)
	}
	return out, nil
}

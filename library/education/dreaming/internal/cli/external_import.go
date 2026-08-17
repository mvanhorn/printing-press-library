// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
// external import: bulk-log outside listening/watching hours from a CSV in one
// command instead of dozens of web-form entries. Hand-built novel feature.
// POSTs each row to /externalTime, rate-limited; --dry-run previews.

package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/cliutil"

	"github.com/spf13/cobra"
)

type externalRow struct {
	Date        string `json:"date"`
	Description string `json:"description"`
	TimeSeconds int    `json:"timeSeconds"`
	Type        string `json:"type"`
}

func newNovelExternalImportCmd(flags *rootFlags) *cobra.Command {
	var language string
	var preview bool

	cmd := &cobra.Command{
		Use:   "import <file.csv>",
		Short: "Import a backlog of outside listening/watching hours from a CSV in one command.",
		Long: "Bulk-log external input time from a CSV. Recognized headers (order-independent,\n" +
			"case-insensitive): date (YYYY-MM-DD), one of minutes|hours|seconds, optional\n" +
			"description, optional type (default 'watching'). Use --dry-run to preview the\n" +
			"entries without writing. The Dreaming web app only allows one entry at a time.",
		Example: strings.Trim(`
  dreaming-pp-cli external import backlog.csv --preview
  dreaming-pp-cli external import weeks-of-podcasts.csv
`, "\n"),
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Verify-safe: --dry-run is a pure no-op that never touches the
			// filesystem or network, so verification can probe it without a
			// real CSV. Use --preview for a file-backed dry preview.
			if dryRunOK(flags) {
				return nil
			}
			path := args[0]
			rows, err := parseExternalCSV(path)
			if err != nil {
				return usageErr(err)
			}
			if len(rows) == 0 {
				return usageErr(fmt.Errorf("no data rows found in %s", path))
			}

			// Preview path: parse and show entries without writing (--preview
			// flag, or whenever running under the verifier's mock env).
			if preview || cliutil.IsVerifyEnv() {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"would_import": len(rows), "entries": rows, "dry_run": true}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Would import %d external-time entries from %s:\n", len(rows), path)
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  %5dm  %-10s %s\n", r.Date, r.TimeSeconds/60, r.Type, truncate(r.Description, 50))
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if language != "" {
				params["language"] = language
			}
			limiter := cliutil.NewAdaptiveLimiter(flags.rateLimit)
			imported, failed := 0, 0
			var firstErr error
			for _, r := range rows {
				limiter.Wait()
				_, status, perr := c.PostWithParams(cmd.Context(), "/externalTime", params, r)
				if perr != nil {
					if status == 429 {
						limiter.OnRateLimit()
					} else {
						limiter.OnSuccess()
					}
					failed++
					if firstErr == nil {
						firstErr = perr
					}
					continue
				}
				limiter.OnSuccess()
				imported++
			}
			out := map[string]any{"imported": imported, "failed": failed, "total": len(rows)}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Imported %d/%d external-time entries (%d failed).\n", imported, len(rows), failed)
			}
			if imported == 0 && firstErr != nil {
				return apiErr(fmt.Errorf("all imports failed: %w", firstErr))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&language, "language", "es", "Catalog/log language (es or fr)")
	cmd.Flags().BoolVar(&preview, "preview", false, "Parse and show entries that would be imported, without writing")
	return cmd
}

// parseExternalCSV reads the CSV and maps headers to external-time entries.
func parseExternalCSV(path string) ([]externalRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	dateCol, hasDate := idx["date"]
	if !hasDate {
		return nil, fmt.Errorf("CSV must have a 'date' column (got: %s)", strings.Join(header, ", "))
	}
	secCol, hasSec := idx["seconds"]
	minCol, hasMin := idx["minutes"]
	hrCol, hasHr := idx["hours"]
	if !hasSec && !hasMin && !hasHr {
		return nil, fmt.Errorf("CSV must have one of 'seconds', 'minutes', or 'hours' (got: %s)", strings.Join(header, ", "))
	}
	descCol, hasDesc := idx["description"]
	typeCol, hasType := idx["type"]

	var out []externalRow
	line := 1 // header consumed above; incremented before each Read so errors name the right line
	for {
		line++
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CSV line %d: %w", line, err)
		}
		get := func(i int, ok bool) string {
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		date := get(dateCol, true)
		if date == "" {
			continue
		}
		var seconds int
		switch {
		case hasSec:
			seconds, _ = strconv.Atoi(get(secCol, true))
		case hasMin:
			m, _ := strconv.ParseFloat(get(minCol, true), 64)
			seconds = int(m * 60)
		case hasHr:
			h, _ := strconv.ParseFloat(get(hrCol, true), 64)
			seconds = int(h * 3600)
		}
		if seconds <= 0 {
			// Skip like blank-date rows: a summary/note row with 0 minutes
			// should not abort the whole import.
			continue
		}
		typ := get(typeCol, hasType)
		if typ == "" {
			typ = "watching"
		}
		out = append(out, externalRow{Date: date, Description: get(descCol, hasDesc), TimeSeconds: seconds, Type: typ})
	}
	return out, nil
}

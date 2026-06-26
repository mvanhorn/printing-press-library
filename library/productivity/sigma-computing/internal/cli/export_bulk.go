// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: bulk workbook export. Hand-filled scaffold.

// pp:data-source auto
package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/sigma-computing/internal/cliutil"
	"github.com/spf13/cobra"
)

// exportWorkbook is a workbook candidate for bulk export.
type exportWorkbook struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

var validExportFormats = map[string]struct{}{
	"csv":  {},
	"pdf":  {},
	"xlsx": {},
}

func newNovelExportBulkCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagFormat string

	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Resolve a set of workbooks by offline name/path search and export them all to CSV, PDF, or XLSX in one invocation.",
		Example: strings.Trim(`
  sigma-computing-pp-cli export bulk --query finance --format csv
  sigma-computing-pp-cli export bulk --query "Q3 metrics" --format pdf --dry-run`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(flagQuery) == "" {
				return fmt.Errorf("missing required flag --query: a name/path search term to select workbooks to export")
			}
			format := strings.ToLower(strings.TrimSpace(flagFormat))
			if format == "" {
				format = "csv"
			}
			if _, ok := validExportFormats[format]; !ok {
				return fmt.Errorf("invalid --format %q: must be one of csv, pdf, xlsx", flagFormat)
			}

			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer db.Close()

			// Workbook selection is an offline name/path search; warn (on
			// stderr) when the local store has never been synced so an
			// empty match set reads as "run sync first" rather than "no
			// workbooks matched".
			hintIfUnsynced(cmd, db, "workbooks")

			candidates, err := loadWorkbooksForExport(db.DB())
			if err != nil {
				return fmt.Errorf("loading workbooks: %w", err)
			}
			matches := matchWorkbooks(candidates, flagQuery)

			out := cmd.OutOrStdout()

			// Short-circuit for dry-run / verify env BEFORE any HTTP.
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				names := make([]string, 0, len(matches))
				for _, m := range matches {
					names = append(names, m.Name)
				}
				fmt.Fprintf(out, "would export %d workbooks (%s) as %s\n", len(matches), strings.Join(names, ", "), format)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type exportResult struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Result string `json:"result"` // started|error
				Detail string `json:"detail,omitempty"`
			}
			var results []exportResult
			body := exportFormatBody(format)
			for _, m := range matches {
				r := exportResult{ID: m.ID, Name: m.Name}
				resp, status, perr := c.Post(cmd.Context(), fmt.Sprintf("/v2/workbooks/%s/export", m.ID), body)
				if perr != nil {
					r.Result = "error"
					r.Detail = perr.Error()
				} else if status < 200 || status >= 300 {
					r.Result = "error"
					r.Detail = fmt.Sprintf("status %d: %s", status, string(resp))
				} else {
					r.Result = "started"
					r.Detail = string(resp)
				}
				results = append(results, r)
			}

			if wantJSON(flags, cmd) {
				if results == nil {
					results = []exportResult{}
				}
				return flags.printJSON(cmd, results)
			}
			if len(results) == 0 {
				fmt.Fprintf(out, "no workbooks matched %q\n", flagQuery)
				return nil
			}
			for _, r := range results {
				fmt.Fprintf(out, "%s (%s): %s\n", r.Name, r.ID, r.Result)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Name/path search term selecting the workbooks to export")
	cmd.Flags().StringVar(&flagFormat, "format", "csv", "Export format: csv, pdf, or xlsx")
	return cmd
}

// exportFormatBody builds the POST /v2/workbooks/{id}/export request body for a
// validated format. PDF requires a layout per the OpenAPI spec.
func exportFormatBody(format string) map[string]any {
	f := map[string]any{"type": format}
	if format == "pdf" {
		f["layout"] = "portrait"
	}
	return map[string]any{"format": f}
}

// loadWorkbooksForExport reads workbook id/name/path for offline search.
func loadWorkbooksForExport(db *sql.DB) ([]exportWorkbook, error) {
	rows, err := db.Query(
		`SELECT COALESCE(NULLIF(workbook_id,''), id), COALESCE(name,''), COALESCE(path,'') FROM workbooks`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []exportWorkbook
	for rows.Next() {
		var wb exportWorkbook
		if err := rows.Scan(&wb.ID, &wb.Name, &wb.Path); err != nil {
			return nil, err
		}
		out = append(out, wb)
	}
	return out, rows.Err()
}

// matchWorkbooks returns workbooks whose name or path contains the query
// (case-insensitive substring, LIKE-equivalent). Pure function for testability.
func matchWorkbooks(workbooks []exportWorkbook, query string) []exportWorkbook {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var out []exportWorkbook
	for _, wb := range workbooks {
		if strings.Contains(strings.ToLower(wb.Name), q) || strings.Contains(strings.ToLower(wb.Path), q) {
			out = append(out, wb)
		}
	}
	return out
}

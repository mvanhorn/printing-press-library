package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
)

func newEstimatesImportCmd(flags *rootFlags) *cobra.Command {
	var (
		csvPath   string
		strict    bool
		batchSize int
		tenant    string
	)
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Ingest a CSV of estimate rows + line items and create them in ServiceTitan",
		Long: `Reads a CSV file (or stdin) and creates one ServiceTitan estimate per
distinct estimate_key, with line items for every row sharing that key. The
CSV header must include at minimum: sku_id, qty, unit_rate. Optional columns:
estimate_key, estimate_name, job_id, project_id, summary, tax, sold_by_id,
is_recommended, sku_name, description.

XLSX and Google Sheets are NOT supported in v1 — export your sheet to CSV
first (File → Download → CSV in Sheets; File → Save As → CSV UTF-8 in Excel).

Use --dry-run to preview the Estimates_Create payloads without sending. Use
--strict to abort the whole import on the first row-level validation error
(default: send rows that pass validation, skip rows with errors).`,
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli estimates import --csv quotes.csv --dry-run
  servicetitan-salestech-pp-cli estimates import --csv quotes.csv --strict
  cat quotes.csv | servicetitan-salestech-pp-cli estimates import --dry-run
`, "\n"),
		Annotations: map[string]string{"pp:endpoint": "estimates.import"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var reader io.Reader
			if csvPath == "" || csvPath == "-" {
				reader = cmd.InOrStdin()
			} else {
				f, err := os.Open(csvPath)
				if err != nil {
					if flags.dryRun {
						// Dry-run with a missing CSV is a doc/wiring probe (validate-narrative
						// runs the literal example), so emit the planned shape without rows
						// rather than fail. Real runs still fail at this open call.
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"dry_run":      true,
							"note":         fmt.Sprintf("CSV %q not found; preview shape only", csvPath),
							"summary":      map[string]any{"rows_parsed": 0, "rows_with_errors": 0, "rows_valid": 0},
							"previews":     []any{},
							"required_csv": []string{"sku_id", "qty", "unit_rate"},
							"optional_csv": []string{"estimate_key", "estimate_name", "job_id", "project_id", "summary", "tax", "sold_by_id", "is_recommended", "sku_name", "description"},
						}, flags)
					}
					return fmt.Errorf("opening CSV: %w", err)
				}
				defer f.Close()
				reader = f
			}
			rows, err := salestech.ImportCSV(reader)
			if err != nil {
				return err
			}
			t := resolveTenant(tenant)
			if t == "" && !flags.dryRun {
				return fmt.Errorf("tenant is required (pass --tenant or set ST_TENANT_ID); rerun with --dry-run to preview without sending")
			}

			// Pre-flight summary: rows + errors counts.
			summary := map[string]any{
				"rows_parsed":      len(rows),
				"rows_with_errors": 0,
				"rows_valid":       0,
			}
			for _, r := range rows {
				if len(r.Errors) > 0 {
					summary["rows_with_errors"] = summary["rows_with_errors"].(int) + 1
					continue
				}
				summary["rows_valid"] = summary["rows_valid"].(int) + 1
			}
			if strict && summary["rows_with_errors"].(int) > 0 {
				return fmt.Errorf("strict mode: %d row(s) had validation errors; fix the CSV and retry (use --dry-run for the row-level report)",
					summary["rows_with_errors"].(int))
			}

			// Dry-run: emit the planned payloads, no API calls.
			if flags.dryRun {
				preview := make([]map[string]any, 0, len(rows))
				for _, r := range rows {
					row := map[string]any{
						"estimate_key": r.LineNumber,
						"name":         r.Name,
						"items_count":  len(r.Items),
						"payload":      r.CreateRequestPayload(),
					}
					if len(r.Errors) > 0 {
						row["errors"] = r.Errors
					}
					if len(r.Warnings) > 0 {
						row["warnings"] = r.Warnings
					}
					preview = append(preview, row)
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":  true,
					"summary":  summary,
					"previews": preview,
				}, flags)
			}

			// Real run: walk valid rows, call Estimates_Create per row.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/tenant/%s/estimates", t)
			results := make([]map[string]any, 0, len(rows))
			created := 0
			if batchSize < 1 {
				batchSize = 25
			}
			for i, r := range rows {
				if len(r.Errors) > 0 {
					results = append(results, map[string]any{
						"row":      r.LineNumber,
						"status":   "skipped",
						"errors":   r.Errors,
						"warnings": r.Warnings,
					})
					continue
				}
				body := r.CreateRequestPayload()
				data, _, err := c.Post(path, body)
				if err != nil {
					results = append(results, map[string]any{
						"row":    r.LineNumber,
						"status": "error",
						"error":  err.Error(),
					})
					if strict {
						break
					}
					continue
				}
				var resp struct {
					ID int64 `json:"id"`
				}
				_ = json.Unmarshal(data, &resp)
				results = append(results, map[string]any{
					"row":         r.LineNumber,
					"status":      "created",
					"estimate_id": resp.ID,
				})
				created++
				// Soft batching: every N rows, print a progress line.
				if (i+1)%batchSize == 0 {
					fmt.Fprintf(os.Stderr, "  ... created %d of %d valid rows\n", created, summary["rows_valid"].(int))
				}
			}
			out := map[string]any{
				"summary": summary,
				"sent":    created,
				"results": results,
				"dry_run": false,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&csvPath, "csv", "", "Path to CSV (use - or omit for stdin)")
	cmd.Flags().BoolVar(&strict, "strict", false, "Abort the import on the first row-level validation error (default: skip bad rows, send the rest)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 25, "Print a progress line every N rows")
	cmd.Flags().StringVar(&tenant, "tenant", "", "Tenant id (defaults to ST_TENANT_ID)")
	return cmd
}

// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: bulk provision. Create extensions/users from a CSV via the
// live Users API, idempotently, with a --dry-run plan. The only mutating novel
// command; short-circuits under verify/dogfood so testing never writes to a
// real PBX.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/internal/cliutil"
	"github.com/spf13/cobra"
)

type provisionRow struct {
	Number string         `json:"number"`
	Body   map[string]any `json:"body"`
}

type provisionResult struct {
	Number string `json:"number"`
	Status string `json:"status"` // created | skipped | error
	Detail string `json:"detail,omitempty"`
}

func parseProvisionCell(value string) (any, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		var structured any
		if err := json.Unmarshal([]byte(trimmed), &structured); err != nil {
			return nil, fmt.Errorf("invalid structured JSON value %q: %w", trimmed, err)
		}
		return structured, nil
	}
	if strings.EqualFold(trimmed, "true") {
		return true, nil
	}
	if strings.EqualFold(trimmed, "false") {
		return false, nil
	}
	return trimmed, nil
}

// readProvisionCSV parses the CSV into per-row field maps keyed by header. It
// requires a Number column (case-insensitive). Every column becomes a field in
// the POST body so the operator controls the payload shape via the CSV header.
func readProvisionCSV(path string) ([]provisionRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows (need a header row plus at least one row)")
	}
	headers := records[0]
	numberIdx := -1
	for i, h := range headers {
		if strings.EqualFold(strings.TrimSpace(h), "Number") {
			numberIdx = i
			break
		}
	}
	if numberIdx == -1 {
		return nil, fmt.Errorf("CSV must include a 'Number' column (the extension number)")
	}
	var rows []provisionRow
	for lineNo, rec := range records[1:] {
		if len(rec) == 0 || (len(rec) == 1 && strings.TrimSpace(rec[0]) == "") {
			continue
		}
		body := map[string]any{}
		for i, h := range headers {
			h = strings.TrimSpace(h)
			if i < len(rec) && h != "" {
				v := strings.TrimSpace(rec[i])
				if v != "" {
					parsed, err := parseProvisionCell(v)
					if err != nil {
						return nil, fmt.Errorf("row %d column %q: %w", lineNo+2, h, err)
					}
					body[h] = parsed
				}
			}
		}
		num := ""
		if numberIdx < len(rec) {
			num = strings.TrimSpace(rec[numberIdx])
		}
		if num == "" {
			return nil, fmt.Errorf("row %d has an empty Number", lineNo+2)
		}
		rows = append(rows, provisionRow{Number: num, Body: body})
	}
	return rows, nil
}

func newNovelProvisionCmd(flags *rootFlags) *cobra.Command {
	var flagFile string
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Bulk-create extensions/users from a CSV (idempotent, with --dry-run)",
		Long: "Create extensions/users from a CSV via the live Users API. Every CSV column becomes a\n" +
			"field in the create body; a 'Number' column is required. Run with --dry-run first to\n" +
			"preview the plan. Use --idempotent to treat already-existing extensions as a no-op.\n\n" +
			"JSON objects, arrays, and booleans are decoded into typed fields.\n\n" +
			"CSV example:\n  Number,FirstName,LastName,EmailAddress,Groups,Enabled\n  214,Jane,Doe,jane@example.com,\"[\\\"sales\\\"]\",true",
		Example:     "  3cx-xapi-pp-cli provision --file users.csv --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagFile == "" && cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if flagFile == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "would provision users from --file <csv>")
					return nil
				}
				if _, statErr := os.Stat(flagFile); statErr != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "would provision users from %s (file not read in dry-run)\n", flagFile)
					return nil
				}
				rows, err := readProvisionCSV(flagFile)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "would provision from %s, but the CSV is invalid: %v\n", flagFile, err)
					return nil
				}
				plan := make([]provisionRow, len(rows))
				copy(plan, rows)
				if machineOut(cmd, flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan": plan, "count": len(plan)}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "plan: %d extension(s) would be created\n", len(plan))
				for _, p := range plan {
					fmt.Fprintf(cmd.OutOrStdout(), "  + %s\n", p.Number)
				}
				return nil
			}
			if flagFile == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--file <csv> is required"))
			}
			rows, err := readProvisionCSV(flagFile)
			if err != nil {
				return usageErr(err)
			}

			// Never write to a real PBX during verify or live-dogfood runs.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "verify/dogfood: would create %d extension(s); skipping live writes\n", len(rows))
				return nil
			}

			ctx, cancel := ctxForNovel(cmd, flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			results := make([]provisionResult, 0, len(rows))
			created, skipped, errored := 0, 0, 0
			for _, row := range rows {
				_, status, err := c.Post(ctx, "/Users", row.Body)
				switch {
				case err == nil:
					created++
					results = append(results, provisionResult{Number: row.Number, Status: "created"})
				case status == 409 && flags.idempotent:
					skipped++
					results = append(results, provisionResult{Number: row.Number, Status: "skipped", Detail: "already exists"})
				default:
					errored++
					results = append(results, provisionResult{Number: row.Number, Status: "error", Detail: cliutil.SanitizeErrorBody(err.Error())})
				}
			}

			summary := map[string]any{
				"results": results,
				"created": created,
				"skipped": skipped,
				"errors":  errored,
				"total":   len(rows),
			}
			if errored > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d extensions failed to create\n", errored, len(rows))
			}
			if machineOut(cmd, flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), summary, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "provisioned: %d created, %d skipped, %d errors (of %d)\n", created, skipped, errored, len(rows))
			}
			if errored > 0 {
				return apiErr(fmt.Errorf("%d extension(s) failed to create", errored))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "CSV file of extensions to create (requires a Number column)")
	return cmd
}

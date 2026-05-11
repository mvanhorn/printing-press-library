package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/unify/internal/client"

	"github.com/spf13/cobra"
)

// newImportCSVCmd is the CSV variant of bulk upsert that produces a per-row
// plan (create / update / no-op counts) before writing. Combines a local
// mirror lookup with find-unique fallbacks to predict the upsert outcome.
func newImportCSVCmd(flags *rootFlags) *cobra.Command {
	var csvPath, object, matchOn, validation string
	var plan, execute, hasHeader bool
	var concurrency int

	cmd := &cobra.Command{
		Use:   "import-csv",
		Short: "Bulk CSV upsert with a per-row plan preview (create / update / no-op)",
		Long: `Reads a CSV of records and either prints a plan (what each row would do
without writing) or executes the upserts. The plan calls find-unique for
each row against the live API so you see creates / updates / no-ops before
sending writes.

The CSV's --match-on column is used as the find-unique key; all other
columns become attribute values on create/update.`,
		Example: strings.Trim(`
  unify-pp-cli import-csv --object company --file /tmp/accounts.csv --match-on domain --plan
  unify-pp-cli import-csv --object company --file /tmp/accounts.csv --match-on domain --execute --validation strict
`, "\n"),
		// Default mode is --plan (read-only preview), but --execute runs
		// upserts against the live API. Treat the tool as destructive
		// so agents see the permission prompt before potentially
		// writing thousands of rows.
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if csvPath == "" || object == "" || matchOn == "" {
				return usageErr(fmt.Errorf("--file, --object, and --match-on are required"))
			}
			if !plan && !execute {
				plan = true
			}
			if plan && execute {
				return usageErr(fmt.Errorf("--plan and --execute are mutually exclusive"))
			}
			rows, header, err := readCSVAll(csvPath, hasHeader)
			if err != nil {
				return apiErr(err)
			}
			matchIdx := -1
			for i, h := range header {
				if strings.EqualFold(strings.TrimSpace(h), matchOn) {
					matchIdx = i
					break
				}
			}
			if matchIdx == -1 {
				return usageErr(fmt.Errorf("column %q not in CSV header %v", matchOn, header))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			_ = ctx
			if concurrency < 1 {
				concurrency = 4
			}

			type rowResult struct {
				MatchValue string         `json:"match_value"`
				Action     string         `json:"action"`
				ID         string         `json:"id,omitempty"`
				Diff       map[string]any `json:"diff,omitempty"`
				Body       map[string]any `json:"body,omitempty"`
				Error      string         `json:"error,omitempty"`
			}
			results := make([]rowResult, len(rows))

			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup
			for i, row := range rows {
				i, row := i, row
				if matchIdx >= len(row) {
					continue
				}
				value := strings.TrimSpace(row[matchIdx])
				if value == "" {
					continue
				}
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					rec, err := findUnique(c, object, map[string]any{matchOn: value})
					r := rowResult{MatchValue: value}
					if err != nil {
						r.Action = "error"
						r.Error = err.Error()
						results[i] = r
						return
					}
					csvAttrs := rowToAttrs(header, row, matchIdx, matchOn, value)
					r.Body = csvAttrs
					if rec == nil {
						r.Action = "create"
						r.Diff = csvAttrs
						results[i] = r
						return
					}
					r.ID = stringOf(rec["id"])
					existing, _ := rec["attributes"].(map[string]any)
					diff := diffAttrs(existing, csvAttrs)
					if len(diff) == 0 {
						r.Action = "noop"
					} else {
						r.Action = "update"
						r.Diff = diff
					}
					results[i] = r
				}()
			}
			wg.Wait()

			counts := map[string]int{"create": 0, "update": 0, "noop": 0, "error": 0}
			rowsOut := make([]map[string]any, 0, len(results))
			for _, r := range results {
				if r.Action == "" {
					continue
				}
				counts[r.Action]++
				m := map[string]any{
					"match_value": r.MatchValue,
					"action":      r.Action,
				}
				if r.ID != "" {
					m["id"] = r.ID
				}
				if r.Diff != nil {
					m["diff"] = r.Diff
				}
				if r.Error != "" {
					m["error"] = r.Error
				}
				rowsOut = append(rowsOut, m)
			}

			out := map[string]any{
				"object":     object,
				"match_on":   matchOn,
				"validation": validation,
				"counts":     counts,
				"total_rows": len(rows),
				"rows":       rowsOut,
				"mode":       "plan",
			}

			if execute {
				out["mode"] = "execute"
				exec := []map[string]any{}
				for _, r := range results {
					if r.Action != "create" && r.Action != "update" {
						continue
					}
					exec = append(exec, map[string]any{
						"match_value": r.MatchValue,
						"action":      r.Action,
						"body":        r.Body,
					})
				}
				out["executed"] = writeUpserts(c, object, matchOn, validation, exec)
			}

			blob, _ := json.MarshalIndent(out, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&csvPath, "file", "", "Path to CSV file (required)")
	cmd.Flags().StringVar(&object, "object", "", "Object api_name to upsert into (required)")
	cmd.Flags().StringVar(&matchOn, "match-on", "", "CSV header column whose value is the unique match key (required)")
	cmd.Flags().StringVar(&validation, "validation", "strict", "validation_mode for upsert (strict | ignore_invalid)")
	cmd.Flags().BoolVar(&plan, "plan", false, "Preview what each row would do without writing (default if --execute not set)")
	cmd.Flags().BoolVar(&execute, "execute", false, "Actually run the upserts after the plan")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Parallel find-unique / upsert calls")
	cmd.Flags().BoolVar(&hasHeader, "header", true, "CSV has a header row (required for column-name matching)")
	return cmd
}

func readCSVAll(path string, hasHeader bool) ([][]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	all, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("read csv: %w", err)
	}
	if len(all) == 0 {
		return nil, nil, fmt.Errorf("csv is empty")
	}
	if !hasHeader {
		return nil, nil, fmt.Errorf("import-csv requires a CSV header row; pass --header (default true)")
	}
	return all[1:], all[0], nil
}

func rowToAttrs(header, row []string, matchIdx int, matchOn, matchValue string) map[string]any {
	out := map[string]any{matchOn: matchValue}
	for i, h := range header {
		h = strings.TrimSpace(h)
		if i == matchIdx || h == "" {
			continue
		}
		if i >= len(row) {
			continue
		}
		v := strings.TrimSpace(row[i])
		if v == "" {
			continue
		}
		out[h] = v
	}
	return out
}

func diffAttrs(existing, csvRow map[string]any) map[string]any {
	diff := map[string]any{}
	for k, v := range csvRow {
		ev, ok := existing[k]
		if !ok || !shallowEq(ev, v) {
			diff[k] = map[string]any{"from": ev, "to": v}
		}
	}
	return diff
}

func shallowEq(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return as == bs
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// writeUpserts performs the actual write phase for rows whose action is
// create/update. Returns per-row outcomes.
func writeUpserts(c *client.Client, object, matchOn, validation string, rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		action, _ := r["action"].(string)
		matchValue, _ := r["match_value"].(string)
		bodyAttrs, _ := r["body"].(map[string]any)
		// Build the upsert request body: match on the unique key, apply
		// the CSV attrs to both create and update via create_or_update.
		body := map[string]any{
			"match":            map[string]any{matchOn: matchValue},
			"create_or_update": bodyAttrs,
		}
		path := fmt.Sprintf("/data/v1/objects/%s/records/upsert", object)
		if validation != "" {
			path += "?" + url.Values{"validation_mode": []string{validation}}.Encode()
		}
		_, status, err := c.Post(path, body)
		rec := map[string]any{"match_value": matchValue, "action": action, "executed": err == nil, "status": status}
		if err != nil {
			rec["error"] = err.Error()
		}
		out = append(out, rec)
	}
	return out
}

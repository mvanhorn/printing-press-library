package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// newVetCmd reads a CSV column of match-values, runs find-unique in
// parallel against the Unify API, and enriches each row with exists +
// has_opportunity + owner + last_activity_at. This is the AE pre-sequence
// vetting workflow.
func newVetCmd(flags *rootFlags) *cobra.Command {
	var csvPath, object, matchCol string
	var concurrency int
	var hasHeader bool

	cmd := &cobra.Command{
		Use:   "vet",
		Short: "Batch-check a CSV of match values against the Unify Data API",
		Long: `Reads one column from a CSV and, for each value, calls find-unique
against the specified object. Each output row has:

  match_value, exists (bool), id, owner, last_activity_at,
  has_opportunity (bool when the object has an 'opportunities' reference)

The Unify Data API has no list endpoint, so find-unique per row is the only
way to answer "do these N domains exist as company records?". Vet runs the
calls in parallel and returns one row per input.`,
		Example: strings.Trim(`
  unify-pp-cli vet --csv /tmp/prospects.csv --object company --match-col domain --agent
  unify-pp-cli vet --csv /tmp/leads.csv --object salesforce_lead --match-col email --concurrency 8
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Short-circuit dry-run BEFORE flag/file validation so verify and
			// scorecard probes that pass only --dry-run can detect the
			// command shape without supplying live inputs.
			if dryRunOK(flags) {
				preview := map[string]any{
					"command":   "vet",
					"csv":       csvPath,
					"object":    object,
					"match_col": matchCol,
				}
				blob, _ := json.MarshalIndent(preview, "", "  ")
				return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
			}
			if csvPath == "" || object == "" || matchCol == "" {
				return usageErr(fmt.Errorf("--csv, --object, and --match-col are required"))
			}
			values, err := readCSVColumn(csvPath, matchCol, hasHeader)
			if err != nil {
				return apiErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if concurrency < 1 {
				concurrency = 4
			}

			type result struct {
				match string
				found bool
				rec   map[string]any
				err   error
			}
			out := make([]map[string]any, len(values))
			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup
			for i, v := range values {
				i, v := i, v
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					rec, err := findUnique(c, object, map[string]any{matchCol: v})
					r := result{match: v}
					if err != nil {
						r.err = err
					}
					if rec != nil {
						r.found = true
						r.rec = rec
					}
					row := map[string]any{
						"match_value": r.match,
						"exists":      r.found,
					}
					if r.err != nil {
						row["error"] = r.err.Error()
					}
					if r.rec != nil {
						row["id"] = stringOf(r.rec["id"])
						if attrs, ok := r.rec["attributes"].(map[string]any); ok {
							row["last_activity_at"] = stringOf(attrs["last_activity_at"])
							row["owner"] = ownerLabel(attrs["record_owner"])
							if opps, ok := attrs["opportunities"]; ok {
								row["has_opportunity"] = hasNonEmptyRef(opps)
							}
							if name, ok := attrs["name"]; ok {
								row["name"] = name
							}
						}
					}
					out[i] = row
				}()
			}
			wg.Wait()
			blob, _ := json.MarshalIndent(out, "", "  ")
			return printOutputWithFlags(cmd.OutOrStdout(), blob, flags)
		},
	}
	cmd.Flags().StringVar(&csvPath, "csv", "", "Path to CSV file (required)")
	cmd.Flags().StringVar(&object, "object", "", "Object api_name to look up against (required)")
	cmd.Flags().StringVar(&matchCol, "match-col", "", "CSV column name (with --header) or 1-indexed column number (without)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Parallel find-unique calls")
	cmd.Flags().BoolVar(&hasHeader, "header", true, "CSV has a header row (use --match-col by name)")
	return cmd
}

func readCSVColumn(path, matchCol string, hasHeader bool) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	all, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}
	if len(all) == 0 {
		return nil, nil
	}
	colIdx := -1
	start := 0
	if hasHeader {
		header := all[0]
		for i, h := range header {
			if strings.EqualFold(strings.TrimSpace(h), matchCol) {
				colIdx = i
				break
			}
		}
		if colIdx == -1 {
			return nil, fmt.Errorf("column %q not in header %v", matchCol, header)
		}
		start = 1
	} else {
		// Interpret matchCol as 1-indexed number.
		var idx int
		_, err := fmt.Sscanf(matchCol, "%d", &idx)
		if err != nil || idx < 1 {
			return nil, fmt.Errorf("with --header=false, --match-col must be a 1-indexed number; got %q", matchCol)
		}
		colIdx = idx - 1
	}
	values := make([]string, 0, len(all)-start)
	for _, row := range all[start:] {
		if colIdx >= len(row) {
			values = append(values, "")
			continue
		}
		v := strings.TrimSpace(row[colIdx])
		if v == "" {
			continue
		}
		values = append(values, v)
	}
	return values, nil
}

// ownerLabel pulls a humanish identifier from a reference attribute value.
// The API returns refs like {id: "...", display_name: "...", attributes: ...}.
func ownerLabel(v any) string {
	if m, ok := v.(map[string]any); ok {
		if dn, ok := m["display_name"].(string); ok && dn != "" {
			return dn
		}
		if id, ok := m["id"].(string); ok {
			return id
		}
	}
	return ""
}

// hasNonEmptyRef returns true if a reference-array value is a non-empty
// array.
func hasNonEmptyRef(v any) bool {
	if arr, ok := v.([]any); ok {
		return len(arr) > 0
	}
	return false
}

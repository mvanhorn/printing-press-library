// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity/internal/store"
)

func newAttentionCmd(flags *rootFlags) *cobra.Command {
	var resellerFilter string
	var noStore bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "attention",
		Short: "Fleet rollup: what needs your eyes-on right now (admin attention + dirty repos + DRaaS-in-flight + open issues)",
		Long: `Composes 4 server-side rollups into one ranked per-company view:
  - GET /admin/attention/      (server's own "needs attention" list)
  - GET /admin/dirty-repos/    (restic repos in inconsistent state)
  - GET /admin/draas-in-progress/ (DRaaS sessions in flight)
  - GET /issues/?state=ACTIVE  (open issues)

Each call is also persisted as a snapshot row so 'drift' can compare today
to yesterday. Use --no-store to skip persistence.`,
		Example: `  # Print today's attention rollup as JSON
  servosity-pp-cli attention --json

  # Filter to one reseller's companies
  servosity-pp-cli attention --reseller 12 --json --select results.score,results.company`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				if flags.asJSON {
					_, _ = cmd.OutOrStdout().Write([]byte(`{"meta":{"source":"dry-run"},"results":[]}` + "\n"))
				}
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			rows, totals, err := collectAttention(ctx, c, resellerFilter)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Persist snapshot unless caller opted out.
			runID := ""
			if !noStore {
				if dbPath == "" {
					dbPath = defaultDBPath("servosity-pp-cli")
				}
				st, oerr := store.OpenWithContext(ctx, dbPath)
				if oerr == nil {
					defer st.Close()
					rid, werr := st.WriteAttentionSnapshot(ctx, resellerFilter, rows)
					if werr == nil {
						runID = rid
					}
				}
			}

			out := buildAttentionView(rows, totals, runID)
			outBytes, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), outBytes, flags)
		},
	}
	cmd.Flags().StringVar(&resellerFilter, "reseller", "", "Filter to companies under one reseller ID")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not persist this run as a snapshot")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite path (default: ~/.local/share/servosity-pp-cli/data.db)")
	return cmd
}

// collectAttention fetches all four rollups and produces normalized rows + totals.
func collectAttention(ctx context.Context, c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, reseller string) ([]store.AttentionSnapshotRow, map[string]int, error) {
	totals := map[string]int{}
	rows := []store.AttentionSnapshotRow{}

	// 1) admin attention
	if data, err := c.Get("/admin/attention/", nil); err == nil {
		var items []map[string]json.RawMessage
		if perr := unmarshalAnyList(data, &items); perr == nil {
			for _, it := range items {
				row := mkAttentionRow("admin_attention", it, "needs admin attention", 3)
				if reseller != "" && row.ResellerID != "" && row.ResellerID != reseller {
					continue
				}
				rows = append(rows, row)
			}
			totals["admin_attention"] = len(items)
		}
	} else {
		_ = err
	}

	// 2) dirty repos (restic)
	if data, err := c.Get("/admin/dirty-repos/", nil); err == nil {
		var items []map[string]json.RawMessage
		if perr := unmarshalAnyList(data, &items); perr == nil {
			for _, it := range items {
				row := mkAttentionRow("dirty_repos", it, "restic repo dirty", 2)
				if reseller != "" && row.ResellerID != "" && row.ResellerID != reseller {
					continue
				}
				rows = append(rows, row)
			}
			totals["dirty_repos"] = len(items)
		}
	}

	// 3) DRaaS in progress
	if data, err := c.Get("/admin/draas-in-progress/", nil); err == nil {
		var items []map[string]json.RawMessage
		if perr := unmarshalAnyList(data, &items); perr == nil {
			for _, it := range items {
				row := mkAttentionRow("draas_in_progress", it, "DRaaS in progress", 2)
				if reseller != "" && row.ResellerID != "" && row.ResellerID != reseller {
					continue
				}
				rows = append(rows, row)
			}
			totals["draas_in_progress"] = len(items)
		}
	}

	// 4) open issues
	params := map[string]string{"state": "ACTIVE"}
	if reseller != "" {
		params["reseller"] = reseller
	}
	if data, err := c.Get("/issues/", params); err == nil {
		var items []map[string]json.RawMessage
		if perr := unmarshalPaginated(data, &items); perr == nil {
			for _, it := range items {
				row := mkAttentionRow("open_issues", it, "open issue", 1)
				if reseller != "" && row.ResellerID != "" && row.ResellerID != reseller {
					continue
				}
				rows = append(rows, row)
			}
			totals["open_issues"] = len(items)
		}
	}

	// Roll up by company; pick highest-score row per company for ranking.
	return rollupByCompany(rows), totals, nil
}

func mkAttentionRow(source string, obj map[string]json.RawMessage, reason string, score int) store.AttentionSnapshotRow {
	row := store.AttentionSnapshotRow{Source: source, Reason: reason, Score: score}
	row.CompanyID = strField(obj, "company_id", "company", "id_company")
	row.CompanyName = strField(obj, "company_name", "company__name", "name", "title")
	row.ResellerID = strField(obj, "reseller_id", "reseller", "id_reseller")
	if r, _ := json.Marshal(obj); r != nil {
		row.Raw = r
	}
	return row
}

// strField returns the first non-empty string value from obj for any of `keys`.
// Handles raw JSON values by unmarshalling each.
func strField(obj map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		raw, ok := obj[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			return s
		}
		var n float64
		if err := json.Unmarshal(raw, &n); err == nil {
			return fmt.Sprintf("%g", n)
		}
	}
	return ""
}

func rollupByCompany(rows []store.AttentionSnapshotRow) []store.AttentionSnapshotRow {
	// Keep all rows; but stable-sort by (company, score desc) so the same company's
	// rows cluster together for human reading. JSON consumers can group themselves.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CompanyName != rows[j].CompanyName {
			return rows[i].CompanyName < rows[j].CompanyName
		}
		return rows[i].Score > rows[j].Score
	})
	return rows
}

func buildAttentionView(rows []store.AttentionSnapshotRow, totals map[string]int, runID string) map[string]any {
	type outRow struct {
		Score       int             `json:"score"`
		Source      string          `json:"source"`
		CompanyID   string          `json:"company_id,omitempty"`
		CompanyName string          `json:"company,omitempty"`
		ResellerID  string          `json:"reseller_id,omitempty"`
		Reason      string          `json:"reason"`
		Raw         json.RawMessage `json:"raw,omitempty"`
	}
	out := make([]outRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, outRow{
			Score: r.Score, Source: r.Source, CompanyID: r.CompanyID,
			CompanyName: r.CompanyName, ResellerID: r.ResellerID,
			Reason: r.Reason, Raw: r.Raw,
		})
	}
	return map[string]any{
		"meta": map[string]any{
			"source":      "live",
			"captured_at": time.Now().UTC().Format(time.RFC3339),
			"run_id":      runID,
			"totals":      totals,
		},
		"results": out,
	}
}

// unmarshalAnyList accepts either a JSON array or a paginated `{results: [...]}` envelope.
func unmarshalAnyList(data json.RawMessage, dst *[]map[string]json.RawMessage) error {
	trimmed := bytes_trimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return json.Unmarshal(data, dst)
	}
	return unmarshalPaginated(data, dst)
}

func unmarshalPaginated(data json.RawMessage, dst *[]map[string]json.RawMessage) error {
	var env struct {
		Results []map[string]json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &env); err == nil && env.Results != nil {
		*dst = env.Results
		return nil
	}
	// Fall back to direct list parse — some endpoints return raw arrays.
	if err := json.Unmarshal(data, dst); err == nil {
		return nil
	}
	return fmt.Errorf("response is neither a list nor a paginated envelope")
}

func bytes_trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\r' || b[0] == '\n') {
		b = b[1:]
	}
	return b
}

// strJoin is a defensive small helper used in human output.
func strJoin(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

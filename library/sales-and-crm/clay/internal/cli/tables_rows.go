// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: tables rows.
//
// Clay lists rows in two steps, which is why a single endpoint never worked:
//   1. GET  /tables/{t}/views/{v}/records/ids   -> ordered record ids for the view
//   2. POST /tables/{t}/bulk-fetch-records      -> cell values for a batch of ids
// This command chains them and resolves f_ field ids back to real column names.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clay/internal/cliutil"
	"github.com/spf13/cobra"
)

type rowCell struct {
	Value  any    `json:"value,omitempty"`
	Status string `json:"status,omitempty"`
}

type tableRow struct {
	ID    string             `json:"id"`
	Cells map[string]rowCell `json:"cells"`
}

type rowsView struct {
	TableID     string     `json:"tableId"`
	ViewID      string     `json:"viewId"`
	TotalRows   int        `json:"totalRows"`
	ScannedRows int        `json:"scannedRows"`
	MaxBatches  int        `json:"maxScanBatches"`
	Rows        []tableRow `json:"rows"`
	Note        string     `json:"note,omitempty"`
}

func newNovelTablesRowsCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace, flagView, flagStatus string
	var flagLimit, flagBatch, flagMaxBatches int
	var flagRaw bool

	cmd := &cobra.Command{
		Use:   "rows <tableId>",
		Short: "Read table rows with column names resolved, optionally filtered by cell run status.",
		Long: "Chains Clay's two-step row read (record ids for the view, then bulk cell fetch) and maps\n" +
			"generated field ids back to column names. Use --status to keep only rows containing a cell\n" +
			"in that run state, for example ERROR.\n" +
			"Do NOT use it to change data; it is read-only.",
		Example: "  clay-pp-cli tables rows t_abc123 --workspace 1234567 --limit 20 --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "<tableId>=t_example;--workspace=1;--dry-run",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tables rows")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<tableId> is required"))
			}
			ws, err := resolveWorkspace(flagWorkspace)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tbl, err := fetchTable(ctx, c, ws, args[0])
			if err != nil {
				return err
			}
			view := flagView
			if view == "" {
				view = tbl.FirstViewID
			}
			if view == "" {
				return usageErr(fmt.Errorf("no view id available; pass --view"))
			}
			byID := indexByID(tbl.Fields)

			// Step 1: ordered record ids for this view.
			idsRaw, err := c.Get(ctx,
				fmt.Sprintf("/workspaces/%s/tables/%s/views/%s/records/ids", ws, tbl.ID, view), nil)
			if err != nil {
				return fmt.Errorf("listing record ids: %w", err)
			}
			var idsResp struct {
				Results []string `json:"results"`
			}
			if err := json.Unmarshal(idsRaw, &idsResp); err != nil {
				return fmt.Errorf("parsing record ids: %w", err)
			}

			maxBatches := flagMaxBatches
			batch := flagBatch
			if cliutil.IsDogfoodEnv() {
				maxBatches, batch = 1, 5
			}
			out := rowsView{
				TableID: tbl.ID, ViewID: view,
				TotalRows: len(idsResp.Results), MaxBatches: maxBatches,
				Rows: make([]tableRow, 0),
			}
			want := strings.ToUpper(strings.TrimSpace(flagStatus))

			// Step 2: page through ids, fetching cell values per batch.
			batchCapHit := false
			for b := 0; b*batch < len(idsResp.Results); b++ {
				if b >= maxBatches {
					batchCapHit = true
					break
				}
				if len(out.Rows) >= flagLimit {
					break
				}
				end := (b + 1) * batch
				if end > len(idsResp.Results) {
					end = len(idsResp.Results)
				}
				chunk := idsResp.Results[b*batch : end]
				raw, _, pErr := c.Post(ctx,
					fmt.Sprintf("/workspaces/%s/tables/%s/bulk-fetch-records", ws, tbl.ID),
					map[string]any{"recordIds": chunk})
				if pErr != nil {
					return fmt.Errorf("fetching records batch %d: %w", b+1, pErr)
				}
				var recs struct {
					Results []struct {
						ID    string `json:"id"`
						Cells map[string]struct {
							Value    any `json:"value"`
							Metadata struct {
								Status string `json:"status"`
							} `json:"metadata"`
						} `json:"cells"`
					} `json:"results"`
				}
				if err := json.Unmarshal(raw, &recs); err != nil {
					return fmt.Errorf("parsing records batch %d: %w", b+1, err)
				}
				for _, r := range recs.Results {
					out.ScannedRows++
					row := tableRow{ID: r.ID, Cells: map[string]rowCell{}}
					matched := want == ""
					for fid, cell := range r.Cells {
						name := fid
						if f, ok := byID[fid]; ok && f.Name != "" && !flagRaw {
							name = f.Name
						}
						st := cell.Metadata.Status
						if want != "" && strings.EqualFold(st, want) {
							matched = true
						}
						row.Cells[name] = rowCell{Value: cell.Value, Status: st}
					}
					if !matched {
						continue
					}
					out.Rows = append(out.Rows, row)
					if len(out.Rows) >= flagLimit {
						break
					}
				}
			}
			if len(out.Rows) == 0 && batchCapHit {
				out.Note = fmt.Sprintf(
					"scanned %d of %d rows across %d batch(es) without a match; raise --max-scan-batches to widen the search",
					out.ScannedRows, out.TotalRows, maxBatches)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out.Rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no matching rows (scanned %d of %d)\n", out.ScannedRows, out.TotalRows)
				return nil
			}
			for _, r := range out.Rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", r.ID)
				for name, cell := range r.Cells {
					if want != "" && !strings.EqualFold(cell.Status, want) {
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "    %-34s %-28v %s\n", name, cell.Value, cell.Status)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d row(s), scanned %d of %d\n", len(out.Rows), out.ScannedRows, out.TotalRows)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	cmd.Flags().StringVar(&flagView, "view", "", "View id (gv_...); defaults to the table's first view")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Keep only rows with a cell in this run status, e.g. ERROR")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum rows to return")
	cmd.Flags().IntVar(&flagBatch, "batch", 25, "Record ids fetched per bulk call")
	cmd.Flags().IntVar(&flagMaxBatches, "max-scan-batches", 8, "Maximum bulk batches to scan before returning partial results")
	cmd.Flags().BoolVar(&flagRaw, "raw-field-ids", false, "Keep generated f_ field ids instead of column names")
	return cmd
}

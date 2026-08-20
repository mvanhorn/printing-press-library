// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #9 — multi-database country fan-out for domain overview.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newDomainRegionsCmd(flags *rootFlags) *cobra.Command {
	var databases string
	var exportColumns string

	cmd := &cobra.Command{
		Use:         "regions [domain]",
		Short:       "Run Domain Overview across multiple country databases and persist each row.",
		Long:        "regions calls Semrush's type=domain_rank endpoint once per --databases entry and emits a side-by-side report with one row per database. Each result is persisted to the local store so later 'drift' queries can compare cross-region.",
		Example:     "  semrush-pp-cli domain regions apple.com --databases us,uk,de,fr,it",
		Annotations: map[string]string{"pp:endpoint": "domain.regions", "pp:method": "GET", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			domain := args[0]
			dbList := strings.Split(databases, ",")
			cleaned := dbList[:0]
			for _, d := range dbList {
				if s := strings.TrimSpace(d); s != "" {
					cleaned = append(cleaned, s)
				}
			}
			if len(cleaned) == 0 {
				return usageErr(fmt.Errorf("--databases is required (e.g. us,uk,de)"))
			}

			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			recordBalanceSnapshotForCmd(ctx, db, flags, cmd.CommandPath(), cmd.ErrOrStderr())

			// Parse the CSV response into structured rows for each database.
			// The Semrush v3 Analytics API returns semicolon-delimited CSV with a
			// header line and one or more data lines. We turn each line into a
			// map[string]any keyed by column name so the result is agent-readable
			// JSON rather than an opaque CSV blob.
			type regionResult struct {
				Database string           `json:"database"`
				Rows     []map[string]any `json:"rows,omitempty"`
				Raw      string           `json:"raw,omitempty"`
				Error    string           `json:"error,omitempty"`
			}
			var results []regionResult

			for _, dbCode := range cleaned {
				if err := ctx.Err(); err != nil {
					return err
				}
				params := map[string]string{
					"type":           "domain_rank",
					"domain":         domain,
					"database":       dbCode,
					"export_columns": exportColumns,
				}
				data, err := c.Get(ctx, "/", params)
				rr := regionResult{Database: dbCode}
				if err != nil {
					rr.Error = err.Error()
					results = append(results, rr)
					continue
				}
				raw := strings.TrimSpace(string(data))
				rr.Raw = raw
				rr.Rows = parseSemrushCSV(raw)
				results = append(results, rr)

				// Persist each parsed row to the resources table keyed by (domain, database).
				rowJSON, _ := json.Marshal(rr.Rows)
				id := fmt.Sprintf("%s|%s", domain, dbCode)
				_, _ = db.DB().ExecContext(ctx,
					`INSERT INTO resources (id, resource_type, data, synced_at, updated_at)
					 VALUES (?, 'domain_regions', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
					 ON CONFLICT(resource_type, id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, updated_at = excluded.updated_at`,
					id, string(rowJSON))
			}

			out := map[string]any{
				"domain":         domain,
				"databases":      cleaned,
				"export_columns": exportColumns,
				"results":        results,
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&databases, "databases", "us,uk,de,fr,it", "Comma-separated Semrush database/country codes to fan out across")
	cmd.Flags().StringVar(&exportColumns, "export-columns", "Dn,Rk,Or,Ot,Oc,Ad,At,Ac", "Columns to request from the Semrush domain_rank endpoint")
	return cmd
}

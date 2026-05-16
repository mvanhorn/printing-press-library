// Copyright 2026 dan-bronson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/harvest/internal/store"
)

type clientMarginRow struct {
	ClientID       int64   `json:"client_id"`
	Client         string  `json:"client"`
	HoursTotal     float64 `json:"hours_total"`
	HoursBillable  float64 `json:"hours_billable"`
	Revenue        float64 `json:"revenue"`
	Cost           float64 `json:"cost"`
	Margin         float64 `json:"margin"`
	RealizationPct float64 `json:"realization_pct"`
	Entries        int     `json:"entries"`
}

func newClientCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Client-level local insights (margin, realization)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newClientMarginCmd(flags))
	return cmd
}

func newClientMarginCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath     string
		clientName string
		from       string
		to         string
	)

	cmd := &cobra.Command{
		Use:   "margin",
		Short: "Per-client revenue, cost, margin, and realization %",
		Long: `Joins time_entries with their billable_rate and cost_rate fields to compute:
  - revenue        = sum(hours * billable_rate) where billable=true
  - cost           = sum(hours * cost_rate)
  - margin         = revenue - cost
  - realization_pct = revenue / (hours * standard_rate) * 100, approximated as revenue / cost when standard rate is missing

LOCAL command: requires 'sync' with time-entries and clients populated. Rates
come from the entry's billable_rate and cost_rate (snapshotted at entry time);
no historical rate joins required.`,
		Example: `  # Single client margin for April
  harvest-pp-cli client margin --client "Acme Corp" --from 2026-04-01 --to 2026-04-30 --json

  # All clients YTD
  harvest-pp-cli client margin --from 2026-01-01 --to 2026-12-31 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return flags.printJSON(cmd, []clientMarginRow{})
			}
			if dbPath == "" {
				dbPath = defaultDBPath("harvest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'harvest-pp-cli sync' first.", err)
			}
			defer db.Close()

			rows, err := computeClientMargin(cmd, db, clientName, from, to)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, rows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&clientName, "client", "", "Filter by client name (case-insensitive contains)")
	cmd.Flags().StringVar(&from, "from", "", "Earliest spent_date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "Latest spent_date (YYYY-MM-DD)")
	return cmd
}

func computeClientMargin(cmd *cobra.Command, db *store.Store, clientFilter, from, to string) ([]clientMarginRow, error) {
	sql := `SELECT
	  IFNULL(CAST(json_extract(data, '$.client.id') AS INTEGER), 0) AS cid,
	  IFNULL(json_extract(data, '$.client.name'), '') AS cname,
	  IFNULL(json_extract(data, '$.hours'), 0) AS h,
	  IFNULL(json_extract(data, '$.billable'), 0) AS billable,
	  IFNULL(json_extract(data, '$.billable_rate'), 0) AS brate,
	  IFNULL(json_extract(data, '$.cost_rate'), 0) AS crate
	FROM time_entries
	WHERE 1=1`
	params := []any{}
	if from != "" {
		sql += ` AND json_extract(data, '$.spent_date') >= ?`
		params = append(params, from)
	}
	if to != "" {
		sql += ` AND json_extract(data, '$.spent_date') <= ?`
		params = append(params, to)
	}
	rows, err := db.DB().QueryContext(cmd.Context(), sql, params...)
	if err != nil {
		return nil, fmt.Errorf("query time_entries: %w", err)
	}
	defer rows.Close()

	agg := map[int64]*clientMarginRow{}
	for rows.Next() {
		var cid int64
		var cname string
		var h, brate, crate float64
		var billable int64
		if err := rows.Scan(&cid, &cname, &h, &billable, &brate, &crate); err != nil {
			return nil, err
		}
		if cid == 0 && cname == "" {
			continue
		}
		if clientFilter != "" && !stringContainsFold(cname, clientFilter) {
			continue
		}
		r := agg[cid]
		if r == nil {
			r = &clientMarginRow{ClientID: cid, Client: cname}
			agg[cid] = r
		}
		r.HoursTotal += h
		r.Entries++
		if billable != 0 {
			r.HoursBillable += h
			r.Revenue += h * brate
		}
		r.Cost += h * crate
	}

	out := make([]clientMarginRow, 0)
	for _, r := range agg {
		r.Margin = r.Revenue - r.Cost
		if r.Cost > 0 {
			r.RealizationPct = math.Round((r.Revenue/r.Cost)*1000) / 10
		}
		// Round monetary to cents
		r.Revenue = math.Round(r.Revenue*100) / 100
		r.Cost = math.Round(r.Cost*100) / 100
		r.Margin = math.Round(r.Margin*100) / 100
		r.HoursTotal = math.Round(r.HoursTotal*100) / 100
		r.HoursBillable = math.Round(r.HoursBillable*100) / 100
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Margin > out[j].Margin })
	return out, nil
}

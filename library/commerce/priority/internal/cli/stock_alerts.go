// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written stock triage: parts whose known on-hand balance is below the
// minimum stock level, or whose open demand exceeds known on-hand. Stock
// balance fields vary per tenant; when synced part data carries none, the
// command says so honestly instead of fabricating zeros.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// pp:data-source local

type stockAlertRow struct {
	PartName   string   `json:"part"`
	PartDes    string   `json:"description,omitempty"`
	OnHand     *float64 `json:"on_hand,omitempty"`
	MinLevel   *float64 `json:"min_level,omitempty"`
	OpenDemand float64  `json:"open_demand"`
	Reason     string   `json:"reason"`
}

type stockAlertsView struct {
	Rows         []stockAlertRow `json:"rows"`
	ScannedParts int             `json:"scanned_parts"`
	Note         string          `json:"note,omitempty"`
}

func newStockCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "stock",
		Short:       "Stock triage from the local mirror",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newStockAlertsCmd(flags))
	return cmd
}

func newStockAlertsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Parts below minimum stock or with open demand exceeding known on-hand",
		Long: strings.Trim(`
Use this command for stock-state triage: on-hand below minimum, or open demand
above known on-hand. For the order-driven shortfall ranking use 'shortage'.

Reads balance-like fields (BALANCE/TBALANCE) and minimum-level fields
(MINQUANT/MINBALANCE/MINSTOCK) from synced part JSON when the tenant exposes
them; falls back to demand-vs-on-hand when only balances exist. When neither
is present in synced data, reports that honestly. Tenants exposing WARHSBAL
can inspect raw warehouse balances via 'entity list WARHSBAL'.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli stock alerts
  priority-pp-cli stock alerts --limit 10 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan synced parts for stock alerts")
				return nil
			}
			if limit < 0 {
				limit = 0
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: priority-pp-cli sync --resources parts,orders --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "parts") {
				hintIfStale(cmd, db, "parts", flags.maxAge)
			}

			// Open demand per part from orders-embedded lines (drain first).
			demandRows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(ordstatusdes,''), data FROM "orders"`)
			if err != nil {
				return err
			}
			type oRow struct {
				status string
				data   []byte
			}
			var oRows []oRow
			for demandRows.Next() {
				var r oRow
				if err := demandRows.Scan(&r.status, &r.data); err != nil {
					_ = demandRows.Close()
					return err
				}
				oRows = append(oRows, r)
			}
			if err := demandRows.Err(); err != nil {
				_ = demandRows.Close()
				return err
			}
			_ = demandRows.Close()
			demand := map[string]float64{}
			for _, r := range oRows {
				if closedStatuses[strings.ToLower(r.status)] {
					continue
				}
				var obj map[string]json.RawMessage
				if json.Unmarshal(r.data, &obj) != nil {
					continue
				}
				sub, ok := obj["ORDERITEMS_SUBFORM"]
				if !ok {
					continue
				}
				var lines []map[string]json.RawMessage
				if json.Unmarshal(sub, &lines) != nil {
					continue
				}
				for _, ln := range lines {
					part := jsonStrField(ln, "PARTNAME")
					if part == "" {
						continue
					}
					q, okQ := jsonNumField(ln, "ABALANCE")
					if !okQ {
						q, _ = jsonNumField(ln, "TQUANT")
					}
					if q > 0 {
						demand[part] += q
					}
				}
			}

			partRows, err := db.DB().QueryContext(ctx, `SELECT COALESCE(partname,''), COALESCE(partdes,''), data FROM "parts"`)
			if err != nil {
				return err
			}
			type pRow struct {
				name, des string
				data      []byte
			}
			var parts []pRow
			for partRows.Next() {
				var pr pRow
				if err := partRows.Scan(&pr.name, &pr.des, &pr.data); err != nil {
					_ = partRows.Close()
					return err
				}
				parts = append(parts, pr)
			}
			if err := partRows.Err(); err != nil {
				_ = partRows.Close()
				return err
			}
			_ = partRows.Close()

			view := stockAlertsView{ScannedParts: len(parts)}
			balanceSeen := false
			for _, pr := range parts {
				if pr.name == "" {
					continue
				}
				var obj map[string]json.RawMessage
				if json.Unmarshal(pr.data, &obj) != nil {
					continue
				}
				var onHand, minLevel *float64
				for _, key := range []string{"BALANCE", "TBALANCE", "STOCKBAL"} {
					if v, ok := jsonNumField(obj, key); ok {
						vv := v
						onHand = &vv
						balanceSeen = true
						break
					}
				}
				for _, key := range []string{"MINQUANT", "MINBALANCE", "MINSTOCK", "MINQTY"} {
					if v, ok := jsonNumField(obj, key); ok && v > 0 {
						vv := v
						minLevel = &vv
						break
					}
				}
				row := stockAlertRow{PartName: pr.name, PartDes: pr.des, OnHand: onHand, MinLevel: minLevel, OpenDemand: demand[pr.name]}
				switch {
				case onHand != nil && minLevel != nil && *onHand < *minLevel:
					row.Reason = "below-minimum"
				case onHand != nil && row.OpenDemand > *onHand:
					row.Reason = "demand-exceeds-on-hand"
				default:
					continue
				}
				view.Rows = append(view.Rows, row)
			}
			sort.Slice(view.Rows, func(i, j int) bool { return view.Rows[i].OpenDemand > view.Rows[j].OpenDemand })
			if len(view.Rows) > limit {
				view.Rows = view.Rows[:limit]
			}
			if view.Rows == nil {
				view.Rows = []stockAlertRow{}
				if !balanceSeen {
					view.Note = fmt.Sprintf("synced part data (%d parts) carries no stock balance fields on this tenant, so no alerts can be computed; use 'shortage' for demand-based triage or 'entity list WARHSBAL' where the tenant exposes it", view.ScannedParts)
				} else {
					view.Note = fmt.Sprintf("no alerts across %d scanned parts", view.ScannedParts)
				}
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "PART\tDESCRIPTION\tON HAND\tMIN\tOPEN DEMAND\tREASON")
			for _, r := range view.Rows {
				onHand, minL := "-", "-"
				if r.OnHand != nil {
					onHand = fmt.Sprintf("%.1f", *r.OnHand)
				}
				if r.MinLevel != nil {
					minL = fmt.Sprintf("%.1f", *r.MinLevel)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.1f\t%s\n", r.PartName, truncate(r.PartDes, 30), onHand, minL, r.OpenDemand, r.Reason)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum parts to return")
	return cmd
}

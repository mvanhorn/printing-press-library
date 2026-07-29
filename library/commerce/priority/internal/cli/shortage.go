// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: shortage — open-order demand vs stock coverage from the local
// mirror. The three-way join (order lines, parts, purchase orders) exists in
// no Priority screen and no single OData call.

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

type shortageRow struct {
	PartName    string   `json:"part"`
	PartDes     string   `json:"description,omitempty"`
	OpenDemand  float64  `json:"open_demand"`
	OpenLines   int      `json:"open_lines"`
	OnHand      *float64 `json:"on_hand,omitempty"`
	Inbound     *float64 `json:"inbound,omitempty"`
	Shortfall   *float64 `json:"shortfall,omitempty"`
	OrdersShown []string `json:"orders,omitempty"`
}

type shortageView struct {
	Rows         []shortageRow `json:"rows"`
	ScannedLines int           `json:"scanned_order_lines"`
	Note         string        `json:"note,omitempty"`
	DataSources  []string      `json:"data_sources"`
}

// closedStatuses marks order statuses excluded from open demand.
var closedStatuses = map[string]bool{
	"canceled": true, "cancelled": true, "closed": true, "done": true, "בוטל": true, "סגור": true,
}

func newNovelShortageCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var includeInbound bool
	cmd := &cobra.Command{
		Use:   "shortage",
		Short: "See which parts will run out given the open order book — open demand vs stock, optionally netting inbound POs",
		Long: strings.Trim(`
Use this command for open-order demand vs on-hand shortfall.
Do NOT use it to verify local mirror freshness; use 'reconcile' instead.

Computes per-part open sales demand from order lines embedded in synced
orders (excluding
canceled/closed orders; populate them by syncing orders with
--resource-param "orders:$expand=ORDERITEMS_SUBFORM"), joins the part catalog for descriptions and any
balance fields present in the synced part JSON, and — with --include-inbound —
nets inbound quantities from purchase-order subform lines when the porders
sync included $expand=PORDERITEMS_SUBFORM. On-hand shows null when the tenant
data synced so far carries no stock balance fields.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli shortage --agent
  priority-pp-cli shortage --include-inbound --limit 10
  priority-pp-cli sync --resources orders,parts --resource-param "orders:$expand=ORDERITEMS_SUBFORM" && priority-pp-cli shortage`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute open-order demand vs stock coverage from the local mirror")
				return nil
			}
			if limit < 0 {
				limit = 0
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: priority-pp-cli sync --resources orders,parts --resource-param \"orders:$expand=ORDERITEMS_SUBFORM\" --db %s\n", dbPath, dbPath)
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
			if !hintIfUnsynced(cmd, db, "orders") {
				hintIfStale(cmd, db, "orders", flags.maxAge)
			}

			view := shortageView{DataSources: []string{"orders ORDERITEMS_SUBFORM (synced, when expanded)", "parts (synced)"}}

			// Order lines live embedded in synced orders JSON (populated when sync
			// runs with --resource-param "orders:$expand=ORDERITEMS_SUBFORM").
			// Drain first (single SQLite connection).
			rows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(ordname,''), COALESCE(ordstatusdes,''), data FROM "orders"`)
			if err != nil {
				return fmt.Errorf("querying orders: %w", err)
			}
			type orderRow struct {
				ord, status string
				data        []byte
			}
			var orderRows []orderRow
			for rows.Next() {
				var or orderRow
				if err := rows.Scan(&or.ord, &or.status, &or.data); err != nil {
					_ = rows.Close()
					return err
				}
				orderRows = append(orderRows, or)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			_ = rows.Close()

			type agg struct {
				demand float64
				lines  int
				orders map[string]bool
			}
			demand := map[string]*agg{}
			linesSeen := false
			for _, or := range orderRows {
				if closedStatuses[strings.ToLower(or.status)] {
					continue
				}
				var obj map[string]json.RawMessage
				if json.Unmarshal(or.data, &obj) != nil {
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
				linesSeen = true
				for _, ln := range lines {
					view.ScannedLines++
					part := jsonStrField(ln, "PARTNAME")
					if part == "" {
						continue
					}
					qty, okQ := jsonNumField(ln, "ABALANCE")
					if !okQ {
						qty, _ = jsonNumField(ln, "TQUANT")
					}
					if qty <= 0 {
						continue
					}
					a := demand[part]
					if a == nil {
						a = &agg{orders: map[string]bool{}}
						demand[part] = a
					}
					a.demand += qty
					a.lines++
					if or.ord != "" && len(a.orders) < 5 {
						a.orders[or.ord] = true
					}
				}
			}

			// Part catalog: description + any balance-like field in the raw JSON.
			partRows, err := db.DB().QueryContext(ctx, `SELECT COALESCE(partname,''), COALESCE(partdes,''), data FROM "parts"`)
			if err != nil {
				return fmt.Errorf("querying parts: %w", err)
			}
			partDes := map[string]string{}
			partOnHand := map[string]*float64{}
			{
				type pRow struct {
					name, des string
					data      []byte
				}
				var pRowsList []pRow
				for partRows.Next() {
					var pr pRow
					if err := partRows.Scan(&pr.name, &pr.des, &pr.data); err != nil {
						_ = partRows.Close()
						return err
					}
					pRowsList = append(pRowsList, pr)
				}
				if err := partRows.Err(); err != nil {
					_ = partRows.Close()
					return err
				}
				_ = partRows.Close()
				for _, pr := range pRowsList {
					partDes[pr.name] = pr.des
					var obj map[string]json.RawMessage
					if json.Unmarshal(pr.data, &obj) == nil {
						for _, key := range []string{"BALANCE", "TBALANCE", "STOCKBAL", "WARHSBAL"} {
							if v, ok := jsonNumField(obj, key); ok {
								vv := v
								partOnHand[pr.name] = &vv
								break
							}
						}
					}
				}
			}

			// Inbound: PO subform lines embedded in synced porders JSON (present
			// when sync ran with --resource-param "porders:$expand=PORDERITEMS_SUBFORM").
			inbound := map[string]float64{}
			inboundSeen := false
			if includeInbound {
				view.DataSources = append(view.DataSources, "porders PORDERITEMS_SUBFORM (synced, when expanded)")
				poRows, err := db.DB().QueryContext(ctx, `SELECT data FROM "porders"`)
				if err != nil {
					return fmt.Errorf("querying porders: %w", err)
				}
				{
					var blobs [][]byte
					for poRows.Next() {
						var b []byte
						if err := poRows.Scan(&b); err != nil {
							_ = poRows.Close()
							return err
						}
						blobs = append(blobs, b)
					}
					if err := poRows.Err(); err != nil {
						_ = poRows.Close()
						return err
					}
					_ = poRows.Close()
					for _, b := range blobs {
						var obj map[string]json.RawMessage
						if json.Unmarshal(b, &obj) != nil {
							continue
						}
						sub, ok := obj["PORDERITEMS_SUBFORM"]
						if !ok {
							continue
						}
						var poLines []map[string]json.RawMessage
						if json.Unmarshal(sub, &poLines) != nil {
							continue
						}
						inboundSeen = true
						for _, pl := range poLines {
							part := jsonStrField(pl, "PARTNAME")
							if part == "" {
								continue
							}
							if q, ok := jsonNumField(pl, "TQUANT"); ok && q > 0 {
								inbound[part] += q
							}
						}
					}
				}
			}

			for part, a := range demand {
				row := shortageRow{PartName: part, PartDes: partDes[part], OpenDemand: a.demand, OpenLines: a.lines}
				for o := range a.orders {
					row.OrdersShown = append(row.OrdersShown, o)
				}
				sort.Strings(row.OrdersShown)
				if oh, ok := partOnHand[part]; ok {
					row.OnHand = oh
				}
				if includeInbound {
					if q, ok := inbound[part]; ok {
						qq := q
						row.Inbound = &qq
					}
				}
				if row.OnHand != nil {
					shortfall := row.OpenDemand - *row.OnHand
					if row.Inbound != nil {
						shortfall -= *row.Inbound
					}
					row.Shortfall = &shortfall
				}
				view.Rows = append(view.Rows, row)
			}
			sort.Slice(view.Rows, func(i, j int) bool {
				si, sj := view.Rows[i], view.Rows[j]
				// Known shortfalls first (largest), then by open demand.
				if (si.Shortfall != nil) != (sj.Shortfall != nil) {
					return si.Shortfall != nil
				}
				if si.Shortfall != nil && sj.Shortfall != nil && *si.Shortfall != *sj.Shortfall {
					return *si.Shortfall > *sj.Shortfall
				}
				return si.OpenDemand > sj.OpenDemand
			})
			if len(view.Rows) > limit {
				view.Rows = view.Rows[:limit]
			}
			if view.Rows == nil {
				view.Rows = []shortageRow{}
				if !linesSeen {
					view.Note = "no embedded order lines in synced orders; re-sync with: priority-pp-cli sync --resources orders,parts --resource-param \"orders:$expand=ORDERITEMS_SUBFORM\""
				} else {
					view.Note = fmt.Sprintf("no open demand found across %d synced order lines", view.ScannedLines)
				}
			}
			if len(partOnHand) == 0 && len(view.Rows) > 0 {
				view.Note = "on-hand is null: synced part data carries no stock balance fields on this tenant (rows rank by open demand); tenants exposing WARHSBAL can inspect stock via 'entity list WARHSBAL'"
			}
			if includeInbound && !inboundSeen && len(view.Rows) > 0 {
				view.Note = strings.TrimSpace(view.Note + " | inbound is null: no PORDERITEMS_SUBFORM data in synced porders; re-sync with --resource-param \"porders:$expand=PORDERITEMS_SUBFORM\"")
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "PART\tDESCRIPTION\tOPEN DEMAND\tLINES\tON HAND\tINBOUND\tSHORTFALL")
			for _, r := range view.Rows {
				onHand, inb, sf := "-", "-", "-"
				if r.OnHand != nil {
					onHand = fmt.Sprintf("%.1f", *r.OnHand)
				}
				if r.Inbound != nil {
					inb = fmt.Sprintf("%.1f", *r.Inbound)
				}
				if r.Shortfall != nil {
					sf = fmt.Sprintf("%.1f", *r.Shortfall)
				}
				fmt.Fprintf(w, "%s\t%s\t%.1f\t%d\t%s\t%s\t%s\n", r.PartName, truncate(r.PartDes, 30), r.OpenDemand, r.OpenLines, onHand, inb, sf)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum parts to return")
	cmd.Flags().BoolVar(&includeInbound, "include-inbound", false, "net inbound PO quantities from synced porders subform lines")
	return cmd
}

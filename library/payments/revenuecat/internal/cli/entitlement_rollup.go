// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
	"github.com/spf13/cobra"
)

type entitlementRollupRow struct {
	EntitlementID   string `json:"entitlement_id"`
	DisplayName     string `json:"display_name,omitempty"`
	State           string `json:"state,omitempty"`
	ActiveCustomers int    `json:"active_customers"`
	Disagreements   int    `json:"disagreements,omitempty"`
}

type entitlementRollupView struct {
	ProjectID        string                 `json:"project_id"`
	Entitlements     []entitlementRollupRow `json:"entitlements"`
	TotalEntitlement int                    `json:"total_entitlements"`
	TotalActive      int                    `json:"total_active_grants"`
	FlagDisagree     bool                   `json:"flag_disagreements"`
	Note             string                 `json:"note,omitempty"`
}

// nonAccessSubStates are subscription statuses that should NOT grant access.
// A customer holding an active entitlement while every one of their
// subscriptions is in one of these states is a disagreement.
var nonAccessSubStates = map[string]bool{
	"expired":          true,
	"in_billing_retry": true,
	"paused":           true,
	"unknown":          true,
	"incomplete":       true,
}

func newNovelEntitlementRollupCmd(flags *rootFlags) *cobra.Command {
	var projectFlag string
	var dbPath string
	var flagDisagreements bool
	cmd := &cobra.Command{
		Use:   "entitlement-rollup",
		Short: "Per-entitlement active-customer counts, with optional entitlement/subscription disagreement flags",
		Long: `Three-way local join of project 'entitlements' × per-customer
'active_entitlements' × 'subscriptions' status:

  - active_customers: distinct customers with a non-expired grant per entitlement
  - with --flag-disagreements: counts customers who hold an active entitlement
    while every one of their subscriptions is in a non-access state
    (expired / in_billing_retry / paused / unknown / incomplete) — a likely
    entitlement/subscription drift worth investigating.

Data source: local. Run 'sync' for entitlements, customer active_entitlements,
and subscriptions first.`,
		Example: "  revenuecat-pp-cli entitlement-rollup --project proj1ab2c3d4 --flag-disagreements --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would roll up active-customer counts per entitlement")
				return nil
			}
			projectID, err := resolveProjectID(projectFlag)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("revenuecat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "entitlements",
				[]string{"active_entitlements", "subscriptions"}, flags.maxAge)

			view, err := buildEntitlementRollup(db, projectID, flagDisagreements)
			if err != nil {
				return err
			}
			return emitEntitlementRollup(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "RevenueCat project id (or set REVENUECAT_PROJECT_ID)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	cmd.Flags().BoolVar(&flagDisagreements, "flag-disagreements", false, "Count customers whose active entitlement disagrees with their subscription states")
	return cmd
}

func emitEntitlementRollup(cmd *cobra.Command, flags *rootFlags, view entitlementRollupView) error {
	if len(view.Entitlements) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Entitlements))
		for _, e := range view.Entitlements {
			row := map[string]any{
				"entitlement_id":   e.EntitlementID,
				"display_name":     e.DisplayName,
				"state":            e.State,
				"active_customers": e.ActiveCustomers,
			}
			if view.FlagDisagree {
				row["disagreements"] = e.Disagreements
			}
			items = append(items, row)
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d entitlement(s)  %d active grant(s)\n",
			view.TotalEntitlement, view.TotalActive)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// buildEntitlementRollup performs the three-way join.
//
// TODO(verify): confirm active_entitlements expires_at semantics and that a
// customer's subscription set fully determines entitlement access against live
// data.
func buildEntitlementRollup(db *store.Store, projectID string, flagDisagreements bool) (entitlementRollupView, error) {
	view := entitlementRollupView{
		ProjectID:    projectID,
		Entitlements: []entitlementRollupRow{},
		FlagDisagree: flagDisagreements,
	}
	now := time.Now().UTC()

	// 1. Project entitlements (id → display_name, state).
	type entMeta struct {
		name  string
		state string
	}
	entitlements := map[string]entMeta{}
	loadResourceRowsRC(db, []string{"entitlements"}, loadEntitlementNamesCap, "entitlementRollupEntitlements", func(obj map[string]any) {
		id := toStringRC(obj["id"])
		if id == "" {
			return
		}
		m := entMeta{}
		if n, ok := obj["display_name"].(string); ok {
			m.name = n
		}
		if s, ok := obj["state"].(string); ok {
			m.state = s
		}
		entitlements[id] = m
	})

	// 2. Subscription states grouped per customer (for disagreement detection).
	custSubStates := map[string][]string{}
	if flagDisagreements {
		loadResourceRowsRC(db, []string{"subscriptions", "customers_subscriptions"}, loadSubscriptionStatusCap, "entitlementRollupSubs", func(obj map[string]any) {
			cid := toStringRC(obj["customer_id"])
			status, _ := obj["status"].(string)
			if cid == "" || status == "" {
				return
			}
			custSubStates[cid] = append(custSubStates[cid], status)
		})
	}

	// 3. Active entitlements per customer; count distinct active customers per
	// entitlement and (optionally) disagreements.
	activeCustomers := map[string]map[string]bool{}
	disagreements := map[string]map[string]bool{}

	rows, err := db.Query(`SELECT customers_id, data FROM "active_entitlements" LIMIT ?`, loadActiveEntsCap)
	if err != nil {
		return view, fmt.Errorf("querying active_entitlements: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var custID sql.NullString
		var data sql.NullString
		if rows.Scan(&custID, &data) != nil || !data.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data.String), &obj) != nil {
			continue
		}
		entID := toStringRC(obj["entitlement_id"])
		if entID == "" {
			continue
		}
		// Skip expired grants (expires_at in the past); null/zero = no expiry.
		exp := rcEpochMSToTime(obj["expires_at"])
		if !exp.IsZero() && exp.Before(now) {
			continue
		}
		cid := custID.String
		if cid == "" {
			cid = toStringRC(obj["customer_id"])
		}
		// Skip rows with no resolvable customer id: an empty key would count
		// an anonymous grant as one distinct active customer and collapse all
		// such rows into a single phantom customer.
		if cid == "" {
			continue
		}
		if activeCustomers[entID] == nil {
			activeCustomers[entID] = map[string]bool{}
		}
		activeCustomers[entID][cid] = true

		if flagDisagreements && cid != "" {
			states := custSubStates[cid]
			if subscriptionsAllNonAccess(states) {
				if disagreements[entID] == nil {
					disagreements[entID] = map[string]bool{}
				}
				disagreements[entID][cid] = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return view, fmt.Errorf("iterating active_entitlements: %w", err)
	}

	// Union of entitlement ids from the catalog and any seen on active grants.
	seen := map[string]bool{}
	for id := range entitlements {
		seen[id] = true
	}
	for id := range activeCustomers {
		seen[id] = true
	}
	for id := range seen {
		meta := entitlements[id]
		row := entitlementRollupRow{
			EntitlementID:   id,
			DisplayName:     meta.name,
			State:           meta.state,
			ActiveCustomers: len(activeCustomers[id]),
		}
		if flagDisagreements {
			row.Disagreements = len(disagreements[id])
		}
		view.Entitlements = append(view.Entitlements, row)
		view.TotalActive += row.ActiveCustomers
	}
	view.TotalEntitlement = len(view.Entitlements)
	sort.Slice(view.Entitlements, func(i, j int) bool {
		if view.Entitlements[i].ActiveCustomers != view.Entitlements[j].ActiveCustomers {
			return view.Entitlements[i].ActiveCustomers > view.Entitlements[j].ActiveCustomers
		}
		return view.Entitlements[i].EntitlementID < view.Entitlements[j].EntitlementID
	})
	if view.TotalEntitlement == 0 {
		view.Note = "no entitlements or active grants in the local mirror; run 'sync' first"
	}
	return view, nil
}

// subscriptionsAllNonAccess returns true when the customer has at least one
// subscription and every one of them is in a non-access state. A customer with
// zero mirrored subscriptions is NOT flagged (we can't conclude drift without
// the subscription side).
func subscriptionsAllNonAccess(states []string) bool {
	if len(states) == 0 {
		return false
	}
	for _, s := range states {
		if !nonAccessSubStates[s] {
			return false
		}
	}
	return true
}

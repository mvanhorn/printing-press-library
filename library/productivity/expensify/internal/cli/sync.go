// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `sync` pulls the full user state from Expensify's ReconnectApp and upserts
// into the local SQLite store. The local store powers offline search, rollups,
// damage, and dupes.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/expensify/internal/store"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var syncAll bool
	var sinceDate, policyID, dbPath, resources string
	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Pull reports, expenses, and workspaces from Expensify into the local store",
		Example: "  expensify-pp-cli sync\n  expensify-pp-cli sync --since 2026-01-01",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would pull reports, expenses, and workspaces from Expensify into the local store")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Post(cmd.Context(), "/ReconnectApp", map[string]any{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("ReconnectApp returned HTTP %d", status))
			}

			st, err := store.Open(dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening local store: %w", err))
			}
			defer st.Close()

			if syncAll {
				fmt.Fprintln(os.Stderr, "sync: --all accepted (no-op: ReconnectApp already returns full state)")
			}
			if resources != "" {
				fmt.Fprintf(os.Stderr, "sync: --resources %s accepted (ReconnectApp returns all resource types)\n", resources)
			}
			if sinceDate != "" {
				fmt.Fprintf(os.Stderr, "sync: --since %s accepted (filter applied after upsert)\n", sinceDate)
			}
			if policyID != "" {
				fmt.Fprintf(os.Stderr, "sync: --policy %s accepted (filter applied after upsert)\n", policyID)
			}

			nReports, nExpenses, nWorkspaces := ingestReconnectApp(st, data, sinceDate, policyID)

			fmt.Fprintf(cmd.OutOrStdout(),
				"Synced %d reports, %d expenses, %d workspaces from Expensify.\n",
				nReports, nExpenses, nWorkspaces)
			return nil
		},
	}
	cmd.Flags().BoolVar(&syncAll, "all", false, "Full sync (accepted for parity; ReconnectApp already returns full state)")
	cmd.Flags().BoolVar(&syncAll, "full", false, "Alias for --all (full sync)")
	cmd.Flags().StringVar(&sinceDate, "since", "", "Only upsert expenses dated on/after this YYYY-MM-DD")
	cmd.Flags().StringVar(&policyID, "policy", "", "Only upsert rows for this policy ID")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite store path (defaults to the per-user cache location)")
	cmd.Flags().StringVar(&resources, "resources", "", "Comma-separated resource types to sync (accepted for parity; ReconnectApp returns all)")
	return cmd
}

// ingestReconnectApp parses the response blob from /ReconnectApp and upserts
// every plausible report / expense / workspace we can find. Expensify's shape
// varies: the payload typically has `onyxData` which is an array of patches,
// each carrying `key` + `value`. Keys like `transactions_*`, `reports_*`, and
// `policy_*` carry what we want.
func ingestReconnectApp(st *store.Store, data json.RawMessage, since, policyFilter string) (nReports, nExpenses, nWorkspaces int) {
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		// The endpoint is documented as returning an array of Workspace; some
		// responses (and the spec-typed verify mock) deliver that bare array
		// instead of the live {onyxData:[...]} envelope. Ingest each element as
		// a workspace rather than erroring out.
		var arr []map[string]any
		if aerr := json.Unmarshal(data, &arr); aerr == nil {
			for _, m := range arr {
				if m == nil {
					continue
				}
				w := workspaceFromMap(m)
				if w.ID == "" {
					continue
				}
				if err := st.UpsertWorkspace(w); err != nil {
					fmt.Fprintf(os.Stderr, "sync: upsert workspace %s: %v\n", w.ID, err)
					continue
				}
				nWorkspaces++
			}
			return
		}
		fmt.Fprintf(os.Stderr, "sync: could not parse response JSON: %v\n", err)
		return
	}

	// ReconnectApp commonly wraps everything in {onyxData: [...]}
	if arr, ok := top["onyxData"].([]any); ok {
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			key, _ := m["key"].(string)
			val := m["value"]
			nR, nE, nW := ingestOnyxSlice(st, key, val, since, policyFilter)
			nReports += nR
			nExpenses += nE
			nWorkspaces += nW
		}
	}

	// Also try top-level shortcuts that some responses include.
	if v, ok := top["reports"]; ok {
		nR, _, _ := ingestOnyxSlice(st, "reports", v, since, policyFilter)
		nReports += nR
	}
	if v, ok := top["transactions"]; ok {
		_, nE, _ := ingestOnyxSlice(st, "transactions", v, since, policyFilter)
		nExpenses += nE
	}
	if v, ok := top["policies"]; ok {
		_, _, nW := ingestOnyxSlice(st, "policies", v, since, policyFilter)
		nWorkspaces += nW
	}
	return
}

func ingestOnyxSlice(st *store.Store, key string, val any, since, policyFilter string) (nReports, nExpenses, nWorkspaces int) {
	switch {
	case strings.HasPrefix(key, "transactions") || strings.HasPrefix(key, "transaction_"):
		nExpenses = upsertTransactions(st, val, since, policyFilter)
	case strings.HasPrefix(key, "reports") || strings.HasPrefix(key, "report_"):
		nReports = upsertReports(st, val, policyFilter)
	case strings.HasPrefix(key, "policies") || strings.HasPrefix(key, "policy_"):
		nWorkspaces = upsertPolicies(st, val)
	}
	return
}

func upsertTransactions(st *store.Store, val any, since, policyFilter string) int {
	count := 0
	process := func(raw map[string]any) {
		e := transactionFromMap(raw)
		if e.TransactionID == "" {
			return
		}
		if since != "" && e.Date != "" && e.Date < since {
			return
		}
		if policyFilter != "" && e.PolicyID != policyFilter {
			return
		}
		if err := st.UpsertExpense(e); err != nil {
			fmt.Fprintf(os.Stderr, "sync: upsert expense %s: %v\n", e.TransactionID, err)
			return
		}
		count++
	}
	walkMaps(val, process)
	return count
}

func upsertReports(st *store.Store, val any, policyFilter string) int {
	count := 0
	process := func(raw map[string]any) {
		r := reportFromMap(raw)
		if r.ReportID == "" {
			return
		}
		if policyFilter != "" && r.PolicyID != policyFilter {
			return
		}
		if err := st.UpsertReport(r); err != nil {
			fmt.Fprintf(os.Stderr, "sync: upsert report %s: %v\n", r.ReportID, err)
			return
		}
		count++
	}
	walkMaps(val, process)
	return count
}

func upsertPolicies(st *store.Store, val any) int {
	count := 0
	process := func(raw map[string]any) {
		w := workspaceFromMap(raw)
		if w.ID == "" {
			return
		}
		if err := st.UpsertWorkspace(w); err != nil {
			fmt.Fprintf(os.Stderr, "sync: upsert workspace %s: %v\n", w.ID, err)
			return
		}
		count++
	}
	walkMaps(val, process)
	return count
}

// walkMaps handles both "val is a single object" and "val is a map of id->object"
// and "val is a slice of objects".
func walkMaps(val any, fn func(map[string]any)) {
	switch v := val.(type) {
	case map[string]any:
		// If every child is a map, treat as id->object.
		allMaps := len(v) > 0
		for _, child := range v {
			if _, ok := child.(map[string]any); !ok {
				allMaps = false
				break
			}
		}
		if allMaps {
			for _, child := range v {
				if m, ok := child.(map[string]any); ok {
					fn(m)
				}
			}
		} else {
			fn(v)
		}
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				fn(m)
			}
		}
	}
}

func transactionFromMap(m map[string]any) store.Expense {
	raw, _ := json.Marshal(m)
	return store.Expense{
		TransactionID: firstString(m, "transactionID", "transaction_id", "id", "iouTransactionID"),
		ReportID:      firstString(m, "reportID", "report_id", "iouReportID"),
		Merchant:      firstString(m, "merchant", "description"),
		Amount:        firstInt64(m, "amount", "modifiedAmount"),
		Currency:      firstString(m, "currency", "modifiedCurrency"),
		Category:      firstString(m, "category"),
		Tag:           firstString(m, "tag"),
		Date:          normalizeDate(firstString(m, "created", "date", "modifiedCreated")),
		Comment:       firstString(m, "comment"),
		Receipt:       firstString(m, "receipt", "receiptPath", "filename"),
		PolicyID:      firstString(m, "policyID", "policy_id"),
		Created:       firstString(m, "created"),
		Billable:      firstBool(m, "billable"),
		Reimbursable:  firstBool(m, "reimbursable"),
		RawJSON:       string(raw),
	}
}

func reportFromMap(m map[string]any) store.Report {
	raw, _ := json.Marshal(m)
	return store.Report{
		ReportID:     firstString(m, "reportID", "report_id", "id"),
		PolicyID:     firstString(m, "policyID", "policy_id"),
		Title:        firstString(m, "reportName", "title", "name"),
		Status:       firstString(m, "stateNum", "state", "status", "statusNum"),
		Total:        firstInt64(m, "total"),
		Currency:     firstString(m, "currency"),
		Created:      firstString(m, "created", "lastActionCreated"),
		LastUpdated:  firstString(m, "lastModified", "lastUpdatedTime"),
		ExpenseCount: int(firstInt64(m, "transactionCount", "expenseCount")),
		RawJSON:      string(raw),
	}
}

func workspaceFromMap(m map[string]any) store.Workspace {
	raw, _ := json.Marshal(m)
	return store.Workspace{
		ID:         firstString(m, "id", "policyID"),
		Name:       firstString(m, "name"),
		Type:       firstString(m, "type"),
		Role:       firstString(m, "role"),
		OwnerEmail: firstString(m, "owner", "ownerEmail"),
		RawJSON:    string(raw),
	}
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch s := v.(type) {
			case string:
				if s != "" {
					return s
				}
			case float64:
				return fmt.Sprintf("%d", int64(s))
			}
		}
	}
	return ""
}

func firstInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case float64:
				return int64(n)
			case int64:
				return n
			case int:
				return int64(n)
			case string:
				if n == "" {
					continue
				}
				var i int64
				_, _ = fmt.Sscanf(n, "%d", &i)
				if i != 0 {
					return i
				}
			}
		}
	}
	return 0
}

func firstBool(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch b := v.(type) {
			case bool:
				return b
			case string:
				return b == "true" || b == "1"
			case float64:
				return b != 0
			}
		}
	}
	return false
}

// normalizeDate returns a YYYY-MM-DD slice when the input starts with that
// shape, else returns the original (empty strings pass through).
func normalizeDate(s string) string {
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	return s
}

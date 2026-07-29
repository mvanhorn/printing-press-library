// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: reconcile — verify the local mirror matches the live tenant
// using windowed probes (Priority's OData has no $count).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// pp:data-source auto

// reconcileResourceMeta maps syncable resources to their live form and the
// key + date fields used for drift probes.
var reconcileResourceMeta = map[string]struct {
	Form      string
	KeyField  string
	DateField string
}{
	"orders":     {"ORDERS", "ORDNAME", "CURDATE"},
	"customers":  {"CUSTOMERS", "CUSTNAME", ""},
	"invoices":   {"AINVOICES", "IVNUM", "IVDATE"},
	"parts":      {"LOGPART", "PARTNAME", ""},
	"porders":    {"PORDERS", "ORDNAME", ""},
	"suppliers":  {"SUPPLIERS", "SUPNAME", ""},
	"warehouses": {"WAREHOUSES", "WARHSNAME", ""},
}

type reconcileView struct {
	Resource    string   `json:"resource"`
	Form        string   `json:"form"`
	LocalRows   int      `json:"local_rows"`
	ProbedKeys  int      `json:"probed_live_keys"`
	MissingKeys []string `json:"missing_locally,omitempty"`
	LiveLatest  string   `json:"live_latest,omitempty"`
	LocalLatest string   `json:"local_latest,omitempty"`
	Verdict     string   `json:"verdict"` // in-sync | stale | drifted | empty-local
	Note        string   `json:"note,omitempty"`
}

func newNovelReconcileCmd(flags *rootFlags) *cobra.Command {
	var resource string
	var probes int
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Verify your local mirror matches the live tenant using windowed probes — the API has no $count",
		Long: strings.Trim(`
Use this command to verify the local database matches the live tenant.
Do NOT use it to pull data; use 'sync' instead.

Strategy: fetch the newest N live keys (ordered by the resource's date field
where one exists) and check each exists locally; compare live-newest vs
local-newest timestamps. Missing keys or a newer live timestamp mean the
mirror is stale or drifted.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli reconcile --resource orders
  priority-pp-cli reconcile --resource invoices --probes 25 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--resource=orders"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would probe live keys and compare against the local mirror")
				return nil
			}
			if resource == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--resource is required (one of: orders, customers, invoices, parts, porders, suppliers, warehouses)"))
			}
			meta, ok := reconcileResourceMeta[strings.ToLower(resource)]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown resource %q (one of: orders, customers, invoices, parts, porders, suppliers, warehouses)", resource))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: priority-pp-cli sync --resources %s --db %s\n", dbPath, resource, dbPath)
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
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			view := reconcileView{Resource: strings.ToLower(resource), Form: meta.Form}

			table := strings.ToLower(resource)
			if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+table+`"`).Scan(&view.LocalRows); err != nil {
				return fmt.Errorf("counting local rows: %w", err)
			}
			if meta.DateField != "" {
				var localLatest string
				dateCol := strings.ToLower(meta.DateField)
				if err := db.DB().QueryRowContext(ctx, `SELECT COALESCE(MAX("`+dateCol+`"), '') FROM "`+table+`"`).Scan(&localLatest); err == nil {
					view.LocalLatest = localLatest
				}
			}

			params := map[string]string{
				"$top":    fmt.Sprintf("%d", probes),
				"$select": meta.KeyField,
			}
			if meta.DateField != "" {
				params["$orderby"] = meta.DateField + " desc"
				params["$select"] = meta.KeyField + "," + meta.DateField
			}
			data, err := c.GetNoCache(ctx, "/"+meta.Form, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var envelope struct {
				Value []map[string]json.RawMessage `json:"value"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				return fmt.Errorf("parsing live probe response: %w", err)
			}
			view.ProbedKeys = len(envelope.Value)
			for i, row := range envelope.Value {
				key := jsonStrField(row, meta.KeyField)
				if key == "" {
					continue
				}
				if i == 0 && meta.DateField != "" {
					view.LiveLatest = jsonStrField(row, meta.DateField)
				}
				var n int
				keyCol := strings.ToLower(meta.KeyField)
				if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM "`+table+`" WHERE "`+keyCol+`" = ?`, key).Scan(&n); err != nil {
					return err
				}
				if n == 0 {
					view.MissingKeys = append(view.MissingKeys, key)
				}
			}

			switch {
			case view.LocalRows == 0:
				view.Verdict = "empty-local"
				view.Note = fmt.Sprintf("local mirror has no %s rows; run: priority-pp-cli sync --resources %s", resource, resource)
			case len(view.MissingKeys) > 0:
				view.Verdict = "drifted"
				view.Note = fmt.Sprintf("%d of the %d newest live records are missing locally; run: priority-pp-cli sync --resources %s", len(view.MissingKeys), view.ProbedKeys, resource)
			case view.LiveLatest != "" && view.LocalLatest != "" && view.LiveLatest > view.LocalLatest:
				view.Verdict = "stale"
				view.Note = fmt.Sprintf("live newest (%s) is newer than local newest (%s); run: priority-pp-cli sync --resources %s", view.LiveLatest, view.LocalLatest, resource)
			default:
				view.Verdict = "in-sync"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s): %s\n  local rows: %d\n  probed live keys: %d\n  missing locally: %d\n",
				view.Resource, view.Form, view.Verdict, view.LocalRows, view.ProbedKeys, len(view.MissingKeys))
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), "  "+view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&resource, "resource", "", "syncable resource to reconcile (orders, customers, invoices, parts, porders, suppliers, warehouses)")
	cmd.Flags().IntVar(&probes, "probes", 20, "how many newest live keys to verify against the mirror")
	return cmd
}

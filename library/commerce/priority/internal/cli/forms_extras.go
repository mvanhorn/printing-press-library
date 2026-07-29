// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written forms children: list (offline form catalog), describe
// (per-form fields/subforms/mandatory), refresh (fetch + parse + cache
// $metadata, optionally clearing the server-side cache first).

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

func newFormsListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all forms on the tenant, offline from the cached schema",
		Example: strings.Trim(`
  priority-pp-cli forms list
  priority-pp-cli forms list --limit 100 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list cached forms")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tenant := tenantKeyFromClient(c)
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local schema cache at %s\nrun: priority-pp-cli forms refresh\n", dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(ctx,
				`SELECT f.form, (SELECT COUNT(*) FROM pp_meta_fields x WHERE x.tenant = f.tenant AND x.form = f.form),
				        (SELECT COUNT(*) FROM pp_meta_subforms s WHERE s.tenant = f.tenant AND s.form = f.form)
				 FROM pp_meta_forms f WHERE f.tenant = ? ORDER BY f.form LIMIT ?`, tenant, limit)
			if err != nil {
				return err
			}
			type formRow struct {
				Form     string `json:"form"`
				Fields   int    `json:"fields"`
				Subforms int    `json:"subforms"`
			}
			var out []formRow
			for rows.Next() {
				var r formRow
				if err := rows.Scan(&r.Form, &r.Fields, &r.Subforms); err != nil {
					_ = rows.Close()
					return err
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			_ = rows.Close()
			if out == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "schema cache empty for tenant %s\nrun: priority-pp-cli forms refresh\n", tenant)
				out = []formRow{}
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "FORM\tFIELDS\tSUBFORMS")
			for _, r := range out {
				fmt.Fprintf(w, "%s\t%d\t%d\n", r.Form, r.Fields, r.Subforms)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 200, "maximum forms to list")
	return cmd
}

func newFormsDescribeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <form>",
		Short: "Show a form's fields (with types and mandatory flags) and subforms, offline",
		Example: strings.Trim(`
  priority-pp-cli forms describe ORDERS
  priority-pp-cli forms describe CUSTOMERS --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "form=ORDERS"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would describe the cached form schema")
				return nil
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("form name is required"))
			}
			form := strings.ToUpper(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tenant := tenantKeyFromClient(c)
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local schema cache at %s\nrun: priority-pp-cli forms refresh\n", dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			forms, err := loadCachedForms(ctx, db, tenant)
			if err != nil {
				return err
			}
			for _, f := range forms {
				if f.Name == form {
					if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
						return printJSONFiltered(cmd.OutOrStdout(), f, flags)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s — %d fields, %d subforms\n\n", f.Name, len(f.Fields), len(f.Subforms))
					w := newTabWriter(cmd.OutOrStdout())
					fmt.Fprintln(w, "FIELD\tTYPE\tMANDATORY\tDESCRIPTION")
					for _, fl := range f.Fields {
						mand := ""
						if fl.Mandatory {
							mand = "yes"
						}
						fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", fl.Name, fl.Type, mand, truncate(fl.Description, 40))
					}
					if err := w.Flush(); err != nil {
						return err
					}
					if len(f.Subforms) > 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "\nSUBFORMS:")
						for _, sf := range f.Subforms {
							kind := "single"
							if sf.Collection {
								kind = "collection"
							}
							fmt.Fprintf(cmd.OutOrStdout(), "  %s -> %s (%s)\n", sf.Name, sf.Target, kind)
						}
					}
					return nil
				}
			}
			return notFoundErr(fmt.Errorf("form %q not in the cached schema; try 'forms search %s' or 'forms refresh'", form, form))
		},
	}
	return cmd
}

func newFormsRefreshCmd(flags *rootFlags) *cobra.Command {
	var clearServer string
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Fetch the tenant's $metadata, parse it, and rebuild the local schema cache",
		Long: strings.Trim(`
Downloads the full EDMX (can exceed 10MB on real tenants), parses forms,
fields, mandatory flags, and subforms, and replaces the local cache that
powers 'forms list/describe/search/diff'. Pass --clear-server-entity to POST
ClearEntityMetadata for one entity (or 'all') before fetching, forcing
Priority to rebuild its own metadata cache.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli forms refresh
  priority-pp-cli forms refresh --clear-server-entity ORDERS`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch and cache $metadata")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if clearServer != "" {
				body := map[string]string{}
				if !strings.EqualFold(clearServer, "all") {
					body["Entity"] = strings.ToUpper(clearServer)
				}
				if _, _, err := c.Post(ctx, "/ClearEntityMetadata", body); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: ClearEntityMetadata failed (continuing with fetch): %v\n", err)
				}
			}
			db, err := store.OpenWithContext(ctx, defaultDBPath("priority-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			formsN, fieldsN, err := refreshMetadataForTenant(ctx, c, db)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{
				"tenant": tenantKeyFromClient(c),
				"forms":  formsN,
				"fields": fieldsN,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&clearServer, "clear-server-entity", "", "POST ClearEntityMetadata for this entity (or 'all') before fetching")
	return cmd
}

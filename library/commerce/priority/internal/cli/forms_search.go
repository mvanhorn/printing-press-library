// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: forms search — offline schema grep over the cached $metadata.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// pp:data-source local

type formsSearchHit struct {
	Kind        string `json:"kind"` // form | field | subform
	Form        string `json:"form"`
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Mandatory   bool   `json:"mandatory,omitempty"`
	Description string `json:"description,omitempty"`
}

type formsSearchView struct {
	Query   string           `json:"query"`
	Tenant  string           `json:"tenant"`
	Hits    []formsSearchHit `json:"hits"`
	Note    string           `json:"note,omitempty"`
	Scanned int              `json:"scanned_definitions"`
}

func newNovelFormsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <term>",
		Short: "Find any form, field, or subform on your tenant by name fragment, instantly and offline",
		Long: strings.Trim(`
Use this command to find forms and fields in the cached tenant schema.
Do NOT use it to search business data (customers, parts, orders); use 'search' instead.

Populate or refresh the cache with 'priority-pp-cli forms refresh'.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli forms search WARHS --json
  priority-pp-cli forms search DUEDATE --limit 10
  priority-pp-cli forms search ORDERITEMS`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "term=ORD", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search cached tenant schema")
				return nil
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("search term is required"))
			}
			term := strings.ToUpper(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tenant := tenantKeyFromClient(c)
			dbPath := defaultDBPath("priority-pp-cli")
			// Empty-cache paths emit the same formsSearchView envelope as a
			// real search so agents can distinguish "cache not populated —
			// search not performed" (note set, scanned_definitions 0) from
			// "searched N definitions, no match". A bare [] here would be a
			// silent null.
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local schema cache at %s\nrun: priority-pp-cli forms refresh\n", dbPath)
				return printFormsSearchEmptyCache(cmd, flags, term, tenant,
					fmt.Sprintf("schema cache not populated; %q was not searched — run 'forms refresh' first", term))
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			present, err := metadataCachePresent(ctx, db, tenant)
			if err != nil {
				return err
			}
			if !present {
				fmt.Fprintf(cmd.ErrOrStderr(), "schema cache empty for tenant %s\nrun: priority-pp-cli forms refresh\n", tenant)
				return printFormsSearchEmptyCache(cmd, flags, term, tenant,
					fmt.Sprintf("schema cache empty for this tenant; %q was not searched — run 'forms refresh' first", term))
			}

			pat := "%" + term + "%"
			view := formsSearchView{Query: term, Tenant: tenant}

			var scanned int
			if err := db.DB().QueryRowContext(ctx,
				`SELECT (SELECT COUNT(*) FROM pp_meta_forms WHERE tenant = ?) + (SELECT COUNT(*) FROM pp_meta_fields WHERE tenant = ?) + (SELECT COUNT(*) FROM pp_meta_subforms WHERE tenant = ?)`,
				tenant, tenant, tenant).Scan(&scanned); err != nil {
				return err
			}
			view.Scanned = scanned

			formRows, err := db.DB().QueryContext(ctx,
				`SELECT form FROM pp_meta_forms WHERE tenant = ? AND form LIKE ? ORDER BY form LIMIT ?`, tenant, pat, limit)
			if err != nil {
				return err
			}
			for formRows.Next() {
				var f string
				if err := formRows.Scan(&f); err != nil {
					_ = formRows.Close()
					return err
				}
				view.Hits = append(view.Hits, formsSearchHit{Kind: "form", Form: f, Name: f})
			}
			if err := formRows.Err(); err != nil {
				_ = formRows.Close()
				return err
			}
			_ = formRows.Close()

			if len(view.Hits) < limit {
				fieldRows, err := db.DB().QueryContext(ctx,
					`SELECT form, field, type, mandatory, COALESCE(description,'') FROM pp_meta_fields WHERE tenant = ? AND (field LIKE ? OR description LIKE ?) ORDER BY form, field LIMIT ?`,
					tenant, pat, pat, limit-len(view.Hits))
				if err != nil {
					return err
				}
				for fieldRows.Next() {
					var h formsSearchHit
					var mand int
					if err := fieldRows.Scan(&h.Form, &h.Name, &h.Type, &mand, &h.Description); err != nil {
						_ = fieldRows.Close()
						return err
					}
					h.Kind = "field"
					h.Mandatory = mand == 1
					view.Hits = append(view.Hits, h)
				}
				if err := fieldRows.Err(); err != nil {
					_ = fieldRows.Close()
					return err
				}
				_ = fieldRows.Close()
			}

			if len(view.Hits) < limit {
				subRows, err := db.DB().QueryContext(ctx,
					`SELECT form, subform, COALESCE(target,'') FROM pp_meta_subforms WHERE tenant = ? AND subform LIKE ? ORDER BY form, subform LIMIT ?`,
					tenant, pat, limit-len(view.Hits))
				if err != nil {
					return err
				}
				for subRows.Next() {
					var h formsSearchHit
					if err := subRows.Scan(&h.Form, &h.Name, &h.Type); err != nil {
						_ = subRows.Close()
						return err
					}
					h.Kind = "subform"
					view.Hits = append(view.Hits, h)
				}
				if err := subRows.Err(); err != nil {
					_ = subRows.Close()
					return err
				}
				_ = subRows.Close()
			}

			if len(view.Hits) == 0 {
				view.Note = fmt.Sprintf("no forms, fields, or subforms matching %q in %d cached definitions; refresh with 'forms refresh' if the schema changed", term, scanned)
			}
			if view.Hits == nil {
				view.Hits = []formsSearchHit{}
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "KIND\tFORM\tNAME\tTYPE\tMANDATORY\tDESCRIPTION")
			for _, h := range view.Hits {
				mand := ""
				if h.Mandatory {
					mand = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", h.Kind, h.Form, h.Name, h.Type, mand, truncate(h.Description, 40))
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum hits to return")
	return cmd
}

// printFormsSearchEmptyCache renders the standard search envelope for the
// cache-not-populated case: hits [], scanned_definitions 0, and a note that
// names the query so callers see the search did not run against any data.
func printFormsSearchEmptyCache(cmd *cobra.Command, flags *rootFlags, term, tenant, note string) error {
	view := formsSearchView{Query: term, Tenant: tenant, Hits: []formsSearchHit{}, Note: note}
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), note)
	return nil
}

// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: forms diff — named schema snapshots + local drift detection.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/priorityx"
	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// pp:data-source auto

type formsDiffView struct {
	Tenant    string                `json:"tenant"`
	Baseline  string                `json:"baseline,omitempty"`
	Saved     string                `json:"saved,omitempty"`
	TakenAt   string                `json:"baseline_taken_at,omitempty"`
	Diff      *priorityx.SchemaDiff `json:"diff,omitempty"`
	Unchanged bool                  `json:"unchanged,omitempty"`
	Note      string                `json:"note,omitempty"`
	Snapshots []string              `json:"snapshots,omitempty"`
}

func newNovelFormsDiffCmd(flags *rootFlags) *cobra.Command {
	var baseline string
	var save string
	var list bool
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Snapshot your tenant's schema and see exactly which forms and fields changed after an upgrade or customization",
		Long: strings.Trim(`
Use this command to compare tenant schema snapshots over time or across tenants.
Do NOT use it to look up current field names; use 'forms search' instead.

Workflow: 'forms diff --save pre-upgrade' stores a named snapshot of the current
cached schema; later, 'forms diff --baseline pre-upgrade' refreshes the cache
from the live tenant and reports added/removed/changed forms and fields.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli forms diff --save pre-upgrade
  priority-pp-cli forms diff --baseline pre-upgrade
  priority-pp-cli forms diff --list`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--list"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would snapshot or diff the tenant schema")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tenant := tenantKeyFromClient(c)
			db, err := store.OpenWithContext(ctx, defaultDBPath("priority-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			view := formsDiffView{Tenant: tenant}

			if list {
				rows, err := db.DB().QueryContext(ctx,
					`SELECT name, taken_at FROM pp_schema_snapshots WHERE tenant = ? ORDER BY taken_at DESC`, tenant)
				if err != nil {
					return err
				}
				for rows.Next() {
					var name, takenAt string
					if err := rows.Scan(&name, &takenAt); err != nil {
						_ = rows.Close()
						return err
					}
					view.Snapshots = append(view.Snapshots, fmt.Sprintf("%s (%s)", name, takenAt))
				}
				if err := rows.Err(); err != nil {
					_ = rows.Close()
					return err
				}
				_ = rows.Close()
				if view.Snapshots == nil {
					view.Snapshots = []string{}
					view.Note = "no snapshots saved yet; run 'forms diff --save <name>'"
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			if save == "" && baseline == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("one of --save, --baseline, or --list is required"))
			}

			if save != "" {
				present, err := metadataCachePresent(ctx, db, tenant)
				if err != nil {
					return err
				}
				if !present {
					fmt.Fprintln(cmd.ErrOrStderr(), "schema cache empty; fetching live $metadata first")
					if _, _, err := refreshMetadataForTenant(ctx, c, db); err != nil {
						return classifyAPIError(err, flags)
					}
				}
				forms, err := loadCachedForms(ctx, db, tenant)
				if err != nil {
					return err
				}
				blob, err := json.Marshal(forms)
				if err != nil {
					return err
				}
				if _, err := db.DB().ExecContext(ctx,
					`INSERT INTO pp_schema_snapshots (tenant, name, taken_at, data) VALUES (?, ?, ?, ?)
					 ON CONFLICT(tenant, name) DO UPDATE SET taken_at = excluded.taken_at, data = excluded.data`,
					tenant, save, time.Now().UTC().Format(time.RFC3339), string(blob)); err != nil {
					return fmt.Errorf("saving snapshot: %w", err)
				}
				view.Saved = save
				view.Note = fmt.Sprintf("snapshot %q saved (%d forms)", save, len(forms))
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			// --baseline: refresh cache live, then diff snapshot -> current.
			var blob string
			var takenAt string
			err = db.DB().QueryRowContext(ctx,
				`SELECT data, taken_at FROM pp_schema_snapshots WHERE tenant = ? AND name = ?`, tenant, baseline).Scan(&blob, &takenAt)
			if err != nil {
				return notFoundErr(fmt.Errorf("snapshot %q not found for tenant %s; run 'forms diff --save %s' first or 'forms diff --list'", baseline, tenant, baseline))
			}
			var baseForms []priorityx.Form
			if err := json.Unmarshal([]byte(blob), &baseForms); err != nil {
				return fmt.Errorf("parsing snapshot %q: %w", baseline, err)
			}
			if _, _, err := refreshMetadataForTenant(ctx, c, db); err != nil {
				return classifyAPIError(err, flags)
			}
			curForms, err := loadCachedForms(ctx, db, tenant)
			if err != nil {
				return err
			}
			d := priorityx.DiffSchemas(baseForms, curForms)
			view.Baseline = baseline
			view.TakenAt = takenAt
			view.Diff = &d
			view.Unchanged = d.Empty()
			if view.Unchanged {
				view.Note = fmt.Sprintf("no schema changes since snapshot %q (%s)", baseline, takenAt)
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&baseline, "baseline", "", "snapshot name to diff the live schema against")
	cmd.Flags().StringVar(&save, "save", "", "save the current cached schema as a named snapshot")
	cmd.Flags().BoolVar(&list, "list", false, "list saved snapshots for this tenant")
	return cmd
}

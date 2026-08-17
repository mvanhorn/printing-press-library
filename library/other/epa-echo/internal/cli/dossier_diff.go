// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/mvanhorn/printing-press-library/library/other/epa-echo/internal/store"
	"github.com/spf13/cobra"
)

func newNovelDossierDiffCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "diff FACILITY_ID",
		Short:       "Compare a live detailed facility report with the prior local snapshot and list changed ECHO sections.",
		Example:     "  epa-echo-pp-cli dossier diff 110009441979 --agent",
		Annotations: map[string]string{"mcp:local-write": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			report, err := getDFR(ctx, flags, args[0])
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(ctx, defaultDBPath("epa-echo-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			previous := map[string]any{}
			raw, getErr := db.Get("echo-dfr-snapshot", args[0])
			baseline := errors.Is(getErr, sql.ErrNoRows)
			if getErr != nil && !baseline {
				return getErr
			}
			if getErr == nil {
				if err := json.Unmarshal(raw, &previous); err != nil {
					return err
				}
			}
			changes := []map[string]any{}
			recordChanges := map[string]any{"added_count": 0, "removed_count": 0, "added": []map[string]any{}, "removed": []map[string]any{}}
			if !baseline {
				changes = changedSections(previous, report)
				recordChanges = recordLevelChanges(previous, report)
			}
			next, _ := json.Marshal(report)
			if err := db.Upsert("echo-dfr-snapshot", args[0], next); err != nil {
				return err
			}
			return emitECHO(cmd, flags, "mixed", map[string]any{"facility_id": args[0], "baseline_created": baseline, "changed_section_count": len(changes), "section_changes": changes, "record_changes": recordChanges, "comparison": "current live DFR versus the immediately prior local snapshot", "caveats": echoCaveats()})
		},
	}
	return cmd
}

// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newProceduresPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "procedures",
		Short:       "Procedure types (cassatie, hoger beroep, kort geding, etc.)",
		Long:        "Procedure-type vocabulary with PSI URIs and human-readable names. Used by `search --procedure NAME` for local filtering since the upstream API ignores procedure=.",
		Example:     "  rechtspraak-pp-cli procedures\n  rechtspraak-pp-cli procedures --json",
		Annotations: map[string]string{"pp:endpoint": "procedures.list", "pp:method": "GET", "pp:path": "/Waardelijst/Proceduresoorten", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			idx, err := getProcedureIndex(cmd.Context())
			if err != nil {
				return err
			}
			ps := idx.All()
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), ps)
			}
			for _, p := range ps {
				fmt.Fprintf(cmd.OutOrStdout(), "%-40s  %s\n", p.Slug, p.Name)
			}
			return nil
		},
	}
	return cmd
}

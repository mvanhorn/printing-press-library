// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRelationsPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "relations",
		Short:       "Formal relation types (FormeleRelaties) between decisions",
		Long:        "Lists every formal relation type — Hoger beroep, Verzet, Cassatie, etc. — with its court-tier pairs (Rolspelers) and disposition outcomes (AfhandelingsWijze). Powers chain command's edge labels.",
		Example:     "  rechtspraak-pp-cli relations\n  rechtspraak-pp-cli relations --json",
		Annotations: map[string]string{"pp:endpoint": "relations.list", "pp:method": "GET", "pp:path": "/Waardelijst/FormeleRelaties", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rels, err := getRelationDefs(cmd.Context())
			if err != nil {
				return err
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), rels)
			}
			for _, r := range rels {
				fmt.Fprintf(cmd.OutOrStdout(), "%-32s  outcomes=%d  rolspelers=%d\n", r.Name, len(r.Outcomes), len(r.Rolspelers))
			}
			return nil
		},
	}
	return cmd
}

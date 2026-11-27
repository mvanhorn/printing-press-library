// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/spf13/cobra"
)

func newPeopleCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "people",
		Short: "Inspect Granola people directory via the internal API",
	}
	cmd.AddCommand(newPeopleListCmd(flags))
	return cmd
}

func newPeopleListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Granola people profiles via the internal API (pecan shard)",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.dataSource == "local" {
				return notFoundErr(fmt.Errorf("people list requires live internal API access"))
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			raw, err := ic.GetPeople()
			if err != nil {
				return apiErr(err)
			}
			return emitJSON(cmd, flags, raw)
		},
	}
}

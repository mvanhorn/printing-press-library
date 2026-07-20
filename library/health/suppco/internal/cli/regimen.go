// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.
// Handwritten SuppCo command; retained through regeneration in .printing-press-patches.

package cli

import (
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/provider"

	"github.com/spf13/cobra"
)

func newRegimenCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "regimen",
		Short:       "Emit a normalized read-only SuppCo regimen snapshot.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRegimenSnapshotCmd(flags))
	return cmd
}

func newRegimenSnapshotCmd(flags *rootFlags) *cobra.Command {
	// pp:data-source live
	return &cobra.Command{
		Use:         "snapshot <date>",
		Short:       "Read the current stack and dated provider schedule and emit deterministic normalized JSON.",
		Example:     "  suppco-pp-cli regimen snapshot 2026-07-19",
		Args:        suppCoDateArgs(flags),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := provider.ValidateDate(args[0]); err != nil {
				return usageErr(err)
			}
			service, err := newSuppCoProvider(flags)
			if err != nil {
				return err
			}
			snapshot, err := service.Snapshot(cmd.Context(), args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(snapshot)
		},
	}
}

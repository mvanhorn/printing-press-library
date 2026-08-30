// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import "github.com/spf13/cobra"

func newNovelPlanFillCmd(flags *rootFlags) *cobra.Command {
	var opts planCopyOptions
	opts.clientSchemaVersion = 2
	opts.mode = "replace-sections"

	cmd := &cobra.Command{
		Use:         "fill",
		Short:       "Fill an existing Wanderlog trip from a shared or public source plan with dry-run and force safeguards.",
		Example:     "  wanderlog-pp-cli plan fill --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --target-key YOUR_TRIP_KEY --dry-run --agent",
		Long:        "Use this command to fill an existing Wanderlog target trip from a shared/public source. Do NOT use it to create a new target trip; use 'plan clone' instead.",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanFill(cmd, flags, opts)
		},
	}
	cmd.Flags().StringVar(&opts.sourceURL, "source-url", "", "Shared/public Wanderlog plan URL")
	cmd.Flags().StringVar(&opts.sourceKey, "source-key", "", "Shared/public Wanderlog plan key")
	cmd.Flags().StringVar(&opts.targetKey, "target-key", "", "Existing target Wanderlog trip key")
	cmd.Flags().StringVar(&opts.mode, "mode", "replace-sections", "Fill mode: replace-sections")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply the fill operation; without this, only preview the operation")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Allow replacing a target trip that already contains blocks")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

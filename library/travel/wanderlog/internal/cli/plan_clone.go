// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import "github.com/spf13/cobra"

func newNovelPlanCloneCmd(flags *rootFlags) *cobra.Command {
	var opts planCopyOptions
	opts.clientSchemaVersion = 2
	opts.privacy = "private"

	cmd := &cobra.Command{
		Use:         "clone",
		Short:       "Create a new Wanderlog trip from a shared or public source plan, then fill it with the source plan template.",
		Example:     "  wanderlog-pp-cli plan clone --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --dry-run --agent",
		Long:        "Use this command to create a new Wanderlog trip from a shared/public source plan. Do NOT use it to modify an existing trip; use 'plan fill' instead.",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanClone(cmd, flags, opts)
		},
	}
	cmd.Flags().StringVar(&opts.sourceURL, "source-url", "", "Shared/public Wanderlog plan URL")
	cmd.Flags().StringVar(&opts.sourceKey, "source-key", "", "Shared/public Wanderlog plan key")
	cmd.Flags().StringVar(&opts.destination, "destination", "", "Destination override for clone creation (reserved for sources without a geo id)")
	cmd.Flags().StringVar(&opts.title, "title", "", "Title for the new target trip")
	cmd.Flags().StringVar(&opts.startDate, "start-date", "", "Start date override, YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.endDate, "end-date", "", "End date override, YYYY-MM-DD")
	cmd.Flags().StringVar(&opts.privacy, "privacy", "private", "Target trip privacy: private, friends, or public")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Create and fill the target trip; without this, only preview the operation")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

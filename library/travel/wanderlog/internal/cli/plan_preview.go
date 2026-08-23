// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import "github.com/spf13/cobra"

func newNovelPlanPreviewCmd(flags *rootFlags) *cobra.Command {
	var opts planCopyOptions
	opts.clientSchemaVersion = 2

	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Inspect a shared plan and report dates, sections, blocks, resources, and clone warnings before any write.",
		Example: "  wanderlog-pp-cli plan preview --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent\n" +
			"  wanderlog-pp-cli plan preview --source-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --dry-run --agent   # --dry-run is accepted and is a no-op: this command only reads",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlanPreview(cmd, flags, opts)
		},
	}
	cmd.Flags().StringVar(&opts.sourceURL, "source-url", "", "Shared/public Wanderlog plan URL")
	cmd.Flags().StringVar(&opts.sourceKey, "source-key", "", "Shared/public Wanderlog plan key")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

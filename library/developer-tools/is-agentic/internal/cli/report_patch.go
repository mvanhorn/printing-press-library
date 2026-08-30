// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// The generated response-path heuristic promoted the nested issues array for
// this single-object report. Keep the generated command/help surfaces, but use
// the official report contract directly for both versioned and compatibility
// runtime responses.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, path := range [][]string{{"report", "get-is-agentic-v1"}, {"report", "get-is-agentic-legacy"}} {
			if cmd, _, err := root.Find(path); err == nil && cmd != nil {
				patchReportCommand(cmd, flags)
			}
		}
		// The API has no collection/list response path. Prevent sync/search
		// helpers from treating the nested issues field as the report resource.
		resourceReadConfigs["report"] = resourceReadConfig{}
	})
}

func patchReportCommand(cmd *cobra.Command, flags *rootFlags) {
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if !hasChangedLocalFlags(cmd) && len(args) == 0 && !flags.dryRun {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "fetch completed Is Agentic report")
		}
		target, err := cmd.Flags().GetString("url")
		if err != nil {
			return err
		}
		if target == "" {
			return usageErr(fmt.Errorf("required flag %q not set", "url"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		s, path, err := openAgenticStore(ctx, flags)
		if err != nil {
			return err
		}
		if s == nil {
			return missingStore(cmd, flags, path)
		}
		defer s.Close()
		report, _, err := fetchAndSave(ctx, s, target)
		if err != nil {
			return err
		}
		return emitReport(cmd, flags, report)
	}
}

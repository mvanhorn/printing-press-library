// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/michi"
	"github.com/spf13/cobra"
)

func newNovelChangesCmd(flags *rootFlags) *cobra.Command {
	var before, after string
	var demo bool
	cmd := &cobra.Command{Use: "changes", Short: "Compare two saved snapshot files offline by stable station ID and factual fields, or use an explicit synthetic demo", Long: "Use this command for changes between two saved observations without network requests. Do NOT use it to refresh an observation; use 'snapshot' and save stdout instead. Do NOT use it for a current shortlist; use 'compare' instead. Inputs must be schema_version 1, kind snapshot JSON. Observation timestamps are excluded from field changes; fetch failures produce unresolved rows instead of false removals. --demo uses clearly labeled synthetic data to demonstrate a published-hours change. Files are read only, capped at 2 MiB each.", Example: "  michi-no-eki-pp-cli changes --demo --agent\n  michi-no-eki-pp-cli changes --before before.json --after after.json --agent", Annotations: michiAnnotations("local", ""), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "changes")
		}
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("changes uses --before and --after paths, not positional arguments"))
		}
		if flags.dataSource == "live" {
			return usageErr(fmt.Errorf("changes has no live equivalent; it compares local snapshot files"))
		}
		flags.agentSource = "local"
		var a, b michi.Comparison
		var e error
		if demo {
			if before != "" || after != "" {
				return usageErr(fmt.Errorf("choose --demo or both snapshot input paths"))
			}
			a, b = michi.DemoSnapshots()
		} else {
			if before == "" || after == "" {
				return usageErr(fmt.Errorf("--before and --after are both required; save snapshot --ids output first, or use --demo"))
			}
			a, e = michi.ReadSnapshot(before)
			if e != nil {
				return usageErr(e)
			}
			b, e = michi.ReadSnapshot(after)
			if e != nil {
				return usageErr(e)
			}
		}
		v, e := michi.Diff(a, b)
		if e != nil {
			return usageErr(e)
		}
		v.Demo = demo
		if demo {
			v.Source = "synthetic demo"
			v.Note = "Synthetic demonstration only; changed hours are not a real station observation. " + v.Note
		}
		return printJSONFiltered(cmd.OutOrStdout(), v, flags)
	}}
	cmd.Flags().StringVar(&before, "before", "", "Read-only path to an earlier factual snapshot JSON, at most2 MiB")
	cmd.Flags().StringVar(&after, "after", "", "Read-only path to a later factual snapshot JSON, at most2 MiB")
	cmd.Flags().BoolVar(&demo, "demo", false, "Use explicit synthetic snapshots to demonstrate an offline field change")
	return cmd
}

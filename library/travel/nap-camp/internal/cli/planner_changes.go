// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/napcamp"
	"github.com/spf13/cobra"
)

func newNovelPlannerChangesCmd(flags *rootFlags) *cobra.Command {
	var before, after string
	cmd := &cobra.Command{Use: "changes [before-file] [after-file]", Short: "Compare two saved observation files using operands or --before and --after flags.", Example: "  " + "nap-camp-pp-cli planner changes before.json after.json --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "before=internal/napcamp/testdata/before.json;after=internal/napcamp/testdata/after.json"}}
	napBind(flags, cmd, "local", func(cmd *cobra.Command, args []string) error {
		if before != "" || after != "" {
			if len(args) != 0 || before == "" || after == "" {
				return usageErr(fmt.Errorf("use two positional paths OR both --before and --after"))
			}
			args = []string{before, after}
		}
		if len(args) != 2 {
			return usageErr(fmt.Errorf("changes requires two snapshot JSON paths; create them with planner snapshot --save-to"))
		}
		a, e := napcamp.LoadSnapshot(args[0])
		if e != nil {
			return fmt.Errorf("before snapshot %q: %w", args[0], e)
		}
		b, e := napcamp.LoadSnapshot(args[1])
		if e != nil {
			return fmt.Errorf("after snapshot %q: %w", args[1], e)
		}
		v, e := napcamp.Diff(a, b)
		if e != nil {
			return usageErr(e)
		}
		return flags.printJSON(cmd, v)
	})
	cmd.Flags().StringVar(&before, "before", "", "Earlier versioned snapshot JSON input file")
	cmd.Flags().StringVar(&after, "after", "", "Later versioned snapshot JSON input file")
	cmd.Long = "Use this command to compare two saved observations of the same entity. For fresh source evidence, use 'planner snapshot'. Timestamp-only differences are ignored; dates present in only one file are separate coverage, not invented status changes."
	return cmd
}

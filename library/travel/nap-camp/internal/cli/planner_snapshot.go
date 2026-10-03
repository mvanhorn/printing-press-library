// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/napcamp"
	"github.com/spf13/cobra"
)

func newNovelPlannerSnapshotCmd(flags *rootFlags) *cobra.Command {
	var month, destination string
	cmd := &cobra.Command{Use: "snapshot <campsite-id> <plan-id>", Short: "Fetch a versioned campsite/pitch/calendar observation and optionally save it to a new file.", Example: "  " + "nap-camp-pp-cli planner snapshot 11007 20005062 --month 2026-10 --json", Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:method": "GET", "pp:happy-args": "campsite=11007;plan=20005062;--month=2026-10"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		if e := napIDs(args, 2); e != nil {
			return e
		}
		if month != "" {
			if e := napMonth(month); e != nil {
				return e
			}
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		s, e := a.Snapshot(cmd.Context(), args[0], args[1], month)
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		if destination != "" {
			if e := napcamp.SaveSnapshot(destination, s); e != nil {
				return e
			}
		}
		return flags.printJSON(cmd, s)
	})
	cmd.Long = "Use this command to fetch and optionally save a fresh source observation. For comparing two saved observations, use 'planner changes'. --save-to writes the full unprojected snapshot atomically, refuses existing paths, and requires an existing parent directory. Saved evidence is dated and does not guarantee current availability."
	cmd.Flags().StringVar(&month, "month", "", "Optional JST month YYYY-MM; calendar is [] when omitted")
	cmd.Flags().StringVar(&destination, "save-to", "", "Optional new JSON output file; existing files are preserved")
	return cmd
}

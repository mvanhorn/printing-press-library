// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
	"github.com/spf13/cobra"
	"time"
)

func newNovelStationsSyncCmd(f *rootFlags) *cobra.Command {
	var db string
	c := &cobra.Command{Use: "sync", Short: "Save an explicit provider snapshot for offline station discovery.", Example: "  hello-cycling-pp-cli stations sync --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}
	c.Annotations["mcp:write-flags"] = "snapshot-db"
	c.Annotations["pp:live-happy-path"] = "true"
	c.Annotations["pp:happy-args"] = "--data-source=live"
	c.Flags().StringVar(&db, "snapshot-db", "", "SQLite snapshot destination; defaults to the CLI data directory")
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcLiveOnly(f); e != nil {
			return e
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "stations sync")
		}
		o := hcOptions{db: db, liveOnly: true}
		s, src, e := hcSnapshot(c, f, o)
		if e != nil {
			return hcErr(e)
		}
		if e = cycling.SaveSnapshot(hcPath(o, f), s); e != nil {
			return apiErr(e)
		}
		return hcPrint(c, f, map[string]any{"meta": s.Meta(src, time.Now()), "snapshot_database": hcPath(o, f), "station_count": len(s.Information), "status_count": len(s.Statuses), "saved": true, "note": "Original source timestamps are preserved; offline counts can become stale."})
	}
	return c
}

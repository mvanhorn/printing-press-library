// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
	"github.com/spf13/cobra"
	"time"
)

func newNovelStationsChangesCmd(f *rootFlags) *cobra.Command {
	var o hcOptions
	var q string
	c := &cobra.Command{Use: "changes", Short: "Compare a saved snapshot with a new provider observation.", Example: "  hello-cycling-pp-cli stations changes --query 新宿 --limit 10 --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}
	// The full live matrix runs in a clean HOME; use the explicitly attributed
	// public baseline fixture for this read-only observation comparison.
	c.Annotations["pp:happy-args"] = "--snapshot-db=internal/cycling/testdata/baseline.db; --query=プラーズタワー東新宿; --data-source=live"

	c.Flags().BoolVar(&o.offline, "offline", false, "Read only the explicitly synchronized local station snapshot")
	c.Flags().StringVar(&o.db, "snapshot-db", "", "SQLite snapshot path; defaults to the CLI data directory")
	c.Flags().DurationVar(&o.age, "status-max-age", 5*time.Minute, "Maximum source status age before counts are treated as stale")
	c.Flags().IntVar(&o.limit, "limit", 10, "Maximum result rows, between 1 and 50 (trip comparisons: 10)")
	c.Flags().StringVar(&o.vehicle, "vehicle-type", "", "Optional GBFS vehicle class ID; use vehicles rules to inspect current classes")

	c.Flags().StringVar(&q, "query", "", "Optional Japanese station/address substring to narrow changes")
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcLiveOnly(f); e != nil {
			return e
		}
		if e := hcValidate(o, 50); e != nil {
			return e
		}
		if o.offline {
			return usageErr(fmt.Errorf("stations changes requires a new live observation; omit --offline"))
		}
		if len(q) > 600 {
			return usageErr(fmt.Errorf("--query exceeds 600 UTF-8 bytes"))
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "stations changes")
		}
		before, e := cycling.LoadSnapshot(hcPath(o, f))
		if e != nil {
			return apiErr(e)
		}
		o.liveOnly = true
		after, src, e := hcSnapshot(c, f, o)
		if e != nil {
			return hcErr(e)
		}
		now := time.Now()
		rows, total := cycling.Changes(before.Stations(before.ObservedAt, o.age, o.vehicle), after.Stations(now, o.age, o.vehicle), q, o.limit)
		out := hcResult(after, src, rows, total)
		out["baseline_observed_at"] = before.ObservedAt
		out["baseline_station_count"] = len(before.Information)
		out["interpretation"] = "Observation differences only; no ride, trend, or future availability is inferred. Baseline is not overwritten."
		return hcPrint(c, f, out)
	}
	return c
}

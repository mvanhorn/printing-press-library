// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func newNovelStationsFindCmd(f *rootFlags) *cobra.Command {
	var o hcOptions
	var q string
	c := &cobra.Command{Use: "find", Short: "Search Japanese station names, addresses or stable IDs.", Example: "  hello-cycling-pp-cli stations find --query 東新宿 --limit 5 --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}

	c.Flags().BoolVar(&o.offline, "offline", false, "Read only the explicitly synchronized local station snapshot")
	c.Flags().StringVar(&o.db, "snapshot-db", "", "SQLite snapshot path; defaults to the CLI data directory")
	c.Flags().DurationVar(&o.age, "status-max-age", 5*time.Minute, "Maximum source status age before counts are treated as stale")
	c.Flags().IntVar(&o.limit, "limit", 10, "Maximum result rows, between 1 and 50 (trip comparisons: 10)")
	c.Flags().StringVar(&o.vehicle, "vehicle-type", "", "Optional GBFS vehicle class ID; use vehicles rules to inspect current classes")

	c.Flags().StringVar(&q, "query", "", "Station name or address substring; Japanese and normalized full-width input supported")
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcValidate(o, 50); e != nil {
			return e
		}
		if strings.TrimSpace(q) == "" || len(q) > 600 {
			return usageErr(fmt.Errorf("--query must be a nonempty station name/address up to 600 UTF-8 bytes"))
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "stations find")
		}
		s, src, e := hcSnapshot(c, f, o)
		if e != nil {
			return hcErr(e)
		}
		if e = hcKnownVehicle(s, o.vehicle); e != nil {
			return e
		}
		rows, total := cycling.Find(s.Stations(time.Now(), o.age, o.vehicle), q, o.limit)
		return hcPrint(c, f, hcResult(s, src, rows, total))
	}
	return c
}

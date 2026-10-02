// Copyright 2026 zjsng. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
	"github.com/spf13/cobra"
	"regexp"
	"time"
)

func newNovelStationsShowCmd(f *rootFlags) *cobra.Command {
	var o hcOptions
	var id string
	c := &cobra.Command{Use: "show", Short: "Inspect a stable station ID with source counts, states and app handoff.", Example: "  hello-cycling-pp-cli stations show --id 5112 --agent", Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}

	c.Flags().BoolVar(&o.offline, "offline", false, "Read only the explicitly synchronized local station snapshot")
	c.Flags().StringVar(&o.db, "snapshot-db", "", "SQLite snapshot path; defaults to the CLI data directory")
	c.Flags().DurationVar(&o.age, "status-max-age", 5*time.Minute, "Maximum source status age before counts are treated as stale")
	c.Flags().IntVar(&o.limit, "limit", 1, "Maximum result rows, between 1 and 50 (trip comparisons: 10)")
	c.Flags().StringVar(&o.vehicle, "vehicle-type", "", "Optional GBFS vehicle class ID; use vehicles rules to inspect current classes")

	c.Flags().StringVar(&id, "id", "", "Exact provider station ID from stations find or nearby")
	c.RunE = func(c *cobra.Command, args []string) error {
		if e := hcValidate(o, 50); e != nil {
			return e
		}
		if !regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`).MatchString(id) {
			return usageErr(fmt.Errorf("--id must be an exact station ID; use stations find"))
		}
		if dryRunOK(f) {
			return writeDryRun(c.OutOrStdout(), f, "stations show")
		}
		s, src, e := hcSnapshot(c, f, o)
		if e != nil {
			return hcErr(e)
		}
		if e = hcKnownVehicle(s, o.vehicle); e != nil {
			return e
		}
		for _, r := range s.Stations(time.Now(), o.age, o.vehicle) {
			if r.ID == id {
				return hcPrint(c, f, hcResult(s, src, []cycling.Station{r}, 1))
			}
		}
		return notFoundErr(fmt.Errorf("station %q is absent from this source snapshot; search with stations find", id))
	}
	return c
}

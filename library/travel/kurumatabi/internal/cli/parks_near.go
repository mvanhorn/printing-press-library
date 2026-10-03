package cli

// pp:data-source local
import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
	"github.com/spf13/cobra"
)

func newNovelParksNearCmd(f *rootFlags) *cobra.Command {
	var lat, lon, radius float64
	var limit, max int
	var db string
	cmd := &cobra.Command{Use: "near", Short: "Rank cached parks by straight-line waypoint distance", Long: "Use this command for straight-line proximity of cached observations to explicit coordinates. Do NOT use this command for screening parks by required facility evidence; use 'parks match' instead. Only the observed cached pool is ranked; no road distance, route, clearance or nearest nationwide claim is made.", Example: "  kurumatabi-pp-cli parks near --latitude 36.7 --longitude 138.2 --radius-km 100 --limit 5 --agent", Annotations: parkAnnotation("local", "--latitude=36.7;--longitude=138.2;--limit=5"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks near")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("use explicit --latitude and --longitude"))
		}
		if !cmd.Flags().Changed("latitude") || !cmd.Flags().Changed("longitude") {
			if !hasChangedLocalFlags(cmd) && !f.agent && !f.asJSON {
				return cmd.Help()
			}
			return usageErr(fmt.Errorf("--latitude and --longitude are both required"))
		}
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("parks near has no live equivalent; cache parks search or detail first"))
		}
		if max < 1 || max > 10000 {
			return usageErr(fmt.Errorf("--max-scan-records must be 1..10000"))
		}
		if _, _, e := parks.Near([]parks.Park{}, lat, lon, radius, limit); e != nil {
			return usageErr(e)
		}
		db = parkResolveDB(db)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		ps, e := parks.Load(ctx, db)
		if e != nil {
			return e
		}
		parkCacheHint(cmd, f, db, ps)
		pool := len(ps)
		if len(ps) > max {
			ps = ps[:max]
		}
		rows, missing, e := parks.Near(ps, lat, lon, radius, limit)
		if e != nil {
			return usageErr(e)
		}
		note := "Cached observed pool only; distance is straight line, not driving."
		if len(rows) == 0 {
			note += " No match in this observed radius; raise --radius-km or --max-scan-records, or cache more parks."
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"meta": map[string]any{"source": "local", "scanned_records": len(ps), "cached_records": pool, "missing_coordinates": missing, "scan_cap_hit": pool > max, "note": note}, "results": rows}, f)
	}}
	cmd.Flags().Float64Var(&lat, "latitude", 0, "Explicit waypoint latitude, finite -90..90 degrees")
	cmd.Flags().Float64Var(&lon, "longitude", 0, "Explicit waypoint longitude, finite -180..180 degrees")
	cmd.Flags().Float64Var(&radius, "radius-km", 100, "Maximum straight-line radius in kilometres, up to 20000")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum nearest cached parks returned, 1..100")
	cmd.Flags().IntVar(&max, "max-scan-records", 1000, "Maximum cached records scanned independently from output size")
	cmd.Flags().StringVar(&db, "db", "", "SQLite path for observations; default follows runtime CLI data paths")
	return cmd
}

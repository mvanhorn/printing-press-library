// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/american-reindustrialization/internal/store"
	"github.com/spf13/cobra"
)

type clusterMember struct {
	Slug   string  `json:"slug"`
	Name   string  `json:"name"`
	State  string  `json:"hq_state,omitempty"`
	City   string  `json:"hq_city,omitempty"`
	Sector string  `json:"primary_sector,omitempty"`
	Lat    float64 `json:"latitude"`
	Lon    float64 `json:"longitude"`
}

type cluster struct {
	CentroidLat    float64         `json:"centroid_lat"`
	CentroidLon    float64         `json:"centroid_lon"`
	Members        int             `json:"members"`
	DominantSector string          `json:"dominant_sector,omitempty"`
	Companies      []clusterMember `json:"companies"`
}

func newAnalyticsGeoClustersCmd(flags *rootFlags) *cobra.Command {
	var radiusKM float64
	var state, dbPath string

	cmd := &cobra.Command{
		Use:   "geo-clusters",
		Short: "Grid-bucket companies by lat/lon and emit cluster centroid, members, and dominant sector",
		Long: "Spatial clustering by lat/lon over locally synced companies using a fixed-size " +
			"grid (default 50km cells, configurable via --radius-km). Companies missing lat/lon " +
			"are skipped. Optionally restrict to a single HQ state with --state.",
		Example:     "  american-reindustrialization-pp-cli analytics geo-clusters --state TX --radius-km 50 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if radiusKM <= 0 {
				radiusKM = 50
			}
			if dbPath == "" {
				dbPath = defaultDBPath("american-reindustrialization-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'american-reindustrialization-pp-cli sync' first.", err)
			}
			defer db.Close()

			q := `SELECT slug, name, COALESCE(hq_state,''), COALESCE(hq_city,''),
			             COALESCE(primary_sector,''), latitude, longitude
			      FROM companies
			      WHERE latitude IS NOT NULL AND longitude IS NOT NULL`
			args2 := []any{}
			if state != "" {
				q += " AND upper(hq_state) = upper(?)"
				args2 = append(args2, strings.TrimSpace(state))
			}

			rows, err := db.DB().QueryContext(cmd.Context(), q, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			degPerKM := 1.0 / 111.0 // simple grid step in degrees; good enough for clustering at 1-100km
			step := radiusKM * degPerKM

			type bucket struct {
				members []clusterMember
				sectors map[string]int
			}
			buckets := map[[2]int]*bucket{}
			for rows.Next() {
				var slug, name, st, city, sector sql.NullString
				var lat, lon sql.NullFloat64
				if err := rows.Scan(&slug, &name, &st, &city, &sector, &lat, &lon); err != nil {
					continue
				}
				if !lat.Valid || !lon.Valid {
					continue
				}
				key := [2]int{int(math.Floor(lat.Float64 / step)), int(math.Floor(lon.Float64 / step))}
				b := buckets[key]
				if b == nil {
					b = &bucket{sectors: map[string]int{}}
					buckets[key] = b
				}
				b.members = append(b.members, clusterMember{
					Slug:   slug.String,
					Name:   name.String,
					State:  st.String,
					City:   city.String,
					Sector: sector.String,
					Lat:    lat.Float64,
					Lon:    lon.Float64,
				})
				if sector.String != "" {
					b.sectors[sector.String]++
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating geo-cluster rows: %w", err)
			}

			out := make([]cluster, 0, len(buckets))
			for _, b := range buckets {
				var sumLat, sumLon float64
				for _, m := range b.members {
					sumLat += m.Lat
					sumLon += m.Lon
				}
				n := float64(len(b.members))
				out = append(out, cluster{
					CentroidLat:    sumLat / n,
					CentroidLon:    sumLon / n,
					Members:        len(b.members),
					DominantSector: argmaxString(b.sectors),
					Companies:      b.members,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Members > out[j].Members })

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().Float64Var(&radiusKM, "radius-km", 50, "Grid cell size in kilometers")
	cmd.Flags().StringVar(&state, "state", "", "Restrict to one HQ state (2-letter code)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path override")
	return cmd
}

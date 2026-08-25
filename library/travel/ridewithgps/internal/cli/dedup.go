// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `dedup` clusters near-duplicate routes from the locally synced routes mirror
// by distance + start/end geometry, and optionally deletes the extras via the
// API. Hand-authored transcendence command.
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/store"
	"github.com/spf13/cobra"
)

type dedupRoute struct {
	ID        string
	Name      string
	Distance  float64
	FirstLat  float64
	FirstLng  float64
	LastLat   float64
	LastLng   float64
	CreatedAt string
	hasCoords bool
}

type dedupClusterView struct {
	Canonical  map[string]any   `json:"canonical"`
	Duplicates []map[string]any `json:"duplicates"`
	DistanceKM float64          `json:"distance_km"`
}

type dedupView struct {
	ScannedRoutes int                `json:"scanned_routes"`
	ThresholdM    float64            `json:"threshold_m"`
	Clusters      []dedupClusterView `json:"clusters"`
	Applied       int                `json:"deleted,omitempty"`
	Note          string             `json:"note,omitempty"`
}

func newNovelDedupCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var apply bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dedup",
		Short: "Find near-duplicate routes by distance and start/end geometry, and optionally delete the extras.",
		Long: `Cluster near-duplicate routes in the local mirror.

Two routes are duplicates when their total distance and both their start and end
points fall within --threshold meters. By default this previews clusters; pass
--apply to delete the extras (keeping the oldest route in each cluster).

For quality issues other than duplication (stale, private) use 'audit'.`,
		Example: strings.Trim(`
  ridewithgps-pp-cli dedup --threshold 100
  ridewithgps-pp-cli dedup --threshold 50 --json
  ridewithgps-pp-cli dedup --threshold 100 --apply
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would cluster near-duplicate routes from the local mirror")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if threshold <= 0 {
				threshold = 100
			}
			if dbPath == "" {
				dbPath = defaultDBPath("ridewithgps-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: ridewithgps-pp-cli sync --resources routes --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "routes", flags.maxAge)

			rows, err := db.DB().QueryContext(ctx, `SELECT id, COALESCE(name,''), distance,
					first_lat, first_lng, last_lat, last_lng, COALESCE(created_at,'')
				FROM routes`)
			if err != nil {
				return fmt.Errorf("querying routes: %w", err)
			}
			defer rows.Close()

			var routes []dedupRoute
			for rows.Next() {
				var id, name, created sql.NullString
				var dist, fLat, fLng, lLat, lLng sql.NullFloat64
				if err := rows.Scan(&id, &name, &dist, &fLat, &fLng, &lLat, &lLng, &created); err != nil {
					continue
				}
				routes = append(routes, dedupRoute{
					ID: id.String, Name: name.String, Distance: dist.Float64,
					FirstLat: fLat.Float64, FirstLng: fLng.Float64,
					LastLat: lLat.Float64, LastLng: lLng.Float64,
					CreatedAt: created.String,
					hasCoords: fLat.Valid && fLng.Valid && lLat.Valid && lLng.Valid,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading routes: %w", err)
			}

			// Sort by distance so we only compare routes within the distance
			// window — bounds comparisons to near-linear for typical libraries.
			sort.Slice(routes, func(i, j int) bool { return routes[i].Distance < routes[j].Distance })

			parent := make([]int, len(routes))
			for i := range parent {
				parent[i] = i
			}
			var find func(int) int
			find = func(x int) int {
				for parent[x] != x {
					parent[x] = parent[parent[x]]
					x = parent[x]
				}
				return x
			}
			union := func(a, b int) { parent[find(a)] = find(b) }

			for i := 0; i < len(routes); i++ {
				if !routes[i].hasCoords {
					continue
				}
				for j := i + 1; j < len(routes); j++ {
					if routes[j].Distance-routes[i].Distance > threshold {
						break
					}
					if !routes[j].hasCoords {
						continue
					}
					if haversineMeters(routes[i].FirstLat, routes[i].FirstLng, routes[j].FirstLat, routes[j].FirstLng) <= threshold &&
						haversineMeters(routes[i].LastLat, routes[i].LastLng, routes[j].LastLat, routes[j].LastLng) <= threshold {
						union(i, j)
					}
				}
			}

			groups := map[int][]int{}
			for i := range routes {
				root := find(i)
				groups[root] = append(groups[root], i)
			}

			view := dedupView{ScannedRoutes: len(routes), ThresholdM: threshold, Clusters: make([]dedupClusterView, 0)}
			var toDelete []dedupRoute
			for _, members := range groups {
				if len(members) < 2 {
					continue
				}
				// Canonical = earliest known created_at. Unknown dates sort last
				// so --apply never deletes a dated route in favor of one whose
				// age cannot be established.
				sort.Slice(members, func(a, b int) bool {
					return canonicalRouteLess(routes[members[a]], routes[members[b]])
				})
				canonical := routes[members[0]]
				cluster := dedupClusterView{
					Canonical:  map[string]any{"id": canonical.ID, "name": canonical.Name, "created_at": canonical.CreatedAt},
					DistanceKM: roundN(metersToKM(canonical.Distance), 1),
					Duplicates: make([]map[string]any, 0, len(members)-1),
				}
				for _, m := range members[1:] {
					r := routes[m]
					cluster.Duplicates = append(cluster.Duplicates, map[string]any{"id": r.ID, "name": r.Name, "created_at": r.CreatedAt})
					toDelete = append(toDelete, r)
				}
				view.Clusters = append(view.Clusters, cluster)
			}

			if apply && len(toDelete) > 0 {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				deleted := 0
				for _, r := range toDelete {
					if _, _, err := c.Delete(ctx, fmt.Sprintf("/api/v1/routes/%s.json", r.ID)); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to delete route %s: %v\n", r.ID, err)
						continue
					}
					deleted++
				}
				view.Applied = deleted
			}

			if len(view.Clusters) == 0 {
				view.Note = "no duplicate clusters found"
			} else if !apply {
				view.Note = fmt.Sprintf("%d duplicate routes across %d clusters; re-run with --apply to delete them", len(toDelete), len(view.Clusters))
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Scanned %d routes (threshold %.0fm)\n", view.ScannedRoutes, view.ThresholdM)
			for i, cl := range view.Clusters {
				fmt.Fprintf(cmd.OutOrStdout(), "\nCluster %d (%.1f km) — keep %s %q\n", i+1, cl.DistanceKM, cl.Canonical["id"], cl.Canonical["name"])
				for _, d := range cl.Duplicates {
					fmt.Fprintf(cmd.OutOrStdout(), "  duplicate: %s %q\n", d["id"], d["name"])
				}
			}
			if view.Applied > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDeleted %d duplicate routes.\n", view.Applied)
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 100, "Max meters of distance + start/end separation to treat routes as duplicates")
	cmd.Flags().BoolVar(&apply, "apply", false, "Delete duplicate routes (keeps the oldest in each cluster)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local mirror)")
	return cmd
}

func canonicalRouteLess(a, b dedupRoute) bool {
	if a.CreatedAt == "" && b.CreatedAt != "" {
		return false
	}
	if a.CreatedAt != "" && b.CreatedAt == "" {
		return true
	}
	if a.CreatedAt == b.CreatedAt {
		return a.ID < b.ID
	}
	return a.CreatedAt < b.CreatedAt
}

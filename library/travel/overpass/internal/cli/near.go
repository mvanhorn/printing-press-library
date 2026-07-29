// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live
//
// `near` and `geojson` both geocode the place through Nominatim and then run
// the built query against Overpass at call time, failing over across mirrors.
// OpenStreetMap is edited continuously and the query space is unbounded — any
// point on earth, crossed with any subject type — so there is no useful subset
// to sync ahead of time. Reads go to the API; the local store is a cache of
// what has already been asked for, not a precondition.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/overpass/internal/subjects"

	"github.com/spf13/cobra"
)

func newNovelNearCmd(flags *rootFlags) *cobra.Command {
	var (
		o      searchOpts
		radius string
	)
	cmd := &cobra.Command{
		Use:   "near",
		Short: "Finds things worth photographing near a place, by name rather than by OpenStreetMap tag",
		Long: strings.Trim(`
What is worth shooting around here.

Give a place and a subject type. The subject name is mapped to the right
OpenStreetMap tags, a valid Overpass query is built, and the query is retried
across public mirrors until one answers — they fall over independently, and
which one is healthy changes by the hour.

Results are ordered nearest-first, by straight-line distance from the resolved
origin, and cover nodes, ways, and relations. Querying only nodes, which is the
common mistake, silently loses most large structures.
`, "\n"),
		Example: strings.Trim(`
  overpass-pp-cli near --at "Los Angeles" --type water_tower --radius 25km
  overpass-pp-cli near --at "San Pedro, CA" --type lighthouse --radius 40km
  overpass-pp-cli near --latitude 34.05 --longitude -118.24 --type viewpoint --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ty, err := subjects.Lookup(o.typ)
			if err != nil {
				return usageErr(err)
			}
			radiusM, err := subjects.ParseDistance(radius)
			if err != nil {
				return usageErr(err)
			}
			// Before ANY network work — geocoding included. The failure this
			// exists for is a silently-accepted radius burning two minutes of
			// retries; a caution printed after the geocode has already paid
			// part of that cost. stderr, so --json/--agent stdout stays clean.
			if w := subjects.LargeRadiusWarning(radiusM); w != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", w)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			origin, err := o.resolveOrigin(ctx, cmd, flags)
			if err != nil {
				return err
			}
			area := subjects.Area{Lat: origin.Latitude, Lon: origin.Longitude, RadiusM: radiusM}
			query, err := subjects.BuildQuery(ty, area, o.timeout, o.limit)
			if err != nil {
				return usageErr(err)
			}

			subs, attempts, remark, err := runQuery(ctx, cmd, flags, query, &area)
			if err != nil {
				return err
			}
			if flags.asJSON {
				// The truncation remark rides inside the document. Printed
				// ahead of it, as prose, it would make stdout unparseable.
				payload := map[string]any{
					"type": ty.Name, "tags": ty.Tags, "origin": origin,
					"radius_m": radiusM, "subjects": subs,
					"mirror_attempts": attempts, "note": ty.Note,
					"partial": remark != "",
				}
				if remark != "" {
					payload["partial_remark"] = remark
				}
				return flags.printJSONLive(cmd, payload)
			}
			return renderSubjects(cmd, flags, subs, ty, origin.Label, remark)
		},
	}
	registerSearchFlags(cmd, &o)
	cmd.Flags().StringVar(&radius, "radius", "20km", "Search radius, e.g. 25km, 10mi, 5000m")
	return cmd
}

func newNovelGeojsonCmd(flags *rootFlags) *cobra.Command {
	var (
		o      searchOpts
		radius string
		outPth string
	)
	cmd := &cobra.Command{
		Use:   "geojson",
		Short: "Writes results as GeoJSON for opening in a map or GIS tool",
		Long: strings.Trim(`
The same search as near, written out as GeoJSON.

Overpass splits coordinates across different fields for nodes and ways, and
returns its own element format rather than GeoJSON, so this does the
conversion: one Point feature per subject, every OpenStreetMap tag carried
through under an osm: prefix.

Features are written nearest-first, by straight-line distance from the resolved
origin, with that distance on each feature as distance_km.
`, "\n"),
		Example: strings.Trim(`
  overpass-pp-cli geojson --at "Los Angeles" --type lighthouse --radius 50km --out spots.geojson
  overpass-pp-cli geojson --at "Los Angeles" --type viewpoint --radius 20km
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ty, err := subjects.Lookup(o.typ)
			if err != nil {
				return usageErr(err)
			}
			radiusM, err := subjects.ParseDistance(radius)
			if err != nil {
				return usageErr(err)
			}
			// Before ANY network work — geocoding included. The failure this
			// exists for is a silently-accepted radius burning two minutes of
			// retries; a caution printed after the geocode has already paid
			// part of that cost. stderr, so --json/--agent stdout stays clean.
			if w := subjects.LargeRadiusWarning(radiusM); w != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", w)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			origin, err := o.resolveOrigin(ctx, cmd, flags)
			if err != nil {
				return err
			}
			area := subjects.Area{Lat: origin.Latitude, Lon: origin.Longitude, RadiusM: radiusM}
			query, err := subjects.BuildQuery(ty, area, o.timeout, o.limit)
			if err != nil {
				return usageErr(err)
			}
			subs, _, remark, err := runQuery(ctx, cmd, flags, query, &area)
			if err != nil {
				return err
			}
			// The remark travels inside the document rather than ahead of it:
			// stdout here is GeoJSON, and a prose line would make it
			// unparseable for every caller that pipes this into a map.
			raw, err := subjects.GeoJSONWithRemark(subs, remark)
			if err != nil {
				return err
			}
			if outPth == "" {
				// The GeoJSON itself is the machine-readable output, so it is
				// printed as-is in both modes.
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			if err := os.WriteFile(outPth, raw, 0o644); err != nil {
				return configErr(fmt.Errorf("writing %s: %w", outPth, err))
			}
			if flags.asJSON {
				// --json must emit JSON even when the payload went to a file;
				// a prose confirmation line breaks any caller parsing stdout.
				payload := map[string]any{
					"written": outPth, "count": len(subs), "type": ty.Name,
					"partial": remark != "",
				}
				if remark != "" {
					payload["partial_remark"] = remark
				}
				return flags.printJSONLive(cmd, payload)
			}
			if remark != "" {
				fmt.Fprintln(cmd.OutOrStdout(), partialNote(remark))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %d %s to %s\n", len(subs), pluralizeType(ty.Name, len(subs)), outPth)
			return nil
		},
	}
	registerSearchFlags(cmd, &o)
	cmd.Flags().StringVar(&radius, "radius", "20km", "Search radius, e.g. 25km, 10mi, 5000m")
	cmd.Flags().StringVar(&outPth, "out", "", "File to write; prints to stdout when unset")
	return cmd
}

// registerSearchFlags declares the shared location and type flags inline.
//
// Inline rather than through a helper that takes the command: verify-skill
// reads cmd.Flags() calls statically, and hiding them behind indirection makes
// the shipped SKILL.md understate what each command accepts.
func registerSearchFlags(cmd *cobra.Command, o *searchOpts) {
	cmd.Flags().StringVar(&o.at, "at", "", "Place name to search around, e.g. \"San Pedro, CA\"")
	cmd.Flags().Float64Var(&o.lat, "latitude", 0, "Latitude in decimal degrees, instead of --at (must be given with --longitude)")
	cmd.Flags().Float64Var(&o.lon, "longitude", 0, "Longitude in decimal degrees, instead of --at (must be given with --latitude)")
	cmd.Flags().StringVar(&o.country, "country", "", "Restrict geocoding to ISO-3166 alpha-2 codes, e.g. us")
	cmd.Flags().StringVar(&o.typ, "type", "water_tower", typeFlagHelp)
	cmd.Flags().IntVar(&o.limit, "limit", 50, "Maximum results to return (0 for no limit)")
	cmd.Flags().IntVar(&o.timeout, "query-timeout", 25, "Overpass server-side timeout in seconds")
}

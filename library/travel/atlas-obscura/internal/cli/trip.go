// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `trip` — saved itineraries persisted in the local SQLite store (hand-authored).
package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

const defaultTripName = "default"

func newNovelTripCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trip",
		Short: "Accumulate places into named itineraries that persist across sessions.",
		Long: "Save Atlas Obscura places into named trips that persist across sessions in the\n" +
			"local SQLite store. Build a trip up over time, then export it (see 'export').",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTripAddCmd(flags))
	cmd.AddCommand(newTripRemoveCmd(flags))
	cmd.AddCommand(newTripListCmd(flags))
	cmd.AddCommand(newTripShowCmd(flags))
	return cmd
}

func newTripAddCmd(flags *rootFlags) *cobra.Command {
	var trip string
	cmd := &cobra.Command{
		Use:     "add <id-or-slug>",
		Short:   "Add a place to a trip",
		Example: "  atlas-obscura-pp-cli trip add winchester-mystery-house --trip california-oddities",
		// Writes to the local SQLite store (ao_trip_items), so it is NOT mcp:read-only.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would add a place to a trip")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a place id or slug is required"))
			}
			if trip == "" {
				trip = defaultTripName
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			place, err := aoFetchPlaceShort(cmd.Context(), c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			s, err := aoDB(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureAOTables(s); err != nil {
				return err
			}
			cachePlace(s, place)
			_, err = s.DB().Exec(
				`INSERT OR REPLACE INTO ao_trip_items (trip, place_id, slug, title, location, lat, lng, added_at)
				 VALUES (?,?,?,?,?,?,?,?)`,
				trip, place.ID, place.Slug, place.Title, place.Location, place.Lat, place.Lng, nowDate())
			if err != nil {
				return fmt.Errorf("saving to trip: %w", err)
			}
			return aoEmit(cmd, flags, map[string]any{
				"added":   place.Title,
				"trip":    trip,
				"place":   place,
				"message": fmt.Sprintf("added %q to trip %q", place.Title, trip),
			})
		},
	}
	cmd.Flags().StringVar(&trip, "trip", "", "Trip name (default: \"default\")")
	return cmd
}

func newTripRemoveCmd(flags *rootFlags) *cobra.Command {
	var trip string
	cmd := &cobra.Command{
		Use:     "remove <id-or-slug>",
		Short:   "Remove a place from a trip",
		Example: "  atlas-obscura-pp-cli trip remove winchester-mystery-house --trip california-oddities",
		// Writes to the local SQLite store (ao_trip_items), so it is NOT mcp:read-only.
		// Removing an absent item is an idempotent no-op (exit 0), not an error.
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would remove a place from a trip")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a place id or slug is required"))
			}
			if trip == "" {
				trip = defaultTripName
			}
			s, err := aoDB(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureAOTables(s); err != nil {
				return err
			}
			res, err := s.DB().Exec(
				`DELETE FROM ao_trip_items WHERE trip=? AND (slug=? OR CAST(place_id AS TEXT)=?)`,
				trip, args[0], args[0])
			if err != nil {
				return fmt.Errorf("removing from trip: %w", err)
			}
			n, _ := res.RowsAffected()
			return aoEmit(cmd, flags, map[string]any{
				"trip":    trip,
				"removed": n,
				"message": fmt.Sprintf("removed %d item(s) from trip %q", n, trip),
			})
		},
	}
	cmd.Flags().StringVar(&trip, "trip", "", "Trip name (default: \"default\")")
	return cmd
}

func newTripListCmd(flags *rootFlags) *cobra.Command {
	var trip string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List trips, or the places in one trip",
		Example:     "  atlas-obscura-pp-cli trip list\n  atlas-obscura-pp-cli trip list --trip california-oddities",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list trips")
				return nil
			}
			s, err := aoDB(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureAOTables(s); err != nil {
				return err
			}
			if trip == "" {
				return listTrips(cmd, flags, s)
			}
			return showTrip(cmd, flags, s, trip)
		},
	}
	cmd.Flags().StringVar(&trip, "trip", "", "Show the places in this trip (omit to list all trips)")
	return cmd
}

func newTripShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show <trip>",
		Short:   "Show all places in a trip",
		Example: "  atlas-obscura-pp-cli trip show california-oddities --json",
		// An unknown trip is an empty result (exit 0), not an error.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show a trip")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a trip name is required"))
			}
			s, err := aoDB(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureAOTables(s); err != nil {
				return err
			}
			return showTrip(cmd, flags, s, args[0])
		},
	}
	return cmd
}

func listTrips(cmd *cobra.Command, flags *rootFlags, s interface{ DB() *sql.DB }) error {
	rows, err := s.DB().Query(`SELECT trip, COUNT(*) FROM ao_trip_items GROUP BY trip ORDER BY trip`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type tripRow struct {
		Trip  string `json:"trip"`
		Count int    `json:"count"`
	}
	trips := make([]tripRow, 0)
	for rows.Next() {
		var tr tripRow
		if err := rows.Scan(&tr.Trip, &tr.Count); err != nil {
			return err
		}
		trips = append(trips, tr)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return aoEmit(cmd, flags, map[string]any{"source": aoSourceNote, "trips": trips, "count": len(trips)})
}

func showTrip(cmd *cobra.Command, flags *rootFlags, s interface{ DB() *sql.DB }, trip string) error {
	places, err := readTripPlaces(s, trip)
	if err != nil {
		return err
	}
	return aoEmitPlaces(cmd, flags, map[string]any{"trip": trip}, places)
}

func readTripPlaces(s interface{ DB() *sql.DB }, trip string) ([]AOPlace, error) {
	rows, err := s.DB().Query(
		`SELECT place_id, slug, title, location, lat, lng FROM ao_trip_items WHERE trip=? ORDER BY added_at, title`, trip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	places := make([]AOPlace, 0)
	for rows.Next() {
		var p AOPlace
		var slug, title, location sql.NullString
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&p.ID, &slug, &title, &location, &lat, &lng); err != nil {
			return nil, err
		}
		p.Slug = slug.String
		p.Title = title.String
		p.Location = location.String
		p.Lat = lat.Float64
		p.Lng = lng.Float64
		p.URL = absoluteAOURL("/places/" + p.Slug)
		places = append(places, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return places, nil
}

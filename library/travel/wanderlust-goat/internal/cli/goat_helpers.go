package cli

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/goatstore"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/walking"
)

// openGoatStore opens the shared goatstore at the canonical path. Used by
// every offline-aware compound command (sync-city, golden-hour, why,
// reddit-quotes, coverage, etc.).
func openGoatStore(cmd *cobra.Command, flags *rootFlags) (*goatstore.Store, error) {
	path := goatstore.DefaultPath("wanderlust-goat-pp-cli")
	return goatstore.Open(cmd.Context(), path)
}

// parseDate accepts YYYY-MM-DD or empty (= today UTC).
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now().UTC(), nil
	}
	return time.Parse("2006-01-02", s)
}

// AnchorResolution is the project-wide alias for dispatch.AnchorResolution.
// Several v2 commands use the unqualified name in JSON shapes for back-
// compat with v1's documented JSON contract.
type AnchorResolution = dispatch.AnchorResolution

// resolveAnchor wraps dispatch.ResolveAnchor for command-package callers.
func resolveAnchor(ctx context.Context, anchor string) (dispatch.AnchorResolution, error) {
	return dispatch.ResolveAnchor(ctx, anchor)
}

// haversineMeters and walking-time helpers are thin wrappers over
// internal/walking. Kept as cli-local funcs so v1 command files compile
// without rewriting every call site.
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	return walking.HaversineMeters(walking.LatLng{Lat: lat1, Lng: lng1}, walking.LatLng{Lat: lat2, Lng: lng2})
}

func metersToWalkingMinutes(meters float64) float64 {
	return walking.MinutesFromMeters(meters)
}

func walkingMinutesToMeters(minutes int) float64 {
	return walking.MetersFromMinutes(float64(minutes))
}

// userAgent returns the contact-bearing UA string Nominatim's policy
// requires (and which the v2 sources use as a polite default). Override
// with WANDERLUST_GOAT_UA.
func userAgent() string {
	if ua := os.Getenv("WANDERLUST_GOAT_UA"); ua != "" {
		return ua
	}
	return "wanderlust-goat-pp-cli/0.2 (+https://github.com/joeheitzeberg/wanderlust-goat)"
}

// appendUnique appends each `more` to s, preserving order, dropping duplicates.
func appendUnique(s []string, more ...string) []string {
	seen := map[string]bool{}
	for _, x := range s {
		seen[x] = true
	}
	for _, x := range more {
		if !seen[x] {
			s = append(s, x)
			seen[x] = true
		}
	}
	return s
}

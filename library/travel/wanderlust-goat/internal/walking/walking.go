// Package walking computes walking distances and time-to-radius mappings.
// Crow-flies meters are wrong for "what's a 15-minute walk" — a city grid
// has detours, traffic lights, and street geometry that adds ~30% to the
// straight-line distance. v2 uses 4.5 km/h × 1.3 tortuosity.
package walking

import "math"

// PaceKMH is the brief's chosen walking pace: 4.5 km/h.
const PaceKMH = 4.5

// Tortuosity is the multiplier that converts crow-flies meters to actual
// walked meters in a typical urban grid. 1.3 is the brief's value.
const Tortuosity = 1.3

// LatLng is a (lat, lng) pair in decimal degrees.
type LatLng struct {
	Lat float64
	Lng float64
}

// HaversineMeters returns the great-circle distance in meters between two
// (lat, lng) pairs. Standard haversine formula; Earth radius 6_371_000 m.
func HaversineMeters(a, b LatLng) float64 {
	const earthRadius = 6_371_000.0
	rad := math.Pi / 180.0
	lat1 := a.Lat * rad
	lat2 := b.Lat * rad
	dlat := (b.Lat - a.Lat) * rad
	dlng := (b.Lng - a.Lng) * rad
	h := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlng/2)*math.Sin(dlng/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadius * c
}

// MinutesFromMeters converts crow-flies meters to walking minutes,
// applying the tortuosity multiplier and the configured pace.
//
//	minutes = (meters * Tortuosity) / (PaceKMH * 1000) * 60
func MinutesFromMeters(meters float64) float64 {
	if meters <= 0 {
		return 0
	}
	walkedMeters := meters * Tortuosity
	hours := walkedMeters / (PaceKMH * 1000.0)
	return hours * 60.0
}

// MetersFromMinutes converts walking minutes to a crow-flies radius in
// meters. Inverse of MinutesFromMeters.
//
//	meters = (minutes / 60) * PaceKMH * 1000 / Tortuosity
func MetersFromMinutes(minutes float64) float64 {
	if minutes <= 0 {
		return 0
	}
	hours := minutes / 60.0
	walkedMeters := hours * PaceKMH * 1000.0
	return walkedMeters / Tortuosity
}

// MinutesBetween is convenience: distance + pace in one call.
func MinutesBetween(a, b LatLng) float64 {
	return MinutesFromMeters(HaversineMeters(a, b))
}

// WithinMinutes reports whether b is within the requested walking-minutes
// radius of a.
func WithinMinutes(a, b LatLng, minutes float64) bool {
	return MinutesBetween(a, b) <= minutes
}

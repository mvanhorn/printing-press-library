// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestHotelDistanceKMUsesCoordinates(t *testing.T) {
	t.Parallel()

	paris := compareHotel{Latitude: 48.8566, Longitude: 2.3522}
	london := compareHotel{Latitude: 51.5074, Longitude: -0.1278}

	got := hotelDistanceKM(paris, london)
	if got < 330 || got > 360 {
		t.Fatalf("hotelDistanceKM Paris/London = %.2f, want about 344km", got)
	}
}

func TestHotelDistanceKMMissingCoordinates(t *testing.T) {
	t.Parallel()

	got := hotelDistanceKM(compareHotel{}, compareHotel{Latitude: 51.5074, Longitude: -0.1278})
	if got != 0 {
		t.Fatalf("hotelDistanceKM with missing coordinates = %.2f, want 0", got)
	}
}

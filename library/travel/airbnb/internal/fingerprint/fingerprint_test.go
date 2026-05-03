package fingerprint

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/airbnb"
)

func TestFromAirbnb(t *testing.T) {
	tests := []struct {
		name string
		in   *airbnb.Listing
		want map[string]string
	}{
		{
			name: "happy path",
			in:   &airbnb.Listing{City: " South Lake Tahoe ", Coordinate: &airbnb.Coord{Latitude: 38.93493, Longitude: -120.01589}, Beds: 3, Baths: 2.5, SleepsMax: 6},
			want: map[string]string{"city": "south lake tahoe", "lat": "38.935", "lng": "-120.016", "beds": "3", "baths": "2.5", "sleeps_max": "6"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromAirbnb(tt.in)
			for k, want := range tt.want {
				if got.Components[k] != want {
					t.Fatalf("Components[%q] = %q, want %q", k, got.Components[k], want)
				}
			}
			if got.Hash == "" {
				t.Fatalf("Hash is empty")
			}
		})
	}
}

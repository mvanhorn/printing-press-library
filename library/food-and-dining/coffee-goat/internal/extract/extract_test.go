// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package extract

import "testing"

func TestFromBody(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Attributes
	}{
		{
			name: "key-value form",
			in:   "Origin: Ethiopia\nProducer: Worka Sakaro\nProcess: Natural\nVarietal: Heirloom\nAltitude: 1900-2100 masl",
			want: Attributes{Origin: "Ethiopia", Producer: "Worka Sakaro", Process: "natural", Varietal: "Heirloom", Altitude: "1900-2100 masl"},
		},
		{
			name: "vocabulary fallback",
			in:   "Bright, juicy Colombian washed Caturra with floral notes.",
			want: Attributes{Origin: "Colombia", Process: "washed", Varietal: "Caturra"},
		},
		{
			name: "altitude alone",
			in:   "Grown at 1800 masl in the hills.",
			want: Attributes{Altitude: "1800 masl"},
		},
		{
			name: "empty body",
			in:   "",
			want: Attributes{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FromBody(c.in)
			if got.Origin != c.want.Origin {
				t.Errorf("Origin = %q, want %q", got.Origin, c.want.Origin)
			}
			if got.Producer != c.want.Producer {
				t.Errorf("Producer = %q, want %q", got.Producer, c.want.Producer)
			}
			if got.Process != c.want.Process {
				t.Errorf("Process = %q, want %q", got.Process, c.want.Process)
			}
			if got.Varietal != c.want.Varietal {
				t.Errorf("Varietal = %q, want %q", got.Varietal, c.want.Varietal)
			}
			if got.Altitude != c.want.Altitude {
				t.Errorf("Altitude = %q, want %q", got.Altitude, c.want.Altitude)
			}
		})
	}
}

func TestCleanupStripsHTML(t *testing.T) {
	in := `<p>Origin: <strong>Ethiopia</strong>.</p> <br/> <em>Heirloom</em>.`
	got := Cleanup(in)
	want := "Origin: Ethiopia . Heirloom ."
	if got != want {
		t.Errorf("Cleanup() = %q, want %q", got, want)
	}
}

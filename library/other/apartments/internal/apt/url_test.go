package apt

import "testing"

func TestBuildSearchURL(t *testing.T) {
	cases := []struct {
		name string
		opts SearchOptions
		want string
	}{
		{
			name: "city+state only",
			opts: SearchOptions{City: "austin", State: "tx"},
			want: "/austin-tx/",
		},
		{
			name: "exact beds",
			opts: SearchOptions{City: "austin", State: "tx", Beds: 2},
			want: "/austin-tx/2-bedrooms/",
		},
		{
			name: "min beds + price max",
			opts: SearchOptions{City: "austin", State: "tx", BedsMin: 2, PriceMax: 2500},
			want: "/austin-tx/min-2-bedrooms-under-2500/",
		},
		{
			name: "exact beds + price range + pets dog",
			opts: SearchOptions{City: "new-york", State: "ny", Beds: 2, PriceMin: 2000, PriceMax: 3500, Pets: "dog"},
			want: "/new-york-ny/2-bedrooms-2000-to-3500-pet-friendly-dog/",
		},
		{
			name: "zip + min beds + pets both",
			opts: SearchOptions{Zip: "78704", BedsMin: 1, Pets: "both"},
			want: "/78704/min-1-bedrooms-pet-friendly-cat-or-dog/",
		},
		{
			name: "house prefix + exact beds",
			opts: SearchOptions{City: "austin", State: "tx", Type: "house", Beds: 3},
			want: "/houses/austin-tx/3-bedrooms/",
		},
		{
			name: "page 3",
			opts: SearchOptions{City: "austin", State: "tx", Page: 3},
			want: "/austin-tx/3/",
		},
		{
			name: "studio",
			opts: SearchOptions{City: "austin", State: "tx", Studio: true},
			want: "/austin-tx/studio/",
		},
		{
			name: "page 1 omitted",
			opts: SearchOptions{City: "austin", State: "tx", Page: 1},
			want: "/austin-tx/",
		},
		{
			name: "condo + price range",
			opts: SearchOptions{City: "austin", State: "tx", Type: "condo", PriceMin: 1500, PriceMax: 2500},
			want: "/condos/austin-tx/1500-to-2500/",
		},
		{
			name: "any pets",
			opts: SearchOptions{City: "austin", State: "tx", Pets: "any"},
			want: "/austin-tx/pet-friendly/",
		},
		{
			name: "min baths",
			opts: SearchOptions{City: "austin", State: "tx", BathsMin: 2},
			want: "/austin-tx/min-2-bathrooms/",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildSearchURL(tc.opts)
			if got != tc.want {
				t.Errorf("BuildSearchURL(%+v)\n  got=%q\n want=%q", tc.opts, got, tc.want)
			}
		})
	}
}

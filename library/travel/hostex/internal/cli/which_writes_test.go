package cli

import "testing"

// The calendar write commands must be discoverable through `which` by a
// natural-language query, ahead of the read-side novel commands.
func TestWhichRanksCalendarWritesFirst(t *testing.T) {
	cases := map[string]string{
		"update prices of a listing":  "listings update-prices",
		"update restrictions":         "listings update-restrictions",
		"update inventories of a day": "listings update-inventories",
		"update price":                "listings update-prices",
		"update restriction":          "listings update-restrictions",
		"update inventory":            "listings update-inventories",
	}
	for q, want := range cases {
		got := rankWhich(whichIndex, q, 3)
		if len(got) == 0 || got[0].Entry.Command != want {
			t.Errorf("which %q: top match = %+v, want %q", q, got, want)
		}
	}
}

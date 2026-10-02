package cli

import "testing"

func TestProviderCapabilitiesDiscoverable(t *testing.T) {
	for _, tc := range []struct{ query, command string }{{"conditions", "bus conditions"}, {"baggage", "bus conditions"}, {"fares", "bus quote"}} {
		matches := rankWhich(whichIndex, tc.query, 1)
		if len(matches) != 1 || matches[0].Entry.Command != tc.command {
			t.Fatalf("%s -> %+v", tc.query, matches)
		}
	}
}

package cli

import "testing"

func TestExtractVenueParts(t *testing.T) {
	cases := []struct {
		in   string
		want venueParts
	}{
		{"northside hall", venueParts{Complex: "northside hall"}},
		{"northside hall (main room)", venueParts{Complex: "northside hall", Room: "main room"}},
		{"northside hall - courtyard", venueParts{Complex: "northside hall", Room: "courtyard"}},
		{"the rooftop at northside hall", venueParts{Complex: "northside hall", Room: "rooftop"}},
		{"northside hall [room a + room b]", venueParts{Complex: "northside hall", Room: "room a + room b"}},
	}
	for _, c := range cases {
		got := extractVenueParts(c.in)
		if got != c.want {
			t.Errorf("extractVenueParts(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

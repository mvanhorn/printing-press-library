package cli

import "testing"

func TestXUserIDFromTWID(t *testing.T) {
	tests := []struct {
		name string
		twid string
		want string
	}{
		{name: "plain escaped", twid: "u%3D1234567890", want: "1234567890"},
		{name: "plain equals", twid: "u=1234567890", want: "1234567890"},
		{name: "quoted escaped", twid: "\"u%3D1234567890\"", want: "1234567890"},
		{name: "url encoded quotes", twid: "%22u%3D1234567890%22", want: "1234567890"},
		{name: "reject non numeric", twid: "u%3Dabc123", want: ""},
		{name: "reject empty", twid: "", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := xUserIDFromTWID(tc.twid); got != tc.want {
				t.Fatalf("xUserIDFromTWID(%q) = %q, want %q", tc.twid, got, tc.want)
			}
		})
	}
}

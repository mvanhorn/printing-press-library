package cli

import "testing"

func TestCDPTargetHost(t *testing.T) {
	cases := map[string]string{
		"https://app.gusto.com":       "app.gusto.com",
		"app.gusto.com":               "app.gusto.com",
		"https://APP.gusto.com/login": "app.gusto.com",
	}
	for in, want := range cases {
		if got := cdpTargetHost(in); got != want {
			t.Fatalf("cdpTargetHost(%q) = %q, want %q", in, got, want)
		}
	}
}

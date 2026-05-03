package customertier

import (
	"testing"
)

func TestRewriteURL(t *testing.T) {
	cases := []struct {
		name, env, in, wantURL string
		wantRewrote            bool
	}{
		{"free tier passthrough when no key", "", "https://api.open-meteo.com/v1/forecast?latitude=47", "https://api.open-meteo.com/v1/forecast?latitude=47", false},
		{"free host swapped to customer when key set", "secret", "https://api.open-meteo.com/v1/forecast?latitude=47", "https://customer-api.open-meteo.com/v1/forecast?apikey=secret&latitude=47", true},
		{"archive host swapped", "k1", "https://archive-api.open-meteo.com/v1/archive?start_date=2024-01-01", "https://customer-archive-api.open-meteo.com/v1/archive?apikey=k1&start_date=2024-01-01", true},
		{"marine host swapped", "k1", "https://marine-api.open-meteo.com/v1/marine", "https://customer-marine-api.open-meteo.com/v1/marine?apikey=k1", true},
		{"air-quality host swapped", "k1", "https://air-quality-api.open-meteo.com/v1/air-quality", "https://customer-air-quality-api.open-meteo.com/v1/air-quality?apikey=k1", true},
		{"already customer host adds apikey only", "k1", "https://customer-api.open-meteo.com/v1/forecast", "https://customer-api.open-meteo.com/v1/forecast?apikey=k1", true},
		{"third-party host passthrough", "k1", "https://example.com/api", "https://example.com/api", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(EnvVar, c.env)
			gotURL, gotRewrote := RewriteURL(c.in)
			if gotURL != c.wantURL {
				t.Errorf("RewriteURL(%q) URL = %q, want %q", c.in, gotURL, c.wantURL)
			}
			if gotRewrote != c.wantRewrote {
				t.Errorf("RewriteURL(%q) rewrote = %v, want %v", c.in, gotRewrote, c.wantRewrote)
			}
		})
	}
}

func TestActive(t *testing.T) {
	t.Setenv(EnvVar, "")
	if Active() {
		t.Error("Active() should be false when env unset")
	}
	t.Setenv(EnvVar, "secret")
	if !Active() {
		t.Error("Active() should be true when env set")
	}
}

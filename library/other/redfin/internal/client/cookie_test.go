package client

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/config"
)

func TestCookieHeaderFor(t *testing.T) {
	cfg := &config.Config{CookieHeader: "_abck=a1"}
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.redfin.com/stingray/api/gis", "_abck=a1"},
		{"https://redfin.com/x", "_abck=a1"},
		{"http://www.redfin.com/x", ""},
		{"https://notredfin.com/x", ""},
		{"https://www.redfin.com.example.com/x", ""},
		{"http://127.0.0.1:8080/x", ""},
	}
	for _, tt := range tests {
		if got := cookieHeaderFor(tt.url, cfg); got != tt.want {
			t.Errorf("cookieHeaderFor(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
	if got := cookieHeaderFor("https://www.redfin.com/x", &config.Config{}); got != "" {
		t.Errorf("no stored cookies: got %q", got)
	}
}

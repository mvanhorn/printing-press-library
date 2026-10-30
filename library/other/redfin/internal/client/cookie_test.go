package client

import (
	"bytes"
	"io"
	"os"
	"strings"
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

func TestDryRunRedactsCookie(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := os.Stderr
	os.Stderr = w
	c := &Client{Config: &config.Config{CookieHeader: "_abck=secretvalue"}}
	_, _, derr := c.dryRun("GET", "https://www.redfin.com/x", "/x", nil, nil, nil, "")
	os.Stderr = stderr
	w.Close()
	if derr != nil {
		t.Fatalf("dryRun: %v", derr)
	}
	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "alue") || !strings.Contains(out.String(), "[REDACTED]") {
		t.Fatalf("dry run output leaked cookie text: %q", out.String())
	}
}

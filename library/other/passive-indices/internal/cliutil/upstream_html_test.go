package cliutil

import (
	"testing"
	"time"
)

func TestLooksLikeHTML(t *testing.T) {
	cases := []struct {
		name string
		ct   string
		body string
		want bool
	}{
		{"html content type", "text/html; charset=utf-8", "anything", true},
		{"html body without content type", "application/octet-stream", "  \n<!DOCTYPE html><html>", true},
		{"real csv", "application/octet-stream", "Company Name,Industry,Symbol,Series,ISIN Code\n", false},
		{"json", "application/json", `{"a":1}`, false},
		{"empty", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LooksLikeHTML(tc.ct, []byte(tc.body)); got != tc.want {
				t.Errorf("LooksLikeHTML = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUpstreamHTMLBackoffBounds(t *testing.T) {
	for attempt := 0; attempt < 40; attempt++ {
		d := UpstreamHTMLBackoff(attempt)
		if d < UpstreamHTMLBackoffBase/2 || d > UpstreamHTMLBackoffMax {
			t.Fatalf("attempt %d: backoff %s outside [%s, %s]", attempt, d, UpstreamHTMLBackoffBase/2, UpstreamHTMLBackoffMax)
		}
	}
	if d := UpstreamHTMLBackoff(30); d < 5*time.Second {
		t.Errorf("late attempts should sit near the cap, got %s", d)
	}
}

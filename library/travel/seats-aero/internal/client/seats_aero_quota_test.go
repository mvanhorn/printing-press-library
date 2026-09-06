package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/seats-aero/internal/config"
)

func isolateNovelTest(t *testing.T) {
	t.Helper()
	t.Setenv("SEATS_AERO_USER_AGENT", "")
}

func TestParseQuotaHeaders(t *testing.T) {
	isolateNovelTest(t)
	headers := make(http.Header)
	headers.Set("x-ratelimit-limit", "1000")
	headers.Set("X-RATELIMIT-REMAINING", "998")
	headers.Set("X-RateLimit-Reset", "42")
	if got, want := ParseQuotaHeaders(headers), (Quota{Limit: 1000, Remaining: 998, ResetSeconds: 42, Observed: true}); got != want {
		t.Fatalf("ParseQuotaHeaders() = %+v, want %+v", got, want)
	}
	if got := ParseQuotaHeaders(http.Header{}); got != (Quota{}) {
		t.Fatalf("empty headers = %+v", got)
	}
}

func TestProbeQuotaStatusesAndHeaders(t *testing.T) {
	isolateNovelTest(t)
	tests := []struct {
		name         string
		status       int
		headers      bool
		wantErr      string
		wantObserved bool
	}{
		{name: "success observed", status: http.StatusOK, headers: true, wantObserved: true},
		{name: "other 4xx observed", status: http.StatusBadRequest, headers: true, wantObserved: true},
		{name: "server error", status: http.StatusInternalServerError, wantErr: "500"},
		{name: "unauthorized despite headers", status: http.StatusUnauthorized, headers: true, wantErr: "401"},
		{name: "forbidden despite headers", status: http.StatusForbidden, headers: true, wantErr: "403"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.RequestURI() != probePath {
					t.Errorf("URI=%q", r.URL.RequestURI())
				}
				if got := r.Header.Get("Partner-Authorization"); got != "test-key" {
					t.Errorf("auth=%q", got)
				}
				if got := r.Header.Get("User-Agent"); got != "configured-agent" {
					t.Errorf("User-Agent=%q", got)
				}
				if got := r.Header.Get("Accept"); got != "application/vnd.test+json" {
					t.Errorf("Accept=%q", got)
				}
				if got := r.Header.Get("X-Required"); got != "yes" {
					t.Errorf("X-Required=%q", got)
				}
				if tt.headers {
					w.Header().Set("X-RateLimit-Limit", "1000")
					w.Header().Set("X-RateLimit-Remaining", "997")
					w.Header().Set("X-RateLimit-Reset", "30")
				}
				w.WriteHeader(tt.status)
			}))
			defer server.Close()
			cfg := &config.Config{SeatsAeroApiKey: "test-key", Headers: map[string]string{"User-Agent": "configured-agent", "Accept": "application/vnd.test+json", "X-Required": "yes"}}
			c := &Client{BaseURL: server.URL, Config: cfg, HTTPClient: server.Client()}
			got, err := c.ProbeQuota(context.Background())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err=%v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if got.Observed != tt.wantObserved || got.ProbePath != probePath {
					t.Fatalf("quota=%+v", got)
				}
			}
			if requests != 1 {
				t.Fatalf("requests=%d", requests)
			}
		})
	}
}

func TestProbeQuotaDefaultHeadersAndDryRun(t *testing.T) {
	isolateNovelTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "seats-aero-pp-cli/1.0" {
			t.Errorf("User-Agent=%q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept=%q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	c := &Client{BaseURL: server.URL, Config: &config.Config{SeatsAeroApiKey: "test-key"}, HTTPClient: server.Client()}
	if _, err := c.ProbeQuota(context.Background()); err != nil {
		t.Fatal(err)
	}
	c.DryRun = true
	got, err := c.ProbeQuota(context.Background())
	if err != nil || got != (Quota{ProbePath: probePath}) {
		t.Fatalf("quota=%+v err=%v", got, err)
	}
}

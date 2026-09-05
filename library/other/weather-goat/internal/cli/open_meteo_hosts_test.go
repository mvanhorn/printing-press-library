package cli

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testOpenMeteoClient(t *testing.T, baseURL, body string, check func(*http.Request)) *client.Client {
	t.Helper()
	c := client.New(&config.Config{BaseURL: baseURL}, time.Second, 5)
	c.NoCache = true
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		check(req)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})}
	return c
}

func TestHistoryUsesArchiveAPIHost(t *testing.T) {
	c := testOpenMeteoClient(t, defaultOpenMeteoBaseURL, `{"daily":{}}`, func(req *http.Request) {
		if req.URL.Host != "archive-api.open-meteo.com" {
			t.Fatalf("history host = %q, want archive-api.open-meteo.com", req.URL.Host)
		}
		if req.URL.Path != "/v1/archive" {
			t.Fatalf("history path = %q, want /v1/archive", req.URL.Path)
		}
	})

	flags := &rootFlags{dataSource: "live"}
	if _, _, err := resolveOpenMeteoRead(c, flags, "history", false, "/archive", archiveAPIBaseURL, map[string]string{"latitude": "40.7"}); err != nil {
		t.Fatalf("history request failed: %v", err)
	}
}

func TestAirQualityUsesAirQualityAPIHost(t *testing.T) {
	c := testOpenMeteoClient(t, defaultOpenMeteoBaseURL, `{"current":{"us_aqi":42}}`, func(req *http.Request) {
		if req.URL.Host != "air-quality-api.open-meteo.com" {
			t.Fatalf("air-quality host = %q, want air-quality-api.open-meteo.com", req.URL.Host)
		}
		if req.URL.Path != "/v1/air-quality" {
			t.Fatalf("air-quality path = %q, want /v1/air-quality", req.URL.Path)
		}
	})

	flags := &rootFlags{dataSource: "live"}
	if _, _, err := resolveOpenMeteoRead(c, flags, "air-quality", false, "/air-quality", airQualityAPIBaseURL, map[string]string{"latitude": "40.7"}); err != nil {
		t.Fatalf("air-quality request failed: %v", err)
	}
}

func TestServiceClientPreservesExplicitBaseURLOverride(t *testing.T) {
	c := testOpenMeteoClient(t, "https://proxy.example/v1", `{}`, func(req *http.Request) {
		if req.URL.Host != "proxy.example" {
			t.Fatalf("host = %q, want proxy.example", req.URL.Host)
		}
	})

	serviceClient := clientForOpenMeteoService(c, archiveAPIBaseURL)
	if serviceClient != c {
		t.Fatal("explicit base URL override should retain the original client")
	}
	if _, err := serviceClient.Get("/archive", nil); err != nil {
		t.Fatalf("custom-base request failed: %v", err)
	}
}

func TestServiceClientPreservesDryRunAndDoesNotSend(t *testing.T) {
	calls := 0
	c := testOpenMeteoClient(t, defaultOpenMeteoBaseURL, `{}`, func(*http.Request) {
		calls++
	})
	c.DryRun = true

	serviceClient := clientForOpenMeteoService(c, archiveAPIBaseURL)
	if !serviceClient.DryRun || serviceClient.HTTPClient != c.HTTPClient || serviceClient.RateLimit() != c.RateLimit() {
		t.Fatal("service client did not preserve client execution settings")
	}
	data, err := serviceClient.Get("/archive", map[string]string{"latitude": "40.7"})
	if err != nil {
		t.Fatalf("dry-run request failed: %v", err)
	}
	if calls != 0 {
		t.Fatalf("dry-run made %d HTTP request(s), want 0", calls)
	}
	if string(data) != `{"dry_run": true}` {
		t.Fatalf("dry-run result = %s, want dry-run marker", data)
	}
}

func TestActivityAQIUsesAirQualityAPIHost(t *testing.T) {
	c := testOpenMeteoClient(t, defaultOpenMeteoBaseURL, `{"current":{"us_aqi":42}}`, func(req *http.Request) {
		if req.URL.Host != "air-quality-api.open-meteo.com" {
			t.Fatalf("activity AQI host = %q, want air-quality-api.open-meteo.com", req.URL.Host)
		}
	})

	if got := fetchAQI(c, 40.7, -74.0); got != 42 {
		t.Fatalf("AQI = %v, want 42", got)
	}
}

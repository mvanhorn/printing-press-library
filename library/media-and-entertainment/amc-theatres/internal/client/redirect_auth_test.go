package client

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/amc-theatres/internal/config"
)

func TestAMCRedirectNeverForwardsCredentialsToAnotherOrigin(t *testing.T) {
	for _, target := range []string{
		"https://other.example/next",
		"http://api.amctheatres.test/next",
	} {
		t.Run(target, func(t *testing.T) {
			c := New(&config.Config{BaseURL: "https://api.amctheatres.test"}, time.Second, 0)
			calls := 0
			c.HTTPClient.Transport = writeSafetyTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				if calls > 1 {
					t.Errorf("redirected request reached %s", req.URL.Host)
				}
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{target}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
			})
			req, err := http.NewRequest(http.MethodGet, "https://api.amctheatres.test/start", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("X-AMC-Vendor-Key", "synthetic-vendor")
			req.Header.Set("X-AMC-Auth-Token", "synthetic-user")
			resp, err := c.HTTPClient.Do(req)
			if resp != nil {
				_ = resp.Body.Close()
			}
			if err == nil || !strings.Contains(err.Error(), "different origin") || calls != 1 {
				t.Fatalf("redirect to %s: err=%v calls=%d, want refusal before second request", target, err, calls)
			}
		})
	}
}

func TestAMCRedirectWithinOriginKeepsUserToken(t *testing.T) {
	c := New(&config.Config{BaseURL: "https://api.amctheatres.test"}, time.Second, 0)
	calls := 0
	c.HTTPClient.Transport = writeSafetyTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"/next"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
		}
		if req.URL.Path != "/next" || req.Header.Get("X-AMC-Auth-Token") != "synthetic-user" {
			t.Error("same-origin redirect lost the user token or changed path")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Request: req}, nil
	})
	req, err := http.NewRequest(http.MethodGet, "https://api.amctheatres.test/start", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-AMC-Auth-Token", "synthetic-user")
	resp, err := c.HTTPClient.Do(req)
	if err != nil || calls != 2 {
		t.Fatalf("same-origin redirect: err=%v calls=%d, want success after two requests", err, calls)
	}
	_ = resp.Body.Close()
}

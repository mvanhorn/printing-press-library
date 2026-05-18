package client

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/namecheap/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// PATCH(namecheap-dry-run-mask): regression coverage for query-param credential masking.
func TestDryRunMasksNamecheapAPIKeyParam(t *testing.T) {
	c := &Client{}
	stderr := captureStderr(t, func() {
		_, _, err := c.dryRun("GET", "https://api.namecheap.com/xml.response", "/xml.response", map[string]string{
			"ApiKey":   "super-secret-key",
			"Command":  "namecheap.domains.check",
			"ClientIp": "203.0.113.10",
		}, nil, nil, "")
		if err != nil {
			t.Fatalf("dryRun returned error: %v", err)
		}
	})
	if strings.Contains(stderr, "super-secret-key") {
		t.Fatalf("dry-run leaked raw API key: %s", stderr)
	}
	if !strings.Contains(stderr, "ApiKey=****-key") {
		t.Fatalf("dry-run should include masked API key, got: %s", stderr)
	}
}

func TestSensitiveNamecheapQueryParams(t *testing.T) {
	sensitive := []string{"ApiKey", "APIKey", "ApiUser", "UserName", "ClientIp"}
	for _, key := range sensitive {
		if !isSensitiveQueryParam(key) {
			t.Fatalf("%s should be treated as sensitive", key)
		}
	}
	if isSensitiveQueryParam("DomainName") {
		t.Fatal("DomainName should not be treated as sensitive")
	}
}

// PATCH(namecheap-client-ip-cache): regression coverage for one public-IP lookup per Client.
func TestPrepareNamecheapRequestCachesDetectedClientIP(t *testing.T) {
	var calls int32
	cfg := &config.Config{APIUser: "user", APIKey: "key"}
	c := New(cfg, time.Second, 0)
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.ipify.org" {
			t.Fatalf("unexpected request host: %s", r.URL.Host)
		}
		atomic.AddInt32(&calls, 1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("198.51.100.42")),
			Header:     make(http.Header),
		}, nil
	})}

	for i := 0; i < 2; i++ {
		_, params, err := c.prepareNamecheapRequest("/xml.response/domains/check", map[string]string{"DomainList": "example.com"})
		if err != nil {
			t.Fatalf("prepareNamecheapRequest call %d returned error: %v", i+1, err)
		}
		if params["ClientIp"] != "198.51.100.42" {
			t.Fatalf("ClientIp = %q, want detected public IP", params["ClientIp"])
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("detectClientIP should call ipify once per Client, got %d calls", got)
	}
}

// PATCH(namecheap-api-error-status): regression coverage for XML Status=ERROR becoming a Go error.
func TestNamecheapXMLStatusErrorReturnsError(t *testing.T) {
	body := []byte(`<ApiResponse Status="ERROR"><Errors><Error Number="1010101">Parameter ApiUser is missing</Error></Errors></ApiResponse>`)
	converted, ok := convertNamecheapXMLToJSON(body)
	if !ok {
		t.Fatal("expected XML body to convert to JSON")
	}
	err := detectNamecheapAPIError("GET", "/xml.response", converted)
	if err == nil {
		t.Fatal("expected Namecheap Status=ERROR to produce an error")
	}
	var namecheapErr *NamecheapAPIError
	if !errors.As(err, &namecheapErr) {
		t.Fatalf("error type = %T, want *NamecheapAPIError", err)
	}
	if !strings.Contains(namecheapErr.Body, "Parameter ApiUser is missing") {
		var decoded map[string]any
		_ = json.Unmarshal(converted, &decoded)
		t.Fatalf("error body should include Namecheap API message, got %s decoded=%v", namecheapErr.Body, decoded)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stderr = w
	fn()
	_ = w.Close()
	os.Stderr = old
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading stderr pipe: %v", err)
	}
	_ = r.Close()
	return string(data)
}

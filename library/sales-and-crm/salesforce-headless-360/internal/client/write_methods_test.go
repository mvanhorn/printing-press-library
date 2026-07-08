package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/config"
)

func TestPatchWithResponseHeadersSendsCallerHeaders(t *testing.T) {
	c := New(&config.Config{
		BaseURL:               "https://mock.salesforce.test",
		SalesforceInstanceUrl: "https://mock.salesforce.test",
		AccessToken:           "test-token",
	}, 2*time.Second, 0)
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		if got := r.Header.Get("If-Match"); got != "2026-04-22T18:00:00.000Z" {
			t.Fatalf("If-Match = %q", got)
		}
		return clientTestResponse(t, http.StatusOK, map[string]any{"success": true}, http.Header{
			"Sforce-Limit-Info": []string{"api-usage=100/100000"},
			"Lastmodifieddate":  []string{"2026-04-22T18:42:00.000Z"},
		}), nil
	})}

	body, status, headers, err := c.PatchWithResponseHeaders(
		"/services/data/v63.0/sobjects/Account/001ACME0001",
		map[string]any{"Name": "Acme Updated"},
		map[string]string{"If-Match": "2026-04-22T18:00:00.000Z"},
	)
	if err != nil {
		t.Fatalf("PatchWithResponseHeaders error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if headers.Get("LastModifiedDate") != "2026-04-22T18:42:00.000Z" {
		t.Fatalf("LastModifiedDate = %q", headers.Get("LastModifiedDate"))
	}
	if headers.Get("Sforce-Limit-Info") != "api-usage=100/100000" {
		t.Fatalf("Sforce-Limit-Info = %q", headers.Get("Sforce-Limit-Info"))
	}
	if string(body) != `{"success":true}` {
		t.Fatalf("body = %s", body)
	}
}

func TestPatchWithHeadersSendsCallerHeaders(t *testing.T) {
	c := New(&config.Config{
		BaseURL:               "https://mock.salesforce.test",
		SalesforceInstanceUrl: "https://mock.salesforce.test",
		AccessToken:           "test-token",
	}, 2*time.Second, 0)
	c.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("If-Match"); got != "etag-1" {
			t.Fatalf("If-Match = %q", got)
		}
		return clientTestResponse(t, http.StatusOK, map[string]any{"success": true}, nil), nil
	})}

	_, status, err := c.PatchWithHeaders(
		"/services/data/v63.0/sobjects/Account/001ACME0001",
		map[string]any{"Name": "Acme Updated"},
		map[string]string{"If-Match": "etag-1"},
	)
	if err != nil {
		t.Fatalf("Patch error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func clientTestResponse(t *testing.T, status int, value any, headers http.Header) *http.Response {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test response: %v", err)
	}
	if headers == nil {
		headers = http.Header{}
	}
	headers.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: status,
		Header:     headers,
		Body:       io.NopCloser(bytes.NewReader(data)),
	}
}

package datacloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchProfileHappyPath(t *testing.T) {
	var requestedPath string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestedPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer offcore-token" {
			t.Fatalf("missing offcore authorization header: %q", r.Header.Get("Authorization"))
		}
		return jsonResponse(http.StatusOK, `{"unifiedProfileId":"up_mock_001","segments":["renewal-risk-low"]}`), nil
	})}

	c := &fakeCoreClient{responses: map[string]fakeResponse{
		tokenPath: {status: http.StatusOK, body: json.RawMessage(`{"access_token":"offcore-token","instance_url":"https://data-cloud.test"}`)},
	}}

	result, err := Fetch(context.Background(), c, "001ACME0001", FetchOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !result.Availability.Available {
		t.Fatalf("expected available: %+v", result.Availability)
	}
	if result.DMO != UnifiedAccountDMO {
		t.Fatalf("expected default DMO, got %s", result.DMO)
	}
	if result.Profile["unifiedProfileId"] != "up_mock_001" {
		t.Fatalf("unexpected profile: %#v", result.Profile)
	}
	if !strings.Contains(requestedPath, "/api/v1/profile/UnifiedAccount__dlm/001ACME0001") {
		t.Fatalf("unexpected profile path: %s", requestedPath)
	}
}

func TestFetchProfileDetectsFallbackDMO(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, UnifiedAccountDMO) {
			return jsonResponse(http.StatusNotFound, `{"error":"not_found"}`), nil
		}
		return jsonResponse(http.StatusOK, `{"unifiedProfileId":"up_individual_001"}`), nil
	})}

	c := &fakeCoreClient{responses: map[string]fakeResponse{
		tokenPath: {status: http.StatusOK, body: json.RawMessage(`{"access_token":"offcore-token","instance_url":"https://data-cloud.test"}`)},
	}}

	result, err := Fetch(context.Background(), c, "001ACME0001", FetchOptions{HTTPClient: httpClient})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if result.DMO != UnifiedIndividualDMO {
		t.Fatalf("expected fallback DMO, got %s", result.DMO)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

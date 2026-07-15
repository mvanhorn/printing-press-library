// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestGetJSONRedactsSensitiveQueryParamsInErrors(t *testing.T) {
	previousClient := httpClient
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Status:     "401 Unauthorized",
			Body:       io.NopCloser(strings.NewReader("bad key")),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	defer func() { httpClient = previousClient }()

	_, err := getJSON(context.Background(), "https://api.example.test/data", url.Values{
		"key":    []string{"census-secret"},
		"UserID": []string{"bea-secret"},
		"get":    []string{"NAME,DP05_0001E"},
	}, nil)
	if err == nil {
		t.Fatal("expected non-2xx error")
	}

	msg := err.Error()
	if strings.Contains(msg, "census-secret") || strings.Contains(msg, "bea-secret") {
		t.Fatalf("error leaked query secret: %s", msg)
	}
	if !strings.Contains(msg, "key=REDACTED") || !strings.Contains(msg, "UserID=REDACTED") {
		t.Fatalf("error did not show redacted query params: %s", msg)
	}
}

func TestHTTPClientDoesNotCapCommandTimeout(t *testing.T) {
	if httpClient.Timeout != 0 {
		t.Fatalf("http client timeout = %s; command context should own request deadlines", httpClient.Timeout)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

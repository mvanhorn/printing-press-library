// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/listingview/internal/client"
)

func TestIsNetworkErrorExcludesProviderAPIErrors(t *testing.T) {
	apiErr := &client.APIError{
		Method:     "GET",
		Path:       "/keywords",
		StatusCode: 502,
		Body:       "upstream connection refused",
	}
	for _, err := range []error{apiErr, fmt.Errorf("wrapped: %w", apiErr)} {
		if isNetworkError(err) {
			t.Fatalf("isNetworkError(%v) = true for typed provider API error", err)
		}
	}
}

func TestIsNetworkErrorRecognizesTransportFailures(t *testing.T) {
	for _, err := range []error{
		errors.New("dial tcp: connection refused"),
		&url.Error{Op: "Get", URL: "https://example.test", Err: errors.New("network is unreachable")},
	} {
		if !isNetworkError(err) {
			t.Fatalf("isNetworkError(%v) = false for transport failure", err)
		}
	}
}

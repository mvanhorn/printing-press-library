// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestSanitizedURLForErrorRedactsAPIKeys(t *testing.T) {
	endpoint, err := url.Parse("https://api.regulations.gov/v4/documents?api_key=secret&filter%5BsearchTerm%5D=water")
	if err != nil {
		t.Fatalf("url.Parse returned error: %v", err)
	}
	got := sanitizedURLForError(endpoint)
	if strings.Contains(got, "secret") {
		t.Fatalf("sanitized URL leaked secret: %s", got)
	}
	if !strings.Contains(got, "api_key=REDACTED") {
		t.Fatalf("sanitized URL did not redact api_key: %s", got)
	}
}

func TestHTTPClientDoesNotCapCommandTimeout(t *testing.T) {
	if httpClient.Timeout != 0*time.Second {
		t.Fatalf("httpClient.Timeout = %s, want 0 so command context owns deadlines", httpClient.Timeout)
	}
}

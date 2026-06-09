// Copyright 2026 Brian Wishan and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import "testing"

func TestCookieDomainFromBaseURL(t *testing.T) {
	tests := []struct {
		baseURL, want string
	}{
		{"https://www.amazon.com", ".amazon.com"},
		{"https://www.amazon.ca", ".amazon.ca"},
		{"https://www.amazon.co.uk", ".amazon.co.uk"},
		{"https://amazon.de", ".amazon.de"},
		{"http://127.0.0.1:8799", ".127.0.0.1"},
		{"", ".amazon.com"},
		{"://broken", ".amazon.com"},
	}
	for _, tt := range tests {
		if got := cookieDomainFromBaseURL(tt.baseURL); got != tt.want {
			t.Errorf("cookieDomainFromBaseURL(%q) = %q, want %q", tt.baseURL, got, tt.want)
		}
	}
}

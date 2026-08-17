// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package glssh

import "testing"

func TestResolveConfigHostFromBaseURL(t *testing.T) {
	t.Setenv("GL_INET_PASSWORD", "x")
	t.Setenv("GL_INET_SSH_HOST", "")
	t.Setenv("GL_INET_SSH_USER", "")
	t.Setenv("GL_INET_SSH_PORT", "")
	t.Setenv("GL_INET_SSH_KEY", "")
	cases := []struct {
		baseURL  string
		wantHost string
	}{
		{"http://192.168.8.1", "192.168.8.1"},
		{"https://192.168.8.1/", "192.168.8.1"},
		{"http://192.168.8.1:8080", "192.168.8.1"},
		{"http://router.lan/rpc", "router.lan"},
	}
	for _, tc := range cases {
		cfg, err := ResolveConfig(tc.baseURL)
		if err != nil {
			t.Fatalf("ResolveConfig(%q) error: %v", tc.baseURL, err)
		}
		if cfg.Host != tc.wantHost {
			t.Errorf("ResolveConfig(%q).Host = %q, want %q", tc.baseURL, cfg.Host, tc.wantHost)
		}
		if cfg.Port != 22 {
			t.Errorf("ResolveConfig(%q).Port = %d, want 22", tc.baseURL, cfg.Port)
		}
		if cfg.User != "root" {
			t.Errorf("ResolveConfig(%q).User = %q, want root", tc.baseURL, cfg.User)
		}
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"network":      "'network'",
		"a b":          "'a b'",
		"it's":         `'it'\''s'`,
		"wireless.cfg": "'wireless.cfg'",
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

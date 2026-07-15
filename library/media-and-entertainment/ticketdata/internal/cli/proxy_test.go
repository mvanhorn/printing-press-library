// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestParseProxyParams(t *testing.T) {
	tests := []struct {
		name    string
		raw     []string
		want    map[string]string
		wantErr bool
	}{
		{
			name: "repeatable params",
			raw:  []string{"venue_slug=lumen-field", "limit=20", "empty="},
			want: map[string]string{"venue_slug": "lumen-field", "limit": "20", "empty": ""},
		},
		{
			name:    "missing equals",
			raw:     []string{"venue_slug"},
			wantErr: true,
		},
		{
			name:    "empty key",
			raw:     []string{"=lumen-field"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProxyParams(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseProxyParams error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseProxyParams error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("params = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProxyUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "reject post", args: []string{"POST", "/x"}, wantErr: "GET only"},
		{name: "missing path", args: []string{"GET"}, wantErr: "method and path are required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newProxyCmd(&rootFlags{})
			cmd.SetArgs(tt.args)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("proxy command error = nil, want usage error")
			}
			if ExitCode(err) != 2 {
				t.Fatalf("ExitCode = %d, want 2; err = %v", ExitCode(err), err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNormalizeProxyPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "/search", want: "/search"},
		{in: "search", want: "/search"},
		{in: "  /events/123  ", want: "/events/123"},
		{in: "", want: "/"},
	}

	for _, tt := range tests {
		if got := normalizeProxyPath(tt.in); got != tt.want {
			t.Fatalf("normalizeProxyPath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

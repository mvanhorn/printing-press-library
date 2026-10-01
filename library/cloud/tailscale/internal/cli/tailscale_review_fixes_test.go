// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutesApproveNoChangeExplainsWhy(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"nothing advertised", []string{"routes", "approve", "nas", "--all-advertised", "--json"}, "advertises no routes"},
		{"already enabled", []string{"routes", "approve", "home-mac", "192.168.1.0/24", "--json"}, "already enabled: 192.168.1.0/24"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, srv := newMockServer(t)
			out, _, err := runTS(t, srv, nil, tc.args...)
			if err != nil {
				t.Fatalf("approve: %v", err)
			}
			if len(m.routeWrites) != 0 {
				t.Fatalf("no-change run wrote routes: %v", m.routeWrites)
			}
			var v routeChangeView
			if err := json.Unmarshal([]byte(out), &v); err != nil {
				t.Fatalf("decode: %v\n%s", err, out)
			}
			if v.Mode != "no-change" {
				t.Fatalf("mode=%s, want no-change", v.Mode)
			}
			if !strings.Contains(strings.Join(v.Warnings, "; "), tc.want) {
				t.Fatalf("warnings=%v, want one containing %q", v.Warnings, tc.want)
			}
		})
	}
}

func TestAccessCheckWarnsOnUnknownFromLogin(t *testing.T) {
	m, _ := newMockServer(t)
	base := m.handler(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v2")
		switch {
		case r.Method == http.MethodPost && path == "/tailnet/-/acl/preview":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"matches": []map[string]any{{"users": []string{"*"}, "ports": []string{"*:*"}, "lineNumber": 6}}})
		case r.Method == http.MethodGet && path == "/tailnet/-/users":
			if r.URL.Query().Get("type") != "all" {
				t.Errorf("users lookup must include shared-in users, got type=%q", r.URL.Query().Get("type"))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"users": []map[string]any{{"loginName": "alice@example.com"}}})
		default:
			base.ServeHTTP(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	check := func(from string) accessPreviewView {
		t.Helper()
		out, _, err := runTS(t, srv, nil, "access", "check", "--to", "100.64.0.1:22", "--from", from, "--json")
		if err != nil {
			t.Fatalf("access check --from %s: %v", from, err)
		}
		var v accessPreviewView
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("decode: %v\n%s", err, out)
		}
		return v
	}

	if v := check("nobody@example.com"); !strings.Contains(strings.Join(v.Warnings, "; "), "not a member or shared-in user") {
		t.Fatalf("unknown login: warnings=%v, want an unknown-user warning", v.Warnings)
	}
	if v := check("Alice@example.com"); len(v.Warnings) != 0 {
		t.Fatalf("known login (case-insensitive): warnings=%v, want none", v.Warnings)
	}
}

func TestWholeSecondTime(t *testing.T) {
	cases := map[string]string{
		"2026-10-01T00:49:57.810756478Z": "2026-10-01T00:49:57Z",
		"2026-10-01T00:49:57Z":           "2026-10-01T00:49:57Z",
		"not-a-time":                     "not-a-time",
	}
	for in, want := range cases {
		if got := wholeSecondTime(in); got != want {
			t.Errorf("wholeSecondTime(%q) = %q, want %q", in, got, want)
		}
	}
}

// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestParseContentsOptionsEnforcesRequestSchema(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantNil bool
		wantErr bool
	}{
		{name: "object", raw: `{"highlights":true}`},
		{name: "null", raw: `null`, wantNil: true},
		{name: "boolean", raw: `true`, wantErr: true},
		{name: "array", raw: `[]`, wantErr: true},
		{name: "string", raw: `"text"`, wantErr: true},
		{name: "number", raw: `1`, wantErr: true},
		{name: "invalid JSON", raw: `{`, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseContentsOptions(test.raw)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseContentsOptions(%q) error = %v, wantErr %v", test.raw, err, test.wantErr)
			}
			if !test.wantErr && (got == nil) != test.wantNil {
				t.Fatalf("parseContentsOptions(%q) = %#v, wantNil %v", test.raw, got, test.wantNil)
			}
		})
	}
}

func TestWebsearchContentsRequestContract(t *testing.T) {
	tests := []struct {
		name      string
		contents  string
		wantValue string
		wantErr   bool
	}{
		{name: "object", contents: `{"highlights":true}`, wantValue: `{"highlights":true}`},
		{name: "null", contents: `null`, wantValue: `null`},
		{name: "boolean", contents: `true`, wantErr: true},
		{name: "array", contents: `[]`, wantErr: true},
		{name: "malformed", contents: `{`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			var calls int
			var requestBody map[string]json.RawMessage
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/search" {
					http.Error(w, "unexpected path", http.StatusNotFound)
					return
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode search body: %v", err)
				}
				mu.Lock()
				calls++
				requestBody = body
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
			t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
			t.Setenv("EXA_CONFIG", filepath.Join(home, "config.toml"))
			t.Setenv("EXA_API_KEY", "synthetic-test-key")
			t.Setenv("EXA_BASE_URL", srv.URL)

			cmd := RootCmd()
			cmd.SetArgs([]string{"websearch", "--query", "synthetic query", "--contents", test.contents, "--json"})
			var output strings.Builder
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			err := cmd.Execute()
			if (err != nil) != test.wantErr {
				t.Fatalf("websearch error = %v, wantErr %t", err, test.wantErr)
			}
			mu.Lock()
			defer mu.Unlock()
			if test.wantErr {
				if calls != 0 {
					t.Fatalf("invalid contents sent %d requests", calls)
				}
				return
			}
			if calls != 1 || strings.TrimSpace(string(requestBody["contents"])) != test.wantValue {
				t.Fatalf("request calls=%d contents=%s, want one request with %s", calls, requestBody["contents"], test.wantValue)
			}
		})
	}
}

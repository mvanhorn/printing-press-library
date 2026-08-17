// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/roblox/internal/config"
	"github.com/spf13/cobra"
)

// TestNovelNetworkOverlapHelpWires smoke-tests that the network overlap command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelNetworkOverlapHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"network", "overlap", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("network overlap --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "overlap"} {
		if !strings.Contains(help, want) {
			t.Fatalf("network overlap --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestFetchArrayRejectsIncompleteResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantRows   int
		wantErr    string
	}{
		{name: "valid array", statusCode: http.StatusOK, body: `{"data":[{"id":1}]}`, wantRows: 1},
		{name: "upstream error", statusCode: http.StatusBadRequest, body: `{"errors":[{"message":"bad request"}]}`, wantErr: "400"},
		{name: "malformed envelope", statusCode: http.StatusOK, body: `{`, wantErr: "decoding response envelope"},
		{name: "missing data", statusCode: http.StatusOK, body: `{"nextPageCursor":null}`, wantErr: "missing a data array"},
		{name: "data is not array", statusCode: http.StatusOK, body: `{"data":{"id":1}}`, wantErr: "decoding data array"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
			c.NoCache = true
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			rows, err := fetchArray(cmd, c, server.URL, nil)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("fetchArray() error = %v", err)
				}
				if len(rows) != tt.wantRows {
					t.Fatalf("fetchArray() rows = %d, want %d", len(rows), tt.wantRows)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("fetchArray() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

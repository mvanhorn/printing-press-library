// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
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

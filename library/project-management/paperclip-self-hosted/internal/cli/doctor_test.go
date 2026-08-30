package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/paperclip-self-hosted/internal/config"
)

func TestDoctorProbePath(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{mode: "board-session", want: "/api/cli-auth/me"},
		{mode: "board-api-key", want: "/api/cli-auth/me"},
		{mode: "agent-bearer", want: "/api/agents/me"},
		{mode: "custom-header", want: "/api/agents/me"},
		{mode: "none", want: "/api/health"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := &config.Config{AuthMode: tt.mode}
			if got := doctorProbePath(cfg); got != tt.want {
				t.Fatalf("doctorProbePath(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestDoctorValidatesBoardCredentialsAtBoardIdentityEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		credential string
		wantHeader string
	}{
		{name: "api key", mode: "board-api-key", credential: "--api-key", wantHeader: "Bearer board-secret"},
		{name: "session", mode: "board-session", credential: "--session-cookie", wantHeader: "paperclip_session=board-secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withTempLearnHome(t)
			var boardCalls, agentCalls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/cli-auth/me":
					boardCalls++
					got := r.Header.Get("Authorization")
					if tt.mode == "board-session" {
						got = r.Header.Get("Cookie")
					}
					if got != tt.wantHeader {
						http.Error(w, fmt.Sprintf("unexpected credential header %q", got), http.StatusUnauthorized)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"id":"board-user"}`)
				case "/api/agents/me":
					agentCalls++
					http.Error(w, "board credentials are not agent credentials", http.StatusUnauthorized)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			t.Setenv("PAPERCLIP_BASE_URL", server.URL)
			stdout, stderr, err := runRootArgs(t,
				"doctor", "--json", "--no-learn",
				"--auth-mode", tt.mode, tt.credential, "board-secret",
			)
			if err != nil {
				t.Fatalf("doctor exited non-zero: %v (stderr=%q)", err, stderr)
			}
			var report map[string]any
			if err := json.Unmarshal([]byte(stdout), &report); err != nil {
				t.Fatalf("doctor JSON: %v (stdout=%q)", err, stdout)
			}
			if got := report["api"]; got != "reachable" {
				t.Fatalf("api = %v, want reachable", got)
			}
			if got := report["credentials"]; got != "valid" {
				t.Fatalf("credentials = %v, want valid", got)
			}
			if boardCalls < 1 {
				t.Fatal("board identity endpoint was not called")
			}
			if agentCalls != 0 {
				t.Fatalf("agent identity endpoint calls = %d, want 0", agentCalls)
			}
		})
	}
}

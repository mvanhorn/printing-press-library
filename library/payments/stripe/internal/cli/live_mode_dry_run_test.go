// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/client"
)

type liveGuardTestTransport struct {
	calls         atomic.Int64
	authorization string
}

func (transport *liveGuardTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	transport.authorization = request.Header.Get("Authorization")
	// Never contact an endpoint, even if the dry-run or confirmation gate regresses.
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"error":"unexpected fixture request"}`)),
		Request:    request,
	}, nil
}

func TestLiveModeDryRunCommands(t *testing.T) {
	liveBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("sk_live_fixture:"))
	restrictedBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("rk_live_fixture:"))
	commands := []struct {
		name string
		args []string
	}{
		{"post", []string{"customers", "post"}},
		{"create alias", []string{"customers", "create"}},
		{"delete", []string{"customers", "delete", "cus_fixture"}},
	}
	cases := []struct {
		name      string
		envKey    string
		configKey string
		header    string
		args      []string
		blocked   bool
	}{
		{"environment live key preview", "sk_live_fixture", "", "", []string{"--dry-run"}, false},
		{"persisted live key preview", "", "sk_live_fixture", "", []string{"--dry-run"}, false},
		{"restricted live key preview", "rk_live_fixture", "", "", []string{"--dry-run"}, false},
		{"environment live request", "sk_live_fixture", "", "", nil, true},
		{"persisted live request", "", "sk_live_fixture", "", nil, true},
		{"explicit false preview", "sk_live_fixture", "", "", []string{"--dry-run=false"}, true},
		{"yes is not live confirmation", "sk_live_fixture", "", "", []string{"--yes"}, true},
		{"agent is not live confirmation", "sk_live_fixture", "", "", []string{"--agent"}, true},
		{"persisted restricted live request", "", "rk_live_fixture", "", nil, true},
		{"encoded Basic live preview", "", "", liveBasic, []string{"--dry-run"}, false},
		{"encoded Basic live request", "", "", liveBasic, nil, true},
		{"encoded restricted Basic preview", "", "", restrictedBasic, []string{"--dry-run"}, false},
		{"encoded restricted Basic request", "", "", restrictedBasic, nil, true},
		{"case-insensitive Bearer live request", "", "", "bEaReR sk_live_fixture", nil, true},
	}
	for _, command := range commands {
		for _, tc := range cases {
			t.Run(command.name+"/"+tc.name, func(t *testing.T) {
				configPath := filepath.Join(t.TempDir(), "config.toml")
				configBytes := []byte(fmt.Sprintf("base_url = %q\naccess_token = %q\nauth_header = %q\n", "https://stripe.invalid", tc.configKey, tc.header))
				if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("STRIPE_CONFIG", configPath)
				t.Setenv("STRIPE_SECRET_KEY", tc.envKey)
				t.Setenv("STRIPE_BASIC_AUTH", "")
				t.Setenv("STRIPE_CONFIRM_LIVE", "")
				t.Setenv("STRIPE_BASE_URL", "")

				transport := &liveGuardTestTransport{}
				previousTransport := http.DefaultTransport
				http.DefaultTransport = transport
				t.Cleanup(func() { http.DefaultTransport = previousTransport })
				previousNoColor, previousHumanFriendly := noColor, humanFriendly
				t.Cleanup(func() { noColor, humanFriendly = previousNoColor, previousHumanFriendly })
				root := RootCmd()
				var output bytes.Buffer
				root.SetOut(&output)
				root.SetErr(io.Discard)
				args := append([]string{"--config", configPath, "--json", "--no-input"}, command.args...)
				root.SetArgs(append(args, tc.args...))
				err := root.Execute()
				if calls := transport.calls.Load(); calls != 0 {
					t.Fatalf("unexpected HTTP requests: %d", calls)
				}
				if after, readErr := os.ReadFile(configPath); readErr != nil || !bytes.Equal(after, configBytes) {
					t.Fatal("command changed its synthetic config")
				}
				if tc.blocked {
					var blocked *liveModeBlockedErr
					if !errors.As(err, &blocked) || ExitCode(err) != 10 {
						t.Fatalf("expected live-mode denial with exit code 10, got %v", err)
					}
					if output.Len() != 0 {
						t.Fatal("blocked command emitted a success payload")
					}
					return
				}
				if err != nil {
					t.Fatalf("dry-run failed: %v", err)
				}
				var envelope struct {
					DryRun  bool `json:"dry_run"`
					Status  int  `json:"status"`
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				if !envelope.DryRun || envelope.Status != 0 || envelope.Success {
					t.Fatalf("preview must identify that no request succeeded: %+v", envelope)
				}
			})
		}
	}
}

func TestLiveModeGuardUsesEffectiveAuthorization(t *testing.T) {
	liveBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("sk_live_fixture:"))
	testBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("sk_test_fixture:"))
	cases := []struct {
		name, envKey, unusedBasic, configKey, header, wantAuth string
		blocked                                                bool
	}{
		{"test environment ignores unused live Basic", "sk_test_fixture", liveBasic, "", "", "Bearer sk_test_fixture", false},
		{"test access token ignores unused live Basic", "", liveBasic, "sk_test_fixture", "", "Bearer sk_test_fixture", false},
		{"test header overrides live candidates", "sk_live_fixture", liveBasic, "rk_live_fixture", "Bearer sk_test_fixture", "Bearer sk_test_fixture", false},
		{"test environment overrides live access token", "sk_test_fixture", "", "sk_live_fixture", "", "Bearer sk_test_fixture", false},
		{"live header overrides test environment", "sk_test_fixture", "", "", "Bearer sk_live_fixture", "", true},
		{"live environment overrides test access token", "sk_live_fixture", "", "sk_test_fixture", "", "", true},
		{"live access token ignores unused test Basic", "", testBasic, "sk_live_fixture", "", "", true},
		{"live encoded Basic overrides test environment", "sk_test_fixture", "", "", liveBasic, "", true},
		{"test encoded Basic overrides live environment", "sk_live_fixture", "", "", testBasic, testBasic, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.toml")
			configBytes := []byte(fmt.Sprintf("base_url = %q\naccess_token = %q\nauth_header = %q\n", "https://stripe.invalid", tc.configKey, tc.header))
			if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("STRIPE_CONFIG", configPath)
			t.Setenv("STRIPE_SECRET_KEY", tc.envKey)
			t.Setenv("STRIPE_BASIC_AUTH", tc.unusedBasic)
			t.Setenv("STRIPE_CONFIRM_LIVE", "")
			t.Setenv("STRIPE_BASE_URL", "")
			transport := &liveGuardTestTransport{}
			previousTransport := http.DefaultTransport
			http.DefaultTransport = transport
			t.Cleanup(func() { http.DefaultTransport = previousTransport })
			previousNoColor, previousHumanFriendly := noColor, humanFriendly
			t.Cleanup(func() { noColor, humanFriendly = previousNoColor, previousHumanFriendly })
			root := RootCmd()
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"--config", configPath, "--json", "--no-input", "customers", "post"})
			err := root.Execute()
			if tc.blocked {
				var blocked *liveModeBlockedErr
				if !errors.As(err, &blocked) || ExitCode(err) != 10 {
					t.Fatalf("expected live-mode denial, got %v", err)
				}
				if calls := transport.calls.Load(); calls != 0 {
					t.Fatalf("blocked request reached transport: %d", calls)
				}
			} else {
				// The fixture rejects the request before any successful-mutation cache effects.
				var apiError *client.APIError
				if !errors.As(err, &apiError) {
					t.Fatalf("expected fixture API error after guard allowed request, got %v", err)
				}
				if calls := transport.calls.Load(); calls != 1 {
					t.Fatalf("expected one stub request, got %d", calls)
				}
				if transport.authorization != tc.wantAuth {
					t.Fatalf("unexpected effective authorization: %q", transport.authorization)
				}
			}
			if after, readErr := os.ReadFile(configPath); readErr != nil || !bytes.Equal(after, configBytes) {
				t.Fatal("command changed its synthetic config")
			}
		})
	}
}

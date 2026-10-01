// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type mcpGuardFixture struct {
	Generic bool           `json:"generic"`
	Method  string         `json:"method"`
	Args    map[string]any `json:"args"`
}

type mcpGuardObservation struct {
	Blocked bool   `json:"blocked"`
	Calls   int    `json:"calls"`
	Body    string `json:"body"`
}

type mcpGuardTransport struct {
	observation *mcpGuardObservation
}

func (transport *mcpGuardTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.observation.Calls++
	if request.Body != nil {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		transport.observation.Body = string(body)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"id":"cus_fixture"}`)),
		Request:    request,
	}, nil
}

// Exercise actual handlers in a subprocess because newMCPClient intentionally
// uses a home-relative config path. The parent supplies a disposable home and
// fake credentials; this transport cannot contact any HTTP endpoint.
func TestMCPLiveModeFixtureProcess(t *testing.T) {
	raw := os.Getenv("STRIPE_MCP_GUARD_FIXTURE")
	if raw == "" {
		return
	}
	var fixture mcpGuardFixture
	if err := json.Unmarshal([]byte(raw), &fixture); err != nil {
		t.Fatal(err)
	}
	var observation mcpGuardObservation
	http.DefaultTransport = &mcpGuardTransport{observation: &observation}
	req := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: fixture.Args}}
	handler := handleCodeOrchExecute
	if fixture.Generic {
		handler = makeAPIHandler(fixture.Method, "/v1/customers", nil, nil)
	}
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	observation.Blocked = result.IsError
	if err := json.NewEncoder(os.Stdout).Encode(observation); err != nil {
		t.Fatal(err)
	}
	os.Exit(0)
}

func TestMCPLiveModeHandlers(t *testing.T) {
	liveBasic := "Basic " + base64.StdEncoding.EncodeToString([]byte("sk_live_fixture:"))
	cases := []struct {
		name       string
		generic    bool
		method     string
		token      string
		header     string
		envKey     string
		envConfirm string
		confirm    any
		nested     bool
		blocked    bool
	}{
		{name: "live POST denied", method: "POST", token: "sk_live_fixture", blocked: true},
		{name: "live DELETE denied", method: "DELETE", token: "sk_live_fixture", blocked: true},
		{name: "restricted live denied", method: "POST", envKey: "rk_live_fixture", blocked: true},
		{name: "live GET allowed", method: "GET", token: "sk_live_fixture"},
		{name: "test POST allowed", method: "POST", token: "sk_test_fixture"},
		{name: "explicit confirmation", method: "POST", token: "sk_live_fixture", confirm: true},
		{name: "environment confirmation", method: "POST", token: "sk_live_fixture", envConfirm: "1"},
		{name: "false confirmation denied", method: "POST", token: "sk_live_fixture", confirm: false, blocked: true},
		{name: "string confirmation denied", method: "POST", token: "sk_live_fixture", confirm: "true", blocked: true},
		{name: "nested confirmation denied", method: "POST", token: "sk_live_fixture", nested: true, blocked: true},
		{name: "explicit confirmation strips nested control", method: "POST", token: "sk_live_fixture", confirm: true, nested: true},
		{name: "environment confirmation strips nested control", method: "POST", token: "sk_live_fixture", envConfirm: "1", nested: true},
		{name: "test mode strips nested control", method: "POST", token: "sk_test_fixture", nested: true},
		{name: "nonexact env confirmation denied", method: "POST", token: "sk_live_fixture", envConfirm: "true", blocked: true},
		{name: "encoded Basic live denied", method: "POST", header: liveBasic, blocked: true},
		{name: "case-insensitive Bearer live denied", method: "POST", header: "bEaReR rk_live_fixture", blocked: true},
		{name: "effective live header overrides test key", method: "POST", header: "Bearer sk_live_fixture", envKey: "sk_test_fixture", blocked: true},
		{name: "effective test header overrides unused live token", method: "POST", header: "Bearer sk_test_fixture", token: "sk_live_fixture"},
		{name: "generic live denied", generic: true, method: "POST", token: "sk_live_fixture", blocked: true},
		{name: "generic DELETE denied", generic: true, method: "DELETE", token: "sk_live_fixture", blocked: true},
		{name: "generic test mutation allowed", generic: true, method: "POST", token: "sk_test_fixture"},
		{name: "generic live read allowed", generic: true, method: "GET", token: "sk_live_fixture"},
		{name: "generic explicit confirmation stripped", generic: true, method: "POST", token: "sk_live_fixture", confirm: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixtureHome := t.TempDir()
			configPath := filepath.Join(fixtureHome, ".config", "stripe-pp-cli", "config.toml")
			if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
				t.Fatal(err)
			}
			configBytes := []byte(fmt.Sprintf("base_url = %q\naccess_token = %q\nauth_header = %q\n", "https://stripe.invalid", tc.token, tc.header))
			if err := os.WriteFile(configPath, configBytes, 0o600); err != nil {
				t.Fatal(err)
			}
			params := map[string]any{"description": "fixture"}
			if tc.method == "DELETE" {
				params["customer"] = "cus_fixture"
			}
			if tc.nested {
				params["confirm_live"] = true
			}
			args := map[string]any{"endpoint_id": "customers." + strings.ToLower(tc.method), "params": params}
			if tc.generic {
				args = params
			}
			if tc.confirm != nil {
				args["confirm_live"] = tc.confirm
			}
			fixture, err := json.Marshal(mcpGuardFixture{Generic: tc.generic, Method: tc.method, Args: args})
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command(os.Args[0], "-test.run=^TestMCPLiveModeFixtureProcess$")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "HOME=") && !strings.HasPrefix(entry, "STRIPE_") {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, "HOME="+fixtureHome, "STRIPE_SECRET_KEY="+tc.envKey,
				"STRIPE_CONFIRM_LIVE="+tc.envConfirm, "STRIPE_MCP_GUARD_FIXTURE="+string(fixture))
			output, err := command.Output()
			if err != nil {
				t.Fatalf("isolated handler process failed: %v", err)
			}
			var got mcpGuardObservation
			if err := json.Unmarshal(output, &got); err != nil {
				t.Fatal(err)
			}
			wantCalls := 1
			if tc.blocked {
				wantCalls = 0
			}
			if got.Blocked != tc.blocked || got.Calls != wantCalls {
				t.Fatalf("blocked=%v, requests=%d; want blocked=%v, requests=%d", got.Blocked, got.Calls, tc.blocked, wantCalls)
			}
			if strings.Contains(got.Body, "confirm_live") {
				t.Fatal("tool confirmation was forwarded in the Stripe request body")
			}
		})
	}
}

func TestMCPLiveModeConfirmationSchema(t *testing.T) {
	s := server.NewMCPServer("fixture", "0")
	RegisterCodeOrchestrationTools(s)
	tool := s.GetTool("stripe_execute")
	property, ok := tool.Tool.InputSchema.Properties["confirm_live"].(map[string]any)
	if !ok || property["type"] != "boolean" {
		t.Fatal("stripe_execute must advertise optional boolean confirm_live")
	}
	for _, required := range tool.Tool.InputSchema.Required {
		if required == "confirm_live" {
			t.Fatal("live confirmation must remain optional for reads and test keys")
		}
	}
}

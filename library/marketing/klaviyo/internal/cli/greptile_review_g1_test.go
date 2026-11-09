// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/mcp/cobratree"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func greptileG1Profile(t *testing.T) *platform.Profile {
	t.Helper()
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	t.Setenv(mcpBoundProfileEnv, "")
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	for _, name := range []string{"KLAVIYO_CONFIG", "KLAVIYO_API_KEY", "KLAVIYO_BASE_URL", "KLAVIYO_DB", "KLAVIYO_DATABASE", "KLAVIYO_DATA_FILE", "KLAVIYO_DATA_DIR", "KLAVIYO_STATE_DIR", "KLAVIYO_CACHE_DIR", "KLAVIYO_HOME"} {
		t.Setenv(name, "")
	}
	t.Setenv("GREPTILE_G1_API_KEY", "test-key")
	profile := &platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: "tenant-a", Sources: map[string]platform.SourceProfile{
		"klaviyo": {CredentialRef: "env:GREPTILE_G1_API_KEY", ExpectedAccountID: "acct-a", ExpectedOrgName: "Acme"},
	}}
	if err := platform.SaveProfile(profile); err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestGreptileG1MCPToolsExcludeProfileManagement(t *testing.T) {
	for _, bound := range []string{"", "tenant-a"} {
		t.Run("bound="+bound, func(t *testing.T) {
			t.Setenv(mcpBoundProfileEnv, bound)
			root := RootCmd()
			s := server.NewMCPServer("g1-profile-management", "test")
			cobratree.RegisterAll(s, root, func() (string, error) { return "", errors.New("do not execute") })
			tools := s.ListTools()
			for _, name := range []string{"client_add", "client_source_set", "client_list", "client_show", "client_validate", "client_set_default", "client_migrate", "client_delete", "client_cache_clear"} {
				if _, exposed := tools[name]; exposed {
					t.Errorf("profile-management tool %q is exposed", name)
				}
			}
			for _, name := range []string{"whoami", "sync"} {
				if _, exposed := tools[name]; !exposed {
					t.Errorf("ordinary tool %q is missing", name)
				}
			}
			provider, _, err := root.Find([]string{"client", "create-event"})
			if err != nil || provider.Name() != "create-event" || !platformCommandNeedsGate(provider) {
				t.Fatalf("provider client endpoint was lost: %v, %v", provider, err)
			}
			if err := enforceMCPBoundProfile(provider, &rootFlags{}); err != nil {
				t.Fatalf("provider client endpoint rejected: %v", err)
			}
		})
	}
}

// Exercise Execute and main's exit-code contract in a child so process stream
// capture and os.Exit cannot interfere with the parent test runner.
func TestGreptileG1CLIProcess(t *testing.T) {
	if os.Getenv("GREPTILE_G1_CLI_PROCESS") != "1" {
		return
	}
	probeURL := os.Getenv("GREPTILE_G1_PROBE_URL")
	if probeURL == "" {
		probeURL = "http://127.0.0.1:1"
	}
	registeredPlatformSource.Adapter = klaviyoIdentityAdapter{baseURL: probeURL, httpClient: &http.Client{Timeout: time.Second}}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"klaviyo-pp-cli"}, os.Args[i+1:]...)
			if err := Execute(); err != nil {
				os.Exit(ExitCode(err))
			}
			os.Exit(0)
		}
	}
	t.Fatal("missing CLI arguments")
}

func greptileG1RunCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, append([]string{"-test.run=^TestGreptileG1CLIProcess$", "--"}, args...)...)
	command.Env = append(os.Environ(), "GREPTILE_G1_CLI_PROCESS=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	if err == nil {
		return stdout.String(), stderr.String(), 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), exitErr.ExitCode()
}

func TestGreptileG1BoundChildRejectsProfileManagement(t *testing.T) {
	profile := greptileG1Profile(t)
	profilePath, err := platform.ProfilePath(profile.Name)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(mcpBoundProfileEnv, profile.Name)
	sourceFlags := []string{"--credential-ref", "env:OTHER_API_KEY", "--expected-account-id", "other-account", "--expected-org-name", "Other"}
	for _, args := range [][]string{
		append([]string{"client", "source", "set", profile.Name}, sourceFlags...),
		append([]string{"client", "source", "set", "other-tenant"}, sourceFlags...),
		append([]string{"client", "add", "new-tenant"}, sourceFlags...),
		append([]string{"client", "migrate", profile.Name}, sourceFlags...),
		{"client", "set-default", profile.Name},
		{"client", "delete", profile.Name, "--yes"},
		{"client", "cache", "clear", profile.Name, "--yes"},
	} {
		t.Run(strings.Join(args[:2], " "), func(t *testing.T) {
			stdout, stderr, code := greptileG1RunCLI(t, args...)
			if code != 1 || stdout != "" || stderr != "Error: MCP tools may not manage client profiles\n" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			after, err := os.ReadFile(profilePath)
			if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("bound profile changed: %q, error %v", after, err)
			}
			if _, err := platform.LoadProfile("new-tenant"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("profile was created: %v", err)
			}
			if _, err := platform.SelectProfile(""); err == nil {
				t.Fatal("a default profile was written")
			}
		})
	}
}

func greptileG1AssertProbeRequest(t *testing.T, r *http.Request) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) != 0 || r.Method != http.MethodGet || r.URL.RequestURI() != "/api/accounts" {
		t.Errorf("probe request = %s %s body=%q, error %v", r.Method, r.URL.RequestURI(), body, err)
	}
	if r.Header.Get("Authorization") != "Klaviyo-API-Key test-key" || r.Header.Get("revision") != "2026-04-15" {
		t.Error("probe did not preserve the exact authorization and revision headers")
	}
}

func TestGreptileG1ReceiptFailureIsPrintedOnce(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			profile := greptileG1Profile(t)
			probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				greptileG1AssertProbeRequest(t, r)
				w.WriteHeader(status)
				fmt.Fprint(w, `{"data":[{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Acme"}}}]}`)
			}))
			defer probe.Close()
			t.Setenv("GREPTILE_G1_PROBE_URL", probe.URL)
			blocker := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(blocker, []byte("block receipt directory creation"), 0o600); err != nil {
				t.Fatal(err)
			}
			mkdirErr := os.MkdirAll(blocker, 0o700)
			if mkdirErr == nil {
				t.Fatal("receipt destination was not blocked")
			}
			receiptPath := filepath.Join(blocker, "receipt.json")
			stdout, stderr, code := greptileG1RunCLI(t, "whoami", "--client-profile", profile.Name, "--json", "--receipt-file", receiptPath)
			wantError := mkdirErr.Error()
			if status == http.StatusOK {
				var envelope struct {
					Data struct{ Gate platform.GateOutcome } `json:"data"`
				}
				if err := json.Unmarshal([]byte(stdout), &envelope); err != nil || envelope.Data.Gate != platform.GateVerified {
					t.Fatalf("successful command output=%q error=%v", stdout, err)
				}
				if stderr != "Error: "+wantError+"\n" {
					t.Fatalf("finalization diagnostic=%q, want exact filesystem error once", stderr)
				}
			} else {
				wantError = `client profile "tenant-a" source "klaviyo" tenant gate insufficient_scope: Klaviyo identity probe failed with status 403`
				if strings.Count(stderr, "Error: ") != 1 || !strings.Contains(stderr, "Error: "+wantError+"\n") || strings.Contains(stderr, mkdirErr.Error()) {
					t.Fatalf("Cobra diagnostic was duplicated or replaced: %q", stderr)
				}
			}
			if code != 1 {
				t.Fatalf("exit code=%d, want 1", code)
			}
			if _, err := os.Stat(receiptPath); err == nil {
				t.Fatal("receipt unexpectedly exists")
			}
		})
	}
}

func TestGreptileG1ProbeRetryAccounting(t *testing.T) {
	for _, entrypoint := range []string{"CLI", "MCP"} {
		for _, test := range []struct {
			name      string
			statuses  []int
			want      platform.RateLimitMetadata
			wantError string
		}{
			{"no retry", []int{200}, platform.RateLimitMetadata{Requests: 1}, ""},
			{"throttle recovery", []int{429, 429, 200}, platform.RateLimitMetadata{Requests: 3, Retries: 2, WaitMS: 3000}, ""},
			{"server recovery", []int{503, 503, 200}, platform.RateLimitMetadata{Requests: 3, Retries: 2, WaitMS: 3000}, ""},
			{"throttle exhausted", []int{429, 429, 429, 429}, platform.RateLimitMetadata{Requests: 4, Retries: 3, WaitMS: 7000, Exhausted: true}, `client profile "tenant-a" source "klaviyo" tenant gate rate_limited: Klaviyo identity probe failed with status 429`},
		} {
			if entrypoint == "MCP" && test.wantError != "" {
				continue // Failed MCP invocations do not return their session.
			}
			t.Run(entrypoint+"/"+test.name, func(t *testing.T) {
				profile := greptileG1Profile(t)
				calls := 0
				probe := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					greptileG1AssertProbeRequest(t, r)
					calls++
					if calls > len(test.statuses) {
						t.Error("unexpected extra probe attempt")
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					w.WriteHeader(test.statuses[calls-1])
					fmt.Fprint(w, `{"data":[{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Acme"}}}]}`)
				}))
				defer probe.Close()
				previousRegistration, previousWait := registeredPlatformSource, waitForIdentityProbeRetry
				t.Cleanup(func() { registeredPlatformSource, waitForIdentityProbeRetry = previousRegistration, previousWait })
				registration := *registeredPlatformSource
				registration.Adapter = klaviyoIdentityAdapter{baseURL: probe.URL, httpClient: probe.Client()}
				registeredPlatformSource = &registration
				var waits []time.Duration
				waitForIdentityProbeRetry = func(_ context.Context, wait time.Duration) error {
					waits = append(waits, wait)
					return nil
				}
				session, err := platform.PrepareProfileSource(profile.Name, "klaviyo-pp-cli", "klaviyo")
				if err != nil {
					t.Fatal(err)
				}
				defer session.ZeroCredentials()
				if entrypoint == "CLI" {
					err = verifyPlatformSession(context.Background(), &rootFlags{platformSession: session})
				} else {
					t.Setenv(mcpBoundProfileEnv, profile.Name)
					session, err = VerifyMCPInvocation(context.Background())
					defer session.ZeroCredentials()
				}
				if (err == nil) != (test.wantError == "") || (err != nil && err.Error() != test.wantError) {
					t.Fatalf("gate error=%v, want %q", err, test.wantError)
				}
				if calls != len(test.statuses) || len(waits) != test.want.Retries || !reflect.DeepEqual(session.RateLimitMetadata(), test.want) {
					t.Fatalf("calls=%d waits=%v accounting=%+v, want %+v", calls, waits, session.RateLimitMetadata(), test.want)
				}
			})
		}
	}
}

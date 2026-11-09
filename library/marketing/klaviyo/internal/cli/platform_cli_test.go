// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
)

type platformCLIResolver struct{ value []byte }

func (r platformCLIResolver) Resolve(context.Context, string) ([]byte, error) {
	return append([]byte(nil), r.value...), nil
}

func TestPlatformProviderClientCommandsStillRequireGate(t *testing.T) {
	previousRegistration := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previousRegistration })
	registeredPlatformSource = &platformSourceRegistration{Source: "test-source", Adapter: platformCLIAdapter{}}
	root := &cobra.Command{Use: "klaviyo-pp-cli"}
	clientGroup := &cobra.Command{Use: "client"}
	providerCommand := &cobra.Command{Use: "create-event"}
	managementCommand := &cobra.Command{Use: "list"}
	clientGroup.AddCommand(providerCommand, managementCommand)
	root.AddCommand(clientGroup)
	if !platformCommandNeedsGate(providerCommand) {
		t.Fatal("provider client command bypassed the tenant gate")
	}
	if platformCommandNeedsGate(managementCommand) {
		t.Fatal("client profile management command unexpectedly required a tenant gate")
	}
}

type platformCLIAdapter struct{ observed platform.ObservedIdentity }

func (a platformCLIAdapter) ProbeIdentity(context.Context, platform.ResolvedCredentials, platform.SourceProfile) (platform.ObservedIdentity, error) {
	return a.observed, nil
}
func (platformCLIAdapter) EndpointClass() string { return "GET /identity" }

func TestPlatformCLIConformanceSurfaceAndGate(t *testing.T) {
	t.Setenv(mcpBoundProfileEnv, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	if err := platform.SaveProfile(&platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: "tenant-a", Sources: map[string]platform.SourceProfile{"test-source": {CredentialRef: "op://Test/Vault/token", ExpectedAccountID: "acct-a"}}}); err != nil {
		t.Fatal(err)
	}

	previousRegistration, previousResolver := registeredPlatformSource, platformResolverFactory
	t.Cleanup(func() { registeredPlatformSource, platformResolverFactory = previousRegistration, previousResolver })
	registeredPlatformSource = &platformSourceRegistration{Source: "test-source", Adapter: platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "acct-a"}}}
	platformResolverFactory = func() platform.CredentialResolver {
		return platformCLIResolver{value: []byte("credential-bytes-never-print")}
	}

	root := RootCmd()
	for _, path := range []string{"client add", "client source set", "client list", "client show", "client validate", "client set-default", "client migrate", "client delete", "client cache clear", "whoami"} {
		parts := strings.Fields(path)
		command, _, err := root.Find(parts)
		if err != nil || command == nil || command.Name() != parts[len(parts)-1] {
			t.Fatalf("missing platform command %q: command=%v err=%v", path, command, err)
		}
	}
	if root.PersistentFlags().Lookup("client-profile") == nil || root.PersistentFlags().Lookup("receipt") == nil || root.PersistentFlags().Lookup("audit-dir") == nil {
		t.Fatal("platform root flags are missing")
	}

	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"whoami", "--client-profile", "tenant-a", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "verified") || !strings.Contains(got, "acct-a") {
		t.Fatalf("whoami did not report verified tenant: %s", got)
	}
	if strings.Contains(got, "credential-bytes-never-print") {
		t.Fatal("whoami leaked credentials")
	}

	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "tenant-a")
	if err := BindMCPServerProfile(); err != nil {
		t.Fatal(err)
	}
	mcpRoot := RootCmd()
	if flag := mcpRoot.PersistentFlags().Lookup("client-profile"); flag == nil || !flag.Hidden {
		t.Fatal("MCP-bound root exposed client-profile")
	}
	mcpRoot.SetArgs([]string{"whoami", "--client-profile", "other", "--json"})
	if err := mcpRoot.Execute(); err == nil || !strings.Contains(err.Error(), "may not switch") {
		t.Fatalf("MCP switch error = %v", err)
	}
}

func TestPlatformCLIConformanceMismatchFailsBeforeCommand(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	if err := platform.SaveProfile(&platform.Profile{SchemaVersion: platform.ProfileSchemaVersion, Name: "tenant-a", Sources: map[string]platform.SourceProfile{"test-source": {CredentialRef: "op://Test/Vault/token", ExpectedAccountID: "acct-a"}}}); err != nil {
		t.Fatal(err)
	}
	previousRegistration, previousResolver := registeredPlatformSource, platformResolverFactory
	t.Cleanup(func() { registeredPlatformSource, platformResolverFactory = previousRegistration, previousResolver })
	registeredPlatformSource = &platformSourceRegistration{Source: "test-source", Adapter: platformCLIAdapter{observed: platform.ObservedIdentity{AccountID: "acct-b"}}}
	platformResolverFactory = func() platform.CredentialResolver { return platformCLIResolver{value: []byte("secret")} }
	root := RootCmd()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"doctor", "--client-profile", "tenant-a", "--json"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "tenant gate mismatch") {
		t.Fatalf("mismatch error = %v", err)
	}
	if got := output.String(); !strings.Contains(got, `"status":"error"`) || !strings.Contains(got, `"identity_status":"mismatch"`) {
		t.Fatalf("doctor did not emit typed mismatch report: %s", got)
	}
}

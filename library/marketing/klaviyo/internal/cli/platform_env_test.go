// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

type envReferenceIdentityAdapter struct {
	t        *testing.T
	calls    int
	observed platform.ObservedIdentity
}

func (a *envReferenceIdentityAdapter) EndpointClass() string { return "GET /identity" }
func (a *envReferenceIdentityAdapter) ProbeIdentity(_ context.Context, credentials platform.ResolvedCredentials, source platform.SourceProfile) (platform.ObservedIdentity, error) {
	a.calls++
	for key, ref := range source.References() {
		if !bytes.Equal(credentials[key], []byte(os.Getenv(strings.TrimPrefix(ref, "env:")))) {
			a.t.Fatal("identity probe did not receive the current environment credential")
		}
	}
	return a.observed, nil
}

func TestClientProfileEnvironmentReferences(t *testing.T) {
	rootDir := t.TempDir()
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
		t.Setenv(name, filepath.Join(rootDir, name))
	}
	t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
	t.Setenv(mcpBoundProfileEnv, "")
	t.Setenv("PRINTING_PRESS_ONEPASSWORD_CLI", filepath.Join(rootDir, "no-1password-installed"))
	secrets := []string{"synthetic-env-key-never-output", "synthetic-env-user-never-output", "synthetic-env-extra-never-output", "synthetic-env-rotated-never-output"}
	names := []string{"KLAVIYO_API_KEY", "PP_TEST_ENV_USER", "PP_TEST_ENV_EXTRA"}
	for i, name := range names {
		t.Setenv(name, secrets[i])
	}
	adapter := &envReferenceIdentityAdapter{t: t, observed: platform.ObservedIdentity{
		AccountID: "account-one", OrgName: "Fixture Org", StoreID: "store-one",
		StoreDomain: "fixture.myshopify.com", PropertyID: "123", BaseURL: "https://fixture.example/api",
	}}
	previous := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previous })
	sourceName := previous.Source
	registration := *previous
	registration.Adapter = adapter
	registration.BindClient = nil
	registeredPlatformSource = &registration
	run := func(args ...string) error {
		t.Helper()
		root := RootCmd()
		var output bytes.Buffer
		root.SetOut(&output)
		root.SetErr(&output)
		root.SetArgs(append(args, "--json"))
		err := root.Execute()
		for _, secret := range secrets {
			if strings.Contains(output.String(), secret) || (err != nil && strings.Contains(err.Error(), secret)) {
				t.Fatal("resolved credential leaked into command output or error")
			}
		}
		return err
	}
	if err := run("client", "add", "env-fixture", "--credential-ref", "env:KLAVIYO_API_KEY", "--username-ref", "env:PP_TEST_ENV_USER", "--additional-credential-ref", "extra=env:PP_TEST_ENV_EXTRA",
		"--expected-account-id", "account-one", "--expected-org-name", "Fixture Org", "--expected-store-id", "store-one", "--expected-store-domain", "fixture.myshopify.com", "--expected-property-id", "123", "--expected-base-url", "https://fixture.example/api"); err != nil {
		t.Fatal(err)
	}
	if err := run("client", "show", "env-fixture"); err != nil {
		t.Fatal(err)
	}
	if err := run("whoami", "--client-profile", "env-fixture", "--receipt-file", filepath.Join(rootDir, "receipt.json")); err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 1 {
		t.Fatal("env references bypassed the identity probe")
	}
	t.Setenv(names[0], secrets[3])
	if err := run("whoami", "--client-profile", "env-fixture"); err != nil {
		t.Fatal(err)
	}
	if adapter.calls != 2 {
		t.Fatal("credential rotation bypassed the identity probe")
	}
	for _, name := range names {
		for _, state := range []string{"unset", "empty"} {
			t.Run(name+"/"+state, func(t *testing.T) {
				t.Setenv(name, "")
				if state == "unset" {
					if err := os.Unsetenv(name); err != nil {
						t.Fatal(err)
					}
				}
				before := adapter.calls
				err := run("whoami", "--client-profile", "env-fixture")
				if err == nil || !strings.Contains(err.Error(), name) {
					t.Fatal("missing credential error must name its environment variable")
				}
				if adapter.calls != before {
					t.Fatal("identity probe ran with missing credentials")
				}
			})
		}
	}
	profile, err := platform.LoadProfile("env-fixture")
	if err != nil {
		t.Fatal(err)
	}
	original := profile.Sources[sourceName]
	for _, field := range []string{"account_id", "org_name", "store_id", "store_domain", "property_id", "base_url"} {
		source := original
		switch field {
		case "account_id":
			source.ExpectedAccountID = "wrong-account"
		case "org_name":
			source.ExpectedOrgName = "Wrong Org"
		case "store_id":
			source.ExpectedStoreID = "wrong-store"
		case "store_domain":
			source.ExpectedStoreDomain = "wrong.myshopify.com"
		case "property_id":
			source.ExpectedPropertyID = "999"
		case "base_url":
			source.ExpectedBaseURL = "https://wrong.example/api"
		}
		profile.Sources[sourceName] = source
		if err := platform.SaveProfile(profile); err != nil {
			t.Fatal(err)
		}
		before := adapter.calls
		err := run("whoami", "--client-profile", "env-fixture")
		var gateErr *platform.GateError
		if !errors.As(err, &gateErr) || gateErr.Outcome != platform.GateMismatch || adapter.calls != before+1 {
			t.Fatalf("expected %s assertion was bypassed", field)
		}
	}
	for _, bad := range []string{"env:", "env:1KEY", "env:KEY-NAME", "env:KEY=value", "env:$KEY", "env: KEY", "env:KEY\n"} {
		source := original
		source.CredentialRef = bad
		if err := source.Validate(); err == nil {
			t.Fatalf("invalid environment reference accepted: %q", bad)
		}
	}
	if err := filepath.WalkDir(rootDir, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, secret := range secrets {
			if bytes.Contains(data, []byte(secret)) {
				t.Fatal("resolved credential persisted in profile, receipt, cache, or state")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestProfileBoundLegacyCredentialEnvironment(t *testing.T) {
	t.Setenv("KLAVIYO_API_KEY", "synthetic-env-credential")
	session := &platform.Session{SourceProfile: platform.SourceProfile{CredentialRef: "env:KLAVIYO_API_KEY"}}
	if err := validateKlaviyoLegacyInputs(RootCmd(), &rootFlags{}, session); err != nil {
		t.Fatal("profile-bound credential environment was treated as an override")
	}
	session.SourceProfile.CredentialRef = "op://Fixture/Item/token"
	if err := validateKlaviyoLegacyInputs(RootCmd(), &rootFlags{}, session); err == nil || !strings.Contains(err.Error(), "KLAVIYO_API_KEY") {
		t.Fatal("an unreferenced legacy credential bypassed the conflict guard")
	}
	t.Setenv("KLAVIYO_API_KEY", "")
	t.Setenv("KLAVIYO_BASE_URL", "https://unexpected.example")
	session.SourceProfile.AdditionalCredentialRefs = map[string]string{"base": "env:KLAVIYO_BASE_URL"}
	if err := validateKlaviyoLegacyInputs(RootCmd(), &rootFlags{}, session); err == nil || !strings.Contains(err.Error(), "KLAVIYO_BASE_URL") {
		t.Fatal("a base URL override bypassed the conflict guard through a credential reference")
	}
}

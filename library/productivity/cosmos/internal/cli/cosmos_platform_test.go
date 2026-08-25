// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/cosmos/internal/platform"
)

func TestCosmosPlatformRegistrationRequiresTenantIdentity(t *testing.T) {
	valid := platform.SourceProfile{
		CredentialRef:     "env:COSMOS_TOKEN",
		ExpectedAccountID: "123",
		ExpectedBaseURL:   "https://api.cosmos.so",
	}
	if err := validateCosmosSourceProfile(valid); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
	missingIdentity := valid
	missingIdentity.ExpectedAccountID = ""
	if err := validateCosmosSourceProfile(missingIdentity); err == nil {
		t.Fatal("profile without immutable account identity was accepted")
	}
	wrongHost := valid
	wrongHost.ExpectedBaseURL = "https://attacker.example"
	if err := validateCosmosSourceProfile(wrongHost); err == nil {
		t.Fatal("profile with an untrusted Cosmos endpoint was accepted")
	}
}

func TestCosmosEnvironmentResolverAllowsOnlyCosmosToken(t *testing.T) {
	t.Setenv("COSMOS_TOKEN", "fixture-token")
	got, err := (cosmosEnvironmentResolver{}).Resolve(context.Background(), "env:COSMOS_TOKEN")
	if err != nil || string(got) != "fixture-token" {
		t.Fatalf("resolve COSMOS_TOKEN = %q, %v", got, err)
	}
	if _, err := (cosmosEnvironmentResolver{}).Resolve(context.Background(), "env:OTHER_SECRET"); err == nil {
		t.Fatal("resolver accepted an arbitrary environment variable")
	}
}

// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package platform

import (
	"bytes"
	"context"
	"testing"
)

func TestMixedEnvironmentAndProviderReferences(t *testing.T) {
	t.Setenv("PP_TEST_MIXED_CREDENTIAL", "  synthetic value\n")
	value, err := (OnePasswordResolver{Binary: "/no-1password-installed"}).Resolve(context.Background(), "env:PP_TEST_MIXED_CREDENTIAL")
	if err != nil || !bytes.Equal(value, []byte("  synthetic value\n")) {
		t.Fatal("default resolver required 1Password for an environment reference")
	}
	source := SourceProfile{
		CredentialRef:            "op://Fixture/Item/token",
		UsernameRef:              "env:PP_TEST_MIXED_CREDENTIAL",
		AdditionalCredentialRefs: map[string]string{"extra": "env:PP_TEST_MIXED_CREDENTIAL"},
	}
	resolver := fixtureResolver{source.CredentialRef: []byte("provider-fixture-value")}
	credentials, err := resolveCredentials(context.Background(), resolver, source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(credentials["credential"], []byte("provider-fixture-value")) ||
		!bytes.Equal(credentials["username"], []byte("  synthetic value\n")) ||
		!bytes.Equal(credentials["extra"], []byte("  synthetic value\n")) {
		t.Fatal("mixed references changed provider behavior or trimmed environment values")
	}
	buffer := credentials["username"]
	credentials.Zero()
	if len(credentials) != 0 || !bytes.Equal(buffer, make([]byte, len(buffer))) {
		t.Fatal("environment credential buffers were not zeroed")
	}
	if _, err := resolveCredentials(context.Background(), nil, source); err == nil {
		t.Fatal("provider reference bypassed its resolver")
	}
}

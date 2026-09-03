// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestAuthLoginDoesNotAcceptClientSecretFlag(t *testing.T) {
	cmd := newAuthLoginCmd(&rootFlags{})
	if flag := cmd.Flags().Lookup("client-secret"); flag != nil {
		t.Fatal("client-secret must be environment-only, but a command-line flag is registered")
	}
}

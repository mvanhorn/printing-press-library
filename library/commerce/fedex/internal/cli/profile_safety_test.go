// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestProfileCannotGrantMutationApproval(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	profile := &Profile{Values: map[string]string{
		"yes":                 "true",
		"operation-id":        "synthetic-operation",
		"confirmation-digest": "sha256:synthetic",
	}}
	if err := ApplyProfileToFlags(cmd, profile); err != nil {
		t.Fatalf("ApplyProfileToFlags: %v", err)
	}
	if flags.yes || flags.operationID != "" || flags.confirmationDigest != "" {
		t.Fatalf("profile granted mutation authority: %#v", flags)
	}
}

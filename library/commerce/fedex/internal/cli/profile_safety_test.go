// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestProfileCannotGrantMutationApproval(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	profile := &Profile{Values: map[string]string{
		"yes":             "true",
		"allow-duplicate": "true",
	}}
	if err := ApplyProfileToFlags(cmd, profile); err != nil {
		t.Fatalf("ApplyProfileToFlags: %v", err)
	}
	if flags.yes || flags.allowDuplicate {
		t.Fatalf("profile granted mutation authority: %#v", flags)
	}
}

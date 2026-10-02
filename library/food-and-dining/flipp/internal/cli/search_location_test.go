// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSearchRequiresExplicitZIP(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"search", "coffee", "--data-source", "local", "--json"})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--zip is required") {
		t.Fatalf("search error = %v, want required ZIP", err)
	}
}

// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommandUsesConfiguredOutput(t *testing.T) {
	cmd := newVersionCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command: %v", err)
	}
	if got := out.String(); !strings.Contains(got, version) {
		t.Fatalf("version output %q does not contain %q", got, version)
	}
}

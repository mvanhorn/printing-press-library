// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionUsesCommandOutputWriter(t *testing.T) {
	cmd := newVersionCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.Run(cmd, nil)
	if got := output.String(); !strings.Contains(got, version) {
		t.Fatalf("captured version output = %q, want version %q", got, version)
	}
}

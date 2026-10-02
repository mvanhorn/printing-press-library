// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"testing"
)

func TestVersionCommandUsesConfiguredWriter(t *testing.T) {
	cmd := newVersionCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), fmt.Sprintf("version %s\n", version); got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
}

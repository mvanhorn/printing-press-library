// Copyright 2026 sidduHERE and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"testing"
)

func TestInstallInvocationPipxUpgrade(t *testing.T) {
	program, args, ok := installInvocation("pipx", true)

	if !ok {
		t.Fatal("expected pipx upgrade to be supported")
	}
	if program != "pipx" {
		t.Fatalf("program = %q, want pipx", program)
	}
	if want := []string{"upgrade", "agentpool-cli"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestInstallInvocationUnsupportedManager(t *testing.T) {
	_, _, ok := installInvocation("brew", false)

	if ok {
		t.Fatal("expected unsupported manager")
	}
}

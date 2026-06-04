// Copyright 2026 sidduHERE and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"strings"
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

func TestInstallPreviewUsesSelectedManager(t *testing.T) {
	lines, ok := installPreviewLines("pipx", false)

	if !ok {
		t.Fatal("expected pipx preview to be supported")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "pipx install agentpool-cli") {
		t.Fatalf("preview = %q, want pipx install command", joined)
	}
	if strings.Contains(joined, "uv tool install agentpool-cli") {
		t.Fatalf("preview = %q, should not show uv install for pipx manager", joined)
	}
	if !strings.Contains(joined, "--run --manager=pipx") {
		t.Fatalf("preview = %q, want manager-specific run hint", joined)
	}
}

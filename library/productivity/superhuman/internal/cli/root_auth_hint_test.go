// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestAddAuthLoginChromeHint401(t *testing.T) {
	base := apiErr(errors.New("gmail returned 401"))
	got := addAuthLoginChromeHint(base)
	if got == base {
		t.Fatal("expected wrapped error with hint")
	}
	if !strings.Contains(got.Error(), "auth login --chrome") {
		t.Fatalf("missing chrome hint: %v", got)
	}
	if ExitCode(got) != 5 {
		t.Fatalf("exit code = %d, want 5", ExitCode(got))
	}
}

func TestAddAuthLoginChromeHintNoDuplicate(t *testing.T) {
	base := errors.New("unauthorized; run auth login --chrome")
	got := addAuthLoginChromeHint(base)
	if got != base {
		t.Fatal("expected existing hint to be preserved")
	}
}

func TestAddAuthLoginChromeHintIgnoresOtherErrors(t *testing.T) {
	base := errors.New("boom")
	got := addAuthLoginChromeHint(base)
	if got != base {
		t.Fatal("expected unrelated error unchanged")
	}
}

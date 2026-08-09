// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestRecallQueryReadsShellMetacharactersLiterallyFromStdin(t *testing.T) {
	want := "what aired after $(touch /tmp/should-not-run) and `whoami`?"
	got, err := recallQuery(strings.NewReader(want+"\n"), []string{"-"})
	if err != nil {
		t.Fatalf("recallQuery: %v", err)
	}
	if got != want {
		t.Fatalf("recallQuery = %q, want %q", got, want)
	}
}

func TestRecallQueryRejectsMixedStdinMarker(t *testing.T) {
	if _, err := recallQuery(strings.NewReader("ignored"), []string{"-", "extra"}); err == nil {
		t.Fatal("mixed stdin marker was accepted")
	}
}

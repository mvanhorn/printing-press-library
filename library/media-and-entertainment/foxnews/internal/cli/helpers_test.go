package cli

import (
	"bytes"
	"testing"
)

func TestWantsHumanTablePipedStdoutUsesJSON(t *testing.T) {
	flags := &rootFlags{}
	if wantsHumanTable(bytes.NewBuffer(nil), flags) {
		t.Fatal("piped writer should not use human table")
	}
}

func TestWantsHumanTableExplicitJSON(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	// os.Stdout is a terminal in CI sometimes; asJSON must force JSON path regardless.
	if wantsHumanTable(nil, flags) {
		t.Fatal("--json should disable human table")
	}
}

package cli

import (
	"strings"
	"testing"
)

func TestImportIsRejected(t *testing.T) {
	cmd := newImportCmd(&rootFlags{})
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected import to fail on a search-only catalog")
	}
	if !strings.Contains(err.Error(), "search-only") {
		t.Fatalf("unexpected error: %v", err)
	}
}

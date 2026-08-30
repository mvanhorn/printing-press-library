package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestLayersAuthDoesNotAcceptOrPersistTokens(t *testing.T) {
	auth := newAuthCmd(&rootFlags{})
	for _, child := range auth.Commands() {
		if child.Name() == "set-token" {
			t.Fatal("auth set-token must not exist; Layers credentials are process-scoped")
		}
	}

	setup := newAuthSetupCmd(&rootFlags{})
	var stdout bytes.Buffer
	setup.SetOut(&stdout)
	setup.SetArgs(nil)
	if err := setup.Execute(); err != nil {
		t.Fatalf("auth setup: %v", err)
	}
	if strings.Contains(stdout.String(), "set-token") {
		t.Fatalf("auth setup advertises credential persistence:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "export LAYERS_TOKEN") {
		t.Fatalf("auth setup does not document process-scoped authentication:\n%s", stdout.String())
	}
}

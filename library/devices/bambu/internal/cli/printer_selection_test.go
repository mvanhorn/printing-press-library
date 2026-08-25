package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrinterSelectionAcceptedOnObservations(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"observations", "--printer", "shop", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"printer": "shop"`) {
		t.Fatalf("observations dry-run ignored printer: %s", output.String())
	}
}

func TestDiscoverDryRunSurfacesPrinterSelection(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"discover", "--printer", "shop", "--dry-run", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"printer": "shop"`) {
		t.Fatalf("dry-run output ignored printer: %s", output.String())
	}
}

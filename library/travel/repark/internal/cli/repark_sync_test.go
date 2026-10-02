package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/cliutil/testenv"
)

func TestReparkSyncRequiresBoundedSuppliedWindow(t *testing.T) {
	t.Setenv("REPARK_SYNC_RANGE", "")
	for _, tc := range []struct {
		rangeValue string
		valid      bool
	}{{"C34.663534,135.516310N34.664W135.515S34.663E135.517", true}, {"", false}, {"C34,135N40W120S20E150", false}, {"C34,135N35W136S33E137", false}, {"bad", false}} {
		p, err := parseSyncUserParams([]string{"range=" + tc.rangeValue}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if (validateReparkSyncParams(p) == nil) != tc.valid {
			t.Errorf("range %q validity", tc.rangeValue)
		}
	}
}

func TestReparkSyncDryRunUsesEnvironmentWindow(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("REPARK_SYNC_RANGE", "C34.663534,135.516310N34.664W135.515S34.663E135.517")
	cmd := RootCmd()
	cmd.SetArgs([]string{"sync", "--dry-run", "--json", "--db", filepath.Join(t.TempDir(), "preview.db")})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("environment preview failed: %v; %s", err, output.String())
	}
	if strings.Contains(output.String(), "missing_required_params") || !strings.Contains(output.String(), `"success":1`) {
		t.Fatalf("environment window did not produce a source preview: %s", output.String())
	}
}

func TestReparkSyncOperatorEnvironmentScopeAndFlagPrecedence(t *testing.T) {
	window := "C34.663534,135.516310N34.664W135.515S34.663E135.517"
	t.Setenv("REPARK_SYNC_RANGE", window)
	p, err := parseSyncUserParams(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateReparkSyncParams(p); err != nil {
		t.Fatal(err)
	}
	request := map[string]string{}
	p.applyTo("site", request, false)
	if request["range"] != window {
		t.Fatalf("environment scope did not reach request: %v", request)
	}
	for _, explicit := range []string{"", "invalid", "C34,135N40W120S20E150"} {
		p, err := parseSyncUserParams([]string{"range=" + explicit}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if validateReparkSyncParams(p) == nil {
			t.Errorf("invalid explicit flag %q must override environment scope", explicit)
		}
	}
}

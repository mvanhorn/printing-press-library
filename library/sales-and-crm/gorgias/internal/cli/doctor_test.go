package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDoctorReportsResolvedVersion(t *testing.T) {
	oldVersion := version
	version = "9.8.7-test"
	t.Cleanup(func() { version = oldVersion })

	t.Setenv("GORGIAS_CONFIG", "")
	t.Setenv("GORGIAS_USERNAME", "")
	t.Setenv("GORGIAS_API_KEY", "")
	t.Setenv("GORGIAS_BASE_URL", "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	flags := &rootFlags{asJSON: true}
	cmd := newDoctorCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--fail-on", "never"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor execute: %v", err)
	}

	var report map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("doctor JSON: %v\n%s", err, out.String())
	}
	if got := report["version"]; got != "9.8.7-test" {
		t.Fatalf("doctor version = %v, want resolved Version()", got)
	}
}

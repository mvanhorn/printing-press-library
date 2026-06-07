package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCommandExposesExpectedCommandShape(t *testing.T) {
	cmd := newRootCmd(defaultApp())

	assertHasCommand(t, cmd, "doctor")
	assertHasCommand(t, cmd, "subscriptions")
	assertHasCommand(t, cmd, "spend")
	assertHasCommand(t, cmd, "anomalies")
	assertHasCommand(t, cmd, "tags")
	assertHasCommand(t, cmd, "price")

	spend := mustCommand(t, cmd, "spend")
	assertHasCommand(t, spend, "summary")
	assertHasCommand(t, spend, "by-service")
	assertHasCommand(t, spend, "by-resource-group")
	assertHasCommand(t, spend, "by-tag")

	price := mustCommand(t, cmd, "price")
	assertHasCommand(t, price, "search")

	tags := mustCommand(t, cmd, "tags")
	assertHasCommand(t, tags, "untagged")
}

func TestRootCommandDoesNotRegisterDuplicateDoctor(t *testing.T) {
	cmd := newRootCmd(defaultApp())

	var count int
	for _, child := range cmd.Commands() {
		if child.Name() == "doctor" {
			count++
		}
	}

	if count != 1 {
		t.Fatalf("doctor command registered %d times, want 1", count)
	}
}

func TestVersionUsesConfiguredOutput(t *testing.T) {
	var out bytes.Buffer
	app := defaultApp()
	app.out = &out

	cmd := newRootCmd(app)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	if got := out.String(); !strings.Contains(got, "azure-cost-admin-pp-cli") {
		t.Fatalf("version output %q did not include binary name", got)
	}
}

func assertHasCommand(t *testing.T, parent *cobra.Command, name string) {
	t.Helper()

	if mustCommand(t, parent, name) == nil {
		t.Fatalf("unreachable")
	}
}

func mustCommand(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()

	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	t.Fatalf("missing command %q", name)
	return nil
}

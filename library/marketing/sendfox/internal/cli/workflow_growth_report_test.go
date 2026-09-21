// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil/testenv"
)

func TestNovelWorkflowGrowthReportHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"workflow", "growth-report", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workflow growth-report --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "growth-report", "--previous", "--window"} {
		if !strings.Contains(help, want) {
			t.Fatalf("workflow growth-report --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelWorkflowGrowthReportRejectsNonPositiveWindow(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"workflow", "growth-report", "--input", "../../examples/snapshot.json", "--window", "0s"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--window must be positive") {
		t.Fatalf("error = %v, output = %s", err, out.String())
	}
}

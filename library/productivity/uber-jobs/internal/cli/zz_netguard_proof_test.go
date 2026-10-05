// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil/testenv"
)

// TestNetGuardRefusesRealHosts proves a command pointed at the real careers
// site cannot leave the test binary: it fails with the transport exit code
// and the guard's message.
func TestNetGuardRefusesRealHosts(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("UBER_JOBS_BASE_URL", "https://jobs.uber.com")
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"get", "303232", "--data-source", "live", "--json"})
	err := cmd.Execute()
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != ExitTransport {
		t.Fatalf("get against the real host: err = %v, want exit %d", err, ExitTransport)
	}
	if !strings.Contains(err.Error(), "test net guard") {
		t.Fatalf("error does not come from the net guard: %v", err)
	}
}

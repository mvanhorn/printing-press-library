// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentModeDoesNotAuthorizeMutations(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--agent", "version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if flags.yes {
		t.Fatal("--agent must not imply --yes")
	}
	if !flags.agent || !flags.asJSON || !flags.compact || !flags.noInput {
		t.Fatalf("agent-safe output defaults were not applied: %#v", flags)
	}
}

func TestExplicitYesStillAuthorizesMutations(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--agent", "--yes", "version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !flags.yes {
		t.Fatal("explicit --yes must remain set")
	}
}

type fakeFedEx struct {
	calls int
	paths []string
}

func newFakeFedEx(t *testing.T) *fakeFedEx {
	t.Helper()
	fake := &fakeFedEx{}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.calls++
		fake.paths = append(fake.paths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"transactionShipments":[]}}`))
	}))
	t.Cleanup(api.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	t.Setenv("FEDEX_BASE_URL", api.URL)
	t.Setenv("FEDEX_API_KEY", "synthetic-test-token")
	return fake
}

func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd.Execute()
}

// Regression: #1912 made every write outside four commands fail even with
// --yes. Generated write commands must run with an explicit --yes.
func TestUnwiredMutatingCommandRunsWithYes(t *testing.T) {
	fake := newFakeFedEx(t)
	if err := runCLI(t, "--json", "--yes", "endofday", "close", "--account-number", "123456789"); err != nil {
		t.Fatalf("endofday close --yes: %v", err)
	}
	if fake.calls != 1 || fake.paths[0] != "POST /ship/v1/endofday/" {
		t.Fatalf("endofday close --yes requests=%v, want one POST", fake.paths)
	}
}

func TestMutatingCommandWithoutYesSendsNothing(t *testing.T) {
	fake := newFakeFedEx(t)
	for _, args := range [][]string{
		{"--json", "endofday", "close", "--account-number", "123456789"},
		{"--agent", "endofday", "close", "--account-number", "123456789"},
	} {
		err := runCLI(t, args...)
		if err == nil || !strings.Contains(err.Error(), "--yes") {
			t.Fatalf("%v: err=%v, want a --yes approval error", args, err)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("unapproved writes reached FedEx %d times", fake.calls)
	}
}

func TestDryRunWriteNeedsNoApproval(t *testing.T) {
	fake := newFakeFedEx(t)
	if err := runCLI(t, "--json", "--dry-run", "endofday", "close", "--account-number", "123456789"); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("dry-run reached FedEx %d times", fake.calls)
	}
}

func TestShipmentCreateRunsWithYesAndRefusesDuplicate(t *testing.T) {
	fake := newFakeFedEx(t)
	shipment := []string{
		"--json", "--yes",
		"shipments", "create",
		"--requested-shipment", `{"serviceType":"FEDEX_GROUND"}`,
		"--account-number", `{"value":"123456789"}`,
	}
	if err := runCLI(t, shipment...); err != nil {
		t.Fatalf("shipments create --yes: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("first create requests=%d, want 1", fake.calls)
	}
	err := runCLI(t, shipment...)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("identical second create err=%v, want duplicate refusal", err)
	}
	if fake.calls != 1 {
		t.Fatalf("duplicate create reached FedEx: requests=%d, want 1", fake.calls)
	}
	if err := runCLI(t, append([]string{"--allow-duplicate"}, shipment...)...); err != nil {
		t.Fatalf("forced duplicate: %v", err)
	}
	if fake.calls != 2 {
		t.Fatalf("forced duplicate requests=%d, want 2", fake.calls)
	}
}

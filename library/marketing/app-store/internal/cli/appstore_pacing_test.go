// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
)

// A send slot the run may not wait for is a "try again later" outcome: exit
// 7 like a throttle, and fan-out loops stop on it instead of recording one
// refusal per remaining item.
func TestPacingRefusalExits7(t *testing.T) {
	paced := &appstore.PacingError{Host: "apps.apple.com", Class: appstore.RatePages, Wait: 18 * time.Second, Interval: 20 * time.Second}
	for _, err := range []error{paced, fmt.Errorf("chart page: %w", paced)} {
		got := classifyAppStoreErr(err)
		if ExitCode(got) != 7 {
			t.Fatalf("pacing refusal %v: exit %d, want 7", err, ExitCode(got))
		}
		if !strings.Contains(got.Error(), "nothing was sent") || !strings.Contains(got.Error(), "run it again in 18s") {
			t.Fatalf("pacing message = %q", got.Error())
		}
		if !isRateLimited(err) {
			t.Fatalf("isRateLimited(%v) = false; fan-out loops would keep going", err)
		}
	}
	if isRateLimited(errors.New("boom")) {
		t.Fatal("an ordinary error is not a rate limit")
	}
}

func TestHarnessWaitUntilFitsEachHarnessBudget(t *testing.T) {
	t.Setenv(cliutil.VerifyEnvVar, "")
	t.Setenv(cliutil.DogfoodEnvVar, "")
	if got := harnessWaitUntil(); !got.IsZero() {
		t.Fatalf("outside a harness WaitUntil = %v, want none (only --timeout bounds waits)", got)
	}
	// The budget counts from when the client is built, so a long-lived MCP
	// server gets a fresh budget per tool call.
	within := func(got time.Time, before time.Time, budget time.Duration) bool {
		return !got.Before(before.Add(budget)) && !got.After(time.Now().Add(budget))
	}
	t.Setenv(cliutil.DogfoodEnvVar, "1")
	before := time.Now()
	if got := harnessWaitUntil(); !within(got, before, dogfoodMaxPacingWait) {
		t.Fatalf("live matrix WaitUntil = %v, want now + %v", got, dogfoodMaxPacingWait)
	}
	t.Setenv(cliutil.VerifyEnvVar, "1")
	before = time.Now()
	if got := harnessWaitUntil(); !within(got, before, verifyMaxPacingWait) {
		t.Fatalf("verify WaitUntil = %v, want now + %v", got, verifyMaxPacingWait)
	}
	// Each budget leaves room for the requests after the last wait.
	if verifyMaxPacingWait >= 15*time.Second-5*time.Second || dogfoodMaxPacingWait >= 30*time.Second-5*time.Second {
		t.Fatal("a harness wait budget leaves less than 5 s for the requests themselves")
	}
}

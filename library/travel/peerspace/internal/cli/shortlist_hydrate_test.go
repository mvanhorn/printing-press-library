// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestNovelShortlistHydrateHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"shortlist", "hydrate", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("shortlist hydrate --help error = %v", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "hydrate", "listing-id", "board-id"} {
		if !strings.Contains(help, want) {
			t.Fatalf("shortlist hydrate --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestCalendarAvailabilityHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"calendar", "availability-start", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("calendar availability-start --help: %v", err)
	}
	if !strings.Contains(out.String(), "space-id") {
		t.Fatalf("missing space-id in help:\n%s", out.String())
	}
}

func TestContractsGuestQuoteHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"contracts", "guest-quote", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("contracts guest-quote --help: %v", err)
	}
	help := out.String()
	for _, want := range []string{"listing-id", "prepare-only", "start-index"} {
		if !strings.Contains(help, want) {
			t.Fatalf("missing %q:\n%s", want, help)
		}
	}
}

func TestVerificationListingHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"verification", "listing", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verification listing --help: %v", err)
	}
	if !strings.Contains(out.String(), "listing-id") {
		t.Fatalf("missing listing-id:\n%s", out.String())
	}
}

func TestPSAccessBearerHelper(t *testing.T) {
	got := psAccessBearerFromCookieHeader("a=1; PSAccess=abc%3D; b=2")
	if got != "Bearer abc=" {
		t.Fatalf("got %q", got)
	}
}

func TestContractsInquiryHelpWires(t *testing.T) {
	for _, args := range [][]string{
		{"contracts", "inquiry-quote", "--help"},
		{"contracts", "inquiry-send", "--help"},
		{"calendar", "availability-month", "--help"},
		{"spaces", "faqs-event", "--help"},
	} {
		cmd := RootCmd()
		cmd.SetArgs(args)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v --help: %v", args, err)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("%v help missing Usage:\n%s", args, out.String())
		}
	}
}

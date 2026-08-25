// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNetInvestmentArchiveHelpWires smoke-tests that the archive command
// resolves at runtime and renders useful --help output.
func TestNetInvestmentArchiveHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"net-investment", "archive", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("net-investment archive --help error = %v (command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "archive", "--date"} {
		if !strings.Contains(help, want) {
			t.Fatalf("net-investment archive --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestDdmmyyyyToArchiveDate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOk bool
	}{
		{"leap day", "29022020", "29-Feb-2020", true},
		{"ordinary date", "10072026", "10-Jul-2026", true},
		{"non-leap Feb 29 rejected", "29022021", "", false},
		{"April 31 rollover rejected", "31042020", "", false},
		{"wrong length", "1072026", "", false},
		{"non-numeric", "abcd2026", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ddmmyyyyToArchiveDate(tt.input)
			if ok != tt.wantOk {
				t.Fatalf("ddmmyyyyToArchiveDate(%q) ok = %v, want %v", tt.input, ok, tt.wantOk)
			}
			if ok && got != tt.want {
				t.Fatalf("ddmmyyyyToArchiveDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestArchiveReportingDateFilter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		match bool
	}{
		{"real date", "29-Feb-2020", true},
		{"total for month", "Total for February", false},
		{"total for year", "Total for 2020", false},
		{"footnote smear", "The data presented for February 20, 2020 is compiled...", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := archiveReportingDateRe.MatchString(tt.value); got != tt.match {
				t.Fatalf("archiveReportingDateRe.MatchString(%q) = %v, want %v", tt.value, got, tt.match)
			}
		})
	}
}

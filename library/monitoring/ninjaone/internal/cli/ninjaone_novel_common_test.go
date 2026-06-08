// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/spf13/cobra"
)

// jsonNum builds a json.Number from an int for test fixtures.
func jsonNum(n int64) json.Number {
	return json.Number(strconv.FormatInt(n, 10))
}

// runNovelDryRun builds the command via newCmd, sets --dry-run, runs it with
// the given args, and returns combined stdout and any error. No network.
func runNovelDryRun(t *testing.T, newCmd func(*rootFlags) *cobra.Command, args ...string) (string, error) {
	t.Helper()
	flags := &rootFlags{dryRun: true}
	cmd := newCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestOrgMatches(t *testing.T) {
	tests := []struct {
		name    string
		filter  string
		orgID   int64
		orgName string
		want    bool
	}{
		{"empty matches all", "", 7, "Acme", true},
		{"numeric id match", "7", 7, "Acme", true},
		{"numeric id mismatch", "8", 7, "Acme", false},
		{"name substring ci", "acm", 7, "Acme Corp", true},
		{"name no match", "globex", 7, "Acme Corp", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orgMatches(tt.filter, tt.orgID, tt.orgName); got != tt.want {
				t.Fatalf("orgMatches(%q,%d,%q) = %v, want %v", tt.filter, tt.orgID, tt.orgName, got, tt.want)
			}
		})
	}
}

func TestCfValueEmpty(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"null", `null`, true},
		{"empty string", `""`, true},
		{"whitespace string", `"   "`, true},
		{"empty array", `[]`, true},
		{"empty object", `{}`, true},
		{"non-empty string", `"abc"`, false},
		{"number", `42`, false},
		{"non-empty array", `[1]`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfValueEmpty(json.RawMessage(tt.raw)); got != tt.want {
				t.Fatalf("cfValueEmpty(%s) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestMissingRequiredFields(t *testing.T) {
	fields := map[string]json.RawMessage{
		"assetTag": json.RawMessage(`"AT-1"`),
		"owner":    json.RawMessage(`""`),
		"Warranty": json.RawMessage(`null`),
	}
	tests := []struct {
		name     string
		required []string
		want     []string
	}{
		{"present", []string{"assetTag"}, []string{}},
		{"empty value missing", []string{"owner"}, []string{"owner"}},
		{"null value missing", []string{"warranty"}, []string{"warranty"}},
		{"absent missing", []string{"location"}, []string{"location"}},
		{"mixed", []string{"assetTag", "owner", "missing"}, []string{"owner", "missing"}},
		{"case insensitive present", []string{"ASSETTAG"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := missingRequiredFields(fields, tt.required)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("missingRequiredFields = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimeBucket(t *testing.T) {
	tests := []struct {
		name      string
		epoch     int64
		windowSec int64
		want      int64
	}{
		{"floor to hour", 3661, 3600, 3600},
		{"exact boundary", 7200, 3600, 7200},
		{"zero window passthrough", 1234, 0, 1234},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := timeBucket(tt.epoch, tt.windowSec); got != tt.want {
				t.Fatalf("timeBucket(%d,%d) = %d, want %d", tt.epoch, tt.windowSec, got, tt.want)
			}
		})
	}
}

func TestParseCSVList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"a", []string{"a"}},
		{" a , b ,, c ", []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		got := parseCSVList(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("parseCSVList(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestBoundLimit(t *testing.T) {
	cases := []struct{ n, limit, want int }{
		{10, 0, 10},
		{10, 5, 5},
		{3, 5, 3},
		{10, -1, 10},
	}
	for _, c := range cases {
		if got := boundLimit(c.n, c.limit); got != c.want {
			t.Fatalf("boundLimit(%d,%d) = %d, want %d", c.n, c.limit, got, c.want)
		}
	}
}

func TestDeviceBestName(t *testing.T) {
	tests := []struct {
		name string
		d    njDevice
		want string
	}{
		{"system name", njDevice{ID: 1, SystemName: "host1"}, "host1"},
		{"display fallback", njDevice{ID: 2, DisplayName: "disp"}, "disp"},
		{"dns fallback", njDevice{ID: 3, DNSName: "dns"}, "dns"},
		{"id fallback", njDevice{ID: 4}, "4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.bestName(); got != tt.want {
				t.Fatalf("bestName = %q, want %q", got, tt.want)
			}
		})
	}
}

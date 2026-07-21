// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// guard tests: wiring smoke test plus table-driven coverage of the pure
// policy-merge function applyGuardFlags and the enforcement summary.

package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"
)

// TestNovelGuardHelpWires smoke-tests that the guard command resolves at
// runtime and renders useful --help output, including the set/status
// subcommands. Catches wiring regressions before review.
func TestNovelGuardHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"guard", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("guard --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "guard", "set", "status"} {
		if !strings.Contains(help, want) {
			t.Fatalf("guard --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestApplyGuardFlags(t *testing.T) {
	base := store.GuardPolicy{
		MaxOrderNotional: 500,
		DailyCapNotional: 2000,
		Allowlist:        []string{"AAPL", "MSFT"},
		Denylist:         []string{"GME"},
		KillSwitch:       false,
	}

	tests := []struct {
		name     string
		existing store.GuardPolicy
		changed  map[string]bool
		maxOrder float64
		dailyCap float64
		allow    []string
		deny     []string
		kill     bool
		disarm   bool
		want     store.GuardPolicy
	}{
		{
			name:     "no flags changed preserves existing policy",
			existing: base,
			changed:  map[string]bool{},
			// Non-zero flag values must be ignored when not marked changed.
			maxOrder: 999, dailyCap: 9999,
			allow: []string{"TSLA"}, deny: []string{"NVDA"},
			kill: true, disarm: true,
			want: base,
		},
		{
			name:     "only max-order changed, everything else preserved",
			existing: base,
			changed:  map[string]bool{"max-order": true},
			maxOrder: 750,
			want: store.GuardPolicy{
				MaxOrderNotional: 750,
				DailyCapNotional: 2000,
				Allowlist:        []string{"AAPL", "MSFT"},
				Denylist:         []string{"GME"},
			},
		},
		{
			name:     "max-order zero clears the per-order cap",
			existing: base,
			changed:  map[string]bool{"max-order": true},
			maxOrder: 0,
			want: store.GuardPolicy{
				DailyCapNotional: 2000,
				Allowlist:        []string{"AAPL", "MSFT"},
				Denylist:         []string{"GME"},
			},
		},
		{
			name:     "daily-cap changed independently",
			existing: base,
			changed:  map[string]bool{"daily-cap": true},
			dailyCap: 5000,
			want: store.GuardPolicy{
				MaxOrderNotional: 500,
				DailyCapNotional: 5000,
				Allowlist:        []string{"AAPL", "MSFT"},
				Denylist:         []string{"GME"},
			},
		},
		{
			name:     "allow replaces list wholesale and normalizes symbols",
			existing: base,
			changed:  map[string]bool{"allow": true},
			allow:    []string{" tsla ", "nvda", ""},
			want: store.GuardPolicy{
				MaxOrderNotional: 500,
				DailyCapNotional: 2000,
				Allowlist:        []string{"TSLA", "NVDA"},
				Denylist:         []string{"GME"},
			},
		},
		{
			name:     "empty allow clears the allowlist",
			existing: base,
			changed:  map[string]bool{"allow": true},
			allow:    nil,
			want: store.GuardPolicy{
				MaxOrderNotional: 500,
				DailyCapNotional: 2000,
				Denylist:         []string{"GME"},
			},
		},
		{
			name:     "deny replaces list wholesale",
			existing: base,
			changed:  map[string]bool{"deny": true},
			deny:     []string{"amc", "BBBY"},
			want: store.GuardPolicy{
				MaxOrderNotional: 500,
				DailyCapNotional: 2000,
				Allowlist:        []string{"AAPL", "MSFT"},
				Denylist:         []string{"AMC", "BBBY"},
			},
		},
		{
			name:     "kill engages the kill switch",
			existing: base,
			changed:  map[string]bool{"kill": true},
			kill:     true,
			want: store.GuardPolicy{
				MaxOrderNotional: 500,
				DailyCapNotional: 2000,
				Allowlist:        []string{"AAPL", "MSFT"},
				Denylist:         []string{"GME"},
				KillSwitch:       true,
			},
		},
		{
			name: "disarm clears an engaged kill switch",
			existing: store.GuardPolicy{
				MaxOrderNotional: 500,
				KillSwitch:       true,
			},
			changed: map[string]bool{"disarm": true},
			disarm:  true,
			want: store.GuardPolicy{
				MaxOrderNotional: 500,
				KillSwitch:       false,
			},
		},
		{
			name:     "kill=false explicitly disarms",
			existing: store.GuardPolicy{KillSwitch: true},
			changed:  map[string]bool{"kill": true},
			kill:     false,
			want:     store.GuardPolicy{KillSwitch: false},
		},
		{
			name:     "disarm=false leaves the kill switch alone",
			existing: store.GuardPolicy{KillSwitch: true},
			changed:  map[string]bool{"disarm": true},
			disarm:   false,
			want:     store.GuardPolicy{KillSwitch: true},
		},
		{
			name:     "caps and lists updated together",
			existing: store.GuardPolicy{},
			changed:  map[string]bool{"max-order": true, "daily-cap": true, "allow": true, "kill": true},
			maxOrder: 100, dailyCap: 300,
			allow: []string{"spy"},
			kill:  true,
			want: store.GuardPolicy{
				MaxOrderNotional: 100,
				DailyCapNotional: 300,
				Allowlist:        []string{"SPY"},
				KillSwitch:       true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyGuardFlags(tc.existing, tc.changed, tc.maxOrder, tc.dailyCap, tc.allow, tc.deny, tc.kill, tc.disarm)
			if !guardPoliciesEqual(got, tc.want) {
				t.Errorf("applyGuardFlags() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestApplyGuardFlagsDoesNotMutateExisting(t *testing.T) {
	existing := store.GuardPolicy{Allowlist: []string{"AAPL"}, Denylist: []string{"GME"}}
	_ = applyGuardFlags(existing, map[string]bool{"allow": true, "deny": true},
		0, 0, []string{"TSLA"}, []string{"AMC"}, false, false)
	if !reflect.DeepEqual(existing.Allowlist, []string{"AAPL"}) || !reflect.DeepEqual(existing.Denylist, []string{"GME"}) {
		t.Errorf("applyGuardFlags mutated its input: %+v", existing)
	}
}

func TestGuardEnforcementSummary(t *testing.T) {
	tests := []struct {
		name          string
		policy        store.GuardPolicy
		wantContains  []string
		wantLineCount int
	}{
		{
			name:          "empty policy says nothing enforced",
			policy:        store.GuardPolicy{},
			wantContains:  []string{"nothing"},
			wantLineCount: 1,
		},
		{
			name: "full policy lists every enforcement",
			policy: store.GuardPolicy{
				MaxOrderNotional: 500,
				DailyCapNotional: 2000,
				Allowlist:        []string{"AAPL"},
				Denylist:         []string{"GME"},
				KillSwitch:       true,
			},
			wantContains:  []string{"KILL SWITCH", "$500.00", "$2000.00", "AAPL", "GME"},
			wantLineCount: 5,
		},
		{
			name:          "kill switch alone",
			policy:        store.GuardPolicy{KillSwitch: true},
			wantContains:  []string{"KILL SWITCH ENGAGED"},
			wantLineCount: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lines := guardEnforcementSummary(tc.policy)
			if len(lines) != tc.wantLineCount {
				t.Fatalf("guardEnforcementSummary() returned %d lines, want %d: %v", len(lines), tc.wantLineCount, lines)
			}
			joined := strings.Join(lines, "\n")
			for _, want := range tc.wantContains {
				if !strings.Contains(joined, want) {
					t.Errorf("guardEnforcementSummary() missing %q in:\n%s", want, joined)
				}
			}
		})
	}
}

func TestGuardPolicyIsEmpty(t *testing.T) {
	if !guardPolicyIsEmpty(store.GuardPolicy{}) {
		t.Error("zero-value policy should be empty")
	}
	for name, p := range map[string]store.GuardPolicy{
		"max-order": {MaxOrderNotional: 1},
		"daily-cap": {DailyCapNotional: 1},
		"allowlist": {Allowlist: []string{"AAPL"}},
		"denylist":  {Denylist: []string{"GME"}},
		"kill":      {KillSwitch: true},
	} {
		if guardPolicyIsEmpty(p) {
			t.Errorf("policy with %s set should not be empty", name)
		}
	}
}

// guardPoliciesEqual compares policies treating nil and empty slices as equal.
func guardPoliciesEqual(a, b store.GuardPolicy) bool {
	sliceEq := func(x, y []string) bool {
		if len(x) != len(y) {
			return false
		}
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
		return true
	}
	return a.MaxOrderNotional == b.MaxOrderNotional &&
		a.DailyCapNotional == b.DailyCapNotional &&
		a.KillSwitch == b.KillSwitch &&
		sliceEq(a.Allowlist, b.Allowlist) &&
		sliceEq(a.Denylist, b.Denylist)
}

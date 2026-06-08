// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseAlertPredicate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr bool
		check   func(p alertPredicate) bool
	}{
		{"age greater", "age>72h", false, func(p alertPredicate) bool { return p.ageOp == ">" && p.ageSecs == 72*3600 }},
		{"age less", "age<1d", false, func(p alertPredicate) bool { return p.ageOp == "<" && p.ageSecs == 86400 }},
		{"org", "org=Acme", false, func(p alertPredicate) bool { return p.org == "Acme" }},
		{"condition", "condition=cpu high", false, func(p alertPredicate) bool { return p.condition == "cpu high" }},
		{"severity", "severity=critical", false, func(p alertPredicate) bool { return p.severity == "critical" }},
		{"combined AND", "age>72h,org=5,condition=disk", false, func(p alertPredicate) bool {
			return p.ageOp == ">" && p.org == "5" && p.condition == "disk"
		}},
		{"bad duration", "age>nope", true, nil},
		{"unknown clause", "foo=bar", true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := parseAlertPredicate(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil && !tt.check(p) {
				t.Fatalf("predicate check failed for %q: %+v", tt.in, p)
			}
		})
	}
}

func TestAlertPredicateMatches(t *testing.T) {
	now := int64(1_000_000)
	mk := func(create int64, cond, sev string) njAlert {
		a := njAlert{ConditionName: cond, Severity: sev}
		a.CreateTime = jsonNum(create)
		return a
	}
	tests := []struct {
		name    string
		pred    string
		alert   njAlert
		orgID   int64
		orgName string
		want    bool
	}{
		{"age over matches old", "age>1h", mk(now-7200, "cpu", "high"), 1, "Acme", true},
		{"age over rejects fresh", "age>1h", mk(now-60, "cpu", "high"), 1, "Acme", false},
		{"org by name", "org=acme", mk(now, "cpu", "high"), 1, "Acme Corp", true},
		{"org mismatch", "org=globex", mk(now, "cpu", "high"), 1, "Acme", false},
		{"condition substring", "condition=cp", mk(now, "cpu load", "high"), 1, "Acme", true},
		{"severity match", "severity=high", mk(now, "cpu", "high"), 1, "Acme", true},
		{"severity mismatch", "severity=low", mk(now, "cpu", "high"), 1, "Acme", false},
		{"combined all true", "age>1h,condition=cpu", mk(now-7200, "cpu", "high"), 1, "Acme", true},
		{"combined one false", "age>1h,condition=disk", mk(now-7200, "cpu", "high"), 1, "Acme", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := parseAlertPredicate(tt.pred)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := p.matches(tt.alert, tt.orgID, tt.orgName, now); got != tt.want {
				t.Fatalf("matches = %v, want %v", got, tt.want)
			}
		})
	}
}

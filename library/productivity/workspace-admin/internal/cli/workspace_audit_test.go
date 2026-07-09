// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Unit tests for the pure audit helpers shared by the GAT-style audit commands.

package cli

import "testing"

func TestEmailDomain(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"alice@example.com", "example.com"},
		{"Bob@Example.COM", "example.com"},
		{"noatsign", ""},
		{"trailing@", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := emailDomain(tc.in); got != tc.want {
			t.Errorf("emailDomain(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestScopeRiskTier(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		scopes    []string
		wantTier  string
		wantDrive bool
	}{
		{"full drive is high + fullDrive", []string{"https://www.googleapis.com/auth/drive"}, "High", true},
		{"mailbox full is high", []string{"https://mail.google.com/"}, "High", false},
		{"directory write is high", []string{"https://www.googleapis.com/auth/admin.directory.user"}, "High", false},
		{"gmail readonly is moderate", []string{"https://www.googleapis.com/auth/gmail.readonly"}, "Moderate", false},
		{"drive.file is moderate", []string{"https://www.googleapis.com/auth/drive.file"}, "Moderate", false},
		{"openid/email/profile is low", []string{"openid", "email", "https://www.googleapis.com/auth/userinfo.profile"}, "Low", false},
		{"max across scopes wins", []string{"openid", "https://www.googleapis.com/auth/drive"}, "High", true},
		{"empty is low", nil, "Low", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotTier, gotDrive := scopeRiskTier(tc.scopes)
			if gotTier != tc.wantTier || gotDrive != tc.wantDrive {
				t.Errorf("scopeRiskTier(%v) = (%q,%v), want (%q,%v)", tc.scopes, gotTier, gotDrive, tc.wantTier, tc.wantDrive)
			}
		})
	}
}

func TestRiskRankOrdering(t *testing.T) {
	t.Parallel()
	if !(riskRank("high") > riskRank("moderate") && riskRank("moderate") > riskRank("low")) {
		t.Fatalf("risk rank ordering wrong: high=%d moderate=%d low=%d", riskRank("high"), riskRank("moderate"), riskRank("low"))
	}
	if riskRank("medium") != riskRank("moderate") {
		t.Errorf("medium should equal moderate")
	}
}

func TestClassifyPermission(t *testing.T) {
	t.Parallel()
	internal := map[string]bool{"example.com": true}
	cases := []struct {
		name                    string
		permType, email, domain string
		wantExt                 bool
		wantType, wantWith      string
	}{
		{"anyone is external", "anyone", "", "", true, "anyone", ""},
		{"internal user not external", "user", "alice@example.com", "", false, "", ""},
		{"external user", "user", "bob@gmail.com", "", true, "external_user", "bob@gmail.com"},
		{"external group", "group", "team@partner.org", "", true, "external_group", "team@partner.org"},
		{"external domain", "domain", "", "partner.org", true, "external_domain", "partner.org"},
		{"internal domain not external", "domain", "", "example.com", false, "", ""},
		{"unknown type ignored", "weird", "x@y.z", "", false, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ext, shareType, with := classifyPermission(tc.permType, tc.email, tc.domain, internal)
			if ext != tc.wantExt || shareType != tc.wantType || with != tc.wantWith {
				t.Errorf("classifyPermission(%q,%q,%q) = (%v,%q,%q), want (%v,%q,%q)",
					tc.permType, tc.email, tc.domain, ext, shareType, with, tc.wantExt, tc.wantType, tc.wantWith)
			}
		})
	}
}

func TestAggregateDomainEdges(t *testing.T) {
	t.Parallel()
	shares := []driveExternalShare{
		{FileID: "f1", ShareType: "external_user", ExternalWith: "a@partner.org"},
		{FileID: "f2", ShareType: "external_user", ExternalWith: "b@partner.org"},
		{FileID: "f2", ShareType: "external_domain", ExternalWith: "partner.org"},
		{FileID: "f3", ShareType: "anyone", ExternalWith: ""},
	}
	edges := aggregateDomainEdges(shares)
	byDomain := map[string]domainEdge{}
	for _, e := range edges {
		byDomain[e.Domain] = e
	}
	partner, ok := byDomain["partner.org"]
	if !ok {
		t.Fatalf("expected partner.org edge, got %+v", edges)
	}
	if partner.FileCount != 2 {
		t.Errorf("partner.org file_count = %d, want 2", partner.FileCount)
	}
	if partner.UserCount != 2 { // a@partner.org and b@partner.org distinct
		t.Errorf("partner.org user_count = %d, want 2", partner.UserCount)
	}
	if _, ok := byDomain["(public link)"]; !ok {
		t.Errorf("expected (public link) edge for anyone share")
	}
	// edges are sorted by file count desc; partner.org (2) should lead.
	if edges[0].Domain != "partner.org" {
		t.Errorf("expected partner.org first, got %q", edges[0].Domain)
	}
}

func TestMergeTimeline(t *testing.T) {
	t.Parallel()
	in := []activityEvent{
		{Time: "2026-06-03T10:00:00Z", Name: "c"},
		{Time: "2026-06-01T10:00:00Z", Name: "a"},
		{Time: "", Name: "no-time"},
		{Time: "2026-06-02T10:00:00Z", Name: "b"},
	}
	got := mergeTimeline(in)
	wantOrder := []string{"a", "b", "c", "no-time"}
	for i, w := range wantOrder {
		if got[i].Name != w {
			t.Errorf("position %d = %q, want %q (order=%v)", i, got[i].Name, w, namesOf(got))
		}
	}
}

func namesOf(evs []activityEvent) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Name
	}
	return out
}

func TestParseReportsActivities(t *testing.T) {
	t.Parallel()
	data := []byte(`{"items":[
		{"id":{"time":"2026-06-01T10:00:00Z","applicationName":"login"},"actor":{"email":"u@example.com"},"ipAddress":"1.2.3.4","events":[{"name":"login_success","parameters":[{"name":"login_type","value":"google_password"}]}]},
		{"id":{"time":"2026-06-02T11:00:00Z","applicationName":"drive"},"actor":{"email":"u@example.com"},"events":[{"name":"edit"}]}
	]}`)
	events, err := parseReportsActivities(data)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Name != "login_success" || events[0].IP != "1.2.3.4" || events[0].Application != "login" {
		t.Errorf("event0 = %+v", events[0])
	}
	if events[0].Detail != "login_type=google_password" {
		t.Errorf("event0 detail = %q", events[0].Detail)
	}
	if _, err := parseReportsActivities(nil); err != nil {
		t.Errorf("nil data should not error, got %v", err)
	}
}

func TestIsLoginFailure(t *testing.T) {
	t.Parallel()
	yes := []string{"login_failure", "login_verification_failed", "suspicious_login", "login_blocked"}
	no := []string{"login_success", "logout", "2sv_enroll"}
	for _, n := range yes {
		if !isLoginFailure(n) {
			t.Errorf("isLoginFailure(%q) = false, want true", n)
		}
	}
	for _, n := range no {
		if isLoginFailure(n) {
			t.Errorf("isLoginFailure(%q) = true, want false", n)
		}
	}
}

func TestInternalDomainSet(t *testing.T) {
	t.Parallel()
	set := internalDomainSet("Example.com, partner.org ,")
	if !set["example.com"] || !set["partner.org"] {
		t.Errorf("internalDomainSet missing expected domains: %v", set)
	}
	if set[""] {
		t.Errorf("empty entry should be ignored")
	}
}

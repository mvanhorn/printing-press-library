// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package tsadmin

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeRoute(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{"192.168.1.0/24", "192.168.1.0/24", false},
		{" 10.0.0.0/8 ", "10.0.0.0/8", false},
		{"::/0", "::/0", false},
		{"0.0.0.0/0", "0.0.0.0/0", false},
		{"192.168.1.5/24", "", true},
		{"192.168.1.0", "", true},
		{"", "", true},
		{"not-a-route", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeRoute(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("NormalizeRoute(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
		if got != c.want {
			t.Fatalf("NormalizeRoute(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestPlanApproveKeepsExistingRoutes(t *testing.T) {
	adv := []string{"192.168.1.0/24", "0.0.0.0/0", "::/0"}
	en := []string{"192.168.1.0/24"}
	p := PlanApprove(adv, en, ExitRoutes())
	want := []string{"192.168.1.0/24", "0.0.0.0/0", "::/0"}
	if !reflect.DeepEqual(p.After, want) {
		t.Fatalf("After=%v want %v", p.After, want)
	}
	if !reflect.DeepEqual(p.Added, []string{"0.0.0.0/0", "::/0"}) {
		t.Fatalf("Added=%v", p.Added)
	}
	if p.NoChange || len(p.Removed) != 0 || len(p.Unadvertised) != 0 {
		t.Fatalf("unexpected plan %+v", p)
	}
}

func TestPlanApproveNoChangeAndUnadvertised(t *testing.T) {
	p := PlanApprove([]string{"10.0.0.0/16"}, []string{"10.0.0.0/16"}, []string{"10.0.0.0/16"})
	if !p.NoChange {
		t.Fatalf("expected no change: %+v", p)
	}
	p = PlanApprove([]string{"10.0.0.0/16"}, nil, []string{"172.16.0.0/12"})
	if !reflect.DeepEqual(p.Unadvertised, []string{"172.16.0.0/12"}) {
		t.Fatalf("Unadvertised=%v", p.Unadvertised)
	}
}

func TestPlanUnapproveRemovesOnlyRequested(t *testing.T) {
	en := []string{"192.168.1.0/24", "0.0.0.0/0", "::/0", "10.1.0.0/16"}
	p := PlanUnapprove(en, en, ExitRoutes())
	if !reflect.DeepEqual(p.After, []string{"192.168.1.0/24", "10.1.0.0/16"}) {
		t.Fatalf("After=%v", p.After)
	}
	if !reflect.DeepEqual(p.Removed, []string{"0.0.0.0/0", "::/0"}) {
		t.Fatalf("Removed=%v", p.Removed)
	}
	p = PlanUnapprove(en, en, []string{"172.16.0.0/12"})
	if !p.NoChange || !reflect.DeepEqual(p.NotEnabled, []string{"172.16.0.0/12"}) {
		t.Fatalf("expected no change with NotEnabled: %+v", p)
	}
}

func TestComputeRouteState(t *testing.T) {
	cases := []struct {
		name    string
		adv, en []string
		exit    string
		pending []string
		stale   []string
	}{
		{"none", nil, nil, ExitNone, []string{}, []string{}},
		{"approved", []string{"0.0.0.0/0", "::/0"}, []string{"0.0.0.0/0", "::/0"}, ExitApproved, []string{}, []string{}},
		{"pending", []string{"0.0.0.0/0", "::/0", "10.0.0.0/8"}, []string{"10.0.0.0/8"}, ExitPending, []string{"0.0.0.0/0", "::/0"}, []string{}},
		{"partial", []string{"0.0.0.0/0", "::/0"}, []string{"0.0.0.0/0"}, ExitPartial, []string{"::/0"}, []string{}},
		{"stale", []string{}, []string{"10.0.0.0/8"}, ExitNone, []string{}, []string{"10.0.0.0/8"}},
		{"enabled-not-advertised", nil, []string{"0.0.0.0/0", "::/0"}, ExitApprovedNotAdvertise, []string{}, []string{"0.0.0.0/0", "::/0"}},
	}
	for _, c := range cases {
		st := ComputeRouteState(c.adv, c.en)
		if st.ExitNode != c.exit {
			t.Fatalf("%s: exit=%s want %s", c.name, st.ExitNode, c.exit)
		}
		if !reflect.DeepEqual(st.Pending, c.pending) {
			t.Fatalf("%s: pending=%v want %v", c.name, st.Pending, c.pending)
		}
		if !reflect.DeepEqual(st.Stale, c.stale) {
			t.Fatalf("%s: stale=%v want %v", c.name, st.Stale, c.stale)
		}
	}
}

func TestSameRouteSet(t *testing.T) {
	if !SameRouteSet([]string{"::/0", "0.0.0.0/0"}, []string{"0.0.0.0/0", "::/0", "::/0"}) {
		t.Fatal("expected equal sets")
	}
	if SameRouteSet([]string{"10.0.0.0/8"}, []string{"10.0.0.0/16"}) {
		t.Fatal("expected different sets")
	}
}

func testDevices() []Device {
	return []Device{
		{NodeID: "nAAAA1CNTRL", ID: "1001", Hostname: "home-mac", Name: "home-mac.example-tailnet.ts.net", Addresses: []string{"100.64.0.1", "fd7a:115c:a1e0::1"}},
		{NodeID: "nBBBB2CNTRL", ID: "1002", Hostname: "laptop", Name: "laptop.example-tailnet.ts.net", Addresses: []string{"100.64.0.2"}},
		{NodeID: "nCCCC3CNTRL", ID: "1003", Hostname: "Laptop", Name: "laptop-1.example-tailnet.ts.net", Addresses: []string{"100.64.0.3"}},
	}
}

func TestMatchDevices(t *testing.T) {
	devs := testDevices()
	cases := []struct {
		sel  string
		want []string
	}{
		{"nAAAA1CNTRL", []string{"nAAAA1CNTRL"}},
		{"1002", []string{"nBBBB2CNTRL"}},
		{"100.64.0.3", []string{"nCCCC3CNTRL"}},
		{"fd7a:115c:a1e0::1", []string{"nAAAA1CNTRL"}},
		{"HOME-MAC", []string{"nAAAA1CNTRL"}},
		{"laptop", []string{"nBBBB2CNTRL", "nCCCC3CNTRL"}},
		{"laptop-1.example-tailnet.ts.net.", []string{"nCCCC3CNTRL"}},
		{"laptop-1", []string{"nCCCC3CNTRL"}},
		{"nas", nil},
		{"", nil},
	}
	for _, c := range cases {
		got := MatchDevices(devs, c.sel)
		var ids []string
		for _, d := range got {
			ids = append(ids, d.NodeID)
		}
		if !reflect.DeepEqual(ids, c.want) {
			t.Fatalf("MatchDevices(%q)=%v want %v", c.sel, ids, c.want)
		}
	}
}

func TestClassifyExpiry(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		expires  string
		disabled bool
		within   int
		status   string
		days     *int
		flagged  bool
	}{
		{"disabled", "2026-10-01T00:00:00Z", true, 30, ExpiryNever, nil, false},
		{"zero-time", "0001-01-01T00:00:00Z", false, 30, ExpiryUnknown, nil, false},
		{"empty", "", false, 30, ExpiryUnknown, nil, false},
		{"expired", "2026-09-29T12:00:00Z", false, 30, ExpiryExpired, intp(-1), true},
		{"expiring", "2026-10-10T12:00:00Z", false, 14, ExpiryExpiring, intp(10), true},
		{"ok", "2026-12-30T12:00:00Z", false, 14, ExpiryOK, intp(91), false},
		{"boundary", "2026-10-14T12:00:00Z", false, 14, ExpiryExpiring, intp(14), true},
	}
	for _, c := range cases {
		st, d, f := ClassifyExpiry(c.expires, c.disabled, c.within, now)
		if st != c.status || f != c.flagged || !reflect.DeepEqual(d, c.days) {
			t.Fatalf("%s: got (%s,%v,%v) want (%s,%v,%v)", c.name, st, deref(d), f, c.status, deref(c.days), c.flagged)
		}
	}
}

func TestSortExpiry(t *testing.T) {
	items := []ExpiryItem{{Hostname: "c", DaysLeft: nil}, {Hostname: "b", DaysLeft: intp(30)}, {Hostname: "a", DaysLeft: intp(-2)}}
	SortExpiry(items)
	got := []string{items[0].Hostname, items[1].Hostname, items[2].Hostname}
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("sort order %v", got)
	}
}

const samplePolicy = `// Example policy for tests.
{
	// Groups go here.
	"groups": {
		"group:admins": ["admin@example.com"],
	},
	"grants": [
		// Everyone can reach everything.
		{"src": ["*"], "dst": ["*"], "ip": ["*"]},
	],
}
`

func TestAddEntryAppendsAndPreservesComments(t *testing.T) {
	res, err := AddEntry([]byte(samplePolicy), "grants", []byte(`{"src":["autogroup:member"],"dst":["tag:nas"],"app":{"tailscale.com/cap/drive":[{"shares":["*"],"access":"rw"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	out := string(res.Text)
	for _, want := range []string{"// Example policy for tests.", "// Groups go here.", "// Everyone can reach everything.", "tailscale.com/cap/drive"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
	if res.AlreadyPresent || res.CreatedSection {
		t.Fatalf("unexpected flags %+v", res)
	}
	std, err := StandardizeJSON(res.Text)
	if err != nil {
		t.Fatalf("result is not valid hujson: %v", err)
	}
	if !strings.Contains(string(std), `"tag:nas"`) {
		t.Fatalf("standardized output missing entry: %s", std)
	}
}

func TestAddEntryCreatesSectionAndDedups(t *testing.T) {
	entry := []byte(`{"target":["autogroup:member"],"attr":["drive:access"]}`)
	res, err := AddEntry([]byte(samplePolicy), "nodeattrs", entry)
	if err != nil {
		t.Fatal(err)
	}
	if !res.CreatedSection || !strings.Contains(string(res.Text), `"nodeAttrs"`) {
		t.Fatalf("expected nodeAttrs section created: %+v\n%s", res, res.Text)
	}
	again, err := AddEntry(res.Text, "nodeAttrs", []byte(`{"attr":["drive:access"],"target":["autogroup:member"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !again.AlreadyPresent || string(again.Text) != string(res.Text) {
		t.Fatalf("expected idempotent no-op, got %+v", again)
	}
}

func TestAddEntryRejectsBadInput(t *testing.T) {
	if _, err := AddEntry([]byte(samplePolicy), "groups", []byte(`{}`)); err == nil {
		t.Fatal("expected error for non-list section")
	}
	if _, err := AddEntry([]byte(samplePolicy), "grants", []byte(`["not","object"]`)); err == nil {
		t.Fatal("expected error for non-object entry")
	}
	if _, err := AddEntry([]byte(`{not json`), "grants", []byte(`{}`)); err == nil {
		t.Fatal("expected error for unparseable policy")
	}
}

func TestInterpretValidate(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		ok     bool
		warn   bool
	}{
		{"empty-ok", 200, "", true, false},
		{"empty-object", 200, "{}", true, false},
		{"tests-failed", 200, `{"message":"test(s) failed","data":[{"user":"a@example.com","errors":["x"]}]}`, false, false},
		{"warnings", 200, `{"message":"warning(s) found","data":[]}`, true, true},
		{"bad-request", 400, `{"message":"line 3: unexpected token"}`, false, false},
		{"server", 500, ``, false, false},
	}
	for _, c := range cases {
		r := InterpretValidate(c.status, []byte(c.body))
		if r.OK != c.ok || r.Warnings != c.warn {
			t.Fatalf("%s: got %+v", c.name, r)
		}
	}
}

func TestLineDiff(t *testing.T) {
	a := "one\ntwo\nthree\nfour\n"
	b := "one\ntwo\nTWO-AND-A-HALF\nthree\nfive\n"
	added, removed := DiffChanges(LineDiff(a, b))
	if len(added) != 2 || added[0].Text != "TWO-AND-A-HALF" || added[0].NewLine != 3 || added[1].Text != "five" {
		t.Fatalf("added=%+v", added)
	}
	if len(removed) != 1 || removed[0].Text != "four" || removed[0].OldLine != 4 {
		t.Fatalf("removed=%+v", removed)
	}
	added, removed = DiffChanges(LineDiff(a, a))
	if len(added) != 0 || len(removed) != 0 {
		t.Fatalf("identical inputs produced changes: %v %v", added, removed)
	}
	u := UnifiedDiff(LineDiff(a, b), 1)
	if !strings.Contains(u, "+    3 | TWO-AND-A-HALF") || !strings.Contains(u, "-    4 | four") {
		t.Fatalf("unified diff missing lines:\n%s", u)
	}
}

func TestShareFilterAndRedeemable(t *testing.T) {
	accepted := Invite{ID: "1", Accepted: true}
	accepted.AcceptedBy = &struct {
		ID        int64  `json:"id"`
		LoginName string `json:"loginName"`
	}{ID: 9, LoginName: "Contractor@Example.com"}
	pending := Invite{ID: "2"}
	multi := Invite{ID: "3", Accepted: true, MultiUse: true}
	if !(ShareFilter{Pending: true}).Match(pending) || (ShareFilter{Pending: true}).Match(accepted) {
		t.Fatal("pending filter wrong")
	}
	if !(ShareFilter{AcceptedBy: "contractor@example.com"}).Match(accepted) || (ShareFilter{AcceptedBy: "x@example.com"}).Match(accepted) {
		t.Fatal("accepted-by filter wrong")
	}
	if accepted.Redeemable() || !pending.Redeemable() || !multi.Redeemable() {
		t.Fatal("redeemable wrong")
	}
	if got := RedactInviteURL("https://login.tailscale.com/admin/invite/abc123"); got != "https://login.tailscale.com/admin/invite/<redacted>" {
		t.Fatalf("redact=%q", got)
	}
}

func TestParseTargetAndIntersect(t *testing.T) {
	h, p, err := ParseTarget("nas:445")
	if err != nil || h != "nas" || p != "445" {
		t.Fatalf("ParseTarget nas:445 = %q %q %v", h, p, err)
	}
	if _, _, err := ParseTarget("[fd7a:115c:a1e0::1]:22"); err != nil {
		t.Fatalf("ipv6 target: %v", err)
	}
	for _, bad := range []string{"nas", "nas:0", "nas:http", ":22"} {
		if _, _, err := ParseTarget(bad); err == nil {
			t.Fatalf("ParseTarget(%q) expected error", bad)
		}
	}
	a := []PreviewMatch{{LineNumber: 10, Users: []string{"*"}}, {LineNumber: 20}}
	b := []PreviewMatch{{LineNumber: 20}, {LineNumber: 30}}
	got := IntersectByLine(a, b)
	if len(got) != 1 || got[0].LineNumber != 20 {
		t.Fatalf("intersect=%+v", got)
	}
}

func intp(i int) *int { return &i }

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestPolicyHelpersNeverMutateInput(t *testing.T) {
	policy := []byte(samplePolicy)
	before := string(policy)
	if _, err := StandardizeJSON(policy); err != nil {
		t.Fatal(err)
	}
	if string(policy) != before {
		t.Fatalf("StandardizeJSON mutated its input:\n%s", policy)
	}
	res, err := AddEntry(policy, "nodeAttrs", []byte(`{"target":["tag:nas"],"attr":["drive:share"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(policy) != before {
		t.Fatalf("AddEntry mutated its input:\n%s", policy)
	}
	text := append([]byte(nil), res.Text...)
	if _, err := StandardizeJSON(res.Text); err != nil {
		t.Fatal(err)
	}
	if string(res.Text) != string(text) || !strings.Contains(string(res.Text), "// Everyone can reach everything.") {
		t.Fatalf("validating the result must not blank its comments:\n%s", res.Text)
	}
}

func TestRuleStartLinesAndAmbiguity(t *testing.T) {
	policy := []byte("{\n\t\"grants\": [\n\t\t{\"src\": [\"*\"], \"dst\": [\"*\"], \"ip\": [\"*\"]},\n\t\t{\"src\": [\"a\"], \"dst\": [\"b\"], \"ip\": [\"22\"]}, {\"src\": [\"c\"], \"dst\": [\"d\"], \"ip\": [\"80\"]},\n\t],\n}\n")
	lines, err := RuleStartLines(policy)
	if err != nil {
		t.Fatal(err)
	}
	if lines[3] != 1 || lines[4] != 2 {
		t.Fatalf("rule lines = %v, want line 3 -> 1 and line 4 -> 2", lines)
	}
	amb := AmbiguousLines([]PreviewMatch{{LineNumber: 3}, {LineNumber: 4}}, lines)
	if len(amb) != 1 || amb[0] != 4 {
		t.Fatalf("ambiguous = %v, want [4]", amb)
	}
}

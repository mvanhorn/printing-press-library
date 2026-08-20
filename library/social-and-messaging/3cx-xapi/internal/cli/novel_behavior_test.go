// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral tests for the novel-command pure cores (audit, trace, diff).

package cli

import (
	"encoding/json"
	"testing"
)

func objs(t *testing.T, jsons ...string) []map[string]json.RawMessage {
	t.Helper()
	out := make([]map[string]json.RawMessage, 0, len(jsons))
	for _, j := range jsons {
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(j), &m); err != nil {
			t.Fatalf("bad fixture %q: %v", j, err)
		}
		out = append(out, m)
	}
	return out
}

func TestFindDanglingRefs(t *testing.T) {
	valid := map[string]bool{"214": true, "215": true, "800": true}
	ringGroups := objs(t, `{"Number":"800","Name":"Sales","Members":[{"Number":"214"},{"Number":"999"}]}`)
	queues := objs(t, `{"Number":"801","Name":"Support","Agents":[{"Number":"215"},{"Number":"998"}]}`)
	inbound := objs(t,
		`{"RuleName":"Main","OfficeHoursDestination":{"Number":"214"},"OutOfOfficeHoursDestination":{"Number":"997"}}`,
		`{"RuleName":"Ext-transfer","OfficeHoursDestination":{"Number":"+61298765432"}}`, // external, must NOT flag
	)

	findings := findDanglingRefs(valid, ringGroups, queues, inbound)

	got := map[string]string{} // ref -> kind
	for _, f := range findings {
		got[f.Ref] = f.Kind
	}
	if got["999"] != "ring-group-member" {
		t.Errorf("expected 999 flagged as ring-group-member, got %q", got["999"])
	}
	if got["998"] != "queue-agent" {
		t.Errorf("expected 998 flagged as queue-agent, got %q", got["998"])
	}
	if got["997"] != "inbound-rule-destination" {
		t.Errorf("expected 997 flagged as inbound-rule-destination, got %q", got["997"])
	}
	if _, ok := got["214"]; ok {
		t.Error("214 is a valid extension and must not be flagged")
	}
	if _, ok := got["+61298765432"]; ok {
		t.Error("external transfer number must not be flagged as dangling")
	}
	if len(findings) != 3 {
		t.Errorf("expected exactly 3 findings, got %d: %+v", len(findings), findings)
	}
}

func TestFindRoutesToExtension(t *testing.T) {
	users := objs(t, `{"Number":"214","FirstName":"Jane","LastName":"Doe"}`)
	ringGroups := objs(t, `{"Number":"800","Name":"Sales","Members":[{"Number":"214"}]}`)
	queues := objs(t, `{"Number":"801","Name":"Support","Agents":[{"Number":"999"}]}`)
	inbound := objs(t, `{"RuleName":"Main","OfficeHoursDestination":{"Number":"214"}}`)
	dids := objs(t, `{"Number":"+61870101000","RoutingRule":{"OfficeHoursDestination":{"Number":"214"}}}`)

	res := findRoutesToExtension("214", users, ringGroups, queues, inbound, dids)
	if !res.Exists {
		t.Error("expected extension 214 to exist")
	}
	if len(res.RingGroups) != 1 || res.RingGroups[0] != "Sales" {
		t.Errorf("expected ring group Sales, got %v", res.RingGroups)
	}
	if len(res.Queues) != 0 {
		t.Errorf("expected no queues (999 not 214), got %v", res.Queues)
	}
	if len(res.InboundRule) != 1 || res.InboundRule[0].When != "office-hours" {
		t.Errorf("expected one office-hours inbound rule, got %+v", res.InboundRule)
	}
	if len(res.Dids) != 1 {
		t.Errorf("expected one DID route, got %v", res.Dids)
	}
	if res.PathCount != 3 {
		t.Errorf("expected path_count 3, got %d", res.PathCount)
	}

	// Negative: an extension with no routes and no record.
	none := findRoutesToExtension("555", users, ringGroups, queues, inbound, dids)
	if none.Exists || none.PathCount != 0 {
		t.Errorf("expected 555 to be absent with no routes, got %+v", none)
	}
}

func TestDiffSnapshots(t *testing.T) {
	a := configSnapshot{Name: "before", Resources: map[string][]json.RawMessage{
		rtUsers: {json.RawMessage(`{"Id":1,"Number":"214","Name":"Jane"}`), json.RawMessage(`{"Id":2,"Number":"215","Name":"Bob"}`)},
	}}
	b := configSnapshot{Name: "after", Resources: map[string][]json.RawMessage{
		rtUsers: {
			json.RawMessage(`{"Id":1,"Number":"214","Name":"Jane Smith"}`), // changed
			json.RawMessage(`{"Id":3,"Number":"216","Name":"Carol"}`),      // added; 215 removed
		},
	}}

	res := diffSnapshots(a, b)
	if res.TotalChanges != 3 {
		t.Fatalf("expected 3 total changes (1 added, 1 removed, 1 changed), got %d: %+v", res.TotalChanges, res.Diffs)
	}
	var d *resourceDiff
	for i := range res.Diffs {
		if res.Diffs[i].Resource == rtUsers {
			d = &res.Diffs[i]
		}
	}
	if d == nil {
		t.Fatal("expected a users diff")
	}
	if len(d.Added) != 1 || d.Added[0] != "3" {
		t.Errorf("expected added [3], got %v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0] != "2" {
		t.Errorf("expected removed [2], got %v", d.Removed)
	}
	if len(d.Changed) != 1 || d.Changed[0] != "1" {
		t.Errorf("expected changed [1], got %v", d.Changed)
	}

	// Identical snapshots produce no diff.
	if same := diffSnapshots(a, a); same.TotalChanges != 0 {
		t.Errorf("expected no changes diffing a snapshot against itself, got %d", same.TotalChanges)
	}
}

func TestLooksInternalExtension(t *testing.T) {
	cases := map[string]bool{"214": true, "8001": true, "99": true, "+61298765432": false, "": false, "12345678": false, "ab12": false}
	for in, want := range cases {
		if got := looksInternalExtension(in); got != want {
			t.Errorf("looksInternalExtension(%q) = %v, want %v", in, got, want)
		}
	}
}

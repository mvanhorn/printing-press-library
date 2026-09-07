// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// `identity resolve` promises the identity in force at a date, so that a
// symbol-keyed join cannot splice two different issuers into one return
// series. It once returned the rename notice's whole prose title in
// name_in_force -- e.g. "Change of Security Name and Symbol - X to Y" -- which
// joins to nothing and so could not do that job. These tests pin the
// extraction, using the same changeRe that `identity ledger` uses so the two
// surfaces cannot drift apart in what they think a rename notice says.

func TestChangeReExtractsTheIdentityInForce(t *testing.T) {
	cases := []struct {
		title    string
		wantFrom string
		wantTo   string
	}{
		{"Change of Security Name and Symbol - Lotte Chemical Pakistan Limited to Lucky Core Industries Limited",
			"Lotte Chemical Pakistan Limited", "Lucky Core Industries Limited"},
		{"Change of Name and Symbol – ABC Limited to XYZ Limited",
			"ABC Limited", "XYZ Limited"},
		{"Change of Symbol and Name: OLDCO to NEWCO",
			"OLDCO", "NEWCO"},
		{"Change of Symbol - AAA to BBB",
			"AAA", "BBB"},
	}
	for _, tc := range cases {
		m := changeRe.FindStringSubmatch(tc.title)
		if m == nil {
			t.Errorf("changeRe did not match %q", tc.title)
			continue
		}
		if got := strings.TrimSpace(m[1]); got != tc.wantFrom {
			t.Errorf("%q: from = %q, want %q", tc.title, got, tc.wantFrom)
		}
		if got := strings.TrimSpace(m[2]); got != tc.wantTo {
			t.Errorf("%q: to = %q, want %q", tc.title, got, tc.wantTo)
		}
	}
}

// The "to" side is the identity in force after the change. Taking the whole
// title, or the "from" side, would both be wrong.
func TestIdentityInForceIsTheToSide(t *testing.T) {
	title := "Change of Security Name and Symbol - Lotte Chemical Pakistan Limited to Lucky Core Industries Limited"
	m := changeRe.FindStringSubmatch(title)
	if m == nil {
		t.Fatal("fixture no longer matches changeRe")
	}
	inForce := strings.TrimSpace(m[2])
	if inForce == title {
		t.Fatal("the extracted identity must not be the whole notice title")
	}
	if strings.Contains(inForce, "Change of") {
		t.Errorf("extracted identity %q still carries notice prose", inForce)
	}
	if strings.Contains(inForce, " to ") {
		t.Errorf("extracted identity %q still spans both sides of the change", inForce)
	}
	if inForce != "Lucky Core Industries Limited" {
		t.Errorf("identity in force = %q, want the post-change name", inForce)
	}
}

// A title that does not match must leave Parsed false rather than yield a
// plausible-looking wrong answer.
func TestUnparseableNoticeTitleIsNotGuessed(t *testing.T) {
	for _, title := range []string{
		"Suspension of Trading in the Securities of Some Company Limited",
		"Notice regarding book closure",
		"",
	} {
		if m := changeRe.FindStringSubmatch(title); m != nil {
			t.Errorf("changeRe matched a non-rename title %q as %q -> %q", title, m[1], m[2])
		}
	}
}

// resolveView's JSON contract: identity_parsed must always be emitted, because
// name_in_force is omitempty and its absence would otherwise be ambiguous.
func TestResolveViewParsedIsAlwaysEmitted(t *testing.T) {
	raw, err := json.Marshal(&resolveView{Query: "X", Outcome: outcomeResolved})
	if err != nil {
		t.Fatal(err)
	}
	b := string(raw)
	if !strings.Contains(b, `"identity_parsed"`) {
		t.Errorf("identity_parsed must be present even when false; got %s", b)
	}
	if strings.Contains(b, `"name_in_force"`) {
		t.Errorf("name_in_force should be omitted when empty; got %s", b)
	}
}

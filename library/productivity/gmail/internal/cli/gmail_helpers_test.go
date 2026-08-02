// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

func TestParseSendAt(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		at, in  string
		wantErr bool
		check   func(time.Time) bool
	}{
		{"relative hours", "", "2h", false, func(got time.Time) bool { return got.Equal(now.Add(2 * time.Hour)) }},
		{"relative days shorthand", "", "3d", false, func(got time.Time) bool { return got.Equal(now.Add(72 * time.Hour)) }},
		{"absolute local", "2026-08-04 09:00", "", false, func(got time.Time) bool { return !got.IsZero() }},
		{"absolute rfc3339", "2026-08-04T09:00:00Z", "", false, func(got time.Time) bool {
			return got.Equal(time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC))
		}},
		{"both set", "2026-08-04 09:00", "2h", true, nil},
		{"neither set", "", "", true, nil},
		{"negative in", "", "-2h", true, nil},
		{"garbage at", "tomorrow-ish", "", true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSendAt(tc.at, tc.in, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.check(got) {
				t.Fatalf("unexpected time %v", got)
			}
		})
	}
}

func TestSplitAddressList(t *testing.T) {
	// Addresses come back in net/mail canonical form.
	got := splitAddressList("Jane <jane@example.com>, bob@example.net")
	if len(got) != 2 {
		t.Fatalf("splitAddressList = %v, want 2 entries", got)
	}
	if gmailmail.ExtractEmail(got[0]) != "jane@example.com" || gmailmail.ExtractEmail(got[1]) != "bob@example.net" {
		t.Fatalf("splitAddressList = %v", got)
	}
	if got := splitAddressList(""); len(got) != 0 {
		t.Fatalf("splitAddressList(empty) = %v", got)
	}
}

// RFC 5322 allows commas inside a quoted display name; splitting on every
// comma would turn one recipient into two broken ones.
func TestSplitAddressListQuotedCommas(t *testing.T) {
	got := splitAddressList(`"Doe, Jane" <jane@example.com>, bob@example.net`)
	if len(got) != 2 {
		t.Fatalf("splitAddressList = %v, want 2 recipients", got)
	}
	if gmailmail.ExtractEmail(got[0]) != "jane@example.com" {
		t.Fatalf("quoted display name split incorrectly: %v", got)
	}
	if gmailmail.ExtractEmail(got[1]) != "bob@example.net" {
		t.Fatalf("second recipient wrong: %v", got)
	}
}

func replyFixture() *gmailmail.Message {
	return &gmailmail.Message{
		ID:       "m1",
		ThreadID: "t1",
		Payload: &gmailmail.Part{Headers: []gmailmail.Header{
			{Name: "From", Value: "Alice <alice@example.com>"},
			{Name: "To", Value: "Me <me@example.org>, Carol <carol@example.com>"},
			{Name: "Cc", Value: "dave@example.com"},
			{Name: "Subject", Value: "Plans"},
		}},
	}
}

// emails reduces an address list to bare addresses for comparison.
func emails(list []string) []string {
	out := make([]string, 0, len(list))
	for _, a := range list {
		out = append(out, gmailmail.ExtractEmail(a))
	}
	return out
}

func TestReplyRecipientsSimple(t *testing.T) {
	to, cc := replyRecipients(replyFixture(), "me@example.org", false)
	if got := emails(to); len(got) != 1 || got[0] != "alice@example.com" {
		t.Fatalf("to = %v", got)
	}
	if len(cc) != 0 {
		t.Fatalf("cc = %v", cc)
	}
}

func TestReplyRecipientsAllDropsSelf(t *testing.T) {
	to, cc := replyRecipients(replyFixture(), "me@example.org", true)
	toEmails := emails(to)
	if !contains(toEmails, "alice@example.com") || !contains(toEmails, "carol@example.com") {
		t.Fatalf("to missing recipients: %v", toEmails)
	}
	if contains(toEmails, "me@example.org") {
		t.Fatalf("self not dropped from to: %v", toEmails)
	}
	if !contains(emails(cc), "dave@example.com") {
		t.Fatalf("cc = %v", cc)
	}
}

// The From address usually reappears in To; reply-all must not address it twice.
func TestReplyRecipientsDeduplicates(t *testing.T) {
	msg := replyFixture()
	msg.Payload.Headers = []gmailmail.Header{
		{Name: "From", Value: "Alice <alice@example.com>"},
		{Name: "To", Value: "Me <me@example.org>, Alice <alice@example.com>"},
	}
	to, _ := replyRecipients(msg, "me@example.org", true)
	seen := 0
	for _, e := range emails(to) {
		if e == "alice@example.com" {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("alice@example.com appears %d times in %v, want 1", seen, to)
	}
}

func TestReplyRecipientsRespectsReplyTo(t *testing.T) {
	msg := replyFixture()
	msg.Payload.Headers = append(msg.Payload.Headers, gmailmail.Header{Name: "Reply-To", Value: "list-reply@example.com"})
	to, _ := replyRecipients(msg, "me@example.org", false)
	if got := emails(to); len(got) != 1 || got[0] != "list-reply@example.com" {
		t.Fatalf("Reply-To not honored: %v", got)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func TestParseUnsubTargets(t *testing.T) {
	httpT, mailtoT, unval := parseUnsubTargets("<mailto:leave@example.com>, <https://list.com/u?id=1>")
	if mailtoT != "mailto:leave@example.com" {
		t.Fatalf("mailto = %q", mailtoT)
	}
	if httpT != "https://list.com/u?id=1" {
		t.Fatalf("http = %q", httpT)
	}
	if len(unval) != 0 {
		t.Fatalf("unvalidated = %v, want none", unval)
	}
	httpT, mailtoT, _ = parseUnsubTargets("<https://only.com/x>")
	if httpT != "https://only.com/x" || mailtoT != "" {
		t.Fatalf("single = %q %q", httpT, mailtoT)
	}
}

// The List-Unsubscribe header is sender-controlled and this command's output
// is meant to be acted on, so non-web schemes and internal hosts must never
// be presented as valid targets.
func TestParseUnsubTargetsRejectsHostileTargets(t *testing.T) {
	cases := []struct {
		name, header string
	}{
		{"scheme prefix trick", "<httpfoo://evil.test/x>"},
		{"javascript", "<javascript:alert(1)>"},
		{"loopback", "<http://127.0.0.1:9000/admin>"},
		{"localhost name", "<http://localhost:8080/delete>"},
		{"link-local metadata", "<http://169.254.169.254/latest/meta-data/>"},
		{"private range", "<http://10.0.0.5/internal>"},
		{"bogus mailto", "<mailto:not-an-address>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			httpT, mailtoT, unval := parseUnsubTargets(tc.header)
			if httpT != "" || mailtoT != "" {
				t.Fatalf("hostile target accepted: http=%q mailto=%q", httpT, mailtoT)
			}
			if len(unval) == 0 {
				t.Fatal("hostile target was dropped silently; expected it under unvalidated")
			}
		})
	}
}

func TestFilterKeyNormalizes(t *testing.T) {
	a := filterKey("A@EXAMPLE.com", "", "", "", []string{"Foo", "bar"}, nil, "")
	b := filterKey("a@example.com", "", "", "", []string{"bar", "foo"}, nil, "")
	if a != b {
		t.Fatalf("filterKey not order/case-insensitive:\n%s\n%s", a, b)
	}
	c := filterKey("a@example.com", "", "", "", []string{"foo"}, nil, "")
	if a == c {
		t.Fatalf("filterKey collided for different label sets")
	}
}

func TestBuildFilterPlan(t *testing.T) {
	idToName := map[string]string{"Label_1": "GitHub", "INBOX": "INBOX"}
	var live []liveFilter
	var lf liveFilter
	lf.ID = "f1"
	lf.Criteria.From = "notifications@github.com"
	lf.Action.AddLabelIDs = []string{"Label_1"}
	live = append(live, lf)

	specs := []filterSpec{
		{From: "notifications@github.com", AddLabels: []string{"GitHub"}}, // matches live f1
		{Query: "list:golang-nuts", RemoveLabels: []string{"INBOX"}},      // new
	}
	plan := buildFilterPlan(specs, live, idToName)
	if plan.Keep != 1 {
		t.Fatalf("Keep = %d, want 1", plan.Keep)
	}
	if len(plan.Create) != 1 || plan.Create[0].Query != "list:golang-nuts" {
		t.Fatalf("Create = %+v", plan.Create)
	}
	if len(plan.Delete) != 0 {
		t.Fatalf("Delete = %+v", plan.Delete)
	}

	// Empty file side: everything live is a delete candidate.
	plan = buildFilterPlan([]filterSpec{{From: "x@y.z", AddLabels: []string{"Foo"}}}, live, idToName)
	if len(plan.Delete) != 1 || plan.Delete[0].ID != "f1" {
		t.Fatalf("Delete = %+v", plan.Delete)
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{512, "512B"}, {2048, "2.0KB"}, {5 << 20, "5.0MB"}, {3 << 30, "3.0GB"},
	}
	for _, tc := range cases {
		if got := humanBytes(tc.in); got != tc.want {
			t.Fatalf("humanBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

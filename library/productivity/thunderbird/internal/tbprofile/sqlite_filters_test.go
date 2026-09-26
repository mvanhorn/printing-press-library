package tbprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func TestReadAddressBook(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	path := filepath.Join(profile, "abook.sqlite")
	before, _ := os.Stat(path)
	cards, err := ReadAddressBook(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("cards = %+v", cards)
	}
	alice, carol := cards[0], cards[1]
	if alice.ID != tbtest.CardAlice || alice.DisplayName != "Alice Example" || alice.FirstName != "Alice" || alice.LastName != "Example" ||
		alice.NickName != "Al" || alice.Company != "Example Corp" || alice.Book != "abook" {
		t.Errorf("alice = %+v", alice)
	}
	if strings.Join(alice.Emails, ",") != "alice@example.com,alice.alt@example.com" {
		t.Errorf("alice emails = %v", alice.Emails)
	}
	if strings.Join(carol.Emails, ",") != "carol@example.com" {
		t.Errorf("carol emails lowercased = %v", carol.Emails)
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Error("live address book was modified")
	}
	if _, err := ReadAddressBook(filepath.Join(profile, "missing.sqlite")); err == nil {
		t.Error("missing book must error")
	}
}

func TestAddressBookFiles(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	got := AddressBookFiles(profile)
	var names []string
	for _, p := range got {
		names = append(names, filepath.Base(p))
	}
	if strings.Join(names, ",") != "abook.sqlite,history.sqlite" {
		t.Fatalf("files = %v", names)
	}
	if len(AddressBookFiles(t.TempDir())) != 0 {
		t.Fatal("empty profile listed books")
	}
}

func TestReadCalendarEvents(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	evs, err := ReadCalendarEvents(profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events = %+v", evs)
	}
	e := evs[0]
	if e.ID != tbtest.EventSync || e.Title != "Team sync" || e.Location != "Room 1" || e.CalendarID != "cal-1" ||
		!e.Start.Equal(time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)) || !e.End.Equal(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("event = %+v", e)
	}
	none, err := ReadCalendarEvents(t.TempDir())
	if err != nil || len(none) != 0 {
		t.Fatalf("no calendar: %v %v", none, err)
	}
}

func TestParseFilterRules(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	fs, err := ParseFilterRules(filepath.Join(profile, "ImapMail", "imap.example.com", "msgFilterRules.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 3 {
		t.Fatalf("filters = %d", len(fs))
	}
	tests := []struct {
		name      string
		enabled   bool
		match     string
		actions   int
		terms     int
		supported []bool
	}{
		{"Newsletters", true, "OR", 1, 2, []bool{true, true}},
		{"Old rule", false, "AND", 2, 3, []bool{false, false, true}},
		{"Everything", true, "ALL", 1, 0, nil},
	}
	for i, tt := range tests {
		f := fs[i]
		if f.Index != i || f.Name != tt.name || f.Enabled != tt.enabled || f.MatchType != tt.match || len(f.Actions) != tt.actions || len(f.Terms) != tt.terms {
			t.Errorf("filter %d = %+v", i, f)
			continue
		}
		for j, s := range tt.supported {
			if f.Terms[j].Supported != s {
				t.Errorf("%s term %d supported = %v (%+v)", tt.name, j, f.Terms[j].Supported, f.Terms[j])
			}
		}
		if !strings.HasPrefix(f.Raw, `name="`+tt.name+`"`) {
			t.Errorf("raw = %q", f.Raw)
		}
	}
	if fs[0].Actions[0].Type != "Move to folder" || !strings.HasSuffix(fs[0].Actions[0].Value, "/Archives/2025") {
		t.Errorf("action = %+v", fs[0].Actions[0])
	}
	if got := fs[1].Terms[0].Field; got != "X-Spam-Flag" {
		t.Errorf("custom header field = %q", got)
	}
	if got := fs[1].Terms[2].Value; got != "a, b (c)" {
		t.Errorf("quoted value = %q", got)
	}
	empty, err := ParseFilterRules(filepath.Join(profile, "nope.dat"))
	if err != nil || len(empty) != 0 {
		t.Fatalf("missing file: %v %v", empty, err)
	}
}

func TestParseFilterRulesReader(t *testing.T) {
	fs, err := ParseFilterRulesReader(strings.NewReader("version=\"9\"\r\nname=\"A\"\r\nenabled=\"no\"\r\naction=\"Delete\"\r\ncondition=\"AND (body,contains,x)\"\r\n"))
	if err != nil || len(fs) != 1 || fs[0].Enabled || fs[0].Actions[0].Type != "Delete" || fs[0].Terms[0].Field != "body" {
		t.Fatalf("filters = %+v, %v", fs, err)
	}
}

func TestParseCondition(t *testing.T) {
	tests := []struct {
		in     string
		match  string
		fields []string
		values []string
		ok     []bool
	}{
		{"ALL", "ALL", nil, nil, nil},
		{"", "ALL", nil, nil, nil},
		{"AND (subject,contains,hi)", "AND", []string{"subject"}, []string{"hi"}, []bool{true}},
		{"OR (from,is,a@x) OR (tag,contains,$label1)", "OR", []string{"from", "tag"}, []string{"a@x", "$label1"}, []bool{true, false}},
		{`AND (from\,to\,cc\,or bcc,contains,bob)`, "AND", []string{"from,to,cc,or bcc"}, []string{"bob"}, []bool{true}},
		{"AND (subject,matches regex,^x)", "AND", []string{"subject"}, []string{"^x"}, []bool{false}},
		{"AND (age in days,is greater than,30)", "AND", []string{"age in days"}, []string{"30"}, []bool{true}},
	}
	for _, tt := range tests {
		match, terms := ParseCondition(tt.in)
		if match != tt.match || len(terms) != len(tt.fields) {
			t.Errorf("%q -> %s %+v", tt.in, match, terms)
			continue
		}
		for i, term := range terms {
			if term.Field != tt.fields[i] || term.Value != tt.values[i] || term.Supported != tt.ok[i] {
				t.Errorf("%q term %d = %+v", tt.in, i, term)
			}
		}
	}
}

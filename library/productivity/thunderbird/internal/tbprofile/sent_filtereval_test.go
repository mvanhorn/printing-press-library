package tbprofile

import (
	"strings"
	"testing"
	"time"
)

func TestIsSentFolderName(t *testing.T) {
	tests := map[string]bool{
		"Sent": true, "sent items": true, "Sent Messages": true, "Posta inviata": true, "POSTA INVIATA": true,
		"Inviati": true, "Gesendet": true, "Envoyés": true, "Enviados": true, "[Gmail]/Sent Mail": true, "Archives/Sent": true,
		"INBOX": false, "Sent.old": false, "Drafts": false, "Posta inviata/2024": false, "": false,
	}
	for in, want := range tests {
		if got := IsSentFolderName(in); got != want {
			t.Errorf("IsSentFolderName(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsTrashOrJunkFolderName(t *testing.T) {
	tests := map[string]bool{"Trash": true, "Cestino": true, "Junk": true, "spam": true, "Posta indesiderata": true, "Deleted Items": true, "INBOX": false, "Sent": false}
	for in, want := range tests {
		if got := IsTrashOrJunkFolderName(in); got != want {
			t.Errorf("IsTrashOrJunkFolderName(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseFolderURI(t *testing.T) {
	tests := []struct {
		in   string
		want FolderURI
		ok   bool
	}{
		{"imap://bob%40example.com@imap.example.com/Posta%20inviata", FolderURI{"imap", "bob@example.com", "imap.example.com", "Posta inviata"}, true},
		{"imap://bob%40example.com@imap.example.com/Posta inviata", FolderURI{"imap", "bob@example.com", "imap.example.com", "Posta inviata"}, true},
		{"mailbox://nobody@Local%20Folders/Trash", FolderURI{"mailbox", "nobody", "Local Folders", "Trash"}, true},
		{"imap://bob%40example.com@imap.example.com/Archives/2025", FolderURI{"imap", "bob@example.com", "imap.example.com", "Archives/2025"}, true},
		{"imap://imap.example.com/INBOX", FolderURI{"imap", "", "imap.example.com", "INBOX"}, true},
		{"imap://bob@imap.example.com/", FolderURI{}, false},
		{"Posta inviata", FolderURI{}, false},
		{"", FolderURI{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseFolderURI(tt.in)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseFolderURI(%q) = %+v %v, want %+v %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func testAccounts() []Account {
	return []Account{
		{Key: "account1", Server: Server{Hostname: "imap.example.com", UserName: "bob@example.com"}, Identities: []Identity{{Key: "id1"}, {Key: "id3"}}},
		{Key: "account3", Server: Server{Hostname: "imap.example.com", UserName: "carol@example.com"}, Identities: []Identity{{Key: "id4"}}},
		{Key: "account2", Server: Server{Hostname: "Local Folders", UserName: "nobody"}},
	}
}

func TestMatchFolderURI(t *testing.T) {
	tests := []struct {
		in, account, path string
		ok                bool
	}{
		{"imap://bob%40example.com@imap.example.com/Posta%20inviata", "account1", "Posta inviata", true},
		{"imap://carol%40example.com@IMAP.example.com/Sent", "account3", "Sent", true},
		{"mailbox://nobody@Local%20Folders/Trash", "account2", "Trash", true},
		{"imap://dave%40example.com@imap.example.com/Sent", "", "", false},
		{"imap://bob%40example.com@other.example.com/Sent", "", "", false},
		{"not a uri", "", "", false},
	}
	for _, tt := range tests {
		acc, path, ok := MatchFolderURI(tt.in, testAccounts())
		if acc != tt.account || path != tt.path || ok != tt.ok {
			t.Errorf("MatchFolderURI(%q) = %q %q %v", tt.in, acc, path, ok)
		}
	}
}

func TestSentFolderURIs(t *testing.T) {
	tests := []struct {
		name  string
		prefs Prefs
		want  []string
	}{
		{"none", Prefs{}, []string{}},
		{"two identities, one duplicate, one fcc disabled, one foreign", Prefs{
			"mail.identity.id1.fcc_folder": "imap://bob%40example.com@imap.example.com/Sent",
			"mail.identity.id3.fcc_folder": "imap://bob%40example.com@imap.example.com/Sent",
			"mail.identity.id4.fcc_folder": "imap://carol%40example.com@imap.example.com/Inviati",
			"mail.identity.id4.fcc":        "false",
			"mail.identity.id9.fcc_folder": "imap://x/Other",
		}, []string{"imap://bob%40example.com@imap.example.com/Sent"}},
		{"fcc true keeps the folder", Prefs{
			"mail.identity.id4.fcc_folder": "imap://carol%40example.com@imap.example.com/Inviati",
			"mail.identity.id4.fcc":        "true",
		}, []string{"imap://carol%40example.com@imap.example.com/Inviati"}},
	}
	for _, tt := range tests {
		if got := tt.prefs.SentFolderURIs(testAccounts()); strings.Join(got, "|") != strings.Join(tt.want, "|") || got == nil {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestEvalTerm(t *testing.T) {
	now := time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC)
	m := FilterSubject{
		FromAddr: "news@example.com", FromName: "Example News", To: []string{"bob@example.com"}, Cc: []string{"carol@example.com"},
		Subject: "Weekly Newsletter", Body: "Top stories and the agenda", ListID: "Example News <news.example.com>",
		Date: time.Date(2025, 1, 10, 12, 0, 0, 0, time.UTC), SizeBytes: 5 * 1024, Read: true, Flagged: false, HasAttachments: true,
	}
	tests := []struct {
		field, op, value string
		match, ok        bool
	}{
		{"subject", "contains", "newsletter", true, true},
		{"subject", "doesn't contain", "newsletter", false, true},
		{"subject", "is", "weekly newsletter", true, true},
		{"subject", "isn't", "weekly newsletter", false, true},
		{"subject", "begins with", "weekly", true, true},
		{"subject", "ends with", "weekly", false, true},
		{"from", "is", "news@example.com", true, true},
		{"from", "is", "Example News", true, true},
		{"from", "contains", "example news <news@", true, true},
		{"from", "begins with", "news@", true, true},
		{"to", "contains", "carol", false, true},
		{"cc", "contains", "carol", true, true},
		{"to or cc", "contains", "carol", true, true},
		{"to or cc", "doesn't contain", "carol", false, true},
		{"to or cc", "doesn't contain", "dave", true, true},
		{"all addresses", "contains", "news@", true, true},
		{"from,to,cc,or bcc", "is", "bob@example.com", true, true},
		{"body", "contains", "AGENDA", true, true},
		{`"List-Id"`, "contains", "news.example.com", true, true},
		{"date", "is before", "15-Jan-2025", true, true},
		{"date", "is after", "15-Jan-2025", false, true},
		{"date", "is", "10-Jan-2025", true, true},
		{"date", "isn't", "10-Jan-2025", false, true},
		{"age in days", "is greater than", "9", true, true},
		{"age in days", "is less than", "9", false, true},
		{"size", "is greater than", "4", true, true},
		{"size", "is less than", "5", false, true},
		{"status", "is", "read", true, true},
		{"status", "isn't", "flagged", true, true},
		{"has attachment status", "is", "true", true, true},
		{"has attachment status", "isn't", "true", false, true},
		{"X-Spam-Flag", "is", "YES", false, false},
		{"junk score origin", "is", "plugin", false, false},
		{"tag", "contains", "$label1", false, false},
		{"subject", "matches regex", "^x", false, false},
		{"date", "is", "someday", false, false},
		{"size", "is", "5", false, false},
		{"status", "is", "new", false, false},
		{"age in days", "is greater than", "many", false, false},
	}
	for _, tt := range tests {
		term := FilterTerm{Field: tt.field, Op: tt.op, Value: tt.value}
		match, ok := EvalTerm(term, m, now)
		if match != tt.match || ok != tt.ok {
			t.Errorf("EvalTerm(%s %s %s) = %v %v, want %v %v", tt.field, tt.op, tt.value, match, ok, tt.match, tt.ok)
		}
		if TermSupported(term) != tt.ok {
			t.Errorf("TermSupported(%s %s %s) = %v", tt.field, tt.op, tt.value, !tt.ok)
		}
	}
}

func TestEvalFilter(t *testing.T) {
	m := FilterSubject{FromAddr: "news@example.com", Subject: "Weekly digest"}
	yes := FilterTerm{Field: "from", Op: "is", Value: "news@example.com"}
	no := FilterTerm{Field: "subject", Op: "contains", Value: "newsletter"}
	bad := FilterTerm{Field: "tag", Op: "contains", Value: "x"}
	tests := []struct {
		name      string
		match     string
		terms     []FilterTerm
		want, wok bool
	}{
		{"all", "ALL", nil, true, true},
		{"or one true", "OR", []FilterTerm{no, yes}, true, true},
		{"and one false", "AND", []FilterTerm{no, yes}, false, true},
		{"and all true", "AND", []FilterTerm{yes, yes}, true, true},
		{"or none", "OR", []FilterTerm{no}, false, true},
		{"unsupported term", "OR", []FilterTerm{yes, bad}, false, false},
		{"no terms", "AND", nil, false, true},
	}
	for _, tt := range tests {
		got, ok := EvalFilter(tt.match, tt.terms, m, time.Now())
		if got != tt.want || ok != tt.wok {
			t.Errorf("%s: %v %v, want %v %v", tt.name, got, ok, tt.want, tt.wok)
		}
	}
}

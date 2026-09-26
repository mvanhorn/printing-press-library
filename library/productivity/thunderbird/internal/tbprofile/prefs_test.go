package tbprofile

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func TestParsePrefsReader(t *testing.T) {
	src := `// comment
user_pref("a.string", "hello \"world\"");
user_pref("a.path", "C:\\Users\\x");
user_pref("a.int", 42);
user_pref("a.bool", false);
user_pref("a.unicode", "caf\u00e9");
not a pref line
`
	p, err := ParsePrefsReader(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]string{
		"a.string":  `hello "world"`,
		"a.path":    `C:\Users\x`,
		"a.int":     "42",
		"a.bool":    "false",
		"a.unicode": "café",
	}
	for k, want := range tests {
		if p[k] != want {
			t.Errorf("%s = %q, want %q", k, p[k], want)
		}
	}
	if len(p) != len(tests) {
		t.Errorf("got %d prefs, want %d", len(p), len(tests))
	}
}

func TestParsePrefsAndAccounts(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	p, err := ParsePrefs(filepath.Join(profile, "prefs.js"))
	if err != nil {
		t.Fatal(err)
	}
	accs := p.Accounts(profile)
	if len(accs) != 2 {
		t.Fatalf("accounts = %d", len(accs))
	}
	tests := []struct {
		key, typ, name, dir string
		identities          []string
	}{
		{"account1", "imap", "bob@example.com", filepath.Join(profile, "ImapMail", "imap.example.com"), []string{"bob@example.com", "bob.work@example.com"}},
		{"account2", "none", "Local Folders", filepath.Join(profile, "Mail", "Local Folders"), nil},
	}
	for i, tt := range tests {
		a := accs[i]
		if a.Key != tt.key || a.Server.Type != tt.typ || a.Name() != tt.name || a.Server.Directory != tt.dir {
			t.Errorf("account %d = %+v", i, a)
		}
		var emails []string
		for _, id := range a.Identities {
			emails = append(emails, id.Email)
			if id.Account != tt.key {
				t.Errorf("identity %s account = %s", id.Key, id.Account)
			}
		}
		if strings.Join(emails, ",") != strings.Join(tt.identities, ",") {
			t.Errorf("%s identities = %v", tt.key, emails)
		}
	}
	if accs[0].Identities[1].FullName != `Bob "Work" Example` {
		t.Errorf("escaped fullName = %q", accs[0].Identities[1].FullName)
	}
}

func TestLoadAccounts(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	accs, err := LoadAccounts(profile)
	if err != nil || len(accs) != 2 || accs[1].Server.Hostname != "Local Folders" {
		t.Fatalf("LoadAccounts = %+v, %v", accs, err)
	}
	if _, err := LoadAccounts(t.TempDir()); err == nil {
		t.Fatal("missing prefs.js must error")
	}
}

func TestAccountsWithoutAccountManagerList(t *testing.T) {
	p := Prefs{
		"mail.account.account9.server": "server9",
		"mail.server.server9.type":     "pop3",
		"mail.server.server9.hostname": "pop.example.com",
		"mail.server.server9.userName": "zed",
		"mail.account.account8.foo":    "x",
	}
	accs := p.Accounts("/p")
	if len(accs) != 1 || accs[0].Key != "account9" || accs[0].Name() != "zed@pop.example.com" {
		t.Fatalf("accounts = %+v", accs)
	}
}

func TestServerDirectory(t *testing.T) {
	tests := []struct {
		rel, abs, want string
	}{
		{"[ProfD]ImapMail/imap.example.com", "/abs/ignored", filepath.Join("/prof", "ImapMail", "imap.example.com")},
		{"", "/abs/dir", filepath.FromSlash("/abs/dir")},
		{"[ProfD]", "", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		if got := ServerDirectory("/prof", tt.rel, tt.abs); got != tt.want {
			t.Errorf("ServerDirectory(%q,%q) = %q, want %q", tt.rel, tt.abs, got, tt.want)
		}
	}
}

package cli

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

const tbInboxRel = "ImapMail/imap.example.com/INBOX"

// tbExtraMessages are appended to the fixture INBOX copy by tests that
// need cases the shared fixture does not carry.
const tbExtraMessages = `From - Tue Jan 14 09:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <x1@example.com>
Date: Tue, 14 Jan 2025 09:00:00 +0000
From: Heidi Example <heidi@example.com>
Reply-To: Heidi Desk <desk@example.com>
To: Bob Work <bob.work@example.com>
Cc: ivan@example.com, bob@example.com
Subject: Work contract
Authentication-Results: mx.example.com; dkim=fail header.d=bad.example; dkim=pass (good sig) header.d=example.com; dmarc=fail header.from=example.com
Received-SPF: softfail (mx.example.com: domain of transitioning heidi@example.com) client-ip=192.0.2.1;
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="B2"

--B2
Content-Type: text/plain

See the files.
--B2
Content-Type: text/plain; name="../evil.txt"
Content-Disposition: attachment; filename="../evil.txt"

first
--B2
Content-Type: text/plain; name="evil.txt"
Content-Disposition: attachment; filename="evil.txt"

second
--B2--

`

type tbSliceB struct {
	home    string
	profile string
	launch  *[][]string
}

func tbSetupB(t *testing.T, extra bool, sync bool) tbSliceB {
	t.Helper()
	root, profile := tbtest.Fixture(t)
	if extra {
		f, err := os.OpenFile(filepath.Join(profile, filepath.FromSlash(tbInboxRel)), os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(tbExtraMessages); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
	t.Setenv(tbprofile.EnvRoot, root)
	t.Setenv(tbprofile.EnvProfile, "")
	t.Setenv(cliutil.VerifyEnvVar, "")
	t.Setenv(cliutil.DogfoodEnvVar, "")
	calls := [][]string{}
	origFind, origLaunch := tbFindThunderbird, tbLaunch
	tbFindThunderbird = func() (string, error) { return `C:\Program Files\Mozilla Thunderbird\thunderbird.exe`, nil }
	tbLaunch = func(exe string, args []string) (int, error) {
		calls = append(calls, append([]string{exe}, args...))
		return 4242, nil
	}
	t.Cleanup(func() { tbFindThunderbird, tbLaunch = origFind, origLaunch })
	s := tbSliceB{home: t.TempDir(), profile: profile, launch: &calls}
	if sync {
		if out, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
			t.Fatalf("sync: %v %s", err, out)
		}
	}
	return s
}

// tbRunBare runs without any flag so NFlag()==0 help paths are reachable;
// TestMain's testenv.RunSandboxed keeps the CLI data dir in a temp home.
func tbRunBare(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func tbID(folder, messageID string) string {
	return tbprofile.MessageKey("account1", folder, messageID, 0)
}

func tbIDs(rows []tbMessageRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.MessageID)
	}
	return out
}

// tbFixtureRaw cuts a message out of the fixture mbox by text search,
// independently of the scanner under test.
func tbFixtureRaw(t *testing.T, fromLine, nextFromLine string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tbtest.TestdataRoot(), filepath.FromSlash(tbtest.ProfileRel), filepath.FromSlash(tbInboxRel)))
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(b, []byte(fromLine+"\n"))
	end := bytes.Index(b, []byte("\n\n"+nextFromLine))
	if start < 0 || end < 0 {
		t.Fatal("fixture markers not found")
	}
	return b[start+len(fromLine)+1 : end+1]
}

func TestTBMessagesList(t *testing.T) {
	s := tbSetupB(t, false, true)
	list := func(args ...string) []string {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"messages", "list", "--json"}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if strings.TrimSpace(out) == "null" {
			t.Fatalf("%v: null output", args)
		}
		return tbIDs(tbDecode[[]tbMessageRow](t, out))
	}
	tests := []struct {
		args []string
		want []string
	}{
		{nil, []string{"m8@example.com", "m7@example.com", "m6@example.com", "m5@example.com", "m4@example.com", "s1@example.com", "m2@example.com", "m1@example.com", "a1@example.com"}},
		{[]string{"--unread"}, []string{"m6@example.com", "m2@example.com"}},
		{[]string{"--flagged"}, []string{"m2@example.com"}},
		{[]string{"--folder", "posta INVIATA"}, []string{"s1@example.com"}},
		{[]string{"--folder", "archives/2025"}, []string{"a1@example.com"}},
		{[]string{"--folder", "2025"}, []string{"a1@example.com"}},
		{[]string{"--folder", "Archives"}, []string{}},
		{[]string{"--folder", "Nope"}, []string{}},
		{[]string{"--account", "account2"}, []string{}},
		{[]string{"--account", "bob@example.com", "--limit", "2"}, []string{"m8@example.com", "m7@example.com"}},
		{[]string{"--from", "ALICE"}, []string{"m7@example.com", "m1@example.com"}},
		{[]string{"--from", "carol example"}, []string{"m8@example.com", "m2@example.com"}},
		{[]string{"--since", "2025-01-12T00:00:00Z"}, []string{"m8@example.com", "m7@example.com"}},
		{[]string{"--since", "7d"}, []string{}},
		{[]string{"--unread", "--from", "news"}, []string{"m6@example.com"}},
	}
	for _, tt := range tests {
		if got := list(tt.args...); strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("messages list %v = %v, want %v", tt.args, got, tt.want)
		}
	}
	out, _, _ := tbRun(t, s.home, "messages", "list", "--json", "--flagged")
	if r := tbDecode[[]tbMessageRow](t, out)[0]; r.Flags != "UF" || r.ID != tbID("INBOX", "m2@example.com") || r.Subject != "Budget question" || r.FromAddr != "carol@example.com" {
		t.Errorf("row fields: %+v", r)
	}
	if out, _, _ := tbRun(t, s.home, "messages", "list", "--json", "--folder", "Nope"); strings.TrimSpace(out) != "[]" {
		t.Errorf("empty result must be [], got %q", out)
	}
	if _, _, err := tbRun(t, s.home, "messages", "list", "--since", "yesterday"); ExitCode(err) != 2 {
		t.Errorf("bad --since exit = %v", err)
	}
}

func TestTBMessagesShowAndRaw(t *testing.T) {
	s := tbSetupB(t, false, true)
	id := tbID("INBOX", "m4@example.com")
	out, _, err := tbRun(t, s.home, "messages", "show", id, "--json")
	det := tbDecode[tbMessageDetail](t, out)
	if err != nil || det.Subject != "Report attached" || det.Source != "mbox" || !strings.Contains(det.BodyText, "Caf\u00e9") ||
		len(det.Attachments) != 1 || det.Attachments[0].Filename != "report.pdf" || det.Attachments[0].SizeBytes != int64(len("Hello attachment world!")) || det.Headers != nil {
		t.Fatalf("show: %v %+v", err, det)
	}
	out, _, err = tbRun(t, s.home, "messages", "get", tbID("INBOX", "m1@example.com"), "--headers", "--json")
	det = tbDecode[tbMessageDetail](t, out)
	if err != nil || det.Subject != "Project kickoff" || len(det.Headers["Authentication-Results"]) != 1 || det.To[0] != "bob@example.com" {
		t.Fatalf("get alias / --headers: %v %+v", err, det)
	}
	out, _, err = tbRun(t, s.home, "messages", "show", id, "--raw")
	want := tbFixtureRaw(t, "From - Thu Jan 09 12:00:00 2025", "From - Fri Jan 10")
	if err != nil || out != string(want) {
		t.Fatalf("--raw differs from the fixture bytes: %v\n%q\n%q", err, out, want)
	}
	for _, ref := range []string{"ffffffffffff", "nobody@example.com"} {
		if _, _, err := tbRun(t, s.home, "messages", "show", ref, "--json"); ExitCode(err) != 3 {
			t.Errorf("unknown %s exit = %v", ref, err)
		}
	}
	out, _, err = tbRun(t, s.home, "messages", "show", "<m5@example.com>", "--json")
	if det = tbDecode[tbMessageDetail](t, out); err != nil || det.ID != tbID("INBOX", "m5@example.com") || det.Subject != "Caf\u00e9 cr\u00e8me" {
		t.Errorf("show by Message-ID: %v %+v", err, det)
	}
}

func TestTBMessagesStaleMbox(t *testing.T) {
	s := tbSetupB(t, false, true)
	inbox := filepath.Join(s.profile, filepath.FromSlash(tbInboxRel))
	b, _ := os.ReadFile(inbox)
	cut := bytes.Index(b, []byte("From - Tue Jan 07"))
	if err := os.WriteFile(inbox, b[cut:], 0o644); err != nil {
		t.Fatal(err)
	}
	id := tbID("INBOX", "m4@example.com")
	if _, _, err := tbRun(t, s.home, "messages", "show", id, "--raw"); err == nil || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("stale --raw must fail with a sync hint: %v", err)
	}
	out, errOut, err := tbRun(t, s.home, "messages", "show", id, "--json")
	if det := tbDecode[tbMessageDetail](t, out); err != nil || det.Source != "store" || len(det.Attachments) != 1 || !strings.Contains(errOut, "warning") {
		t.Fatalf("stale show fallback: %v %+v %q", err, det, errOut)
	}
	if _, _, err := tbRun(t, s.home, "messages", "export", id, "--out", t.TempDir()); err == nil {
		t.Fatal("export of a stale message must fail")
	}
}

func TestTBMessagesAuth(t *testing.T) {
	s := tbSetupB(t, true, true)
	tests := []struct {
		id                string
		spf, dkim, dmarc  string
		verdicts, arCount int
	}{
		{tbID("INBOX", "m1@example.com"), "pass", "pass", "pass", 3, 1},
		{tbID("INBOX", "x1@example.com"), "softfail", "pass", "fail", 4, 1},
		{tbID("INBOX", "m2@example.com"), "", "", "", 0, 0},
	}
	for _, tt := range tests {
		out, _, err := tbRun(t, s.home, "messages", "auth", tt.id, "--json")
		rep := tbDecode[tbAuthReport](t, out)
		if err != nil || rep.SPF != tt.spf || rep.DKIM != tt.dkim || rep.DMARC != tt.dmarc || len(rep.Verdicts) != tt.verdicts ||
			len(rep.AuthenticationResults) != tt.arCount || rep.Verdicts == nil || rep.AuthenticationResults == nil {
			t.Errorf("auth %s: %v %+v", tt.id, err, rep)
		}
	}
	out, _, _ := tbRun(t, s.home, "messages", "auth", tbID("INBOX", "x1@example.com"), "--json")
	rep := tbDecode[tbAuthReport](t, out)
	if len(rep.ReceivedSPF) != 1 || rep.Verdicts[3].Source != "Received-SPF" || rep.Verdicts[1].Details != "header.d=example.com" {
		t.Errorf("auth details: %+v", rep)
	}
	if _, _, err := tbRun(t, s.home, "messages", "auth", "ffffffffffff"); ExitCode(err) != 3 {
		t.Errorf("unknown id exit = %v", err)
	}
}

func TestTBMessagesExport(t *testing.T) {
	s := tbSetupB(t, false, true)
	m4, m1 := tbID("INBOX", "m4@example.com"), tbID("INBOX", "m1@example.com")
	raw4 := tbFixtureRaw(t, "From - Thu Jan 09 12:00:00 2025", "From - Fri Jan 10")
	raw1 := tbFixtureRaw(t, "From - Mon Jan 06 09:00:00 2025", "From - Tue Jan 07")
	out, _, err := tbRun(t, s.home, "messages", "export", m4)
	if err != nil || out != string(raw4) {
		t.Fatalf("eml to stdout: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "export")
	out, _, err = tbRun(t, s.home, "messages", "export", m4, m1, "--output", dir, "--json")
	files := tbDecode[[]tbExportedFile](t, out)
	if err != nil || len(files) != 2 {
		t.Fatalf("export --out: %v %q", err, out)
	}
	for id, want := range map[string][]byte{m4: raw4, m1: raw1} {
		got, err := os.ReadFile(filepath.Join(dir, id+".eml"))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s.eml differs: %v", id, err)
		}
	}
	if _, _, err := tbRun(t, s.home, "messages", "export", m4, "--output", dir); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Errorf("overwrite without --force: %v", err)
	}
	if _, _, err := tbRun(t, s.home, "messages", "export", m4, "--output", dir, "--force"); err != nil {
		t.Errorf("--force: %v", err)
	}
	out, _, err = tbRun(t, s.home, "messages", "export", m4, "--format", "json")
	dets := tbDecode[[]tbMessageDetail](t, out)
	if err != nil || len(dets) != 1 || !strings.Contains(dets[0].BodyText, "Caf\u00e9") || dets[0].Headers["Subject"][0] != "Report attached" {
		t.Errorf("json export: %v %q", err, out)
	}
	if _, _, err := tbRun(t, s.home, "messages", "export", m4, m1); ExitCode(err) != 2 {
		t.Errorf("multi-id eml to stdout exit = %v", err)
	}
	if _, _, err := tbRun(t, s.home, "messages", "export", m4, "--format", "pdf"); ExitCode(err) != 2 {
		t.Errorf("bad format exit = %v", err)
	}
}

func TestTBThreadsShow(t *testing.T) {
	s := tbSetupB(t, false, true)
	thread := func(ref string) []tbMessageRow {
		t.Helper()
		out, _, err := tbRun(t, s.home, "threads", "show", ref, "--json")
		if err != nil {
			t.Fatalf("threads show %s: %v", ref, err)
		}
		return tbDecode[[]tbMessageRow](t, out)
	}
	rows := thread(tbID("INBOX", "m2@example.com"))
	if strings.Join(tbIDs(rows), ",") != "m2@example.com,s1@example.com" || rows[0].Direction != "in" || rows[1].Direction != "out" || rows[1].FolderPath != "Posta inviata" {
		t.Fatalf("budget thread: %+v", rows)
	}
	if again := thread(rows[0].ThreadID); strings.Join(tbIDs(again), ",") != "m2@example.com,s1@example.com" {
		t.Errorf("by thread id: %v", tbIDs(again))
	}
	if fromSent := thread(tbprofile.MessageKey("account1", "Posta inviata", "s1@example.com", 0)); len(fromSent) != 2 {
		t.Errorf("from the sent message: %v", tbIDs(fromSent))
	}
	if kickoff := thread(tbID("INBOX", "m8@example.com")); strings.Join(tbIDs(kickoff), ",") != "m1@example.com,m7@example.com,m8@example.com" {
		t.Errorf("kickoff thread: %v", tbIDs(kickoff))
	}
	if _, _, err := tbRun(t, s.home, "threads", "show", "ffffffffffff"); ExitCode(err) != 3 {
		t.Errorf("unknown thread exit = %v", err)
	}
}

func TestTBAttachments(t *testing.T) {
	s := tbSetupB(t, true, true)
	m4 := tbID("INBOX", "m4@example.com")
	out, _, err := tbRun(t, s.home, "attachments", "list", m4, "--json")
	atts := tbDecode[[]tbAttachmentDoc](t, out)
	if err != nil || len(atts) != 1 || atts[0].Filename != "report.pdf" || atts[0].ContentType != "application/pdf" {
		t.Fatalf("list: %v %+v", err, atts)
	}
	if out, _, err := tbRun(t, s.home, "attachments", "list", tbID("INBOX", "m1@example.com"), "--json"); err != nil || strings.TrimSpace(out) != "[]" {
		t.Errorf("no attachments must be []: %v %q", err, out)
	}
	if _, _, err := tbRun(t, s.home, "attachments", "list", "ffffffffffff"); ExitCode(err) != 3 {
		t.Errorf("unknown message exit = %v", err)
	}

	dir := filepath.Join(t.TempDir(), "out")
	if _, _, err := tbRun(t, s.home, "attachments", "save", m4, "--output", dir, "--json"); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "report.pdf")
	if got, _ := os.ReadFile(target); string(got) != "Hello attachment world!" {
		t.Fatalf("saved bytes = %q", got)
	}
	if err := os.WriteFile(target, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tbRun(t, s.home, "attachments", "save", m4, "--output", dir); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("overwrite without --force: %v", err)
	}
	out, _, err = tbRun(t, s.home, "attachments", "save", m4, "--output", dir, "--force", "--dry-run", "--json")
	plan := tbDecode[tbSavePlan](t, out)
	if got, _ := os.ReadFile(target); err != nil || !plan.DryRun || len(plan.Files) != 1 || !plan.Files[0].Exists || string(got) != "mine" {
		t.Fatalf("dry-run: %v %+v %q", err, plan, got)
	}
	if _, _, err := tbRun(t, s.home, "attachments", "save", m4, "--output", dir, "--force"); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "Hello attachment world!" {
		t.Fatalf("--force did not overwrite: %q", got)
	}

	x1 := tbID("INBOX", "x1@example.com")
	dir2 := filepath.Join(t.TempDir(), "x")
	out, _, err = tbRun(t, s.home, "attachments", "save", x1, "--output", dir2, "--json")
	saved := tbDecode[[]tbSavedAttachment](t, out)
	if err != nil || len(saved) != 2 {
		t.Fatalf("x1 save: %v %q", err, out)
	}
	for name, want := range map[string]string{"evil.txt": "first", "evil-1.txt": "second"} {
		if got, err := os.ReadFile(filepath.Join(dir2, name)); err != nil || strings.TrimSpace(string(got)) != want {
			t.Errorf("%s = %q %v", name, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir2), "evil.txt")); err == nil {
		t.Error("path traversal escaped --out")
	}
	dir3 := filepath.Join(t.TempDir(), "one")
	if out, _, err := tbRun(t, s.home, "attachments", "save", x1, "--index", "1", "--output", dir3, "--json"); err != nil || len(tbDecode[[]tbSavedAttachment](t, out)) != 1 {
		t.Fatalf("--index 1: %v %q", err, out)
	}
	entries, _ := os.ReadDir(dir3)
	if len(entries) != 1 || entries[0].Name() != "evil.txt" {
		t.Errorf("--index 1 wrote %v", entries)
	}
	if _, _, err := tbRun(t, s.home, "attachments", "save", x1, "--index", "5", "--output", dir3); ExitCode(err) != 3 {
		t.Errorf("bad index exit = %v", err)
	}
	if _, _, err := tbRun(t, s.home, "attachments", "save", m4); ExitCode(err) != 2 {
		t.Errorf("missing --out exit = %v", err)
	}
}

func TestTBSafeFilename(t *testing.T) {
	tests := []struct{ in, want string }{
		{"report.pdf", "report.pdf"},
		{"../../etc/passwd", "passwd"},
		{`C:\Windows\evil.bat`, "evil.bat"},
		{"a<b>c:d\"e|f?g*h.txt", "a_b_c_d_e_f_g_h.txt"},
		{"CON.txt", "_CON.txt"},
		{"nul", "_nul"},
		{" .hidden. ", "hidden"},
		{"", "attachment-3"},
		{"..", "attachment-3"},
		{"tab\there.txt", "tab_here.txt"},
	}
	for _, tt := range tests {
		if got := tbSafeFilename(tt.in, 3); got != tt.want {
			t.Errorf("tbSafeFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	if got := tbSafeFilename(strings.Repeat("\u00e9", 150)+".pdf", 0); len(got) > 200 || !strings.HasSuffix(got, ".pdf") || !strings.HasPrefix(got, "\u00e9") {
		t.Errorf("long name = %q (%d bytes)", got, len(got))
	}
}

func TestTBContacts(t *testing.T) {
	s := tbSetupB(t, false, true)
	search := func(q string) []string {
		t.Helper()
		out, _, err := tbRun(t, s.home, "contacts", "search", q, "--json")
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		ids := []string{}
		for _, c := range tbDecode[[]tbContactDoc](t, out) {
			ids = append(ids, c.ID)
		}
		return ids
	}
	tests := []struct {
		q    string
		want []string
	}{
		{"ALICE", []string{tbtest.CardAlice}},
		{"aLiCe ExAmPlE", []string{tbtest.CardAlice}},
		{"carol@EXAMPLE", []string{tbtest.CardCarol}},
		{"alice.alt", []string{tbtest.CardAlice}},
		{"al", []string{tbtest.CardAlice}},
		{"frank", []string{tbtest.CardFrank}},
		{"example.com", []string{tbtest.CardAlice, tbtest.CardCarol, tbtest.CardFrank}},
		{"zzz", []string{}},
	}
	for _, tt := range tests {
		if got := search(tt.q); strings.Join(got, ",") != strings.Join(tt.want, ",") {
			t.Errorf("search %q = %v, want %v", tt.q, got, tt.want)
		}
	}
	out, _, err := tbRun(t, s.home, "contacts", "show", "ALICE.alt@example.com", "--json")
	if c := tbDecode[tbContactDoc](t, out); err != nil || c.ID != tbtest.CardAlice || c.Company != "Example Corp" || len(c.Emails) != 2 {
		t.Errorf("show by email: %v %+v", err, c)
	}
	out, _, err = tbRun(t, s.home, "contacts", "show", tbtest.CardFrank, "--json")
	if c := tbDecode[tbContactDoc](t, out); err != nil || !c.Collected || c.PrimaryEmail != "frank@example.com" {
		t.Errorf("show by id: %v %+v", err, c)
	}
	if _, _, err := tbRun(t, s.home, "contacts", "show", "nobody@example.com"); ExitCode(err) != 3 {
		t.Errorf("unknown contact exit = %v", err)
	}
	if out, _, err := tbRun(t, s.home, "contacts"); err != nil || !strings.Contains(out, "search") || !strings.Contains(out, "top") {
		t.Errorf("bare contacts must show help listing search and top: %v %q", err, out)
	}
}

func TestTBSortContactsCollectedLast(t *testing.T) {
	rows := []tbContactDoc{
		{ID: "3", Collected: true, PrimaryEmail: "aaron@example.com"},
		{ID: "2", DisplayName: "zoe"},
		{ID: "1", DisplayName: "Bea"},
	}
	tbSortContacts(rows)
	if got := rows[0].ID + rows[1].ID + rows[2].ID; got != "123" {
		t.Errorf("order = %s, want address-book contacts by name, then collected", got)
	}
}

func TestTBFiltersList(t *testing.T) {
	s := tbSetupB(t, false, true)
	out, _, err := tbRun(t, s.home, "filters", "list", "--json")
	rows := tbDecode[[]tbFilterRow](t, out)
	if err != nil || len(rows) != 3 {
		t.Fatalf("filters list: %v %q", err, out)
	}
	news, old, all := rows[0], rows[1], rows[2]
	if news.Name != "Newsletters" || !news.Enabled || news.Action != "Move to folder" || news.Target != "imap://bob%40example.com@imap.example.com/Archives/2025" ||
		news.Summary != "subject contains newsletter OR from is news@example.com" || news.Terms != 2 || news.SupportedTerms != 2 || news.AccountName != "bob@example.com" {
		t.Errorf("Newsletters row: %+v", news)
	}
	if old.Name != "Old rule" || old.Enabled || old.Action != "Move to folder" || !strings.HasSuffix(old.Target, "/Gone") || old.Terms != 3 || old.SupportedTerms != 1 || len(old.Actions) != 2 {
		t.Errorf("Old rule row: %+v", old)
	}
	if all.Name != "Everything" || all.Index != 2 || all.Summary != "all messages" || all.Action != "Mark flagged" || all.Target != "" {
		t.Errorf("Everything row: %+v", all)
	}
	out, _, _ = tbRun(t, s.home, "filters", "list", "--json", "--account", "BOB@example.com", "--limit", "2")
	if got := tbDecode[[]tbFilterRow](t, out); len(got) != 2 || got[0].Name != "Newsletters" {
		t.Errorf("--account by name: %+v", got)
	}
	if out, _, _ := tbRun(t, s.home, "filters", "list", "--json", "--account", "account2"); strings.TrimSpace(out) != "[]" {
		t.Errorf("account without rules: %q", out)
	}
	if out, _, _ := tbRun(t, s.home, "filters", "list", "--json", "--account", "nope"); strings.TrimSpace(out) != "[]" {
		t.Errorf("unknown account: %q", out)
	}
	if out, _, err := tbRun(t, s.home, "filters"); err != nil || !strings.Contains(out, "list") || !strings.Contains(out, "audit") {
		t.Errorf("bare filters must show help: %v %q", err, out)
	}
}

func TestTBCalendarEvents(t *testing.T) {
	s := tbSetupB(t, false, true)
	titles := func(args ...string) []string {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"calendar", "events", "--json"}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		got := []string{}
		for _, e := range tbDecode[[]tbEventDoc](t, out) {
			got = append(got, e.Title)
		}
		return got
	}
	tests := []struct {
		args []string
		want string
	}{
		{nil, "Team sync,Dentist"},
		{[]string{"--since", "2025-01-20", "--until", "2025-02-02"}, "Dentist"},
		{[]string{"--until", "2025-01-15"}, "Team sync"},
		{[]string{"--since", "7d"}, ""},
		{[]string{"--limit", "1"}, "Team sync"},
	}
	for _, tt := range tests {
		if got := strings.Join(titles(tt.args...), ","); got != tt.want {
			t.Errorf("events %v = %q, want %q", tt.args, got, tt.want)
		}
	}
	if _, _, err := tbRun(t, s.home, "calendar", "events", "--until", "soon"); ExitCode(err) != 2 {
		t.Errorf("bad --until exit = %v", err)
	}
}

// tbGetArgs ports Thunderbird's MsgComposeCommands.js GetArgs so tests check
// that the compose string decodes back to the intended fields.
func tbGetArgs(original string) map[string]string {
	var data strings.Builder
	quote, prev := byte(0), byte(0)
	for i := 0; i < len(original); i++ {
		c := original[i]
		next := byte(0)
		if i < len(original)-1 {
			next = original[i+1]
		}
		switch {
		case quote != 0 && c == quote && (next == ',' || next == 0):
			quote = 0
			data.WriteByte(c)
		case (c == '\'' || c == '"') && prev == '=':
			if quote == 0 {
				quote = c
			}
			data.WriteByte(c)
		case c == ',':
			if quote == 0 {
				data.WriteByte(1)
			} else {
				data.WriteByte(c)
			}
		default:
			data.WriteByte(c)
		}
		prev = c
	}
	args := map[string]string{}
	for _, pair := range strings.Split(data.String(), "\x01") {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		if len(value) >= 2 && strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
			args[name] = value[1 : len(value)-1]
			continue
		}
		value = strings.Trim(value, `"`)
		if dec, err := url.PathUnescape(value); err == nil {
			value = dec
		}
		args[name] = value
	}
	return args
}

func TestTBComposeValueRoundTrip(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain text", "'plain text'"},
		{"a, b", "'a, b'"},
		{"50% done", "'50% done'"},
		{"it's fine", "'it's fine'"},
		{"line1\nline2", "'line1\nline2'"},
		{"stop',here", "stop%27%2Chere"},
	}
	for _, tt := range tests {
		got := tbComposeValue(tt.in)
		if got != tt.want {
			t.Errorf("tbComposeValue(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if back := tbGetArgs("body=" + got + ",subject='x'"); back["body"] != tt.in || back["subject"] != "x" {
			t.Errorf("round trip of %q = %q", tt.in, back)
		}
	}
}

func TestTBDraftsNew(t *testing.T) {
	s := tbSetupB(t, false, true)
	attDir := t.TempDir()
	att := filepath.Join(attDir, "a,b file.txt")
	if err := os.WriteFile(att, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "Hi,\nit's 50% done',\nbye"
	out, _, err := tbRun(t, s.home, "drafts", "new", "--to", "alice@example.com", "--to", `"Doe, John" <john@example.com>`,
		"--cc", "carol@example.com", "--subject", "Status, today", "--body", body, "--attach", att, "--from-identity", "bob.work@example.com", "--json")
	spec := tbDecode[tbComposeSpec](t, out)
	if err != nil || spec.Opened || len(*s.launch) != 0 || spec.Identity == nil || spec.Identity.ID != "id3" {
		t.Fatalf("drafts new: %v %q launched=%v", err, out, *s.launch)
	}
	if strings.Join(spec.To, "|") != `alice@example.com|"Doe, John" <john@example.com>` || spec.Argv[1] != "-compose" || spec.Argv[2] != spec.ComposeArg {
		t.Errorf("spec: %+v", spec)
	}
	args := tbGetArgs(spec.ComposeArg)
	fileURL := args["attachment"]
	if args["to"] != `alice@example.com, "Doe, John" <john@example.com>` || args["cc"] != "carol@example.com" || args["subject"] != "Status, today" ||
		args["body"] != body || args["preselectid"] != "id3" || args["format"] != "text" || !strings.HasPrefix(fileURL, "file:///") || strings.Contains(fileURL, ",") {
		t.Errorf("decoded compose args: %q", args)
	}
	if u, err := url.Parse(fileURL); err != nil || !strings.EqualFold(filepath.Clean(strings.TrimPrefix(u.Path, "/")), filepath.Clean(att)) && filepath.Clean(u.Path) != filepath.Clean(att) {
		t.Errorf("attachment url %q does not point at %s", fileURL, att)
	}
	if _, _, err := tbRun(t, s.home, "drafts", "new", "--to", "x", "--from-identity", "id9"); ExitCode(err) != 2 {
		t.Errorf("invalid address exit = %v", err)
	}
	if _, _, err := tbRun(t, s.home, "drafts", "new", "--to", "a@example.com", "--from-identity", "id9"); ExitCode(err) != 3 {
		t.Errorf("unknown identity exit = %v", err)
	}
	if _, _, err := tbRun(t, s.home, "drafts", "new", "--to", "a@example.com", "--attach", filepath.Join(attDir, "missing.pdf")); ExitCode(err) != 2 {
		t.Errorf("missing attachment exit = %v", err)
	}
	if _, _, err := tbRun(t, s.home, "drafts", "new", "--body", "x", "--body-file", att); ExitCode(err) != 2 {
		t.Errorf("--body with --body-file exit = %v", err)
	}
	out, _, err = tbRun(t, s.home, "drafts", "new", "--to", "alice@example.com", "--body-file", att, "--from-identity", "id1", "--json")
	if spec = tbDecode[tbComposeSpec](t, out); err != nil || spec.Body != "payload" || spec.Identity.Email != "bob@example.com" {
		t.Errorf("--body-file / idN: %v %+v", err, spec)
	}
}

func TestTBDraftsReply(t *testing.T) {
	s := tbSetupB(t, true, true)
	reply := func(args ...string) tbComposeSpec {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"drafts", "reply", "--json"}, args...)...)
		if err != nil {
			t.Fatalf("reply %v: %v", args, err)
		}
		return tbDecode[tbComposeSpec](t, out)
	}
	sp := reply(tbID("INBOX", "m8@example.com"), "--all", "--body", "Agreed.")
	if strings.Join(sp.To, "|") != "Carol Example <carol@example.com>" || strings.Join(sp.Cc, "|") != "alice@example.com" || sp.Subject != "Re: Project kickoff" ||
		sp.Identity == nil || sp.Identity.ID != "id1" || sp.InReplyTo != "m8@example.com" || strings.Join(sp.References, " ") != "m1@example.com m7@example.com m8@example.com" {
		t.Errorf("reply-all m8: %+v", sp)
	}
	if !strings.HasPrefix(sp.Body, "Agreed.\n\nOn Mon, 13 Jan 2025 16:00, Carol Example wrote:\n> Sounds good.") {
		t.Errorf("quoted body = %q", sp.Body)
	}
	if got := tbGetArgs(sp.ComposeArg); got["subject"] != "Re: Project kickoff" || got["body"] != sp.Body || got["cc"] != "alice@example.com" {
		t.Errorf("compose arg decodes to %q", got)
	}

	sp = reply(tbID("INBOX", "m2@example.com"))
	if strings.Join(sp.To, "|") != "Carol Example <carol@example.com>" || len(sp.Cc) != 0 || sp.Subject != "Re: Budget question" || !strings.Contains(sp.Body, "> Can you check the budget?\n>\n> From the desk") {
		t.Errorf("reply m2: %+v", sp)
	}
	sp = reply(tbID("INBOX", "m2@example.com"), "--all")
	if strings.Join(sp.Cc, "|") != `dave@example.com|"Erin E." <erin@example.com>` {
		t.Errorf("reply-all m2 cc = %v", sp.Cc)
	}

	sp = reply(tbID("INBOX", "x1@example.com"), "--all")
	if strings.Join(sp.To, "|") != "Heidi Desk <desk@example.com>" || strings.Join(sp.Cc, "|") != "ivan@example.com" || sp.Identity.ID != "id3" || sp.Subject != "Re: Work contract" {
		t.Errorf("reply-all x1 (Reply-To, bob.work identity): %+v %+v", sp, sp.Identity)
	}

	sp = reply(tbprofile.MessageKey("account1", "Posta inviata", "s1@example.com", 0))
	if strings.Join(sp.To, "|") != "Carol Example <carol@example.com>" || sp.Identity.ID != "id1" || sp.Subject != "Re: Budget question" {
		t.Errorf("reply to own sent message: %+v", sp)
	}
	if len(*s.launch) != 0 {
		t.Fatalf("reply without --open launched %v", *s.launch)
	}
	if _, _, err := tbRun(t, s.home, "drafts", "reply", "ffffffffffff"); ExitCode(err) != 3 {
		t.Errorf("unknown message exit = %v", err)
	}
}

func TestTBReplySubject(t *testing.T) {
	for in, want := range map[string]string{
		"Hello":           "Re: Hello",
		"Re: Hello":       "Re: Hello",
		"RE:Hello":        "RE:Hello",
		" re : Hello":     "re : Hello",
		"Fwd: Hello":      "Re: Fwd: Hello",
		"Regarding plans": "Re: Regarding plans",
	} {
		if got := tbReplySubject(in); got != want {
			t.Errorf("tbReplySubject(%q) = %q, want %q", in, got, want)
		}
	}
}

func tbFindFiles(t *testing.T, root, ext string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.EqualFold(filepath.Ext(p), ext) {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func TestTBDraftsOpen(t *testing.T) {
	s := tbSetupB(t, false, true)
	att := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(att, []byte("attached bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"drafts", "new", "--to", "alice@example.com", "--subject", "Caf\u00e9 plan", "--body", "See attached.", "--attach", att, "--open", "--json"}

	t.Setenv(cliutil.VerifyEnvVar, "1")
	out, _, err := tbRun(t, s.home, args...)
	if err != nil || !strings.Contains(out, `"refused":true`) || len(*s.launch) != 0 || len(tbFindFiles(t, s.home, ".eml")) != 0 {
		t.Fatalf("harness must refuse --open: %v %q launched=%v", err, out, *s.launch)
	}
	t.Setenv(cliutil.VerifyEnvVar, "")
	t.Setenv(cliutil.DogfoodEnvVar, "1")
	if out, _, _ := tbRun(t, s.home, "drafts", "reply", tbID("INBOX", "m1@example.com"), "--open", "--json"); !strings.Contains(out, `"refused":true`) || len(*s.launch) != 0 {
		t.Fatalf("dogfood must refuse reply --open: %q", out)
	}
	t.Setenv(cliutil.DogfoodEnvVar, "")

	out, _, err = tbRun(t, s.home, args...)
	spec := tbDecode[tbComposeSpec](t, out)
	if err != nil || !spec.Opened || spec.PID != 4242 || len(*s.launch) != 1 {
		t.Fatalf("--open: %v %q", err, out)
	}
	call := (*s.launch)[0]
	if len(call) != 3 || call[1] != "-compose" || call[2] != spec.ComposeArg {
		t.Errorf("launched %q", call)
	}
	emls := tbFindFiles(t, s.home, ".eml")
	if len(emls) != 1 || !strings.EqualFold(emls[0], spec.EMLPath) || filepath.Base(filepath.Dir(emls[0])) != "drafts" {
		t.Fatalf("eml copy %v vs %s", emls, spec.EMLPath)
	}
	if len(tbFindFiles(t, s.profile, ".eml")) != 0 {
		t.Fatal("wrote into the profile")
	}
	raw, _ := os.ReadFile(emls[0])
	m := tbprofile.ParseMessage(raw)
	_, data, err := tbprofile.ExtractAttachment(raw, 0)
	if m.Subject != "Caf\u00e9 plan" || strings.Join(m.To, ",") != "alice@example.com" || m.BodyText != "See attached." || err != nil || string(data) != "attached bytes" {
		t.Errorf("eml: %+v %q %v", m, data, err)
	}
}

func TestTBSliceBEmptyStore(t *testing.T) {
	s := tbSetupB(t, false, false)
	for _, args := range [][]string{
		{"messages", "list", "--json"},
		{"threads", "show", "abc", "--json"},
		{"attachments", "list", "abc", "--json"},
		{"contacts", "search", "a", "--json"},
		{"filters", "list", "--json"},
		{"calendar", "events", "--json"},
		{"messages", "auth", "abc", "--json"},
	} {
		out, errOut, err := tbRun(t, s.home, args...)
		if err != nil || strings.TrimSpace(out) != "[]" || !strings.Contains(errOut, "run: thunderbird-pp-cli sync") {
			t.Errorf("%v: err=%v out=%q stderr=%q", args, err, out, errOut)
		}
	}
	for _, args := range [][]string{
		{"messages", "show"}, {"threads", "show"}, {"attachments", "save"}, {"drafts", "reply"}, {"drafts", "new"}, {"contacts", "search"},
	} {
		out, err := tbRunBare(t, args...)
		if err != nil || !strings.Contains(out, "Usage:") {
			t.Errorf("bare %v must print help: %v %q", args, err, out)
		}
	}
	for _, args := range [][]string{
		{"messages", "show", "abc", "--dry-run", "--json"}, {"attachments", "save", "abc", "--output", "x", "--dry-run", "--json"}, {"drafts", "new", "--to", "a@example.com", "--open", "--dry-run", "--json"},
	} {
		out, _, err := tbRun(t, s.home, args...)
		if err != nil || !strings.Contains(out, `"dry_run":true`) {
			t.Errorf("%v: %v %q", args, err, out)
		}
	}
	if len(*s.launch) != 0 {
		t.Fatal("dry-run launched Thunderbird")
	}
}

func TestTBSliceBHumanOutput(t *testing.T) {
	s := tbSetupB(t, false, true)
	m2 := tbID("INBOX", "m2@example.com")
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"messages", "list", "--flagged"}, []string{"ID\tDATE\tFROM\tSUBJECT\tFOLDER\tFLAGS", m2 + "\t", "Carol Example", "Budget question", "UF"}},
		{[]string{"messages", "show", m2}, []string{"Subject: Budget question", "Cc:      dave@example.com, erin@example.com", "Can you check the budget?"}},
		{[]string{"threads", "show", m2}, []string{"DIR", "\tin\t", "\tout\t", "Re: Budget question"}},
		{[]string{"filters", "list"}, []string{"Newsletters", "subject contains newsletter OR from is news@example.com"}},
		{[]string{"drafts", "reply", m2}, []string{"To:      Carol Example <carol@example.com>", "not opened (add --open", "-compose"}},
	}
	for _, tt := range tests {
		out, _, err := tbRun(t, s.home, append(tt.args, "--human-friendly")...)
		out = strings.Join(strings.Fields(strings.ReplaceAll(out, "\t", " \t ")), " ")
		for _, w := range tt.want {
			if w = strings.Join(strings.Fields(strings.ReplaceAll(w, "\t", " \t ")), " "); err != nil || !strings.Contains(out, w) {
				t.Errorf("%v: missing %q in %q (%v)", tt.args, w, out, err)
			}
		}
	}
}

func TestTBFrameworkSearchFindsMessages(t *testing.T) {
	s := tbSetupB(t, false, true)
	out, _, err := tbRun(t, s.home, "search", "budget", "--type", "messages", "--json")
	if err != nil || !strings.Contains(out, "Budget question") || strings.Contains(out, "Project kickoff") {
		t.Fatalf("search --type messages: %v %q", err, out)
	}
}

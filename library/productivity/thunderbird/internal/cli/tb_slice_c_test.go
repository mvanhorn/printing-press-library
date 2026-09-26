package cli

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
)

var itoaT = strconv.Itoa

// Extra fictional mail for slice C, appended to temp copies of the fixture.
const tbSliceCInbox = `From - Tue Jan 14 08:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <o1@example.com>
In-Reply-To: <m5@example.com>
References: <m5@example.com>
Date: Tue, 14 Jan 2025 08:00:00 +0000
From: Bob Example <bob@example.com>
To: jose@example.com
Subject: Re: Cafe

Own-address copy sitting in the Inbox.

From - Tue Jan 14 09:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <w1@example.com>
Date: Tue, 14 Jan 2025 09:00:00 +0000
From: Ivan Example <ivan@example.com>
To: bob.work@example.com
Subject: Work question

Can you confirm?

From - Tue Jan 14 10:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <n2@example.com>
Date: Tue, 14 Jan 2025 10:00:00 +0000
From: Example News <news@example.com>
To: bob@example.com
Subject: Weekly digest
List-Id: Example News <news.example.com>
List-Unsubscribe: <mailto:unsubscribe@example.com>

Digest two.

From - Tue Jan 14 11:00:00 2025
X-Mozilla-Status: 0000
Message-ID: <p1@example.com>
Date: Tue, 14 Jan 2025 11:00:00 +0000
From: Shop Example <promo@shop.example.com>
To: bob@example.com
Subject: Sale
List-Unsubscribe: <https://shop.example.com/unsub>

Sale one.

From - Wed Jan 15 09:00:00 2025
X-Mozilla-Status: 0003
Message-ID: <r1@example.com>
Date: Wed, 15 Jan 2025 09:00:00 +0000
From: Heidi Example <heidi@example.com>
To: bob@example.com
Subject: Lunch

Lunch tomorrow?

From - Wed Jan 15 10:00:00 2025
X-Mozilla-Status: 0000
Message-ID: <nr1@example.com>
Date: Wed, 15 Jan 2025 10:00:00 +0000
From: Service <no-reply@service.example.com>
To: bob@example.com
Subject: Your receipt

Receipt.

From - Wed Jan 15 11:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <p2@example.com>
Date: Wed, 15 Jan 2025 11:00:00 +0000
From: Shop Example <promo@shop.example.com>
To: bob@example.com
Subject: Sale again
List-Unsubscribe: <https://shop.example.com/unsub>

Sale two.

From - Wed Jan 15 12:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <own2@example.com>
Date: Wed, 15 Jan 2025 12:00:00 +0000
From: Bob Example <bob@example.com>
To: bob@example.com
Subject: Note to self

Remember the milk.

From - Thu Jan 16 09:00:00 2025
X-Mozilla-Status: 0000
Message-ID: <n3@example.com>
Date: Thu, 16 Jan 2025 09:00:00 +0000
From: Example News <news@example.com>
To: bob@example.com
Subject: Weekly digest
List-Id: Example News <news.example.com>
List-Unsubscribe: <https://news.example.com/u/3>, <mailto:u3@example.com>

Digest three.

From - Thu Jan 16 10:00:00 2025
X-Mozilla-Status: 0003
Message-ID: <big@example.com>
Date: Thu, 16 Jan 2025 10:00:00 +0000
From: Grace Example <grace@example.com>
To: bob@example.com
Subject: Big file
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="B3"

--B3
Content-Type: text/plain

Big one.
--B3
Content-Type: application/octet-stream; name="big.bin"
Content-Disposition: attachment; filename="big.bin"
Content-Transfer-Encoding: base64

QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFB
--B3--

`

// Sent copy stored in a folder whose name no heuristic knows; only the
// identity's fcc_folder in prefs.js marks it as a sent folder.
const tbSliceCOutbox = `From - Wed Jan 15 08:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <s2@example.com>
In-Reply-To: <w1@example.com>
References: <w1@example.com>
Date: Wed, 15 Jan 2025 08:00:00 +0000
From: Bob Work <bob.work@example.com>
To: Ivan Example <ivan@example.com>
Cc: bob@example.com
Subject: Re: Work question

Confirmed.

`

// A second copy of m7 (same Message-ID) filed in the archive.
const tbSliceCArchiveCopy = `
From - Sun Jan 12 15:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <m7@example.com>
In-Reply-To: <m1@example.com>
References: <m1@example.com>
Date: Sun, 12 Jan 2025 15:00:00 +0000
From: Alice Example <alice@example.com>
To: bob@example.com
Subject: Re: Project kickoff

Adding the agenda.

`

const tbSliceCTrash = `From - Thu Jan 16 11:00:00 2025
X-Mozilla-Status: 0000
Message-ID: <t1@example.com>
Date: Thu, 16 Jan 2025 11:00:00 +0000
From: Ivan Example <ivan@example.com>
To: bob@example.com
Subject: Old thing

Deleted.

`

const tbSliceCFilters = `name="Newsletters copy"
enabled="yes"
type="17"
action="Move to folder"
actionValue="imap://bob%40example.com@imap.example.com/Archives/2025"
condition="OR (subject,contains,newsletter) OR (from,is,news@example.com)"
name="Agenda"
enabled="yes"
type="17"
action="Mark flagged"
condition="AND (body,contains,agenda)"
name="Work to outbox"
enabled="yes"
type="17"
action="Copy to folder"
actionValue="imap://bob%40example.com@imap.example.com/Outbox%20Work"
condition="AND (to,contains,bob) AND (subject,begins with,work)"
`

var tbSliceCNow = time.Date(2025, 1, 20, 12, 0, 0, 0, time.UTC)

func tbAppendFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func tbSetupC(t *testing.T) tbSliceB {
	t.Helper()
	s := tbSetupB(t, false, false)
	imap := filepath.Join(s.profile, "ImapMail", "imap.example.com")
	tbAppendFile(t, filepath.Join(imap, "INBOX"), tbSliceCInbox)
	tbAppendFile(t, filepath.Join(imap, "Outbox Work"), tbSliceCOutbox)
	tbAppendFile(t, filepath.Join(imap, "Archives.sbd", "2025"), tbSliceCArchiveCopy)
	tbAppendFile(t, filepath.Join(imap, "msgFilterRules.dat"), tbSliceCFilters)
	tbAppendFile(t, filepath.Join(s.profile, "Mail", "Local Folders", "Trash"), tbSliceCTrash)
	tbAppendFile(t, filepath.Join(s.profile, "prefs.js"),
		`user_pref("mail.identity.id3.fcc_folder", "imap://bob%40example.com@imap.example.com/Outbox%20Work");`+"\n")
	orig := tbNow
	tbNow = func() time.Time { return tbSliceCNow }
	t.Cleanup(func() { tbNow = orig })
	if out, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
		t.Fatalf("sync: %v %s", err, out)
	}
	return s
}

func TestTBRootEnvDoesNotMoveDataDir(t *testing.T) {
	testenv.Isolate(t)
	root := t.TempDir()
	t.Setenv("THUNDERBIRD_ROOT", root)
	if got := tbprofile.RootDir(); got != root {
		t.Fatalf("THUNDERBIRD_ROOT not honoured: RootDir = %q", got)
	}
	dataDir, err := cliutil.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(strings.ToLower(filepath.Clean(dataDir)), strings.ToLower(filepath.Clean(root))) {
		t.Fatalf("THUNDERBIRD_ROOT moved the CLI data dir under the Thunderbird root: %q", dataDir)
	}
	t.Setenv("THUNDERBIRD_ROOT", "")
	t.Setenv("THUNDERBIRD_HOME", root)
	if got := tbprofile.RootDir(); got == root {
		t.Fatal("THUNDERBIRD_HOME must not select the Thunderbird root")
	}
}

func TestTBSentFolderSemantics(t *testing.T) {
	s := tbSetupC(t)
	out, _, err := tbRun(t, s.home, "folders", "--json")
	folders := tbDecode[[]tbFolderDoc](t, out)
	if err != nil {
		t.Fatal(err)
	}
	sent := map[string]bool{}
	for _, f := range folders {
		sent[f.ID] = f.SentFolder
	}
	if !sent["account1:Posta inviata"] || !sent["account1:Outbox Work"] || sent["account1:INBOX"] || sent["account2:Trash"] {
		t.Errorf("sent_folder flags = %v", sent)
	}
	db, err := tbOpenStoreForTest(t, s.home)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mb, err := tbLoadMailbox(db)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := tbQueryMessages(db, "", nil, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	dirs := map[string]int{}
	for _, d := range docs {
		dirs[d.MessageID] = mb.direction(d)
	}
	for id, want := range map[string]int{
		"s1@example.com": tbDirSent, "s2@example.com": tbDirSent,
		"o1@example.com": tbDirOther, "own2@example.com": tbDirOther,
		"m2@example.com": tbDirInbound, "t1@example.com": tbDirInbound,
	} {
		if dirs[id] != want {
			t.Errorf("%s direction = %d, want %d", id, dirs[id], want)
		}
	}
}

func tbOpenStoreForTest(t *testing.T, home string) (*store.Store, error) {
	t.Helper()
	if _, err := cliutil.SetHomeOverride(home); err != nil {
		return nil, err
	}
	path := defaultDBPath(tbCLIName)
	_, _ = cliutil.SetHomeOverride("")
	return store.OpenReadOnlyContext(context.Background(), path)
}

func TestTBAwaitingReply(t *testing.T) {
	s := tbSetupC(t)
	run := func(args ...string) []tbAwaitingRow {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"awaiting-reply", "--json"}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		return tbDecode[[]tbAwaitingRow](t, out)
	}
	ids := func(rows []tbAwaitingRow) []string {
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.LastMessageID)
		}
		return out
	}
	eq := func(name string, got []string, want ...string) {
		t.Helper()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	a1 := tbprofile.MessageKey("account1", "Archives/2025", "a1@example.com", 0)
	m4, m5, m8 := tbID("INBOX", "m4@example.com"), tbID("INBOX", "m5@example.com"), tbID("INBOX", "m8@example.com")

	rows := run("--days", "30")
	eq("pending oldest first", ids(rows), a1, m4, m5, m8)
	for _, r := range rows {
		if r.LastMessageID == m8 && (r.Counterpart != "carol@example.com" || r.CounterpartName != "Carol Example" || r.MessagesInThread != 3 ||
			r.AgeDays != 6 || r.Folder != "INBOX" || r.Account != "account1" || r.Subject != "Re: Project kickoff" || r.LastDate != "2025-01-13T16:00:00Z") {
			t.Errorf("kickoff row = %+v", r)
		}
		if r.LastMessageID == m5 && r.MessagesInThread != 2 {
			t.Errorf("m5 thread counts the own Inbox copy as a thread message: %+v", r)
		}
	}
	eq("--days window", ids(run("--days", "10")), m5, m8)
	eq("--limit", ids(run("--days", "30", "--limit", "2")), a1, m4)
	eq("--account without pending", ids(run("--days", "30", "--account", "account2")))
	eq("--account by name", ids(run("--days", "30", "--account", "BOB@example.com", "--limit", "1")), a1)

	bulk := ids(run("--days", "30", "--include-bulk"))
	for _, want := range []string{tbID("INBOX", "m6@example.com"), tbID("INBOX", "nr1@example.com"), tbID("INBOX", "p1@example.com")} {
		if !strings.Contains(strings.Join(bulk, ","), want) {
			t.Errorf("--include-bulk misses %s: %v", want, bulk)
		}
	}
	if len(bulk) != 10 {
		t.Errorf("--include-bulk rows = %d (%v)", len(bulk), bulk)
	}

	theirs := run("--days", "30", "--theirs")
	eq("--theirs", ids(theirs), tbID("Posta inviata", "s1@example.com"), tbID("Outbox Work", "s2@example.com"))
	if len(theirs) == 2 && (theirs[0].Counterpart != "carol@example.com" || theirs[1].Counterpart != "ivan@example.com" || theirs[1].MessagesInThread != 2) {
		t.Errorf("--theirs rows = %+v", theirs)
	}

	if _, _, err := tbRun(t, s.home, "awaiting-reply", "--days", "0", "--json"); ExitCode(err) != 2 {
		t.Errorf("--days 0 must be a usage error, got %v", err)
	}
}

func TestTBContactsTop(t *testing.T) {
	s := tbSetupC(t)
	run := func(args ...string) []tbCorrespondentRow {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"contacts", "top", "--json", "--since", "30d"}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		return tbDecode[[]tbCorrespondentRow](t, out)
	}
	summary := func(rows []tbCorrespondentRow) string {
		parts := make([]string, 0, len(rows))
		for _, r := range rows {
			parts = append(parts, strings.Join([]string{r.Address, itoaT(r.Received), itoaT(r.SentTo), itoaT(r.Total)}, "/"))
		}
		return strings.Join(parts, " ")
	}
	rows := run()
	want := "carol@example.com/2/1/3 grace@example.com/2/0/2 ivan@example.com/1/1/2 alice@example.com/2/0/2 heidi@example.com/1/0/1 jose@example.com/1/0/1 frank@example.com/1/0/1"
	if got := summary(rows); got != want {
		t.Errorf("contacts top =\n %s\nwant\n %s", got, want)
	}
	byAddr := map[string]tbCorrespondentRow{}
	for _, r := range rows {
		byAddr[r.Address] = r
	}
	if c := byAddr["carol@example.com"]; !c.InAddressBook || c.Collected || c.Name != "Carol Example" || c.LastContact != "2025-01-13T16:00:00Z" {
		t.Errorf("carol = %+v", c)
	}
	if f := byAddr["frank@example.com"]; f.InAddressBook || !f.Collected || f.Name != "Frank Example" {
		t.Errorf("frank = %+v", f)
	}
	if j := byAddr["jose@example.com"]; j.Name != "José Example" || j.InAddressBook {
		t.Errorf("jose = %+v", j)
	}
	if got := summary(run("--not-in-abook")); got != "grace@example.com/2/0/2 ivan@example.com/1/1/2 heidi@example.com/1/0/1 jose@example.com/1/0/1 frank@example.com/1/0/1" {
		t.Errorf("--not-in-abook = %s", got)
	}
	if got := summary(run("--limit", "2")); got != "carol@example.com/2/1/3 grace@example.com/2/0/2" {
		t.Errorf("--limit = %s", got)
	}
	if got := summary(run("--since", "2025-01-12")); got != "ivan@example.com/1/1/2 grace@example.com/1/0/1 heidi@example.com/1/0/1 carol@example.com/1/0/1 alice@example.com/1/0/1" {
		t.Errorf("--since date = %s", got)
	}
	if _, _, err := tbRun(t, s.home, "contacts", "top", "--since", "soon", "--json"); ExitCode(err) != 2 {
		t.Errorf("bad --since must be a usage error, got %v", err)
	}
}

func TestTBFiltersAudit(t *testing.T) {
	s := tbSetupC(t)
	out, _, err := tbRun(t, s.home, "filters", "audit", "--json", "--days", "30")
	rows := tbDecode[[]tbFilterAuditRow](t, out)
	if err != nil || len(rows) != 6 {
		t.Fatalf("filters audit: %v %q", err, out)
	}
	hits := func(r tbFilterAuditRow) int {
		if r.Hits == nil {
			return -1
		}
		return *r.Hits
	}
	news, old, all, dup, agenda, work := rows[0], rows[1], rows[2], rows[3], rows[4], rows[5]
	if news.Status != "evaluated" || hits(news) != 3 || news.TargetExists == nil || !*news.TargetExists || news.TargetFolder != "account1:Archives/2025" || news.DuplicateOf != "" || len(news.Issues) != 0 {
		t.Errorf("Newsletters = %+v", news)
	}
	if news.ScannedMessages != 18 {
		t.Errorf("scanned_messages = %d, want 18 (sent folders excluded)", news.ScannedMessages)
	}
	if old.Enabled || old.Status != "unevaluated" || old.Hits != nil || len(old.UnsupportedTerms) != 2 || old.TargetExists == nil || *old.TargetExists ||
		strings.Join(old.Issues, ",") != "disabled,missing_target_folder,unevaluated" {
		t.Errorf("Old rule = %+v", old)
	}
	if all.Status != "evaluated" || hits(all) != all.ScannedMessages || all.TargetExists != nil {
		t.Errorf("Everything = %+v", all)
	}
	if dup.DuplicateOf != news.ID || hits(dup) != 3 || strings.Join(dup.Issues, ",") != "duplicate" {
		t.Errorf("duplicate = %+v", dup)
	}
	if hits(agenda) != 1 {
		t.Errorf("body filter hits = %d", hits(agenda))
	}
	if hits(work) != 1 || work.TargetExists == nil || !*work.TargetExists || work.TargetFolder != "account1:Outbox Work" {
		t.Errorf("Work to outbox = %+v", work)
	}
	out, _, _ = tbRun(t, s.home, "filters", "audit", "--json", "--days", "1")
	if r := tbDecode[[]tbFilterAuditRow](t, out); len(r) != 6 || hits(r[0]) != 0 || r[0].ScannedMessages != 0 || !strings.Contains(strings.Join(r[0].Issues, ","), "no_hits") {
		t.Errorf("--days 1 = %+v", r)
	}
	if out, _, _ := tbRun(t, s.home, "filters", "audit", "--json", "--account", "account2"); strings.TrimSpace(out) != "[]" {
		t.Errorf("--account account2 = %q", out)
	}
}

func TestTBLargest(t *testing.T) {
	s := tbSetupC(t)
	out, _, err := tbRun(t, s.home, "messages", "list", "--limit", "0", "--json")
	if err != nil {
		t.Fatal(err)
	}
	all := tbDecode[[]tbMessageRow](t, out)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].SizeBytes != all[j].SizeBytes {
			return all[i].SizeBytes > all[j].SizeBytes
		}
		return all[i].ID < all[j].ID
	})
	out, _, err = tbRun(t, s.home, "largest", "--limit", "3", "--json")
	rows := tbDecode[[]tbLargestMessageRow](t, out)
	if err != nil || len(rows) != 3 {
		t.Fatalf("largest: %v %q", err, out)
	}
	for i := range rows {
		if rows[i].ID != all[i].ID || rows[i].SizeBytes != all[i].SizeBytes || rows[i].SizeBytes == 0 {
			t.Errorf("largest[%d] = %+v, want %s (%d)", i, rows[i], all[i].ID, all[i].SizeBytes)
		}
	}
	if rows[0].SizeBytes < rows[1].SizeBytes || rows[1].SizeBytes < rows[2].SizeBytes {
		t.Errorf("not descending: %+v", rows)
	}
	out, _, _ = tbRun(t, s.home, "largest", "--folder", "posta INVIATA", "--limit", "0", "--json")
	if r := tbDecode[[]tbLargestMessageRow](t, out); len(r) != 1 || r[0].Folder != "Posta inviata" {
		t.Errorf("--folder = %+v", r)
	}
	out, _, _ = tbRun(t, s.home, "largest", "--account", "account2", "--limit", "0", "--json")
	if r := tbDecode[[]tbLargestMessageRow](t, out); len(r) != 1 || r[0].Account != "account2" {
		t.Errorf("--account = %+v", r)
	}
	out, _, err = tbRun(t, s.home, "largest", "--attachments", "--json")
	atts := tbDecode[[]tbLargestAttachmentRow](t, out)
	if err != nil || len(atts) != 2 || atts[0].Filename != "big.bin" || atts[0].SizeBytes != 57 || atts[1].Filename != "report.pdf" ||
		atts[0].MessageID != tbID("INBOX", "big@example.com") || atts[0].Folder != "INBOX" || atts[0].ContentType != "application/octet-stream" {
		t.Errorf("--attachments = %+v (%v)", atts, err)
	}
	if out, _, _ := tbRun(t, s.home, "largest", "--attachments", "--account", "account2", "--json"); strings.TrimSpace(out) != "[]" {
		t.Errorf("attachments of account2 = %q", out)
	}
}

func TestTBNewsletters(t *testing.T) {
	s := tbSetupC(t)
	run := func(args ...string) []tbNewsletterRow {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"newsletters", "--json", "--since", "30d"}, args...)...)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, out)
		}
		return tbDecode[[]tbNewsletterRow](t, out)
	}
	rows := run()
	if len(rows) != 1 {
		t.Fatalf("default --min 3 = %+v", rows)
	}
	n := rows[0]
	if n.Key != "news.example.com" || n.Kind != "list" || n.Name != "Example News" || n.FromAddr != "news@example.com" || n.Messages != 3 || n.Unread != 2 ||
		n.UnreadShare != 0.667 || n.LastReceived != "2025-01-16T09:00:00Z" || n.LastRead != "2025-01-14T10:00:00Z" || n.ListUnsubscribe != "https://news.example.com/u/3" {
		t.Errorf("list row = %+v", n)
	}
	rows = run("--min", "2")
	if len(rows) != 2 || rows[1].Key != "promo@shop.example.com" || rows[1].Kind != "automated" || rows[1].Messages != 2 || rows[1].Unread != 1 ||
		rows[1].UnreadShare != 0.5 || rows[1].ListUnsubscribe != "https://shop.example.com/unsub" || rows[1].LastRead != "2025-01-15T11:00:00Z" {
		t.Errorf("--min 2 = %+v", rows)
	}
	if rows := run("--min", "1", "--limit", "1"); len(rows) != 1 || rows[0].Key != "news.example.com" {
		t.Errorf("--limit = %+v", rows)
	}
	if rows := run("--since", "2025-01-15", "--min", "1"); len(rows) != 3 || rows[0].Messages != 1 || rows[1].Messages != 1 || rows[2].Messages != 1 {
		t.Errorf("--since date = %+v", rows)
	}
}

func TestTBSliceCEmptyStoreAndHelp(t *testing.T) {
	s := tbSetupB(t, false, false)
	for _, args := range [][]string{
		{"awaiting-reply", "--json"}, {"contacts", "top", "--json"}, {"filters", "audit", "--json"}, {"largest", "--json"}, {"largest", "--attachments", "--json"}, {"newsletters", "--json"},
	} {
		out, errOut, err := tbRun(t, s.home, args...)
		if err != nil || strings.TrimSpace(out) != "[]" || !strings.Contains(errOut, "run: thunderbird-pp-cli sync") {
			t.Errorf("%v: err=%v out=%q stderr=%q", args, err, out, errOut)
		}
		out, _, err = tbRun(t, s.home, append(args, "--dry-run")...)
		if err != nil || !strings.Contains(out, `"dry_run":true`) {
			t.Errorf("%v --dry-run: %v %q", args, err, out)
		}
	}
	sync := tbSetupC(t)
	for _, tt := range []struct {
		args []string
		want []string
	}{
		{[]string{"awaiting-reply", "--days", "30"}, []string{"AGE\tLAST\tCOUNTERPART", "Carol Example", "Re: Project kickoff"}},
		{[]string{"contacts", "top", "--since", "30d"}, []string{"ADDRESS\tNAME\tRECEIVED", "carol@example.com", "collected"}},
		{[]string{"filters", "audit", "--days", "30"}, []string{"HITS", "Newsletters", "disabled,missing_target_folder,unevaluated"}},
		{[]string{"largest", "--attachments"}, []string{"SIZE\tFILENAME", "big.bin", "57 B"}},
		{[]string{"newsletters", "--since", "30d"}, []string{"MESSAGES\tUNREAD\tSHARE", "Example News", "67%"}},
	} {
		out, _, err := tbRun(t, sync.home, append(tt.args, "--human-friendly")...)
		out = strings.Join(strings.Fields(strings.ReplaceAll(out, "\t", " \t ")), " ")
		for _, w := range tt.want {
			if w = strings.Join(strings.Fields(strings.ReplaceAll(w, "\t", " \t ")), " "); err != nil || !strings.Contains(out, w) {
				t.Errorf("%v: missing %q in %q (%v)", tt.args, w, out, err)
			}
		}
	}
}

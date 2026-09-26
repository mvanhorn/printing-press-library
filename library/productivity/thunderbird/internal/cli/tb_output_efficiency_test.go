package cli

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
)

const tbInlineMessages = `From - Fri Jan 10 09:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <q1@example.com>
Date: Fri, 10 Jan 2025 09:00:00 +0000
From: Mario Esempio <mario@example.com>
To: Anna Prova <anna@example.com>
Subject: Re: Offerta
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="Q1"

--Q1
Content-Type: multipart/related; boundary="Q2"

--Q2
Content-Type: multipart/alternative; boundary="Q3"

--Q3
Content-Type: text/plain

Confermo l'offerta.

--
Mario Esempio

Il 09/01/2025 09:00, Anna Prova ha scritto:
> Mi confermi l'offerta?
> Anna
--Q3
Content-Type: text/html

<p>Confermo</p><img src="cid:logo@example.com">
--Q3--
--Q2
Content-Type: image/png; name="logo.png"
Content-ID: <logo@example.com>

PNGDATA-PNGDATA-PNGDATA-PNGDATA-PNGDATA-PNGDATA-PNGDATA-PNGDATA-PNGDATA-PNGDATA
--Q2--
--Q1
Content-Type: image/jpeg; name="photo.jpg"
Content-Disposition: inline; filename="photo.jpg"
Content-Transfer-Encoding: base64

SlBFR0RBVEE=
--Q1
Content-Type: application/pdf; name="offerta.pdf"
Content-Disposition: attachment; filename="offerta.pdf"

PDFDATA
--Q1--

From - Sat Jan 11 09:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <q2@example.com>
Date: Sat, 11 Jan 2025 09:00:00 +0000
From: Anna Prova <anna@example.com>
To: Mario Esempio <mario@example.com>
Subject: Solo firma
MIME-Version: 1.0
Content-Type: multipart/related; boundary="R1"

--R1
Content-Type: text/html

<p>Ciao</p><img src="cid:sig@example.com">
--R1
Content-Type: image/gif; name="banner.gif"
Content-ID: <sig@example.com>
Content-Transfer-Encoding: base64

R0lGREFUQQ==
--R1--

From - Sun Jan 12 09:00:00 2025
X-Mozilla-Status: 0001
Message-ID: <q3@example.com>
Date: Sun, 12 Jan 2025 09:00:00 +0000
From: Bob Sample <bob@example.com>
To: Mario Esempio <mario@example.com>
Subject: Re: Riunione
MIME-Version: 1.0
Content-Type: text/html; charset=utf-8

<div dir="ltr">Confermo per lunedì.</div><br><div class="gmail_quote"><div dir="ltr" class="gmail_attr">On Sat, Jan 11, 2025 at 9:00 AM Mario Esempio &lt;mario@example.com&gt; wrote:<br></div><blockquote class="gmail_quote">Ci vediamo lunedì?<br>Mario</blockquote></div>

`

func tbSetupInline(t *testing.T) tbSliceB {
	t.Helper()
	s := tbSetupB(t, false, false)
	f, err := os.OpenFile(filepath.Join(s.profile, filepath.FromSlash(tbInboxRel)), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(tbInlineMessages); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if out, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
		t.Fatalf("sync: %v %s", err, out)
	}
	return s
}

func tbAttNames[T any](rows []T, name func(T) string) string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, name(r))
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func TestTBShowNoQuotes(t *testing.T) {
	s := tbSetupInline(t)
	q1 := tbID("INBOX", "q1@example.com")
	out, _, err := tbRun(t, s.home, "messages", "show", q1, "--json")
	if full := tbDecode[tbMessageDetail](t, out); err != nil || !strings.Contains(full.BodyText, "> Mi confermi") {
		t.Fatalf("default show must keep the quote: %v %q", err, full.BodyText)
	}
	out, _, err = tbRun(t, s.home, "messages", "show", q1, "--no-quotes", "--json")
	if got := tbDecode[tbMessageDetail](t, out).BodyText; err != nil || got != "Confermo l'offerta.\n\n--\nMario Esempio" {
		t.Fatalf("--no-quotes body = %q (%v)", got, err)
	}
	q3 := tbID("INBOX", "q3@example.com")
	out, _, err = tbRun(t, s.home, "messages", "show", q3, "--no-quotes", "--json")
	if got := tbDecode[tbMessageDetail](t, out).BodyText; err != nil || got != "Confermo per lunedì." {
		t.Errorf("HTML-only reply --no-quotes body = %q (%v)", got, err)
	}
	out, _, err = tbRun(t, s.home, "messages", "show", q1, "--no-quotes", "--human-friendly")
	if err != nil || strings.Contains(out, "ha scritto") || !strings.Contains(out, "Mario Esempio") {
		t.Errorf("human --no-quotes: %v %q", err, out)
	}
}

func TestTBThreadsLast(t *testing.T) {
	s := tbSetupB(t, false, true)
	m8 := tbID("INBOX", "m8@example.com")
	thread := func(args ...string) []tbMessageRow {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"threads", "show", m8, "--json"}, args...)...)
		if err != nil {
			t.Fatalf("threads show %v: %v", args, err)
		}
		return tbDecode[[]tbMessageRow](t, out)
	}
	tests := []struct {
		args []string
		want string
	}{
		{nil, "m1@example.com,m7@example.com,m8@example.com"},
		{[]string{"--last", "0"}, "m1@example.com,m7@example.com,m8@example.com"},
		{[]string{"--last", "2"}, "m7@example.com,m8@example.com"},
		{[]string{"--last", "1"}, "m8@example.com"},
		{[]string{"--last", "5"}, "m1@example.com,m7@example.com,m8@example.com"},
	}
	for _, tt := range tests {
		rows := thread(tt.args...)
		if got := strings.Join(tbIDs(rows), ","); got != tt.want {
			t.Errorf("%v: ids %s, want %s", tt.args, got, tt.want)
		}
		for _, r := range rows {
			if r.TotalMessages != 3 {
				t.Errorf("%v: total_messages = %d, want 3", tt.args, r.TotalMessages)
			}
		}
	}
	if _, errOut, err := tbRun(t, s.home, "threads", "show", m8, "--last", "2", "--human-friendly"); err != nil || !strings.Contains(errOut, "last 2 of 3") {
		t.Errorf("human --last note: %v %q", err, errOut)
	}
	if _, _, err := tbRun(t, s.home, "threads", "show", m8, "--last", "-1"); ExitCode(err) != 2 {
		t.Errorf("negative --last exit = %v", err)
	}
	if out, _, _ := tbRun(t, s.home, "messages", "list", "--json", "--limit", "1"); strings.Contains(out, "total_messages") {
		t.Errorf("messages list must not carry total_messages: %s", out)
	}
}

func TestTBInlineAttachments(t *testing.T) {
	s := tbSetupInline(t)
	q1, q2 := tbID("INBOX", "q1@example.com"), tbID("INBOX", "q2@example.com")

	out, _, err := tbRun(t, s.home, "messages", "list", "--json", "--limit", "0")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range tbDecode[[]tbMessageRow](t, out) {
		switch r.ID {
		case q1:
			if !r.HasAttachments || r.AttachmentCount != 1 || !strings.Contains(r.Flags, "A") {
				t.Errorf("q1 counts inline parts: %+v", r)
			}
		case q2:
			if r.HasAttachments || r.AttachmentCount != 0 {
				t.Errorf("q2 has only inline parts: %+v", r)
			}
		}
	}

	attName := func(a tbAttachmentDoc) string { return a.Filename }
	out, _, err = tbRun(t, s.home, "attachments", "list", q1, "--json")
	atts := tbDecode[[]tbAttachmentDoc](t, out)
	if err != nil || len(atts) != 1 || atts[0].Filename != "offerta.pdf" || atts[0].Index != 2 || atts[0].Inline == nil || *atts[0].Inline {
		t.Fatalf("list default: %v %s", err, out)
	}
	out, _, _ = tbRun(t, s.home, "attachments", "list", q1, "--include-inline", "--json")
	all := tbDecode[[]tbAttachmentDoc](t, out)
	if tbAttNames(all, attName) != "logo.png,offerta.pdf,photo.jpg" {
		t.Fatalf("list --include-inline: %s", out)
	}
	for _, a := range all {
		if (a.Filename != "offerta.pdf") != *a.Inline {
			t.Errorf("%s inline = %v", a.Filename, *a.Inline)
		}
	}

	out, _, _ = tbRun(t, s.home, "messages", "show", q1, "--json")
	if det := tbDecode[tbMessageDetail](t, out); len(det.Attachments) != 1 || det.Attachments[0].Filename != "offerta.pdf" || det.Attachments[0].Index != 2 {
		t.Errorf("show default attachments: %+v", det.Attachments)
	}
	out, _, _ = tbRun(t, s.home, "messages", "show", q1, "--include-inline", "--json")
	if det := tbDecode[tbMessageDetail](t, out); len(det.Attachments) != 3 || !det.Attachments[0].Inline || !det.Attachments[1].Inline || det.Attachments[2].Inline {
		t.Errorf("show --include-inline attachments: %+v", det.Attachments)
	}

	largest := func(args ...string) string {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"largest", "--attachments", "--limit", "0", "--json"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		return tbAttNames(tbDecode[[]tbLargestAttachmentRow](t, out), func(r tbLargestAttachmentRow) string { return r.Filename })
	}
	if got := largest(); strings.Contains(got, "logo.png") || strings.Contains(got, "photo.jpg") || strings.Contains(got, "banner.gif") || !strings.Contains(got, "offerta.pdf") {
		t.Errorf("largest default: %s", got)
	}
	if got := largest("--include-inline"); !strings.Contains(got, "logo.png") || !strings.Contains(got, "banner.gif") {
		t.Errorf("largest --include-inline: %s", got)
	}
}

func TestTBInlineSave(t *testing.T) {
	s := tbSetupInline(t)
	q1, q2 := tbID("INBOX", "q1@example.com"), tbID("INBOX", "q2@example.com")
	saved := func(dir string, args ...string) string {
		t.Helper()
		out, _, err := tbRun(t, s.home, append([]string{"attachments", "save", "--output", dir, "--json"}, args...)...)
		if err != nil {
			t.Fatalf("save %v: %v %s", args, err, out)
		}
		return tbAttNames(tbDecode[[]tbSavedAttachment](t, out), func(f tbSavedAttachment) string { return f.Filename })
	}
	if got := saved(t.TempDir(), q1); got != "offerta.pdf" {
		t.Errorf("save default = %s", got)
	}
	if got := saved(t.TempDir(), q1, "--include-inline"); got != "logo.png,offerta.pdf,photo.jpg" {
		t.Errorf("save --include-inline = %s", got)
	}
	dir := t.TempDir()
	if got := saved(dir, q1, "--index", "0"); got != "logo.png" {
		t.Errorf("save --index 0 = %s", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "logo.png")); !strings.HasPrefix(string(b), "PNGDATA-") {
		t.Errorf("logo bytes = %q", b)
	}
	if _, _, err := tbRun(t, s.home, "attachments", "save", q2, "--output", t.TempDir()); ExitCode(err) != 3 || !strings.Contains(err.Error(), "--include-inline") {
		t.Errorf("only-inline save must be not-found with a hint: %v", err)
	}
	if got := saved(t.TempDir(), q2, "--include-inline"); got != "banner.gif" {
		t.Errorf("q2 --include-inline = %s", got)
	}
}

func TestTBInlineReadTimeFallback(t *testing.T) {
	s := tbSetupInline(t)
	if _, err := cliutil.SetHomeOverride(s.home); err != nil {
		t.Fatal(err)
	}
	path := defaultDBPath(tbCLIName)
	_, _ = cliutil.SetHomeOverride("")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE resources SET data = json_remove(data,'$.inline') WHERE resource_type = 'attachments'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	q1 := tbID("INBOX", "q1@example.com")
	out, _, err := tbRun(t, s.home, "attachments", "list", q1, "--json")
	if got := tbAttNames(tbDecode[[]tbAttachmentDoc](t, out), func(a tbAttachmentDoc) string { return a.Filename }); err != nil || got != "offerta.pdf,photo.jpg" {
		t.Errorf("fallback list = %s (%v)", got, err)
	}
	out, _, _ = tbRun(t, s.home, "largest", "--attachments", "--limit", "0", "--json")
	got := tbAttNames(tbDecode[[]tbLargestAttachmentRow](t, out), func(r tbLargestAttachmentRow) string { return r.Filename })
	if strings.Contains(got, "logo.png") || !strings.Contains(got, "photo.jpg") || !strings.Contains(got, "banner.gif") {
		t.Errorf("fallback largest = %s", got)
	}
	out, _, _ = tbRun(t, s.home, "largest", "--attachments", "--limit", "1", "--json")
	if rows := tbDecode[[]tbLargestAttachmentRow](t, out); len(rows) != 1 || rows[0].Filename != "report.pdf" {
		t.Errorf("fallback largest --limit 1 = %+v", rows)
	}
}

func tbOpenStoreRW(t *testing.T, home string) *store.Store {
	t.Helper()
	if _, err := cliutil.SetHomeOverride(home); err != nil {
		t.Fatal(err)
	}
	path := defaultDBPath(tbCLIName)
	_, _ = cliutil.SetHomeOverride("")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

// tbDowngradeStore rewrites the store the way a pre-inline version left it.
func tbDowngradeStore(t *testing.T, db *store.Store) {
	t.Helper()
	for _, q := range []string{
		`UPDATE resources SET data = json_remove(data,'$.inline') WHERE resource_type = 'attachments'`,
		`UPDATE resources SET data = json_set(data,'$.attachment_count',(SELECT count(*) FROM resources a WHERE a.resource_type = 'attachments' AND json_extract(a.data,'$.message_id') = resources.id),
			'$.has_attachments',json('true')) WHERE resource_type = 'messages' AND json_extract(data,'$.message_id') IN ('q1@example.com','q2@example.com')`,
		`DELETE FROM tb_meta`,
	} {
		if _, err := db.DB().Exec(q); err != nil {
			t.Fatal(err)
		}
	}
}

func tbCheckUpgraded(t *testing.T, db *store.Store) {
	t.Helper()
	msgs := tbMessagesByMsgID(t, db)
	if q1, q2 := msgs["q1@example.com"], msgs["q2@example.com"]; q1.AttachmentCount != 1 || !q1.HasAttachments || q2.AttachmentCount != 0 || q2.HasAttachments {
		t.Errorf("counts after upgrade: q1=%d/%v q2=%d/%v", q1.AttachmentCount, q1.HasAttachments, q2.AttachmentCount, q2.HasAttachments)
	}
	atts, _ := tbLoadDocs[tbAttachmentDoc](db, "attachments")
	for _, a := range atts {
		if a.Inline == nil {
			t.Errorf("attachment %s still lacks inline", a.ID)
		}
	}
}

func TestTBStoreFormatUpgrade(t *testing.T) {
	s := tbSetupInline(t)
	out, _, err := tbRun(t, s.home, "sync", "--json")
	if sum := tbDecode[tbSyncSummary](t, out); err != nil || sum.FormatUpgrade || sum.FoldersScanned != 0 {
		t.Fatalf("fresh store must be current and incremental: %v %+v", err, sum)
	}
	db := tbOpenStoreRW(t, s.home)
	tbDowngradeStore(t, db)
	db.Close()

	out, errOut, err := tbRun(t, s.home, "sync", "--json")
	if sum := tbDecode[tbSyncSummary](t, out); err != nil || !sum.FormatUpgrade || sum.FoldersScanned == 0 || !strings.Contains(errOut, "store format upgrade") {
		t.Fatalf("old store must upgrade on plain sync: %v %+v %q", err, sum, errOut)
	}
	db = tbOpenStoreRW(t, s.home)
	tbCheckUpgraded(t, db)
	db.Close()
	out, errOut, _ = tbRun(t, s.home, "sync", "--json")
	if sum := tbDecode[tbSyncSummary](t, out); sum.FormatUpgrade || sum.FoldersScanned != 0 || strings.Contains(errOut, "upgrade") {
		t.Errorf("second sync must be incremental: %+v %q", sum, errOut)
	}

	db = tbOpenStoreRW(t, s.home)
	defer db.Close()
	tbDowngradeStore(t, db)
	if sum := tbSync(t, db, s.profile, tbSyncOptions{MaxNewMessages: 1}); !sum.FormatUpgrade || sum.Capped {
		t.Errorf("capped upgrade must still converge in one run: %+v", sum)
	}
	tbCheckUpgraded(t, db)
	if sum := tbSync(t, db, s.profile, tbSyncOptions{MaxNewMessages: 1}); sum.FormatUpgrade || sum.FoldersScanned != 0 {
		t.Errorf("after capped upgrade: %+v", sum)
	}

	tbDowngradeStore(t, db)
	f, err := os.OpenFile(filepath.Join(s.profile, filepath.FromSlash(tbInboxRel)), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(tbExtraMessages + strings.ReplaceAll(tbExtraMessages, "<x1@", "<x2@")); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if sum := tbSync(t, db, s.profile, tbSyncOptions{MaxNewMessages: 1}); !sum.FormatUpgrade || !sum.Capped {
		t.Fatalf("upgrade with new mail over the cap: %+v", sum)
	}
	for i := 0; i < 3; i++ {
		sum := tbSync(t, db, s.profile, tbSyncOptions{MaxNewMessages: 1})
		if !sum.FormatUpgrade {
			t.Fatalf("run %d: a capped upgrade must be retried until it completes: %+v", i, sum)
		}
		if !sum.Capped {
			break
		}
	}
	tbCheckUpgraded(t, db)
	if sum := tbSync(t, db, s.profile, tbSyncOptions{MaxNewMessages: 1}); sum.FormatUpgrade || sum.Capped {
		t.Errorf("after the capped upgrade converged: %+v", sum)
	}
}

func TestTBStoreFormat2UpgradesForHTMLQuotes(t *testing.T) {
	s := tbSetupInline(t)
	db := tbOpenStoreRW(t, s.home)
	defer db.Close()
	if err := db.SetTBMeta("store_format", "2"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE resources SET data = json_set(data,'$.body_text','stale') WHERE resource_type = 'messages' AND json_extract(data,'$.message_id') = 'q3@example.com'`); err != nil {
		t.Fatal(err)
	}
	if sum := tbSync(t, db, s.profile, tbSyncOptions{}); !sum.FormatUpgrade || sum.UpgradePending {
		t.Fatalf("a format 2 store must re-parse once: %+v", sum)
	}
	if body := tbMessagesByMsgID(t, db)["q3@example.com"].BodyText; !strings.Contains(body, "> > Ci vediamo") {
		t.Errorf("stored HTML body after upgrade = %q", body)
	}
}

// tbGhostEntry lists a mailbox file that vanishes before sync can stat it.
type tbGhostEntry struct{ os.DirEntry }

func (tbGhostEntry) Name() string { return "Ghost" }

func tbGhostFolder(name string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(name)
	if err != nil || filepath.Base(name) != "imap.example.com" {
		return entries, err
	}
	for _, e := range entries {
		if e.Name() == "INBOX" {
			return append(entries, tbGhostEntry{e}), nil
		}
	}
	return entries, nil
}

func TestTBStoreFormatUpgradeUnreadable(t *testing.T) {
	for _, tt := range []struct {
		name     string
		readDir  tbprofile.DirReader
		q1Frozen bool
	}{
		{"account", tbDenyDir("imap.example.com"), true},
		{"subtree", tbDenyDir("Archives.sbd"), false},
		{"folder", tbGhostFolder, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tbStoreFormatUpgradeUnreadable(t, tt.readDir, tt.q1Frozen)
		})
	}
}

func tbStoreFormatUpgradeUnreadable(t *testing.T, readDir tbprofile.DirReader, q1Frozen bool) {
	s := tbSetupInline(t)
	db := tbOpenStoreRW(t, s.home)
	defer db.Close()
	tbDowngradeStore(t, db)
	sum := tbSync(t, db, s.profile, tbSyncOptions{ReadDir: readDir})
	if !sum.FormatUpgrade || !sum.UpgradePending || len(sum.Warnings) == 0 {
		t.Fatalf("upgrade with unreadable mail must stay pending: %+v", sum)
	}
	if v, ok, _ := db.GetTBMeta("store_format"); ok {
		t.Fatalf("store format recorded as %s although some mail was not re-parsed", v)
	}
	if q1 := tbMessagesByMsgID(t, db)["q1@example.com"]; q1Frozen && q1.AttachmentCount != 3 {
		t.Fatalf("unreadable account rows must be kept as they were: %+v", q1)
	}
	sum = tbSync(t, db, s.profile, tbSyncOptions{})
	if !sum.FormatUpgrade || sum.UpgradePending {
		t.Fatalf("readable again: the upgrade must run and complete: %+v", sum)
	}
	tbCheckUpgraded(t, db)
	if v, _, _ := db.GetTBMeta("store_format"); v != strconv.Itoa(tbStoreFormat) {
		t.Errorf("store format = %q", v)
	}
	if sum := tbSync(t, db, s.profile, tbSyncOptions{}); sum.FormatUpgrade || sum.FoldersScanned != 0 {
		t.Errorf("after the upgrade: %+v", sum)
	}
}

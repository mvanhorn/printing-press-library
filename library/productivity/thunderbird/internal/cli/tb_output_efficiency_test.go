package cli

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
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

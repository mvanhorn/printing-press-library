package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func tbTouchLater(t *testing.T, path string) {
	t.Helper()
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
}

func tbAppend(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func tbMsgBlock(id, extra, body string) string {
	return "From - Tue Jan 14 17:00:00 2025\nX-Mozilla-Status: 0000\nMessage-ID: <" + id + ">\n" + extra +
		"From: new@example.com\nSubject: " + id + "\n\n" + body + "\n\n"
}

// tbCompactInbox removes m7 and appends filler so the size ends equal to or
// larger than before.
func tbCompactInbox(t *testing.T, profile string, grow bool) {
	t.Helper()
	raw, _ := os.ReadFile(tbInbox(profile))
	s := string(raw)
	start := strings.Index(s, "From - Sun Jan 12")
	end := strings.Index(s, "From - Mon Jan 13")
	removed := end - start
	filler := tbMsgBlock("filler@example.com", "", "")
	pad := removed - len(filler)
	if grow {
		pad += 500
	}
	if pad < 0 {
		t.Fatalf("m7 block too small for filler: %d", removed)
	}
	filler = tbMsgBlock("filler@example.com", "", strings.Repeat("x", pad))
	out := s[:start] + s[end:] + filler
	if !grow && len(out) != len(s) {
		t.Fatalf("size %d != %d", len(out), len(s))
	}
	if err := os.WriteFile(tbInbox(profile), []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
	tbTouchLater(t, tbInbox(profile))
}

func TestTBSyncCompactionSameOrLargerSize(t *testing.T) {
	for _, grow := range []bool{false, true} {
		name := "equal"
		if grow {
			name = "larger"
		}
		t.Run(name, func(t *testing.T) {
			_, profile := tbtest.Fixture(t)
			db := tbTestStore(t)
			tbSync(t, db, profile, tbSyncOptions{})
			tbCompactInbox(t, profile, grow)
			sum := tbSync(t, db, profile, tbSyncOptions{})
			msgs := tbMessagesByMsgID(t, db)
			if _, ok := msgs["m7@example.com"]; ok {
				t.Fatal("compacted message still stored")
			}
			if _, ok := msgs["filler@example.com"]; !ok || sum.PrunedMessages != 1 || sum.Resources["messages"] != 9 {
				t.Fatalf("summary = %+v", sum)
			}
			m8 := msgs["m8@example.com"]
			if _, err := tbReadRaw(&m8); err != nil {
				t.Fatalf("m8 offset stale: %v", err)
			}
		})
	}
}

func TestTBSyncMidWriteMessageReRead(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	tbAppend(t, tbInbox(profile), "From - Tue Jan 14 17:00:00 2025\nX-Mozilla-Status: 0000\nMessage-ID: <m9@example.com>\nFrom: new@example.com\nSubject: Partial\n\nfirst half")
	first := tbSync(t, db, profile, tbSyncOptions{})
	if first.NewMessages != 1 || tbMessagesByMsgID(t, db)["m9@example.com"].BodyText != "first half" {
		t.Fatalf("partial run = %+v", first)
	}
	tbAppend(t, tbInbox(profile), " second half\n\n")
	tbTouchLater(t, tbInbox(profile))
	second := tbSync(t, db, profile, tbSyncOptions{})
	m9 := tbMessagesByMsgID(t, db)["m9@example.com"]
	if m9.BodyText != "first half second half" || second.NewMessages != 0 || second.Resources["messages"] != 10 {
		t.Fatalf("completed run = %+v, body %q", second, m9.BodyText)
	}
	third := tbSync(t, db, profile, tbSyncOptions{})
	if third.FoldersScanned != 0 {
		t.Fatalf("unchanged run rescanned: %+v", third)
	}
}

func TestTBSyncReReadIDChangePrunesOld(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	tbAppend(t, tbInbox(profile), "From - Tue Jan 14 17:00:00 2025\nX-Mozilla-Status: 0000\nFrom: new@example.com\n")
	tbSync(t, db, profile, tbSyncOptions{})
	tbAppend(t, tbInbox(profile), "Message-ID: <m9@example.com>\nSubject: Late id\n\nbody\n\n")
	tbTouchLater(t, tbInbox(profile))
	sum := tbSync(t, db, profile, tbSyncOptions{})
	if sum.Resources["messages"] != 10 || tbMessagesByMsgID(t, db)["m9@example.com"].Subject != "Late id" {
		t.Fatalf("run = %+v", sum)
	}
}

type tbCountdownCtx struct {
	context.Context
	n int
}

func (c *tbCountdownCtx) Err() error {
	c.n--
	if c.n < 0 {
		return context.Canceled
	}
	return nil
}

func TestTBSyncCancelledSavesCheckpoint(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	if err := os.RemoveAll(filepath.Join(profile, "ImapMail", "imap.example.com", "Archives.sbd")); err != nil {
		t.Fatal(err)
	}
	before, _, _ := db.GetMboxState(tbInbox(profile))
	tbAppend(t, tbInbox(profile), tbMsgBlock("n1@example.com", "", "one")+tbMsgBlock("n2@example.com", "", "two")+tbMsgBlock("n3@example.com", "", "three"))
	_, err := runTBSync(&tbCountdownCtx{Context: context.Background(), n: 3}, db, profile, tbSyncOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	st, ok, _ := db.GetMboxState(tbInbox(profile))
	if !ok || st.LastOffset <= before.LastOffset || st.MTime != 0 {
		t.Fatalf("checkpoint before=%+v after=%+v", before, st)
	}
	msgs := tbMessagesByMsgID(t, db)
	if _, ok := msgs["n2@example.com"]; !ok {
		t.Fatal("messages read before cancel not stored")
	}
	if _, ok := msgs["a1@example.com"]; !ok {
		t.Fatal("cancelled sync pruned a missing folder")
	}
	sum := tbSync(t, db, profile, tbSyncOptions{})
	msgs = tbMessagesByMsgID(t, db)
	if _, ok := msgs["n3@example.com"]; !ok || sum.NewMessages != 1 {
		t.Fatalf("resumed run = %+v", sum)
	}
	if _, ok := msgs["a1@example.com"]; ok {
		t.Fatal("removed folder not pruned after the resumed run")
	}
}

func TestTBSyncAccountKeyChangePrunesOldRows(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	prefsPath := filepath.Join(profile, "prefs.js")
	b, _ := os.ReadFile(prefsPath)
	if err := os.WriteFile(prefsPath, []byte(strings.ReplaceAll(string(b), "account1", "account9")), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := tbSync(t, db, profile, tbSyncOptions{})
	msgs := tbMessagesByMsgID(t, db)
	if sum.Resources["messages"] != 9 || msgs["m1@example.com"].Account != "account9" {
		t.Fatalf("run = %+v", sum)
	}
	if n, _ := db.Count("attachments"); n != 1 {
		t.Fatalf("attachments = %d", n)
	}
}

func TestTBSyncInReplyToOnlyJoinsRoot(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	tbAppend(t, tbInbox(profile),
		tbMsgBlock("p2@example.com", "In-Reply-To: <p1@example.com>\n", "child")+
			tbMsgBlock("p1@example.com", "In-Reply-To: <m8@example.com>\n", "parent"))
	tbSync(t, db, profile, tbSyncOptions{})
	msgs := tbMessagesByMsgID(t, db)
	root := msgs["m1@example.com"]
	for _, id := range []string{"p1@example.com", "p2@example.com"} {
		if m := msgs[id]; m.ThreadRoot != "m1@example.com" || m.ThreadID != root.ThreadID {
			t.Errorf("%s root=%q thread=%s want %s", id, m.ThreadRoot, m.ThreadID, root.ThreadID)
		}
	}
}

func TestTBSyncReadErrorsDoNotPrune(t *testing.T) {
	t.Run("account dir", func(t *testing.T) {
		_, profile := tbtest.Fixture(t)
		db := tbTestStore(t)
		tbSync(t, db, profile, tbSyncOptions{})
		dir := filepath.Join(profile, "ImapMail", "imap.example.com")
		if err := os.Rename(dir, dir+".moved"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir, []byte("not a dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := tbSync(t, db, profile, tbSyncOptions{})
		if len(sum.Warnings) == 0 || sum.Resources["messages"] != 9 || sum.Resources["folders"] != 7 || sum.PrunedMessages != 0 {
			t.Fatalf("run = %+v", sum)
		}
	})
	t.Run("subtree", func(t *testing.T) {
		_, profile := tbtest.Fixture(t)
		db := tbTestStore(t)
		tbSync(t, db, profile, tbSyncOptions{})
		sum := tbSync(t, db, profile, tbSyncOptions{ReadDir: tbDenyDir("Archives.sbd")})
		if len(sum.Warnings) != 1 || sum.Resources["messages"] != 9 || sum.Resources["folders"] != 7 || sum.PrunedMessages != 0 {
			t.Fatalf("run = %+v", sum)
		}
	})
	t.Run("address book", func(t *testing.T) {
		_, profile := tbtest.Fixture(t)
		db := tbTestStore(t)
		tbSync(t, db, profile, tbSyncOptions{})
		if err := os.WriteFile(filepath.Join(profile, "abook.sqlite"), []byte("garbage, not sqlite"), 0o644); err != nil {
			t.Fatal(err)
		}
		sum := tbSync(t, db, profile, tbSyncOptions{})
		if len(sum.Warnings) == 0 || sum.Resources["contacts"] != 3 {
			t.Fatalf("run = %+v", sum)
		}
	})
	t.Run("filters", func(t *testing.T) {
		_, profile := tbtest.Fixture(t)
		db := tbTestStore(t)
		tbSync(t, db, profile, tbSyncOptions{})
		p := filepath.Join(profile, "ImapMail", "imap.example.com", "msgFilterRules.dat")
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		sum := tbSync(t, db, profile, tbSyncOptions{})
		if len(sum.Warnings) == 0 || sum.Resources["filters"] != 3 {
			t.Fatalf("run = %+v", sum)
		}
	})
}

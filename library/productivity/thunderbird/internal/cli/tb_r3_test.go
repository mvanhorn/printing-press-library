package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

const tbR3Cap = 2

func tbBlocks(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(tbMsgBlock(fmt.Sprintf("%s%d@example.com", prefix, i), "", "body"))
	}
	return b.String()
}

// tbSyncUntilDone runs capped syncs until one is not capped; check runs after every sync.
func tbSyncUntilDone(t *testing.T, run func() *tbSyncSummary, check func(run int)) int {
	t.Helper()
	for i := 1; i <= 6; i++ {
		sum := run()
		check(i)
		if !sum.Capped {
			return i
		}
	}
	t.Fatal("capped sync never converged")
	return 0
}

func TestTBSyncCappedReparseConverges(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	n := 2*tbR3Cap + 1
	if err := os.WriteFile(tbInbox(profile), []byte(tbBlocks("c", n)), 0o644); err != nil {
		t.Fatal(err)
	}
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	compacted := strings.Replace(tbBlocks("c", n), tbMsgBlock("c1@example.com", "", "body"), "", 1) + tbBlocks("d", n)
	if err := os.WriteFile(tbInbox(profile), []byte(compacted), 0o644); err != nil {
		t.Fatal(err)
	}
	tbTouchLater(t, tbInbox(profile))
	sent := filepath.Join(profile, "ImapMail", "imap.example.com", "Posta inviata")
	tbAppend(t, sent, tbMsgBlock("p1@example.com", "", "later folder"))
	tbTouchLater(t, sent)
	runs := tbSyncUntilDone(t, func() *tbSyncSummary {
		return tbSync(t, db, profile, tbSyncOptions{MaxNewMessages: tbR3Cap})
	}, func(int) {})
	if runs > (n+tbR3Cap-1)/tbR3Cap+1 {
		t.Fatalf("converged after %d runs", runs)
	}
	msgs := tbMessagesByMsgID(t, db)
	if _, stale := msgs["c1@example.com"]; stale {
		t.Fatal("compacted message not pruned after the capped re-parse finished")
	}
	for _, id := range []string{"c5@example.com", "d1@example.com", "d5@example.com", "p1@example.com"} {
		if _, ok := msgs[id]; !ok {
			t.Fatalf("%s not ingested", id)
		}
	}
	if st, ok, _ := db.GetMboxState(tbInbox(profile)); !ok || st.MTime == 0 {
		t.Fatalf("inbox state still a checkpoint: %+v", st)
	}
}

func TestTBSyncCappedSpellingChangeKeepsMessages(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	before := tbMessagesByMsgID(t, db)
	old, _ := os.ReadFile(tbInbox(profile))
	if err := os.WriteFile(tbInbox(profile), append([]byte(tbBlocks("e", 2*tbR3Cap+1)), old...), 0o644); err != nil {
		t.Fatal(err)
	}
	tbTouchLater(t, tbInbox(profile))
	alt := tbAltSpelling(t, profile)
	tbSyncUntilDone(t, func() *tbSyncSummary {
		return tbSync(t, db, alt, tbSyncOptions{MaxNewMessages: tbR3Cap})
	}, func(run int) {
		now := tbMessagesByMsgID(t, db)
		for id := range before {
			if _, ok := now[id]; !ok {
				t.Fatalf("run %d deleted live message %s", run, id)
			}
		}
	})
	if _, ok := tbMessagesByMsgID(t, db)["e5@example.com"]; !ok {
		t.Fatal("prepended messages not ingested")
	}
}

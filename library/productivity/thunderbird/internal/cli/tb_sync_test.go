package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func tbTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func tbSync(t *testing.T, db *store.Store, profile string, opts tbSyncOptions) *tbSyncSummary {
	t.Helper()
	sum, err := runTBSync(context.Background(), db, profile, opts)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	return sum
}

func tbMessagesByMsgID(t *testing.T, db *store.Store) map[string]tbMessageDoc {
	t.Helper()
	docs, err := tbLoadDocs[tbMessageDoc](db, "messages")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]tbMessageDoc{}
	for _, d := range docs {
		if _, dup := out[d.MessageID]; dup {
			t.Fatalf("duplicate message %s", d.MessageID)
		}
		out[d.MessageID] = d
	}
	return out
}

func tbInbox(profile string) string {
	return filepath.Join(profile, "ImapMail", "imap.example.com", "INBOX")
}

func TestRunTBSyncFixture(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	sum := tbSync(t, db, profile, tbSyncOptions{})
	want := map[string]int{"accounts": 2, "identities": 2, "folders": 7, "messages": 9, "attachments": 1, "contacts": 3, "filters": 3, "events": 2}
	for r, n := range want {
		if sum.Resources[r] != n {
			t.Errorf("%s = %d, want %d", r, sum.Resources[r], n)
		}
		if _, last, _, _ := db.GetSyncState(r); last.IsZero() {
			t.Errorf("%s has no sync_state", r)
		}
	}
	if sum.NewMessages != 9 || sum.FoldersScanned != 5 {
		t.Errorf("new=%d scanned=%d", sum.NewMessages, sum.FoldersScanned)
	}
	msgs := tbMessagesByMsgID(t, db)
	if _, ok := msgs["m3@example.com"]; ok {
		t.Error("expunged message stored")
	}
	m1, m7, m8, m2, s1 := msgs["m1@example.com"], msgs["m7@example.com"], msgs["m8@example.com"], msgs["m2@example.com"], msgs["s1@example.com"]
	if !m1.Read || !m1.Replied || m1.Flagged || m1.Account != "account1" || m1.AccountName != "bob@example.com" || m1.FolderPath != "INBOX" || m1.Date != "2025-01-06T08:00:00Z" {
		t.Errorf("m1 = %+v", m1)
	}
	if m2.Read || !m2.Flagged || m2.Outgoing {
		t.Errorf("m2 flags = %+v", m2)
	}
	if m1.ThreadID == "" || m7.ThreadID != m1.ThreadID || m8.ThreadID != m1.ThreadID || m8.ThreadRoot != "m1@example.com" {
		t.Errorf("thread ids m1=%s m7=%s m8=%s", m1.ThreadID, m7.ThreadID, m8.ThreadID)
	}
	if !s1.Outgoing || s1.Folder != "Posta inviata" || s1.ThreadID != m2.ThreadID {
		t.Errorf("sent = %+v", s1)
	}
	if a := msgs["a1@example.com"]; a.Folder != "2025" || a.FolderPath != "Archives/2025" || a.FolderKey != "account1:Archives/2025" {
		t.Errorf("subfolder message = %+v", a)
	}
	raw, err := os.ReadFile(m1.MboxPath)
	if err != nil || !strings.HasPrefix(string(raw[m1.Offset:m1.Offset+m1.Length]), "X-Mozilla-Status: 0003") {
		t.Errorf("offset/length do not point at m1")
	}
	m4 := msgs["m4@example.com"]
	if !m4.HasAttachments || m4.AttachmentCount != 1 {
		t.Errorf("m4 = %+v", m4)
	}
	atts, _ := tbLoadDocs[tbAttachmentDoc](db, "attachments")
	if len(atts) != 1 || atts[0].ID != m4.ID+":0" || atts[0].MessageID != m4.ID || atts[0].Filename != "report.pdf" || atts[0].SizeBytes != 23 || atts[0].FolderKey != "account1:INBOX" {
		t.Errorf("attachments = %+v", atts)
	}
	folders, _ := tbLoadDocs[tbFolderDoc](db, "folders")
	byKey := map[string]tbFolderDoc{}
	for _, f := range folders {
		byKey[f.ID] = f
	}
	if f := byKey["account1:INBOX"]; f.Total != 7 || f.Unread != 2 || f.Flagged != 1 || !f.Offline || f.SizeBytes == 0 {
		t.Errorf("INBOX folder = %+v", f)
	}
	if f, ok := byKey["account1:Spam"]; !ok || f.Offline || f.Total != 0 || f.MboxPath != "" {
		t.Errorf("Spam folder = %+v (present %v)", f, ok)
	}
	if f := byKey["account2:Unsent Messages"]; !f.Offline || f.Total != 0 {
		t.Errorf("empty mbox folder = %+v", f)
	}
	var filter map[string]any
	data, _ := db.Get("filters", "account1:1")
	_ = json.Unmarshal(data, &filter)
	if filter["name"] != "Old rule" || filter["enabled"] != false || filter["match_type"] != "AND" || filter["action"] != "Mark read" {
		t.Errorf("filter = %v", filter)
	}
	if conds, _ := filter["conditions"].([]any); len(conds) != 3 {
		t.Errorf("filter conditions = %v", filter["conditions"])
	}
	var contact map[string]any
	data, _ = db.Get("contacts", tbtest.CardAlice)
	_ = json.Unmarshal(data, &contact)
	if contact["display_name"] != "Alice Example" || contact["primary_email"] != "alice@example.com" || contact["company"] != "Example Corp" || contact["nickname"] != "Al" {
		t.Errorf("contact = %v", contact)
	}
	data, _ = db.Get("contacts", tbtest.CardFrank)
	contact = nil
	_ = json.Unmarshal(data, &contact)
	if contact["collected"] != true {
		t.Errorf("history contact = %v", contact)
	}
	var ev map[string]any
	data, _ = db.Get("events", tbtest.EventSync)
	_ = json.Unmarshal(data, &ev)
	if ev["title"] != "Team sync" || ev["start"] != "2025-01-15T09:00:00Z" || ev["location"] != "Room 1" {
		t.Errorf("event = %v", ev)
	}
	hits, err := db.Search("budget", 10, "messages")
	if err != nil || len(hits) < 2 {
		t.Errorf("search budget: %d hits %v", len(hits), err)
	}
}

func TestRunTBSyncIncrementalAppend(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	again := tbSync(t, db, profile, tbSyncOptions{})
	if again.NewMessages != 0 || again.FoldersScanned != 0 || again.FoldersSkipped != 5 {
		t.Fatalf("second run = %+v", again)
	}
	before := tbMessagesByMsgID(t, db)["m1@example.com"].ID
	f, err := os.OpenFile(tbInbox(profile), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("From - Tue Jan 14 17:00:00 2025\nX-Mozilla-Status: 0000\nMessage-ID: <m9@example.com>\nFrom: new@example.com\nSubject: Appended\n\nfresh\n\n")
	f.Close()
	third := tbSync(t, db, profile, tbSyncOptions{})
	if third.NewMessages != 1 || third.FoldersScanned != 1 || third.Resources["messages"] != 10 {
		t.Fatalf("append run = %+v", third)
	}
	msgs := tbMessagesByMsgID(t, db)
	if msgs["m9@example.com"].Subject != "Appended" || msgs["m1@example.com"].ID != before {
		t.Fatal("appended message missing or ids changed")
	}
}

func TestRunTBSyncCompactionPrunes(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	raw, _ := os.ReadFile(tbInbox(profile))
	s := string(raw)
	start := strings.Index(s, "From - Sun Jan 12")
	end := strings.Index(s, "From - Mon Jan 13")
	if err := os.WriteFile(tbInbox(profile), []byte(s[:start]+s[end:]), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := tbSync(t, db, profile, tbSyncOptions{})
	if sum.PrunedMessages != 1 || sum.Resources["messages"] != 8 {
		t.Fatalf("compaction run = %+v", sum)
	}
	msgs := tbMessagesByMsgID(t, db)
	if _, ok := msgs["m7@example.com"]; ok {
		t.Fatal("compacted message still stored")
	}
	if n, _ := db.Count("attachments"); n != 1 {
		t.Fatalf("attachment of surviving message lost: %d", n)
	}
}

func TestRunTBSyncFullReparse(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	sum := tbSync(t, db, profile, tbSyncOptions{Full: true})
	if sum.NewMessages != 9 || sum.FoldersScanned != 5 || sum.PrunedMessages != 0 || sum.Resources["messages"] != 9 {
		t.Fatalf("full run = %+v", sum)
	}
}

func TestRunTBSyncRemovedFolder(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	dir := filepath.Join(profile, "ImapMail", "imap.example.com", "Archives.sbd")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	sum := tbSync(t, db, profile, tbSyncOptions{})
	if sum.PrunedMessages != 1 || sum.Resources["messages"] != 8 || sum.Resources["folders"] != 6 {
		t.Fatalf("removed folder run = %+v", sum)
	}
	states, _ := db.ListMboxStates()
	for _, st := range states {
		if strings.Contains(st.Path, "Archives.sbd") {
			t.Fatal("state of removed mbox kept")
		}
	}
}

func TestRunTBSyncResourceSubset(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	sum := tbSync(t, db, profile, tbSyncOptions{Resources: map[string]bool{"contacts": true}})
	if len(sum.Resources) != 1 || sum.Resources["contacts"] != 3 {
		t.Fatalf("subset = %+v", sum.Resources)
	}
	if n, _ := db.Count("messages"); n != 0 {
		t.Fatalf("messages synced in contacts-only run: %d", n)
	}
}

func TestRunTBSyncCapResumes(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	first := tbSync(t, db, profile, tbSyncOptions{MaxNewMessages: 3})
	if !first.Capped || first.NewMessages != 3 || first.Resources["messages"] != 3 {
		t.Fatalf("capped run = %+v", first)
	}
	second := tbSync(t, db, profile, tbSyncOptions{MaxNewMessages: 3})
	if second.NewMessages != 3 || second.Resources["messages"] != 6 {
		t.Fatalf("second capped run did not resume: %+v", second)
	}
	third := tbSync(t, db, profile, tbSyncOptions{MaxNewMessages: 3})
	if third.Resources["messages"] != 9 {
		t.Fatalf("third capped run = %+v", third)
	}
	final := tbSync(t, db, profile, tbSyncOptions{})
	if final.Capped || final.Resources["messages"] != 9 {
		t.Fatalf("after resumes = %+v", final)
	}
	if len(tbMessagesByMsgID(t, db)) != 9 {
		t.Fatal("messages lost or duplicated across capped runs")
	}
}

func TestParseTBResources(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"", 0, false},
		{"messages, Folders", 2, false},
		{"messages,bogus", 0, true},
	}
	for _, tt := range tests {
		got, err := parseTBResources(tt.in)
		if (err != nil) != tt.wantErr || !tt.wantErr && len(got) != tt.want {
			t.Errorf("parseTBResources(%q) = %v, %v", tt.in, got, err)
		}
	}
}

package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func tbStoredMessages(t *testing.T, dbPath string) []tbMessageDoc {
	t.Helper()
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	docs, err := tbLoadDocs[tbMessageDoc](db, "messages")
	if err != nil {
		t.Fatal(err)
	}
	return docs
}

func TestTBProfilesSyncIntoSeparateStores(t *testing.T) {
	rootA, profA := tbtest.Fixture(t)
	_, profB := tbtest.Fixture(t)
	tbAppendFile(t, filepath.Join(profB, filepath.FromSlash(tbInboxRel)), tbExtraMessages)
	t.Setenv(tbprofile.EnvRoot, rootA)
	t.Setenv(tbprofile.EnvProfile, "")
	home := t.TempDir()
	sync := func(args ...string) tbSyncSummary {
		t.Helper()
		out, _, err := tbRun(t, home, append([]string{"sync", "--json"}, args...)...)
		if err != nil {
			t.Fatalf("sync %v: %v %s", args, err, out)
		}
		return tbDecode[tbSyncSummary](t, out)
	}
	a := sync("--profile", profA)
	b := sync("--profile", profB)
	if a.DBPath == b.DBPath {
		t.Fatalf("both profiles synced into %s", a.DBPath)
	}
	if b.Resources["messages"] <= a.Resources["messages"] {
		t.Fatalf("profile B must hold more messages: a=%d b=%d", a.Resources["messages"], b.Resources["messages"])
	}
	for name, s := range map[string]struct {
		sum     tbSyncSummary
		profile string
	}{"A": {a, profA}, "B": {b, profB}} {
		docs := tbStoredMessages(t, s.sum.DBPath)
		if len(docs) != s.sum.Resources["messages"] {
			t.Errorf("profile %s store has %d messages, want %d", name, len(docs), s.sum.Resources["messages"])
		}
		for _, d := range docs {
			if !strings.HasPrefix(d.MboxPath, s.profile) {
				t.Errorf("profile %s store holds a message of another profile", name)
				break
			}
		}
	}
	out, _, err := tbRun(t, home, "messages", "list", "--limit", "0", "--json")
	if err != nil || len(tbDecode[[]tbMessageRow](t, out)) != a.Resources["messages"] {
		t.Errorf("auto-resolved profile must read profile A's store: %v", err)
	}
	if _, err := cliutil.SetHomeOverride(home); err != nil {
		t.Fatal(err)
	}
	got := defaultDBPath(tbCLIName)
	_, _ = cliutil.SetHomeOverride("")
	if got != a.DBPath {
		t.Errorf("default store path = %s, want %s", got, a.DBPath)
	}
	out, _, err = tbRun(t, home, "messages", "list", "--limit", "0", "--json", "--profile", profB)
	if err != nil || len(tbDecode[[]tbMessageRow](t, out)) != b.Resources["messages"] {
		t.Errorf("--profile B must read profile B's store: %v", err)
	}
	variant := profA + string(filepath.Separator)
	if runtime.GOOS == "windows" {
		variant = strings.ToUpper(variant)
	}
	if v := sync("--profile", variant, "--resources", "accounts"); v.DBPath != a.DBPath {
		t.Errorf("same profile spelled differently got store %s, want %s", v.DBPath, a.DBPath)
	}
}

func TestTBIncrementalSyncRefreshesInPlaceFlags(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	inbox := tbInbox(profile)
	raw, err := os.ReadFile(inbox)
	if err != nil {
		t.Fatal(err)
	}
	for _, sw := range [][2]string{
		{"X-Mozilla-Status: 0004\nMessage-ID: <m2@", "X-Mozilla-Status: 0005\nMessage-ID: <m2@"},
		{"X-Mozilla-Status: 0001\nMessage-ID: <m4@", "X-Mozilla-Status: 0009\nMessage-ID: <m4@"},
		{"X-Mozilla-Status: 0001\nMessage-ID: <m5@", "X-Mozilla-Status: 0000\nMessage-ID: <m5@"},
		{"X-Mozilla-Status: 0000\nMessage-ID: <m6@", "X-Mozilla-Status: 0001\nMessage-ID: <m6@"},
	} {
		if !bytes.Contains(raw, []byte(sw[0])) {
			t.Fatalf("fixture lacks %q", sw[0])
		}
		raw = bytes.Replace(raw, []byte(sw[0]), []byte(sw[1]), 1)
	}
	info, _ := os.Stat(inbox)
	if err := os.WriteFile(inbox, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	later := info.ModTime().Add(10 * time.Second)
	if err := os.Chtimes(inbox, later, later); err != nil {
		t.Fatal(err)
	}
	sum := tbSync(t, db, profile, tbSyncOptions{})
	if sum.NewMessages != 0 || sum.FlagsUpdated != 3 || sum.FlagFolders != 1 {
		t.Errorf("new=%d flags_updated=%d flag_folders=%d, want an incremental refresh of 3 rows in 1 folder", sum.NewMessages, sum.FlagsUpdated, sum.FlagFolders)
	}
	msgs := tbMessagesByMsgID(t, db)
	if !msgs["m2@example.com"].Read || msgs["m5@example.com"].Read || !msgs["m6@example.com"].Read || !msgs["m2@example.com"].Flagged {
		t.Errorf("flags not refreshed: m2=%+v m5=%v m6=%v", msgs["m2@example.com"], msgs["m5@example.com"].Read, msgs["m6@example.com"].Read)
	}
	if _, ok := msgs["m4@example.com"]; ok {
		t.Error("message expunged in place is still stored")
	}
	if atts, _ := tbLoadDocs[tbAttachmentDoc](db, "attachments"); len(atts) != 0 {
		t.Errorf("attachments of the expunged message remain: %d", len(atts))
	}
	folders, _ := tbLoadDocs[tbFolderDoc](db, "folders")
	for _, f := range folders {
		if f.ID == "account1:INBOX" && (f.Total != 6 || f.Unread != 1 || f.Flagged != 1) {
			t.Errorf("INBOX totals = %d/%d/%d, want 6/1/1", f.Total, f.Unread, f.Flagged)
		}
	}
	if again := tbSync(t, db, profile, tbSyncOptions{}); again.FlagFolders != 0 || again.FoldersScanned != 0 {
		t.Errorf("an untouched folder must not be refreshed: %+v", again)
	}
}

func TestTBWriteNewFileNeverOverwritesWithoutForce(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tbWriteNewFile(p, []byte("new"), false); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("existing file without --force: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "keep" {
		t.Errorf("existing file changed to %q", b)
	}
	if err := tbWriteNewFile(p, []byte("new"), true); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "new" {
		t.Errorf("--force wrote %q", b)
	}
	if err := tbWriteNewFile(dir, []byte("x"), true); err == nil {
		t.Error("--force must not replace a directory")
	}
}

func TestTBWriteNewFileDoesNotFollowSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "outside.txt")
	link := filepath.Join(dir, "att.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := tbWriteNewFile(link, []byte("x"), false); err == nil {
		t.Error("a dangling symlink must be refused without --force")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("write followed the symlink")
	}
	if err := tbWriteNewFile(link, []byte("x"), true); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Error("--force must replace the link with a regular file")
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Error("--force followed the symlink")
	}
}

func TestTBFiltersAuditBccTermUnevaluated(t *testing.T) {
	s := tbSetupC(t)
	tbAppendFile(t, filepath.Join(s.profile, "ImapMail", "imap.example.com", "msgFilterRules.dat"),
		"name=\"Any address\"\nenabled=\"yes\"\ntype=\"17\"\naction=\"Mark read\"\ncondition=\"AND (all addresses,contains,news@)\"\n"+
			"name=\"Any incl bcc\"\nenabled=\"yes\"\ntype=\"17\"\naction=\"Mark read\"\ncondition=\"AND (from\\,to\\,cc\\,or bcc,contains,news@)\"\n")
	if out, _, err := tbRun(t, s.home, "sync", "--json", "--resources", "filters"); err != nil {
		t.Fatalf("sync: %v %s", err, out)
	}
	out, _, err := tbRun(t, s.home, "filters", "audit", "--json", "--days", "30")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, r := range tbDecode[[]tbFilterAuditRow](t, out) {
		if r.Name == "Any address" || r.Name == "Any incl bcc" {
			seen++
			if r.Status != "unevaluated" || r.Hits != nil {
				t.Errorf("%s: status=%s hits=%v, want unevaluated", r.Name, r.Status, r.Hits)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("bcc filters in audit = %d, want 2", seen)
	}
}

package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/mcp/cobratree"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func tbDenyDir(base string) tbprofile.DirReader {
	return func(name string) ([]os.DirEntry, error) {
		if filepath.Base(name) == base {
			return nil, errors.New("access denied")
		}
		return os.ReadDir(name)
	}
}

// tbAltSpelling returns another path to the same profile directory.
func tbAltSpelling(t *testing.T, profile string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return filepath.Join(filepath.Dir(profile), strings.ToUpper(filepath.Base(profile)))
	}
	link := filepath.Join(t.TempDir(), "profile-link")
	if err := os.Symlink(profile, link); err != nil {
		t.Skipf("symlink: %v", err)
	}
	return link
}

func tbCount(t *testing.T, sum *tbSyncSummary, resource string) int {
	t.Helper()
	return sum.Resources[resource]
}

func TestTBSyncPathSpellingChangeKeepsMessages(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	db := tbTestStore(t)
	first := tbSync(t, db, profile, tbSyncOptions{})
	alt := tbSync(t, db, tbAltSpelling(t, profile), tbSyncOptions{})
	if tbCount(t, alt, "messages") != tbCount(t, first, "messages") || tbCount(t, alt, "attachments") != 1 || alt.PrunedMessages != 0 {
		t.Fatalf("alternate spelling: first %+v, alt %+v", first.Resources, alt)
	}
	back := tbSync(t, db, profile, tbSyncOptions{})
	if tbCount(t, back, "messages") != tbCount(t, first, "messages") || back.PrunedMessages != 0 {
		t.Fatalf("original spelling again: %+v", back)
	}
}

func TestTBSyncAccountKeySwapAcrossDirs(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	local := filepath.Join(profile, "Mail", "Local Folders", "INBOX")
	if err := os.WriteFile(local, []byte(tbMsgBlock("l1@example.com", "", "one")+tbMsgBlock("l2@example.com", "", "two")), 0o644); err != nil {
		t.Fatal(err)
	}
	db := tbTestStore(t)
	before := tbSync(t, db, profile, tbSyncOptions{})
	prefsPath := filepath.Join(profile, "prefs.js")
	b, _ := os.ReadFile(prefsPath)
	swapped := strings.NewReplacer("[ProfD]ImapMail/imap.example.com", "[ProfD]Mail/Local Folders", "[ProfD]Mail/Local Folders", "[ProfD]ImapMail/imap.example.com").Replace(string(b))
	if err := os.WriteFile(prefsPath, []byte(swapped), 0o644); err != nil {
		t.Fatal(err)
	}
	after := tbSync(t, db, profile, tbSyncOptions{})
	msgs := tbMessagesByMsgID(t, db)
	if tbCount(t, after, "messages") != tbCount(t, before, "messages") || msgs["l1@example.com"].Account != "account1" || msgs["m1@example.com"].Account != "account2" {
		t.Fatalf("before %+v after %+v l1=%q", before.Resources, after.Resources, msgs["l1@example.com"].Account)
	}
}

func TestTBSyncNonASCIISubtreeProtected(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	dir := filepath.Join(profile, "ImapMail", "imap.example.com")
	if err := os.WriteFile(filepath.Join(dir, "Archivé"), []byte(tbMsgBlock("u1@example.com", "", "one")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Archivé.sbd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Archivé.sbd", "Sub"), []byte(tbMsgBlock("u2@example.com", "", "two")), 0o644); err != nil {
		t.Fatal(err)
	}
	db := tbTestStore(t)
	before := tbSync(t, db, profile, tbSyncOptions{})
	after := tbSync(t, db, profile, tbSyncOptions{ReadDir: tbDenyDir("Archivé.sbd")})
	if len(after.Warnings) != 1 || tbCount(t, after, "folders") != tbCount(t, before, "folders") || tbCount(t, after, "messages") != tbCount(t, before, "messages") {
		t.Fatalf("before %+v after %+v", before.Resources, after)
	}
}

func TestTBSyncCancelledAfterReReadPrunesOldID(t *testing.T) {
	for _, mode := range []string{"cancelled", "capped"} {
		t.Run(mode, func(t *testing.T) {
			_, profile := tbtest.Fixture(t)
			db := tbTestStore(t)
			tbSync(t, db, profile, tbSyncOptions{})
			tbAppend(t, tbInbox(profile), "From - Tue Jan 14 17:00:00 2025\nX-Mozilla-Status: 0000\nFrom: new@example.com\n")
			partial := tbSync(t, db, profile, tbSyncOptions{})
			tbAppend(t, tbInbox(profile), "Message-ID: <m9@example.com>\nSubject: Late id\n\nbody\n\n"+tbMsgBlock("n1@example.com", "", "one")+tbMsgBlock("n2@example.com", "", "two"))
			tbTouchLater(t, tbInbox(profile))
			want := tbCount(t, partial, "messages")
			if mode == "cancelled" {
				if _, err := runTBSync(&tbCountdownCtx{Context: context.Background(), n: 1}, db, profile, tbSyncOptions{}); !errors.Is(err, context.Canceled) {
					t.Fatalf("err = %v", err)
				}
			} else {
				if sum := tbSync(t, db, profile, tbSyncOptions{MaxNewMessages: 1}); !sum.Capped {
					t.Fatalf("not capped: %+v", sum)
				}
			}
			n, _ := db.Count("messages")
			msgs := tbMessagesByMsgID(t, db)
			_, idless := msgs[""]
			if _, ok := msgs["m9@example.com"]; !ok || idless || n != want {
				t.Fatalf("messages = %d, want %d with m9 replacing the id-less row", n, want)
			}
		})
	}
}

// tbEqualInbox writes messages of equal length so a compaction can keep every
// offset a message boundary.
func tbEqualInbox(t *testing.T, profile string, ids ...string) {
	t.Helper()
	var b strings.Builder
	for _, id := range ids {
		b.WriteString(tbMsgBlock(id, "", "body"))
	}
	if err := os.WriteFile(tbInbox(profile), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	tbTouchLater(t, tbInbox(profile))
}

func TestTBSyncPartialReparseForcesReparse(t *testing.T) {
	for _, mode := range []string{"cancelled", "capped"} {
		t.Run(mode, func(t *testing.T) {
			_, profile := tbtest.Fixture(t)
			tbEqualInbox(t, profile, "x1@example.com", "x2@example.com", "x3@example.com")
			db := tbTestStore(t)
			tbSync(t, db, profile, tbSyncOptions{})
			compacted := []string{"x2@example.com", "x3@example.com", "x4@example.com"}
			if mode == "capped" {
				// Stored rows do not count against the cap: x4 fits a cap of 1, x5 hits it, and the x4 checkpoint would re-read cleanly.
				compacted = []string{"x2@example.com", "x3@example.com", "x4@example.com", "x5@example.com"}
			}
			tbEqualInbox(t, profile, compacted...)
			later := time.Now().Add(2 * time.Hour)
			if err := os.Chtimes(tbInbox(profile), later, later); err != nil {
				t.Fatal(err)
			}
			if mode == "cancelled" {
				if _, err := runTBSync(&tbCountdownCtx{Context: context.Background(), n: 2}, db, profile, tbSyncOptions{}); !errors.Is(err, context.Canceled) {
					t.Fatalf("err = %v", err)
				}
			} else if sum := tbSync(t, db, profile, tbSyncOptions{MaxNewMessages: 1}); !sum.Capped {
				t.Fatalf("not capped: %+v", sum)
			}
			tbSync(t, db, profile, tbSyncOptions{})
			msgs := tbMessagesByMsgID(t, db)
			if _, stale := msgs["x1@example.com"]; stale {
				t.Fatal("message removed by compaction survived a partial re-parse")
			}
			if _, ok := msgs["x4@example.com"]; !ok {
				t.Fatal("x4 not ingested")
			}
		})
	}
}

func TestTBSyncExpungedSharedCopyReparses(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	copyBlock := tbMsgBlock("dup@example.com", "", "copy")
	tbEqualInbox(t, profile, "y1@example.com")
	tbAppend(t, tbInbox(profile), copyBlock+tbMsgBlock("y2@example.com", "", "body")+copyBlock)
	db := tbTestStore(t)
	tbSync(t, db, profile, tbSyncOptions{})
	b, _ := os.ReadFile(tbInbox(profile))
	s := string(b)
	i := strings.LastIndex(s, "X-Mozilla-Status: 0000")
	s = s[:i] + "X-Mozilla-Status: 0008" + s[i+len("X-Mozilla-Status: 0000"):]
	if err := os.WriteFile(tbInbox(profile), []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	tbTouchLater(t, tbInbox(profile))
	tbSync(t, db, profile, tbSyncOptions{})
	dup, ok := tbMessagesByMsgID(t, db)["dup@example.com"]
	if !ok {
		t.Fatal("row of the live earlier copy was pruned")
	}
	if _, err := tbReadRaw(&dup); err != nil {
		t.Fatalf("dup points at the expunged copy: %v", err)
	}
	if dup.Offset >= int64(strings.LastIndex(s, "From - ")) {
		t.Fatalf("dup offset %d is the expunged copy", dup.Offset)
	}
}

const tbC1AttachmentMessage = "From - Wed Jan 15 09:00:00 2025\nX-Mozilla-Status: 0000\nMessage-ID: <c1att@example.com>\n" +
	"Date: Wed, 15 Jan 2025 09:00:00 +0000\nFrom: mallory@example.com\nTo: bob@example.com\nSubject: file\n" +
	"MIME-Version: 1.0\nContent-Type: multipart/mixed; boundary=\"E1\"\n\n" +
	"--E1\nContent-Type: text/plain\n\nsee file\n" +
	"--E1\nContent-Type: text/plain\nContent-Disposition: attachment; filename=\"=?utf-8?Q?ev=C2=9Bil.txt?=\"\n\nx\n--E1--\n\n"

func TestTBAttachmentSaveC1Filename(t *testing.T) {
	s := tbSetupB(t, false, false)
	tbAppend(t, filepath.Join(s.profile, filepath.FromSlash(tbInboxRel)), tbC1AttachmentMessage)
	if _, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	id := tbID("INBOX", "c1att@example.com")
	outDir := t.TempDir()
	dry, _, err := tbRun(t, s.home, "attachments", "save", id, "--output", outDir, "--dry-run", "--human-friendly")
	if err != nil || strings.ContainsRune(dry, 0x9b) || !strings.Contains(dry, "would write") {
		t.Fatalf("dry-run: %v %q", err, dry)
	}
	out, _, err := tbRun(t, s.home, "attachments", "save", id, "--output", outDir, "--json")
	if err != nil {
		t.Fatal(err)
	}
	files := tbDecode[[]tbSavedAttachment](t, out)
	if len(files) != 1 || strings.ContainsRune(files[0].Path, 0x9b) || !strings.ContainsRune(files[0].Filename, 0x9b) {
		t.Fatalf("saved = %+v", files)
	}
	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if strings.ContainsRune(e.Name(), 0x9b) {
			t.Fatalf("C1 control character in the name on disk: %q", e.Name())
		}
	}
	human, _, err := tbRun(t, s.home, "attachments", "save", id, "--output", t.TempDir(), "--human-friendly")
	if err != nil || strings.ContainsRune(human, 0x9b) || !strings.Contains(human, "saved ") {
		t.Fatalf("saved line: %v %q", err, human)
	}
}

func TestTBDraftFileFlagsCLIOnly(t *testing.T) {
	root := RootCmd()
	s := server.NewMCPServer("test", "0.0.0")
	cobratree.RegisterAll(s, root, func() (string, error) { return "missing-binary", nil })
	tool, ok := s.ListTools()[cobratree.ToolNameForCommand(s, root, "drafts new")]
	if !ok {
		t.Fatal("no MCP tool for drafts new")
	}
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("local secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"body-file", "attach"} {
		if _, exposed := tool.Tool.InputSchema.Properties[name]; exposed {
			t.Errorf("MCP schema exposes %q", name)
		}
		res, err := tool.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
			Arguments: map[string]any{name: secret, "to": "alice@example.com"},
		}})
		if err != nil || !res.IsError {
			t.Fatalf("MCP accepted %q", name)
		}
		if txt := res.Content[0].(mcplib.TextContent).Text; !strings.Contains(txt, "unknown MCP parameter") {
			t.Errorf("%q: %s", name, txt)
		}
	}
	b := tbSetupB(t, false, false)
	out, _, err := tbRun(t, b.home, "drafts", "new", "--to", "alice@example.com", "--body-file", secret, "--attach", secret, "--json")
	if err != nil {
		t.Fatal(err)
	}
	spec := tbDecode[tbComposeSpec](t, out)
	if spec.Body != "local secret" || len(spec.Attachments) != 1 || spec.Attachments[0] != secret {
		t.Fatalf("CLI draft = %+v", spec)
	}
}

func TestTBHumanOutputSanitizedAtSink(t *testing.T) {
	s := tbSetupB(t, false, false)
	prefsPath := filepath.Join(s.profile, "prefs.js")
	p, _ := os.ReadFile(prefsPath)
	if err := os.WriteFile(prefsPath, []byte(strings.Replace(string(p), `server2.name", "Local Folders"`, "server2.name\", \"Local\u009bFolders\"", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(s.profile, "ImapMail", "imap.example.com", "msgFilterRules.dat")
	r, _ := os.ReadFile(rules)
	if err := os.WriteFile(rules, []byte(strings.Replace(string(r), `name="Newsletters"`, "name=\"News\u009bletters\"", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	ab, err := os.ReadFile(filepath.Join(s.profile, "abook.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.profile, "abook-\u009b.sqlite"), ab, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"accounts"},
		{"folders"},
		{"filters", "list"},
		{"filters", "audit"},
		{"stats"},
		{"contacts", "search", "example"},
	} {
		out, _, err := tbRun(t, s.home, append(args, "--human-friendly")...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if strings.ContainsRune(out, 0x9b) || !strings.Contains(out, "�") {
			t.Errorf("%v: C1 control character reached the terminal: %q", args, out)
		}
	}
	out, _, _ := tbRun(t, s.home, "filters", "list", "--json")
	if !strings.ContainsRune(out, 0x9b) {
		t.Errorf("JSON output must keep the original text: %s", out)
	}
}

func TestTBSafeWriterReportsInputLength(t *testing.T) {
	var b strings.Builder
	n, err := tbSafeWriter{&b}.Write([]byte("a\x1bb"))
	if err != nil || n != 3 || b.String() != "a�b" {
		t.Fatalf("n=%d err=%v out=%q", n, err, b.String())
	}
}

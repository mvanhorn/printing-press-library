package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/mcp/cobratree"
)

func TestTBOutputDirFlagBlockedForMCP(t *testing.T) {
	root := RootCmd()
	s := server.NewMCPServer("test", "0.0.0")
	cobratree.RegisterAll(s, root, func() (string, error) { return "missing-binary", nil })
	tools := s.ListTools()
	for _, path := range []string{"attachments save", "messages export"} {
		c, _, err := root.Find(strings.Fields(path))
		if err != nil {
			t.Fatal(err)
		}
		if f := c.Flags().Lookup("output"); f == nil || f.Shorthand != "o" {
			t.Fatalf("%s: --output/-o missing", path)
		}
		if f := c.Flags().Lookup("out"); f == nil || f.Deprecated == "" {
			t.Fatalf("%s: --out alias missing or not deprecated", path)
		}
		tool, ok := tools[cobratree.ToolNameForCommand(s, root, path)]
		if !ok {
			t.Fatalf("%s: no MCP tool", path)
		}
		for _, name := range []string{"output", "out", "o"} {
			if _, exposed := tool.Tool.InputSchema.Properties[name]; exposed {
				t.Errorf("%s: MCP schema exposes %q", path, name)
			}
			res, err := tool.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
				Arguments: map[string]any{name: t.TempDir(), "args": "0123456789ab"},
			}})
			if err != nil || !res.IsError {
				t.Fatalf("%s: MCP accepted %q", path, name)
			}
			if txt := res.Content[0].(mcplib.TextContent).Text; !strings.Contains(txt, "unknown MCP parameter") {
				t.Errorf("%s %q: %s", path, name, txt)
			}
		}
	}
}

func TestTBSyncAnnotatedLocalWrite(t *testing.T) {
	root := RootCmd()
	for _, path := range []string{"sync", "workflow archive"} {
		c, _, err := root.Find(strings.Fields(path))
		if err != nil {
			t.Fatal(err)
		}
		if c.Annotations["mcp:local-write"] != "true" || c.Annotations["mcp:read-only"] == "true" {
			t.Errorf("%s annotations = %v", path, c.Annotations)
		}
	}
}

const tbHostileMessage = "From - Wed Jan 15 09:00:00 2025\nX-Mozilla-Status: 0000\nMessage-ID: <evil@example.com>\n" +
	"Date: Wed, 15 Jan 2025 09:00:00 +0000\nFrom: =?utf-8?Q?Mallory=C2=9B?= <mallory@example.com>\nTo: bob@example.com\n" +
	"Subject: Hi \x1b[31mRED\x07\nMIME-Version: 1.0\nContent-Type: multipart/mixed; boundary=\"E1\"\n\n" +
	"--E1\nContent-Type: text/plain\n\nbody \x1b]0;pwned\x07 end\n" +
	"--E1\nContent-Type: text/plain; name=\"a\x1b[2J.txt\"\nContent-Disposition: attachment; filename=\"a\x1b[2J.txt\"\n\nx\n--E1--\n\n"

func TestTBHumanOutputSanitized(t *testing.T) {
	s := tbSetupB(t, false, false)
	tbAppend(t, filepath.Join(s.profile, filepath.FromSlash(tbInboxRel)), tbHostileMessage)
	if _, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	id := tbID("INBOX", "evil@example.com")
	for _, args := range [][]string{
		{"messages", "list", "--limit", "0"},
		{"messages", "show", id, "--headers"},
		{"attachments", "list", id},
		{"threads", "show", id},
		{"largest", "--attachments"},
		{"drafts", "reply", id},
		{"search", "pwned"},
	} {
		out, _, err := tbRun(t, s.home, append(args, "--human-friendly")...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		escaped := args[0] == "search" && strings.Contains(out, `\u001b`)
		if strings.ContainsAny(out, "\x1b\x07\u009b") || !escaped && !strings.Contains(out, "�") {
			t.Errorf("%v: control characters reached the terminal: %q", args, out)
		}
	}
	out, _, _ := tbRun(t, s.home, "messages", "show", id, "--json")
	if !strings.Contains(out, `\u001b[31mRED`) {
		t.Errorf("JSON output must keep the original text: %s", out)
	}
}

func TestTBSafe(t *testing.T) {
	in := "a\x00b\x1bc\x7fd\u0085e\u009ff\tg\nh\ré"
	if got := tbSafe(in); got != "a�b�c�d�e�f\tg\nh�é" {
		t.Fatalf("tbSafe = %q", got)
	}
}

func TestTBSaveCollisionNeverReusesAName(t *testing.T) {
	raw := "Subject: x\nContent-Type: multipart/mixed; boundary=B\n\n" +
		"--B\nContent-Disposition: attachment; filename=a.txt\n\none\n" +
		"--B\nContent-Disposition: attachment; filename=a-2.txt\n\ntwo\n" +
		"--B\nContent-Disposition: attachment; filename=a.txt\n\nthree\n--B--\n"
	files, payloads, err := tbExtractForSave(&tbMessageDoc{ID: "m"}, []byte(raw), t.TempDir(), -1)
	if err != nil || len(files) != 3 {
		t.Fatalf("files %+v err %v", files, err)
	}
	seen := map[string]string{}
	for i, f := range files {
		if prev, dup := seen[strings.ToLower(f.Path)]; dup {
			t.Fatalf("%s written twice (%q and %q)", f.Path, prev, payloads[i])
		}
		seen[strings.ToLower(f.Path)] = string(payloads[i])
	}
}

func TestTBSavedFilesArePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	s := tbSetupB(t, false, true)
	m4 := tbID("INBOX", "m4@example.com")
	dir := filepath.Join(t.TempDir(), "att")
	if _, _, err := tbRun(t, s.home, "attachments", "save", m4, "--output", dir, "--json"); err != nil {
		t.Fatal(err)
	}
	exp := filepath.Join(t.TempDir(), "exp")
	if _, _, err := tbRun(t, s.home, "messages", "export", m4, "-o", exp, "--json"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{dir, exp} {
		st, _ := os.Stat(d)
		if st.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s mode %v", d, st.Mode().Perm())
		}
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			info, _ := e.Info()
			if info.Mode().Perm()&0o077 != 0 {
				t.Errorf("%s mode %v", e.Name(), info.Mode().Perm())
			}
		}
	}
}

func TestTBThreadDirectionIsFolderBased(t *testing.T) {
	s := tbSetupB(t, false, false)
	tbAppend(t, filepath.Join(s.profile, filepath.FromSlash(tbInboxRel)),
		"From - Wed Jan 15 09:00:00 2025\nX-Mozilla-Status: 0001\nMessage-ID: <self1@example.com>\nIn-Reply-To: <m1@example.com>\n"+
			"References: <m1@example.com>\nDate: Wed, 15 Jan 2025 09:00:00 +0000\nFrom: bob@example.com\nTo: bob@example.com\nSubject: note to self\n\nnote\n\n")
	if _, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	out, _, err := tbRun(t, s.home, "threads", "show", "m1@example.com", "--json")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range tbDecode[[]tbMessageRow](t, out) {
		if r.MessageID == "self1@example.com" && r.Direction != "in" {
			t.Fatalf("own-address inbox message = %+v", r)
		}
	}
	if strings.Contains(string(out), `"outgoing"`) {
		t.Fatal("address-based outgoing contradicts direction in the same row")
	}
}

func TestTBGetMessagePrefersNonSentCopy(t *testing.T) {
	s := tbSetupB(t, false, false)
	sent := filepath.Join(s.profile, "ImapMail", "imap.example.com", "Posta inviata")
	b, _ := os.ReadFile(sent)
	tbAppend(t, filepath.Join(s.profile, filepath.FromSlash(tbInboxRel)), string(b))
	if _, _, err := tbRun(t, s.home, "sync", "--json"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		out, _, err := tbRun(t, s.home, "messages", "show", "s1@example.com", "--json")
		if det := tbDecode[tbMessageDetail](t, out); err != nil || det.FolderPath != "INBOX" {
			t.Fatalf("show by Message-ID picked %q (%v)", det.FolderPath, err)
		}
	}
}

func TestTBSinceFlagsAndDaysAlias(t *testing.T) {
	s := tbSetupC(t)
	for _, tt := range []struct {
		cmd  []string
		def  string
		days string
	}{
		{[]string{"awaiting-reply"}, "14d", "40"},
		{[]string{"filters", "audit"}, "90d", "40"},
	} {
		c, _, _ := RootCmd().Find(tt.cmd)
		if f := c.Flags().Lookup("since"); f == nil || f.DefValue != tt.def {
			t.Fatalf("%v: --since default", tt.cmd)
		}
		if f := c.Flags().Lookup("days"); f == nil || !f.Hidden {
			t.Fatalf("%v: --days must be a hidden alias", tt.cmd)
		}
		viaSince, _, err1 := tbRun(t, s.home, append(tt.cmd, "--since", tt.days+"d", "--json")...)
		viaDays, _, err2 := tbRun(t, s.home, append(tt.cmd, "--days", tt.days, "--json")...)
		narrow, _, _ := tbRun(t, s.home, append(tt.cmd, "--since", "1d", "--json")...)
		if err1 != nil || err2 != nil || viaSince != viaDays || viaSince == narrow {
			t.Fatalf("%v: since=%q days=%q narrow=%q (%v %v)", tt.cmd, viaSince, viaDays, narrow, err1, err2)
		}
		if _, _, err := tbRun(t, s.home, append(tt.cmd, "--since", "bogus")...); ExitCode(err) != 2 {
			t.Errorf("%v: bad --since exit %v", tt.cmd, err)
		}
		if _, _, err := tbRun(t, s.home, append(tt.cmd, "--days", "0")...); ExitCode(err) != 2 {
			t.Errorf("%v: --days 0 exit %v", tt.cmd, err)
		}
	}
}

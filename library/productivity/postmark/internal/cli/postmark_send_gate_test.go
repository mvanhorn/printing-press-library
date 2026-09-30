package cli

import (
	"strings"
	"testing"
)

func countPosts(f *postmarkFake, path string) int {
	n := 0
	for _, r := range f.log() {
		if r.Method == "POST" && r.Path == path {
			n++
		}
	}
	return n
}

func TestEmailSendPreviewsWithoutSendFlag(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	t.Setenv("POSTMARK_SERVER_TOKEN", "server-token")
	f.reply("POST /email", 200, map[string]any{"ErrorCode": 0, "Message": "OK", "MessageID": "m-1", "To": "jane@example.com"})

	args := []string{"email", "send", "--from", "sender@example.com", "--to", "jane@example.com", "--subject", "Hi", "--text-body", "Hello", "--json"}
	_, stderr, err := postmarkRun(t, args...)
	if err != nil {
		t.Fatalf("preview run error = %v", err)
	}
	if got := countPosts(f, "/email"); got != 0 {
		t.Fatalf("POST /email without --send = %d, want 0", got)
	}
	if !strings.Contains(stderr, "preview only") {
		t.Errorf("stderr missing preview notice: %q", stderr)
	}

	if _, _, err := postmarkRun(t, append(args, "--send")...); err != nil {
		t.Fatalf("send run error = %v", err)
	}
	if got := countPosts(f, "/email"); got != 1 {
		t.Fatalf("POST /email with --send = %d, want 1", got)
	}
}

func TestEmailSendSandboxRunsWithoutSendFlag(t *testing.T) {
	f := newPostmarkFake(t)
	postmarkTestEnv(t, f)
	f.reply("POST /email", 200, map[string]any{"ErrorCode": 0, "Message": "Test job accepted", "MessageID": "t-1", "To": "jane@example.com"})

	args := []string{"email", "send", "--sandbox", "--from", "sender@example.com", "--to", "jane@example.com", "--subject", "Hi", "--text-body", "Hello", "--json"}
	if _, _, err := postmarkRun(t, args...); err != nil {
		t.Fatalf("sandbox run error = %v", err)
	}
	reqs := f.log()
	if countPosts(f, "/email") != 1 {
		t.Fatalf("sandbox POST /email count = %d, want 1", countPosts(f, "/email"))
	}
	for _, r := range reqs {
		if r.Method == "POST" && r.Path == "/email" && r.ServerToken != postmarkSandboxToken {
			t.Errorf("sandbox send used token %q, want %s", r.ServerToken, postmarkSandboxToken)
		}
	}
}

func TestSendGateCoversEveryDeliveringCommand(t *testing.T) {
	root := RootCmd()
	emailCmd, _, err := root.Find([]string{"email"})
	if err != nil {
		t.Fatalf("find email: %v", err)
	}
	for _, name := range postmarkGatedSendCommands {
		sub, _, err := emailCmd.Find([]string{name})
		if err != nil || sub == emailCmd {
			t.Errorf("email %s not found", name)
			continue
		}
		if sub.Flags().Lookup("send") == nil {
			t.Errorf("email %s has no --send gate", name)
		}
	}
}

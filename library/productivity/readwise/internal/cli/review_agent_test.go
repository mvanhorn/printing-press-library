package cli

import (
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewAgentConsent(t *testing.T) {
	testenv.Isolate(t)
	cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected request") })
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	backup := path + ".v4.bak"
	if err := os.WriteFile(backup, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := sliceBExecute(t, cfg, "local-data", "purge", "--agent", "--no-learn")
	if code != 0 {
		t.Fatal(stderr)
	}
	if b, err := os.ReadFile(backup); err != nil || string(b) != "private" {
		t.Fatalf("purge consumed backup: %s %v", b, err)
	}
	_, stderr, code = sliceBExecute(t, cfg, "webhook", "add", "https://example.test/hook", "--agent", "--no-learn")
	if code == 0 || !strings.Contains(stderr, "requires consent") {
		t.Fatalf("missing consent error: %s", stderr)
	}
}

func TestR2AgentHelpDescribesDestructiveConsent(t *testing.T) {
	flag := RootCmd().PersistentFlags().Lookup("agent")
	if !strings.Contains(flag.Usage, "destructive commands still require --yes") {
		t.Fatalf("misleading agent help: %s", flag.Usage)
	}
}

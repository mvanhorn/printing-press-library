package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureRun(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	oldOut := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	cmdErr := func() error {
		// Execute uses os.Stdout through cobra/default helpers.
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()
		os.Args = append([]string{"profilepress"}, args...)
		return Execute()
	}()
	w.Close()
	os.Stdout = oldOut
	_, _ = buf.ReadFrom(r)
	return buf.String(), cmdErr
}

func TestPrintedCLIWorkflow(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "pp.db")
	fixture := filepath.Join(dir, "profile.json")
	job := filepath.Join(dir, "job.txt")
	if err := os.WriteFile(fixture, []byte(`{"section_map":{"headline":"Old headline","about":"Old about"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(job, []byte("Research Engineer role"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureRun(t, "snapshot", "--db", db, "--fixture", fixture)
	if err != nil {
		t.Fatal(err)
	}
	var snap map[string]any
	if err := json.Unmarshal([]byte(out), &snap); err != nil {
		t.Fatal(err)
	}

	out, err = captureRun(t, "propose-for-job", "--db", db, "--snapshot", snap["snapshot_id"].(string), "--job-file", job, "--change", "headline=AI Evaluation and Privacy Research Leader", "--source-note", "resume")
	if err != nil {
		t.Fatal(err)
	}
	var pkt map[string]any
	if err := json.Unmarshal([]byte(out), &pkt); err != nil {
		t.Fatal(err)
	}

	out, err = captureRun(t, "diff", "--db", db, "--packet", pkt["packet_id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "- Old headline") || !strings.Contains(out, "+ AI Evaluation and Privacy Research Leader") {
		t.Fatalf("bad diff: %s", out)
	}

	_, err = captureRun(t, "apply-packet", "--db", db, "--packet", pkt["packet_id"].(string), "--privacy-status", "unknown", "--dry-run")
	if err == nil {
		t.Fatal("unknown privacy should block")
	}

	out, err = captureRun(t, "apply-packet", "--db", db, "--packet", pkt["packet_id"].(string), "--privacy-status", "disabled", "--dry-run", "--confirm-sensitive", "APPLY-SENSITIVE")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "dry-run-passed") || !strings.Contains(out, "network-notify-disabled-default") {
		t.Fatalf("bad apply: %s", out)
	}

	out, err = captureRun(t, "apply-packet", "--db", db, "--packet", pkt["packet_id"].(string), "--privacy-status", "disabled", "--simulate-live", "--confirm-sensitive", "APPLY-SENSITIVE", "--confirm-apply", "APPLY")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "simulated-apply-passed") || strings.Contains(out, `"result": "applied"`) {
		t.Fatalf("simulate-live apply should not look real: %s", out)
	}
}

func TestMessagesDraftSendRequiresExplicitConfirmation(t *testing.T) {
	dir := t.TempDir()
	db := filepath.Join(dir, "pp.db")
	body := filepath.Join(dir, "body.txt")
	if err := os.WriteFile(body, []byte("Great to reconnect."), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := captureRun(t, "messages", "draft", "--db", db, "--to", "https://www.linkedin.com/in/example", "--body-file", body, "--source-note", "test")
	if err != nil {
		t.Fatal(err)
	}
	var draft map[string]any
	if err := json.Unmarshal([]byte(out), &draft); err != nil {
		t.Fatal(err)
	}
	if draft["status"] != "draft" {
		t.Fatalf("bad draft: %s", out)
	}

	_, err = captureRun(t, "messages", "send", "--db", db, "--draft", draft["draft_id"].(string))
	if err == nil {
		t.Fatal("send without confirmation should fail")
	}

	out, err = captureRun(t, "messages", "send", "--db", db, "--draft", draft["draft_id"].(string), "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "dry-run-passed") {
		t.Fatalf("bad dry-run send: %s", out)
	}

	out, err = captureRun(t, "messages", "send", "--db", db, "--draft", draft["draft_id"].(string), "--confirm-send", "SEND-MESSAGE", "--simulate-live")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sent") || !strings.Contains(out, "simulate-live") {
		t.Fatalf("bad send: %s", out)
	}
}

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVoiceNoLocalDataBlocksCollectionBeforeIO(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOOGLE_VOICE_NO_LOCAL_DATA", "1")
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"mirror", "sync", "--home", home, "--json"})
	e := root.Execute()
	if e == nil || !strings.Contains(e.Error(), "disabled") {
		t.Fatalf("collection was not blocked: %v", e)
	}
	if _, e = os.Stat(filepath.Join(home, "config", "voice-profiles")); !os.IsNotExist(e) {
		t.Fatal("blocked sync accessed private profile state")
	}
}
func TestVoiceDoctorFailOnWarnKeepsOneJSONDocument(t *testing.T) {
	t.Setenv("GOOGLE_VOICE_CONFIG_DIR", filepath.Join(t.TempDir(), "missing-config"))
	t.Setenv("GOOGLE_VOICE_DATA_DIR", t.TempDir())
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"doctor", "--fail-on", "warn", "--json"})
	e := root.Execute()
	if e == nil {
		t.Fatal("doctor warnings did not fail")
	}
	var payload map[string]any
	if json.Unmarshal(out.Bytes(), &payload) != nil {
		t.Fatalf("doctor output is not one JSON document: %q", out.String())
	}
	if payload["session_configured"] != false || len(payload["warnings"].([]any)) == 0 {
		t.Fatal("doctor hid missing session")
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/voice"
	"github.com/spf13/cobra"
	"os/exec"
)

func TestVoiceLiteralSearch(t *testing.T) {
	t.Setenv("GOOGLE_VOICE_CONFIG_DIR", t.TempDir())
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "messages.sqlite3")
	m, err := voice.OpenMirror(path, "account-id", true)
	if err != nil {
		t.Fatal(err)
	}
	message := voice.Message{ID: "m-literal", ThreadID: "t-literal", Text: "appointment?! café"}
	data, _ := json.Marshal(message)
	_, err = m.DB.Exec("INSERT INTO voice_messages(id,thread_id,ts,direction,kind,text,has_attachment,data,first_run,first_seen) VALUES(?,?,0,'inbound','text',?,0,?,1,'now')", message.ID, message.ThreadID, message.Text, string(data))
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
	for _, query := range []string{"point", "?!", "CAFÉ"} {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(io.Discard)
		root.SetArgs([]string{"search", query, "--db", path, "--json", "--no-learn"})
		if err := root.Execute(); err != nil || !strings.Contains(out.String(), "m-literal") {
			t.Fatalf("query %q: %s error=%v", query, out.String(), err)
		}
	}
}

func TestVoiceMCPHomeSurvivesChildLaunch(t *testing.T) {
	t.Setenv("GOOGLE_VOICE_CONFIG_DIR", "")
	t.Setenv("GOOGLE_VOICE_HOME", t.TempDir())
	home := t.TempDir()
	f := &rootFlags{homePath: home}
	var output bytes.Buffer
	c := &cobra.Command{}
	c.SetContext(context.Background())
	c.SetOut(&output)
	exe, err := exec.LookPath("env")
	if err != nil {
		t.Skip("env subprocess probe is unavailable on this platform")
	}
	child := voiceMCPChild(c, exe, f)
	if err := child.Run(); err != nil {
		t.Fatal(err)
	}
	homes := []string{}
	for _, line := range strings.Split(output.String(), "\n") {
		if strings.HasPrefix(line, "GOOGLE_VOICE_HOME=") {
			homes = append(homes, line)
		}
	}
	if len(homes) != 1 || homes[0] != "GOOGLE_VOICE_HOME="+home {
		t.Fatalf("child home lost: %v", homes)
	}
	root := RootCmd()
	output.Reset()
	root.SetOut(&output)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"mcp", "--home", home, "--json", "--no-learn"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(output.Bytes(), &settings); err != nil || settings.Env["GOOGLE_VOICE_HOME"] != home {
		t.Fatalf("connection settings lost home: %q error=%v", output.String(), err)
	}
}

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

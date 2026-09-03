package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOCRPresentationCommandsAreNonOverlapping(t *testing.T) {
	root := RootCmd()
	for _, path := range []string{"observe", "verify", "act", "act click-text", "act press-key"} {
		if _, _, err := root.Find(strings.Fields(path)); err != nil {
			t.Fatalf("expected command %q in root tree: %v", path, err)
		}
	}
	if command, _, err := root.Find([]string{"semantic", "observe"}); err != nil || command == nil {
		t.Fatalf("generic semantic observe compatibility command missing: %v", err)
	}
}

func TestOCRPresentationActionsRequireYesAndObservation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"click needs yes", []string{"act", "click-text", "Advanced", "--observation", "sha256:observation"}, "--yes is required"},
		{"click needs observation", []string{"act", "click-text", "Advanced", "--yes"}, "required flag(s) \"observation\" not set"},
		{"press needs yes", []string{"act", "press-key", "F10", "--observation", "sha256:observation"}, "--yes is required"},
		{"press needs observation", []string{"act", "press-key", "F10", "--yes"}, "required flag(s) \"observation\" not set"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := RootCmd()
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(tc.args)
			err := root.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("args %v error = %v, want %q", tc.args, err, tc.want)
			}
		})
	}
}

func TestOCRPresentationVerifyForwardsExpectedText(t *testing.T) {
	t.Setenv("KVMCTL_OCR_PROTOCOL", "json")
	t.Setenv("KVMCTL_OCR_COMMAND", writePresentationFakeOCR(t, `{"width":100,"height":50,"words":[{"text":"Save Changes","confidence":95,"x":10,"y":10,"width":40,"height":10}]}`))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/streamer/snapshot" {
			t.Fatalf("unexpected request path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("snapshot"))
	}))
	defer srv.Close()
	t.Setenv("KVMCTL_URL", srv.URL)
	t.Setenv("KVMCTL_TOKEN", "test-token")

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"verify", "--expect-text", "Save Changes", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("verify output is not JSON: %v\n%s", err, out.String())
	}
	if got["operation"] != "verify-text" {
		t.Fatalf("operation = %#v, want verify-text", got["operation"])
	}
	evidence := got["evidence"].(map[string]any)
	if evidence["text"] != "Save Changes" {
		t.Fatalf("verify text = %#v, want forwarded expected text", evidence["text"])
	}
}

func writePresentationFakeOCR(t *testing.T, response string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-ocr")
	program := "#!/bin/sh\ncat >/dev/null\nprintf '%s\\n' '" + response + "'\n"
	if err := os.WriteFile(path, []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

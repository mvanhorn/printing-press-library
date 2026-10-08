package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
)

func TestFilesUploadContentDryRunAndSizeGuard(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	content := []byte{9, 8, 7, 6}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"root_info":{"root_namespace_id":"root","home_namespace_id":"home"}}`)
		case "/files/upload":
			if r.Header.Get("Content-Type") != "application/octet-stream" {
				t.Errorf("content type %q", r.Header.Get("Content-Type"))
			}
			var arg struct {
				Path string `json:"path"`
				Mode struct {
					Tag string `json:".tag"`
				} `json:"mode"`
				Autorename     bool `json:"autorename"`
				Mute           bool `json:"mute"`
				StrictConflict bool `json:"strict_conflict"`
			}
			if err := json.Unmarshal([]byte(r.Header.Get("Dropbox-API-Arg")), &arg); err != nil || arg.Path != "/Documents/report.bin" || arg.Mode.Tag != "add" || arg.Autorename || arg.Mute || arg.StrictConflict {
				t.Errorf("arg=%+v err=%v", arg, err)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil || !bytes.Equal(body, content) {
				t.Errorf("body=%v err=%v", body, err)
			}
			io.WriteString(w, `{"id":"id:one","name":"report.bin","path_display":"/Documents/report.bin","rev":"r2","size":4,"content_hash":"h"}`)
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	local := filepath.Join(t.TempDir(), "report.bin")
	if err := os.WriteFile(local, content, 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := runRead(t, "files", "upload", local, "/Documents/report.bin", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got transferMetadata
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Path != "/Documents/report.bin" || got.Rev != "r2" || got.Size != 4 {
		t.Fatalf("result=%+v", got)
	}
	before := requests
	data, err = runRead(t, "files", "upload", filepath.Join(t.TempDir(), "does-not-exist"), "/Documents/x", "--dry-run", "--json")
	if err != nil || requests != before || !bytes.Contains(data, []byte(`"dry_run"`)) {
		t.Fatalf("dry-run: %v requests=%d data=%s", err, requests, data)
	}
	oldLimit := uploadSizeLimit
	uploadSizeLimit = 2
	t.Cleanup(func() { uploadSizeLimit = oldLimit })
	_, err = runRead(t, "files", "upload", local, "/Documents/report.bin", "--json")
	if err == nil || !strings.Contains(err.Error(), "upload sessions are not supported") || requests != before {
		t.Fatalf("size guard: %v requests=%d", err, requests)
	}
}

func TestFilesUploadVerifyNoop(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++; w.WriteHeader(500) }))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	data, err := runRead(t, "files", "upload", filepath.Join(t.TempDir(), "missing"), "/Documents/x", "--json")
	if err != nil || requests != 0 {
		t.Fatalf("verify: %v requests=%d output=%s", err, requests, data)
	}
	var got struct {
		VerifyNoop bool `json:"verify_noop"`
	}
	if err := json.Unmarshal(data, &got); err != nil || !got.VerifyNoop {
		t.Fatalf("output=%s err=%v", data, err)
	}
}

func TestFilesUploadRejectsCredentialSourcesAndOverwriteWithoutYes(t *testing.T) {
	testenv.Isolate(t)
	if newFilesUploadCmd(&rootFlags{}).Annotations["mcp:hidden"] != "true" {
		t.Fatal("upload must be hidden from MCP")
	}
	for _, name := range []string{"config.json", "credentials.toml", "server.pem", "private.key", ".env", ".env.local"} {
		if err := safeUploadSource(filepath.Join(t.TempDir(), name), &rootFlags{}); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
	configDir, err := cliutil.ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(configDir, "ordinary.txt")
	if err := os.WriteFile(secret, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := safeUploadSource(secret, &rootFlags{}); err == nil {
		t.Fatal("accepted config directory file")
	}
	link := filepath.Join(t.TempDir(), "link.txt")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if err := safeUploadSource(link, &rootFlags{}); err == nil {
		t.Fatal("accepted symlink to config directory")
	}
	if _, err := runRead(t, "files", "upload", link, "/Docs/file", "--mode", "overwrite", "--json"); err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("overwrite error=%v", err)
	}
}

func TestFilesUploadDogfoodNoop(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	data, err := runRead(t, "files", "upload", filepath.Join(t.TempDir(), "missing.txt"), "/Docs/file", "--json")
	if err != nil || !bytes.Contains(data, []byte(`"verify_noop": true`)) {
		t.Fatalf("dogfood output=%s err=%v", data, err)
	}
}

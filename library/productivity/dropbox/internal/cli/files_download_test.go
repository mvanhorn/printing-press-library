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

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
)

func TestFilesDownloadContentAndOverwriteGuard(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/users/get_current_account":
			io.WriteString(w, `{"root_info":{"root_namespace_id":"root","home_namespace_id":"home"}}`)
		case "/files/download":
			arg := r.Header.Get("Dropbox-API-Arg")
			for _, ch := range arg {
				if ch > 0x7e {
					t.Errorf("non-ASCII header %q", arg)
				}
			}
			var payload struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(arg), &payload); err != nil || payload.Path != "/Fotos/Ñandú.jpg" {
				t.Errorf("API arg %q: %v", arg, err)
			}
			var root map[string]string
			if err := json.Unmarshal([]byte(r.Header.Get("Dropbox-API-Path-Root")), &root); err != nil || root["root"] != "root" {
				t.Errorf("path root %q: %v", r.Header.Get("Dropbox-API-Path-Root"), err)
			}
			w.Header().Set("Dropbox-API-Result", `{"name":"Ñandú.jpg","path_display":"/Fotos/Ñandú.jpg","rev":"r1","size":5,"content_hash":"hash"}`)
			w.Write([]byte{0, 1, 2, 3, 4})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	output := filepath.Join(t.TempDir(), "image.jpg")
	data, err := runRead(t, "files", "download", "/Fotos/Ñandú.jpg", "--output", output, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got downloadResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Path != "/Fotos/Ñandú.jpg" || got.Rev != "r1" || got.Size != 5 || got.WrittenTo != output {
		t.Fatalf("result=%+v", got)
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, []byte{0, 1, 2, 3, 4}) {
		t.Fatalf("content=%v", content)
	}
	before := requests
	_, err = runRead(t, "files", "download", "/Fotos/Ñandú.jpg", "--output", output, "--json")
	if err == nil {
		t.Fatal("expected overwrite refusal")
	}
	content, err = os.ReadFile(output)
	if err != nil || !bytes.Equal(content, []byte{0, 1, 2, 3, 4}) || requests != before {
		t.Fatalf("file changed: %v %v requests=%d", content, err, requests)
	}
}

func TestFilesDownloadRequiresExplicitOutput(t *testing.T) {
	testenv.Isolate(t)
	cmd := newFilesDownloadCmd(&rootFlags{})
	if !strings.Contains(cmd.Annotations["mcp:write-flags"], "force") {
		t.Fatalf("annotations=%v", cmd.Annotations)
	}
	_, err := runRead(t, "files", "download", "/Docs/report.pdf", "--json")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "--output") {
		t.Fatalf("missing output error=%v", err)
	}
}

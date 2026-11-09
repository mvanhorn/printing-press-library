package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/spf13/cobra"
)

func TestImageUploadSendsPNGAndOptionalFields(t *testing.T) {
	var contents bytes.Buffer
	if err := png.Encode(&contents, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(filename, contents.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		args   []string
		fields map[string][]string
		query  string
	}{
		{"omitted", nil, map[string][]string{}, ""},
		{"explicit false", []string{"--name", "Upload test", "--hidden=false"}, map[string][]string{"name": {"Upload test"}, "hidden": {"false"}}, ""},
		{"explicit true", []string{"--hidden"}, map[string][]string{"hidden": {"true"}}, ""},
		{"sparse fields", []string{"--fields-image", "id,name"}, map[string][]string{}, "fields%5Bimage%5D=id%2Cname"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/api/image-upload" || r.URL.RawQuery != tt.query {
					t.Errorf("request = %s %s, want POST /api/image-upload with query %q", r.Method, r.URL.RequestURI(), tt.query)
				}
				if tt.query != "" && r.URL.Query().Get("fields[image]") != "id,name" {
					t.Errorf("fields[image] = %q, want id,name", r.URL.Query().Get("fields[image]"))
				}
				if r.Header.Get("Authorization") != "Klaviyo-API-Key test-key" || r.Header.Get("revision") != "2026-04-15" {
					t.Errorf("auth or revision header missing: %#v", r.Header)
				}
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Errorf("parse multipart: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				defer r.MultipartForm.RemoveAll()
				if !reflect.DeepEqual(r.MultipartForm.Value, tt.fields) {
					t.Errorf("fields = %#v, want %#v", r.MultipartForm.Value, tt.fields)
				}
				upload, header, err := r.FormFile("file")
				if err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				defer upload.Close()
				got, err := io.ReadAll(upload)
				if err != nil || !bytes.Equal(got, contents.Bytes()) {
					t.Errorf("uploaded bytes differ: %q, %v", got, err)
				}
				if header.Filename != "image.png" || header.Header.Get("Content-Type") != "image/png" {
					t.Errorf("file metadata = %#v", header)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"data":{"id":"image-1","type":"image"}}`)
			}))
			defer server.Close()
			cmd := imageUploadTestCommand(t, server, &rootFlags{asJSON: true})
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetArgs(append([]string{"--file", filename}, tt.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 1 {
				t.Errorf("HTTP calls = %d, want 1", calls.Load())
			}
			var result struct {
				Results struct {
					Data struct{ ID string }
				}
				Meta struct{ Source string }
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Results.Data.ID != "image-1" || result.Meta.Source != "live" {
				t.Errorf("output/provenance = %s, error = %v", output.Bytes(), err)
			}
		})
	}
}

func TestImageUploadFileErrorsBeforeHTTP(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.png")
	directory := t.TempDir()
	unreadable := filepath.Join(t.TempDir(), "unreadable.png")
	if err := os.WriteFile(unreadable, []byte("unreadable"), 0); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"no flag", nil, `required flag "file" not set`},
		{"empty path", []string{"--file", ""}, `required flag "file" not set`},
		{"missing", []string{"--file", missing}, fmt.Sprintf("open image file %q: %s", missing, (&os.PathError{Op: "open", Path: missing, Err: syscall.ENOENT}).Error())},
		{"directory", []string{"--file", directory}, fmt.Sprintf("upload file %q is a directory", directory)},
		{"unreadable", []string{"--file", unreadable}, fmt.Sprintf("open image file %q: %s", unreadable, (&os.PathError{Op: "open", Path: unreadable, Err: syscall.EACCES}).Error())},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "unreadable" && os.Geteuid() == 0 {
				t.Skip("root can read files without read permission")
			}
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()
			cmd := imageUploadTestCommand(t, server, &rootFlags{})
			cmd.SetArgs(append([]string{}, tt.args...))
			if err := cmd.Execute(); err == nil || err.Error() != tt.want {
				t.Errorf("error = %v, want %q", err, tt.want)
			}
			if calls.Load() != 0 {
				t.Errorf("HTTP calls = %d before file validation, want 0", calls.Load())
			}
		})
	}
}

func TestImageUploadDryRunMakesNoHTTPCall(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(filename, []byte("not for transmission"), 0o600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	cmd := imageUploadTestCommand(t, server, &rootFlags{asJSON: true, dryRun: true})
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--file", filename, "--name", "Preview", "--hidden=false", "--fields-image", "id,name"})
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stderrReader.Close()
	defer stderrWriter.Close()
	previousStderr := os.Stderr
	defer func() { os.Stderr = previousStderr }()
	os.Stderr = stderrWriter
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = previousStderr
	if err := stderrWriter.Close(); err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || !strings.Contains(output.String(), `"dry_run": true`) {
		t.Errorf("dry run made %d HTTP calls, output = %s", calls.Load(), output.String())
	}
	if !strings.Contains(string(stderr), "fields%5Bimage%5D=id%2Cname") && !strings.Contains(string(stderr), "fields[image]=id,name") {
		t.Errorf("dry run missing sparse-field query: %s", stderr)
	}
}

func imageUploadTestCommand(t *testing.T, server *httptest.Server, flags *rootFlags) *cobra.Command {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KLAVIYO_BASE_URL", server.URL)
	t.Setenv("KLAVIYO_API_KEY", "test-key")
	previous := registeredPlatformSource
	t.Cleanup(func() { registeredPlatformSource = previous })
	registeredPlatformSource = nil // Wire contract only; tenant gates have separate coverage.
	flags.configPath = filepath.Join(t.TempDir(), "unused.toml")
	cmd := newImageUploadPromotedCmd(flags)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd
}

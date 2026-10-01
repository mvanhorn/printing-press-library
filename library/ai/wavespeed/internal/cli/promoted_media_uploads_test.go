package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
)

func executeMediaUpload(t *testing.T, endpoint string, dryRun bool, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv("WAVESPEED_API_KEY", "synthetic-upload-key")
	t.Setenv("WAVESPEED_BASE_URL", endpoint)
	flags := &rootFlags{configPath: filepath.Join(t.TempDir(), "absent.toml"), asJSON: true, dryRun: dryRun, timeout: time.Second}
	cmd := newMediaUploadsPromotedCmd(flags)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func mediaUploadFixture(t *testing.T) (string, []byte) {
	t.Helper()
	payload := []byte("synthetic media\x00\xff\n")
	name := filepath.Join(t.TempDir(), "sample file.bin")
	if err := os.WriteFile(name, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	return name, payload
}

func TestMediaUploadRequiresExactlyOneFile(t *testing.T) {
	for _, args := range [][]string{nil, {"one.bin", "two.bin"}} {
		t.Run(fmt.Sprint(len(args)), func(t *testing.T) {
			_, _, err := executeMediaUpload(t, "http://127.0.0.1:1", false, args...)
			if err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
				t.Fatalf("expected argument validation, got %v", err)
			}
		})
	}
}

func TestMediaUploadSendsMultipartFile(t *testing.T) {
	name, payload := mediaUploadFixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/api/v3/media/upload/binary" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1024); err != nil {
			t.Errorf("multipart request: %v", err)
			http.Error(w, "expected multipart", http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Error(err)
			return
		}
		defer file.Close()
		got, err := io.ReadAll(file)
		if err != nil || !bytes.Equal(got, payload) || header.Filename != filepath.Base(name) {
			t.Errorf("uploaded file does not match synthetic fixture: %v", err)
		}
		if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.Value) != 0 {
			t.Errorf("unexpected multipart fields")
		}
		w.Header().Set("Content-Type", "application/json")
		// Prevent the existing mutation-store hook from persisting this fixture.
		fmt.Fprint(w, `{"data":{"__pp_verify_synthetic__":true,"download_url":"https://media.invalid/sample.bin"}}`)
	}))
	defer server.Close()
	stdout, stderr, err := executeMediaUpload(t, server.URL+"/api/v3", false, name)
	if err != nil || requests.Load() != 1 {
		t.Fatalf("upload error=%v requests=%d", err, requests.Load())
	}
	if !json.Valid([]byte(stdout)) || !strings.Contains(stdout, "https://media.invalid/sample.bin") {
		t.Fatalf("expected JSON upload result, got %q", stdout)
	}
	if strings.Contains(stdout+stderr, "synthetic-upload-key") {
		t.Fatal("credential appeared in output")
	}
}

func TestMediaUploadRejectsMissingFileAndDirectoryWithoutHTTP(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Error(w, "unexpected HTTP", http.StatusInternalServerError)
	}))
	defer server.Close()
	dir := t.TempDir()
	for _, dryRun := range []bool{false, true} {
		for _, name := range []string{dir, filepath.Join(dir, "missing.bin")} {
			_, _, err := executeMediaUpload(t, server.URL, dryRun, name)
			if err == nil || !strings.Contains(err.Error(), "reading upload file") {
				t.Fatalf("expected local file error, got %v", err)
			}
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("invalid files caused %d requests", requests.Load())
	}
}

func TestMediaUploadDryRunDoesNotSendHTTP(t *testing.T) {
	name, _ := mediaUploadFixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	stdout, stderr, err := executeMediaUpload(t, server.URL, true, name)
	if err != nil || requests.Load() != 0 {
		t.Fatalf("dry-run error=%v requests=%d", err, requests.Load())
	}
	if !json.Valid([]byte(stdout)) || !strings.Contains(stdout, `"dry_run": true`) {
		t.Fatalf("missing dry-run result: %q", stdout)
	}
	if !strings.Contains(stderr, "multipart field file=@"+name) || !strings.Contains(stderr, "no request sent") {
		t.Fatalf("missing multipart diagnostic: %q", stderr)
	}
	if strings.Contains(stdout+stderr, "synthetic-upload-key") {
		t.Fatal("credential appeared in dry-run output")
	}
}

func TestMediaUploadPreservesProviderErrors(t *testing.T) {
	fastUploadBackoff(t)
	name, _ := mediaUploadFixture(t)
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "synthetic provider failure", status)
			}))
			defer server.Close()
			stdout, _, err := executeMediaUpload(t, server.URL, false, name)
			if err == nil || ExitCode(err) == 0 {
				t.Fatalf("provider error lost: %v", err)
			}
			// 401 and 5xx keep the provider status. An exhausted 429 is
			// reported by the generated client's rate-limit path instead.
			var apiError *client.APIError
			if status != http.StatusTooManyRequests && (!errors.As(err, &apiError) || apiError.StatusCode != status) {
				t.Fatalf("provider status lost: %v", err)
			}
			// --json callers get the structured error envelope, never a
			// success body.
			if stdout != "" && (!strings.Contains(stdout, `"error"`) || strings.Contains(stdout, "download_url")) {
				t.Fatalf("provider failure printed success output: %q", stdout)
			}
		})
	}
}

type mediaUploadFailingWriter struct{ err error }

func (w mediaUploadFailingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestMediaUploadPropagatesOutputFailure(t *testing.T) {
	name, _ := mediaUploadFixture(t)
	t.Setenv("WAVESPEED_API_KEY", "synthetic-upload-key")
	t.Setenv("WAVESPEED_BASE_URL", "http://127.0.0.1:1")
	flags := &rootFlags{configPath: filepath.Join(t.TempDir(), "absent.toml"), dryRun: true, asJSON: true}
	cmd := newMediaUploadsPromotedCmd(flags)
	want := errors.New("synthetic output failure")
	cmd.SetOut(mediaUploadFailingWriter{err: want})
	cmd.SetErr(io.Discard)
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs([]string{name})
	if err := cmd.Execute(); !errors.Is(err, want) {
		t.Fatalf("output error lost: %v", err)
	}
}

func TestMediaUploadPropagatesTransportFailure(t *testing.T) {
	fastUploadBackoff(t)
	name, _ := mediaUploadFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	stdout, _, err := executeMediaUpload(t, server.URL, false, name)
	if err == nil || ExitCode(err) == 0 {
		t.Fatalf("transport failure did not fail: %v", err)
	}
	// --json callers get the structured error envelope, never a success body.
	if stdout != "" && (!strings.Contains(stdout, `"error"`) || strings.Contains(stdout, "download_url")) {
		t.Fatalf("transport failure printed non-error output: %q", stdout)
	}
}

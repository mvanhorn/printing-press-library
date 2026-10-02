// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/platform"
)

func TestSTTRetryEscapesOriginalAndReplacementJobIDs(t *testing.T) {
	const originalID = "original/one?x"
	const replacementID = "replacement/two#y"
	var statusCalls, startCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/speech-to-text/job/v1/original%2Fone%3Fx/status":
			statusCalls++
			_, _ = w.Write([]byte(`{"job_state":"failed","failed_files_count":1,"job_details":[{"inputs":[{"file_name":"audio.wav"}],"state":"failed"}]}`))
		case "/speech-to-text/job/v1/replacement%2Ftwo%23y/start":
			startCalls++
			_, _ = w.Write([]byte(`{}`))
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("SARVAM_BASE_URL", server.URL)
	t.Setenv("SARVAM_API_KEY", "sk_test_fixture")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	checkpointPath, err := sttRetryCheckpointPath(originalID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := saveSTTRetryCheckpoint(checkpointPath, sttRetryCheckpoint{
		Version: sttRetryCheckpointVersion, OriginalJobID: originalID,
		ReplacementJobID: replacementID, Files: []string{"audio.wav"},
		UploadedFiles: []string{"audio.wav"},
	}); err != nil {
		t.Fatal(err)
	}
	cmd := RootCmd()
	cmd.SetArgs([]string{"stt-job", "retry", originalID, "--dir", t.TempDir(), "--config", filepath.Join(t.TempDir(), "missing.toml")})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if statusCalls != 1 || startCalls != 1 {
		t.Fatalf("status calls=%d start calls=%d, want one each", statusCalls, startCalls)
	}
}

func TestSTTJobPathSegmentRejectsDotSegmentsAndEncodesReservedCharacters(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{"a/b", "a%2Fb"},
		{"a?b", "a%3Fb"}, {"a#b", "a%23b"},
	} {
		got, err := sttJobPathSegment(tc.id)
		if err != nil || got != tc.want {
			t.Fatalf("job ID %q encoded as %q (error %v), want %q", tc.id, got, err, tc.want)
		}
	}
	if _, err := sttJobPathSegment(" "); err == nil {
		t.Fatal("blank job ID accepted")
	}
	for _, id := range []string{".", ".."} {
		if _, err := sttJobPathSegment(id); err == nil {
			t.Fatalf("dot-only job ID %q accepted", id)
		}
	}
}

func TestSTTRetryPresignedTransportErrorDoesNotLeakURL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "audio.wav"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/speech-to-text/job/v1/old/status":
			_, _ = w.Write([]byte(`{"job_state":"failed","failed_files_count":1,"job_details":[{"inputs":[{"file_name":"audio.wav"}],"state":"failed"}]}`))
		case "/speech-to-text/job/v1":
			_, _ = w.Write([]byte(`{"job_id":"replacement"}`))
		case "/speech-to-text/job/v1/upload-files":
			_, _ = w.Write([]byte(`{"upload_urls":{"audio.wav":{"file_url":"https://127.0.0.1:1/audio?sig=fixture-signed-token"}}}`))
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	t.Setenv("SARVAM_BASE_URL", server.URL)
	t.Setenv("SARVAM_API_KEY", "sk_test_fixture")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	cmd := RootCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"stt-job", "retry", "old", "--dir", dir, "--config", filepath.Join(t.TempDir(), "missing.toml")})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("retry unexpectedly succeeded: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(err.Error(), "presigned upload request") {
		t.Fatalf("retry did not reach signed upload transport: %v", err)
	}
	if strings.Contains(err.Error(), "fixture-signed-token") || strings.Contains(stderr.String(), "fixture-signed-token") {
		t.Fatal("signed upload URL leaked through retry error")
	}
}

func TestSTTRetryMissingInitialInputDoesNotClaimCheckpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/speech-to-text/job/v1/old/status" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"job_state":"failed","failed_files_count":1,"job_details":[{"inputs":[{"file_name":"missing.wav"}],"state":"failed"}]}`))
	}))
	defer server.Close()
	t.Setenv("SARVAM_BASE_URL", server.URL)
	t.Setenv("SARVAM_API_KEY", "sk_test_fixture")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cmd := RootCmd()
	cmd.SetArgs([]string{"stt-job", "retry", "old", "--dir", t.TempDir(), "--config", filepath.Join(t.TempDir(), "missing.toml")})
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing initial input was accepted")
	}
	checkpointPath, err := sttRetryCheckpointPath("old", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(checkpointPath); !os.IsNotExist(err) {
		t.Fatalf("missing input left a checkpoint claim: stat error=%v", err)
	}
}

// TestNovelSttJobRetryHelpWires smoke-tests that the stt-job retry command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSttJobRetryHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"stt-job", "retry", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stt-job retry --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "retry"} {
		if !strings.Contains(help, want) {
			t.Fatalf("stt-job retry --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestPrepareSTTRetryFilesValidatesEveryInputBeforeJobCreation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "present.wav"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSTTRetryFiles(dir, []string{"present.wav", "missing.wav"}); err == nil {
		t.Fatal("prepareSTTRetryFiles() unexpectedly accepted a missing input")
	}
	if err := os.Symlink(filepath.Join(dir, "present.wav"), filepath.Join(dir, "linked.wav")); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSTTRetryFiles(dir, []string{"linked.wav"}); err == nil {
		t.Fatal("prepareSTTRetryFiles() unexpectedly accepted a symlink")
	}

	prepared, err := prepareSTTRetryFiles(dir, []string{"present.wav"})
	if err != nil {
		t.Fatalf("prepareSTTRetryFiles() error = %v", err)
	}
	defer closePreparedSTTRetryFiles(prepared)
	if len(prepared) != 1 || prepared[0].Name != "present.wav" || prepared[0].Size != 5 {
		t.Fatalf("prepareSTTRetryFiles() = %#v", prepared)
	}
}

func TestSTTRetryCheckpointUsesVerifiedProfileState(t *testing.T) {
	profileState := filepath.Join(t.TempDir(), "client-state")
	flags := &rootFlags{platformSession: &platform.Session{Paths: platform.Paths{StateDir: profileState}}}
	checkpointPath, err := sttRetryCheckpointPath("job-1", flags)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(checkpointPath) != profileState {
		t.Fatalf("checkpoint path %q escaped selected client profile", checkpointPath)
	}
	if _, err := sttRetryCheckpointPath("job-1", &rootFlags{platformSession: &platform.Session{}}); err == nil {
		t.Fatal("accepted a verified client profile without a state directory")
	}
}

func TestSTTRetryCheckpointResumesReplacementAndTracksUploads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retry.json")
	files := []string{"one.wav", "two.wav"}

	checkpoint, resumed, err := acquireSTTRetryCheckpoint(path, "old-job", files)
	if err != nil {
		t.Fatalf("acquireSTTRetryCheckpoint() error = %v", err)
	}
	if resumed || checkpoint.ReplacementJobID != "" {
		t.Fatalf("initial checkpoint = %#v, resumed = %v", checkpoint, resumed)
	}
	checkpoint.ReplacementJobID = "replacement-job"
	checkpoint.UploadedFiles = []string{"one.wav"}
	if err := saveSTTRetryCheckpoint(path, checkpoint); err != nil {
		t.Fatalf("saveSTTRetryCheckpoint() error = %v", err)
	}

	got, resumed, err := acquireSTTRetryCheckpoint(path, "old-job", []string{"two.wav", "one.wav"})
	if err != nil {
		t.Fatalf("resuming checkpoint: %v", err)
	}
	if !resumed || got.ReplacementJobID != "replacement-job" || len(got.UploadedFiles) != 1 {
		t.Fatalf("resumed checkpoint = %#v, resumed = %v", got, resumed)
	}
	pending := pendingSTTRetryFileNames(files, got.UploadedFiles)
	if len(pending) != 1 || pending[0] != "two.wav" {
		t.Fatalf("pendingSTTRetryFileNames() = %#v", pending)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "two.wav"), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareSTTRetryFiles(dir, pending)
	if err != nil {
		t.Fatalf("missing already-uploaded input blocked resumption: %v", err)
	}
	closePreparedSTTRetryFiles(prepared)

	if _, _, err := acquireSTTRetryCheckpoint(path, "old-job", []string{"different.wav"}); err == nil {
		t.Fatal("acquireSTTRetryCheckpoint() accepted a different pending file set")
	}
	got.Started = true
	if err := saveSTTRetryCheckpoint(path, got); err != nil {
		t.Fatal(err)
	}
	completed, resumed, err := acquireSTTRetryCheckpoint(path, "old-job", files)
	if err != nil {
		t.Fatalf("reading completed checkpoint: %v", err)
	}
	if !resumed || !completed.Started || completed.ReplacementJobID != "replacement-job" {
		t.Fatalf("completed checkpoint = %#v, resumed = %v", completed, resumed)
	}
}

func TestPreparedSTTRetryFileRejectsChangedSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audio.wav")
	if err := os.WriteFile(path, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareSTTRetryFiles(dir, []string{"audio.wav"})
	if err != nil {
		t.Fatal(err)
	}
	defer closePreparedSTTRetryFiles(prepared)
	if err := validatePreparedSTTRetryFileSize(prepared[0]); err != nil {
		t.Fatalf("unchanged file rejected: %v", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("more")); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validatePreparedSTTRetryFileSize(prepared[0]); err == nil {
		t.Fatal("changed input size was accepted for upload checkpoint")
	}
}

func TestUnsavedSTTRetryIDRequiresManualReconciliation(t *testing.T) {
	err := unsavedSTTRetryIDError("replacement-1", "/state/retry.json", os.ErrPermission)
	for _, want := range []string{"replacement-1", "reconcile", "/state/retry.json"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "resumable") || strings.Contains(err.Error(), "rerun this command") {
		t.Fatalf("error falsely promises automatic resumption: %q", err)
	}
}

func TestSTTRetryCheckpointRefusesUnknownStartOutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retry.json")
	checkpoint, _, err := acquireSTTRetryCheckpoint(path, "old-job", []string{"one.wav"})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint.ReplacementJobID = "replacement-job"
	checkpoint.StartAttempted = true
	if err := saveSTTRetryCheckpoint(path, checkpoint); err != nil {
		t.Fatal(err)
	}
	if _, _, err := acquireSTTRetryCheckpoint(path, "old-job", []string{"one.wav"}); err == nil || !strings.Contains(err.Error(), "refusing to send /start again") {
		t.Fatalf("expected fail-closed unknown start outcome, got %v", err)
	}
}

func TestSTTRetryCheckpointRefusesUnknownInitiationOutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retry.json")
	if _, resumed, err := acquireSTTRetryCheckpoint(path, "old-job", []string{"one.wav"}); err != nil || resumed {
		t.Fatalf("initial acquire resumed=%v err=%v", resumed, err)
	}
	if _, _, err := acquireSTTRetryCheckpoint(path, "old-job", []string{"one.wav"}); err == nil || !strings.Contains(err.Error(), "refusing to create a duplicate") {
		t.Fatalf("second acquire error = %v", err)
	}
}

func TestSTTRetryLeaseSerializesResumedWork(t *testing.T) {
	checkpointPath := filepath.Join(t.TempDir(), "retry.json")
	lease, err := acquireSTTRetryLease(checkpointPath)
	if err != nil {
		t.Fatalf("acquireSTTRetryLease() error = %v", err)
	}
	if _, err := acquireSTTRetryLease(checkpointPath); err == nil || !strings.Contains(err.Error(), "another retry") {
		t.Fatalf("concurrent acquire error = %v", err)
	}
	if err := lease.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, err := os.Stat(checkpointPath + ".lock"); err != nil {
		t.Fatalf("persistent lock file missing after release: %v", err)
	}

	next, err := acquireSTTRetryLease(checkpointPath)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := next.Release(); err != nil {
		t.Fatalf("second Release() error = %v", err)
	}
}

func TestSTTRetryLeaseRecoversWhenOwnerClosesWithoutCleanup(t *testing.T) {
	checkpointPath := filepath.Join(t.TempDir(), "retry.json")
	crashed, err := acquireSTTRetryLease(checkpointPath)
	if err != nil {
		t.Fatalf("initial acquire: %v", err)
	}
	// Closing the descriptor models process teardown where deferred Release
	// does not run. The OS must release ownership even though the file stays.
	if err := crashed.file.Close(); err != nil {
		t.Fatalf("closing simulated crashed owner: %v", err)
	}
	crashed.file = nil

	recovered, err := acquireSTTRetryLease(checkpointPath)
	if err != nil {
		t.Fatalf("acquire after owner teardown: %v", err)
	}
	if err := recovered.Release(); err != nil {
		t.Fatalf("recovered Release() error = %v", err)
	}
}

func TestSafeRetryFilePathRejectsAPIPathTraversal(t *testing.T) {
	dir := t.TempDir()
	got, err := safeRetryFilePath(dir, "recording.wav")
	if err != nil {
		t.Fatalf("safeRetryFilePath() error = %v", err)
	}
	if want := filepath.Join(dir, "recording.wav"); got != want {
		t.Fatalf("safeRetryFilePath() = %q, want %q", got, want)
	}
	for _, name := range []string{"", ".", "..", "../secret", "nested/recording.wav", `nested\recording.wav`, filepath.Join(dir, "absolute.wav")} {
		if _, err := safeRetryFilePath(dir, name); err == nil {
			t.Errorf("safeRetryFilePath(%q) unexpectedly succeeded", name)
		}
	}
}

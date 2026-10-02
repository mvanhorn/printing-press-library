// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSTTJobReportEscapesJobID(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/speech-to-text/job/v1/original%2Fone%3Fx/status" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		calls++
		_, _ = w.Write([]byte(`{"job_state":"completed","total_files":0,"failed_files_count":0}`))
	}))
	defer server.Close()
	t.Setenv("SARVAM_BASE_URL", server.URL)
	t.Setenv("SARVAM_API_KEY", "sk_test_fixture")
	cmd := RootCmd()
	cmd.SetArgs([]string{"stt-job", "report", "original/one?x", "--config", filepath.Join(t.TempDir(), "missing.toml")})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("status calls=%d, want one encoded request", calls)
	}
}

// TestNovelSttJobReportHelpWires smoke-tests that the stt-job report command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSttJobReportHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"stt-job", "report", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("stt-job report --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "report"} {
		if !strings.Contains(help, want) {
			t.Fatalf("stt-job report --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSTTJobStatusUsesNestedInputFiles(t *testing.T) {
	details := []sttJobAPIDetail{
		{
			State:        "API Error",
			ErrorMessage: "codec rejected",
			Inputs: []sttJobFileReference{
				{FileName: "failed.wav", FileID: "input-1"},
			},
			Outputs: []sttJobFileReference{{FileName: "failed.json", FileID: "output-1"}},
		},
		{
			State:  "Success",
			Inputs: []sttJobFileReference{{FileName: "ok.wav", FileID: "input-2"}},
		},
	}

	gotFailed := sttJobInputFileNames(details, true)
	if len(gotFailed) != 1 || gotFailed[0] != "failed.wav" {
		t.Fatalf("failed inputs = %#v, want [failed.wav]", gotFailed)
	}
	gotAll := sttJobInputFileNames(details, false)
	if len(gotAll) != 2 || gotAll[0] != "failed.wav" || gotAll[1] != "ok.wav" {
		t.Fatalf("all inputs = %#v, want [failed.wav ok.wav]", gotAll)
	}
	report := sttJobReportFileDetails(details)
	if len(report) != 2 || report[0].FileName != "failed.wav" || report[0].FileID != "input-1" || report[0].ErrorMessage != "codec rejected" {
		t.Fatalf("report details = %#v", report)
	}
}

func TestSTTJobStatusAcceptsLegacyFlatFileDetails(t *testing.T) {
	details := []sttJobAPIDetail{{FileName: "legacy.wav", FileID: "legacy-1", State: "Failed"}}
	got := sttJobInputFileNames(details, true)
	if len(got) != 1 || got[0] != "legacy.wav" {
		t.Fatalf("legacy failed inputs = %#v, want [legacy.wav]", got)
	}
}

func TestSTTJobStatusMarkerMatchesFailedStatePredicate(t *testing.T) {
	for _, state := range []string{"API Error", "Internal Server Error", "failed", "Failure", " error "} {
		if got := sttJobStatusMarker(state); got != "FAIL" {
			t.Errorf("sttJobStatusMarker(%q) = %q, want FAIL", state, got)
		}
	}
	for _, state := range []string{"Accepted", "Pending", "Running", "Completed", "Success"} {
		if got := sttJobStatusMarker(state); got != "ok" {
			t.Errorf("sttJobStatusMarker(%q) = %q, want ok", state, got)
		}
	}
}

func TestSTTJobReportPreservesZeroSuccessWhileRunning(t *testing.T) {
	running := buildSTTJobReportView("job-1", sttJobStatusPayload{JobState: "running", TotalFiles: 10})
	if running.SuccessfulFiles != 0 {
		t.Fatalf("running success count=%d, want provider zero", running.SuccessfulFiles)
	}
	completed := buildSTTJobReportView("job-1", sttJobStatusPayload{JobState: "completed", TotalFiles: 10, FailedFiles: 2})
	if completed.SuccessfulFiles != 8 {
		t.Fatalf("completed success count=%d, want eight", completed.SuccessfulFiles)
	}
}

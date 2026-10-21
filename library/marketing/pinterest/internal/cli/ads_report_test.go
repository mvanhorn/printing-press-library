// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil/testenv"
)

func TestNormalizeReportRows(t *testing.T) {
	rows, err := normalizeReportRows(json.RawMessage(`{"campaign-1":[{"SPEND":12}],"campaign-2":[{"SPEND":5,"entity_id":"override"}]}`))
	if err != nil || len(rows) != 2 || rows[0]["entity_id"] != "campaign-1" || rows[1]["entity_id"] != "override" {
		t.Fatalf("rows=%#v err=%v", rows, err)
	}
}

func TestAdsReportPollsAndDownloadsWithoutAuth(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PINTEREST_ACCESS_TOKEN", "test-token")
	t.Setenv("PINTEREST_AD_ACCOUNT_ID", "")
	oldNow := nowUTC
	defer func() { nowUTC = oldNow }()
	nowUTC = func() time.Time { return time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC) }
	var polls int
	var baseURL string
	srv := newLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ad_accounts/1234567890123456789/reports":
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"token":"tok","report_status":"IN_PROGRESS"}`))
				return
			}
			polls++
			if polls == 1 {
				_, _ = w.Write([]byte(`{"report_status":"IN_PROGRESS"}`))
			} else {
				_, _ = w.Write([]byte(`{"report_status":"FINISHED","url":"` + baseURL + `/download"}`))
			}
		case "/download":
			if r.Header.Get("Authorization") != "" {
				t.Errorf("download leaked Authorization: %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"campaign-1":[{"SPEND":12}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	baseURL = srv.URL
	t.Setenv("PINTEREST_BASE_URL", srv.URL)
	out, err := runPinterestCmd(t, "", "ads", "report", "--ad-account-id=1234567890123456789", "--poll-interval=10ms", "--json")
	if err != nil {
		t.Fatalf("report: %v %s", err, out)
	}
	var result struct {
		Rows []map[string]any `json:"rows"`
	}
	if err = json.Unmarshal([]byte(out), &result); err != nil || len(result.Rows) != 1 || result.Rows[0]["entity_id"] != "campaign-1" {
		t.Fatalf("report rows %s: %v", out, err)
	}
}

func TestAdsReportMissingAccount(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PINTEREST_ACCESS_TOKEN", "test-token")
	t.Setenv("PINTEREST_AD_ACCOUNT_ID", "")
	var accounts string
	var created string
	srv := newLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/ad_accounts":
			_, _ = w.Write([]byte(accounts))
		case strings.HasSuffix(r.URL.Path, "/reports") && r.Method == http.MethodPost:
			created = r.URL.Path
			_, _ = w.Write([]byte(`{"report_status":"FAILED","token":"tok"}`))
		case strings.HasSuffix(r.URL.Path, "/reports"):
			_, _ = w.Write([]byte(`{"report_status":"FAILED"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Setenv("PINTEREST_BASE_URL", srv.URL)

	accounts = `{"items":[]}`
	_, err := runPinterestCmd(t, "", "ads", "report", "--days=7", "--poll-interval=10ms")
	if err == nil || !strings.Contains(err.Error(), "--ad-account-id") {
		t.Fatalf("no visible ad account should ask for --ad-account-id, got %v", err)
	}

	accounts = `{"items":[{"id":"777"}]}`
	_, _ = runPinterestCmd(t, "", "ads", "report", "--days=7", "--poll-interval=10ms")
	if created != "/ad_accounts/777/reports" {
		t.Fatalf("report should default to the first ad account, created at %q", created)
	}
}

func TestAdsReportFailedStatus(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("PINTEREST_ACCESS_TOKEN", "test-token")
	t.Setenv("PINTEREST_AD_ACCOUNT_ID", "")
	srv := newLoopbackTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"token":"tok"}`))
			return
		}
		_, _ = w.Write([]byte(`{"report_status":"FAILED"}`))
	}))
	t.Setenv("PINTEREST_BASE_URL", srv.URL)
	_, err := runPinterestCmd(t, "", "ads", "report", "--ad-account-id=x", "--poll-interval=10ms")
	if err == nil || !strings.Contains(err.Error(), "FAILED") {
		t.Fatalf("expected FAILED status, got %v", err)
	}
}

// TestNovelAdsReportHelpWires smoke-tests that the ads report command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAdsReportHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"ads", "report", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ads report --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "report"} {
		if !strings.Contains(help, want) {
			t.Fatalf("ads report --help missing %q in output:\n%s", want, help)
		}
	}
}

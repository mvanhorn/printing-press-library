// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
)

// TestNovelOverviewHelpWires smoke-tests that the overview command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelOverviewHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"overview", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("overview --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "overview"} {
		if !strings.Contains(help, want) {
			t.Fatalf("overview --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestOverviewAggregatesAndQuotaFallback(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/Big", "folder", "", "", 0),
		fixtureRow("/Big/one.jpg", "file", "H", "2015-01-01T00:00:00Z", 100),
		fixtureRow("/Big/two.jpg", "file", "H", "2018-01-01T00:00:00Z", 100),
		fixtureRow("/Small/three.txt", "file", "U", "2020-01-01T00:00:00Z", 10),
	)
	failure := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/get_space_usage" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if failure {
			w.WriteHeader(500)
			w.Write([]byte(`{"error_summary":"broken"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"used":210,"allocation":{".tag":"individual","allocated":1000}}`))
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	data, err := runRead(t, "overview", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got overviewResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Quota == nil || got.Quota.Used != 210 || got.Quota.Allocated != 1000 || got.Totals.Files != 3 || len(got.TopFolders) != 2 || got.TopFolders[0].Name != "/Big" || got.Duplicates.Groups != 1 || got.Duplicates.ReclaimableBytes != 100 {
		t.Fatalf("overview %+v", got)
	}
	sum := 0
	for _, y := range got.ByYear {
		sum += y.Files
	}
	if sum != got.Totals.Files {
		t.Fatalf("year sum %d, files %d", sum, got.Totals.Files)
	}
	failure = true
	data, err = runRead(t, "overview", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Quota != nil || !strings.Contains(got.Note, "quota unavailable:") {
		t.Fatalf("fallback %+v", got)
	}
}

func TestOverviewReportsDevDirectoriesSeparately(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/A/node_modules", "folder", "", "", 0),
		fixtureRow("/B/node_modules", "folder", "", "", 0),
		fixtureRow("/C/.git", "folder", "", "", 0),
		fixtureRow("/A/node_modules/x.js", "file", "dev", "", 10),
		fixtureRow("/B/node_modules/x.js", "file", "dev", "", 10),
		fixtureRow("/C/a.js", "file", "normal", "", 5),
		fixtureRow("/D/a.js", "file", "normal", "", 5),
	)
	data, err := runRead(t, "overview", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got overviewResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Duplicates.Groups != 1 || len(got.DevDirs) != 2 || got.DevDirs[0].Kind != "node_modules" || got.DevDirs[0].Files != 2 || got.DevDirs[0].Bytes != 20 || got.DevDirs[0].Folders != 2 || got.DevDirs[1].Kind != ".git" || got.DevDirs[1].Files != 0 || got.DevDirs[1].Folders != 1 || len(got.TopDevFolders) != 3 || got.TopDevFolders[0].Path != "/A/node_modules" || got.TopDevFolders[0].Bytes != 10 || got.TopDevFolders[2].Path != "/C/.git" {
		t.Fatalf("overview dev directories: %+v", got)
	}
}

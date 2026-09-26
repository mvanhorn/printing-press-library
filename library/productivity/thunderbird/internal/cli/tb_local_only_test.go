package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func tbLocalOnlySetup(t *testing.T) string {
	t.Helper()
	root, _ := tbtest.Fixture(t)
	t.Setenv(tbprofile.EnvRoot, root)
	t.Setenv(tbprofile.EnvProfile, "")
	return t.TempDir()
}

func tbJSONLines(t *testing.T, s string) []map[string]any {
	t.Helper()
	out := []map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line != "" {
			out = append(out, tbDecode[map[string]any](t, line))
		}
	}
	return out
}

func TestTBExportLocal(t *testing.T) {
	home := tbLocalOnlySetup(t)

	out, errOut, err := tbRun(t, home, "export", "messages", "--format", "json")
	if err != nil || strings.TrimSpace(out) != "[]" || !strings.Contains(errOut, "sync") {
		t.Fatalf("empty store: %v %q %q", err, out, errOut)
	}
	if _, _, err := tbRun(t, home, "sync", "--json"); err != nil {
		t.Fatal(err)
	}

	out, _, _ = tbRun(t, home, "messages", "list", "--limit", "0", "--json")
	var wantIDs []string
	for _, m := range tbDecode[[]map[string]any](t, out) {
		wantIDs = append(wantIDs, m["id"].(string))
	}
	sort.Strings(wantIDs)

	out, _, err = tbRun(t, home, "export", "messages")
	lines := tbJSONLines(t, out)
	var gotIDs []string
	kickoff := false
	for _, m := range lines {
		gotIDs = append(gotIDs, m["id"].(string))
		if m["subject"] == "Project kickoff" && m["folder"] != nil {
			kickoff = true
		}
	}
	if err != nil || len(wantIDs) != 9 || strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") || !kickoff {
		t.Fatalf("export messages jsonl: %v got %v want %v kickoff=%v", err, gotIDs, wantIDs, kickoff)
	}

	out, _, err = tbRun(t, home, "export", "messages", "--limit", "2")
	if lines := tbJSONLines(t, out); err != nil || len(lines) != 2 || lines[0]["id"] != wantIDs[0] {
		t.Fatalf("--limit 2: %v %d", err, len(lines))
	}

	out, _, err = tbRun(t, home, "export", "messages", "--format", "json", "--limit", "3")
	if arr := tbDecode[[]map[string]any](t, out); err != nil || len(arr) != 3 || arr[2]["id"] != wantIDs[2] {
		t.Fatalf("--format json: %v %q", err, out)
	}

	out, _, err = tbRun(t, home, "export", "messages", wantIDs[4], "--format", "json")
	if one := tbDecode[map[string]any](t, out); err != nil || one["id"] != wantIDs[4] {
		t.Fatalf("single json: %v %q", err, out)
	}
	out, _, err = tbRun(t, home, "export", "messages", wantIDs[4])
	if lines := tbJSONLines(t, out); err != nil || len(lines) != 1 || lines[0]["id"] != wantIDs[4] {
		t.Fatalf("single jsonl: %v %q", err, out)
	}

	out, _, err = tbRun(t, home, "export", "contacts")
	contacts := tbJSONLines(t, out)
	if err != nil || len(contacts) == 0 {
		t.Fatalf("export contacts: %v %q", err, out)
	}
	for _, c := range contacts {
		if _, isMsg := c["message_id"]; isMsg {
			t.Fatalf("contacts export leaked a message doc: %v", c)
		}
	}

	file := filepath.Join(t.TempDir(), "m.jsonl")
	out, errOut, err = tbRun(t, home, "export", "messages", "-o", file)
	data, rerr := os.ReadFile(file)
	if err != nil || rerr != nil || out != "" || len(tbJSONLines(t, string(data))) != 9 || !strings.Contains(errOut, "Exported 9 records") {
		t.Fatalf("--output: %v %v %q %q", err, rerr, out, errOut)
	}

	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"export", "bogus"}, 2},
		{[]string{"export", "messages", "--format", "xml"}, 2},
		{[]string{"export", "messages", "--limit", "-1"}, 2},
		{[]string{"export", "messages", "nosuchid0000"}, 3},
	} {
		out, _, err := tbRun(t, home, tc.args...)
		if err == nil || ExitCode(err) != tc.code || out != "" {
			t.Errorf("%v: want exit %d, got %v (%d) out=%q", tc.args, tc.code, err, ExitCode(err), out)
		}
	}
}

func TestTBWorkflowArchiveLocal(t *testing.T) {
	home := tbLocalOnlySetup(t)
	out, _, err := tbRun(t, home, "workflow", "archive", "--json")
	sum := tbDecode[tbSyncSummary](t, out)
	if err != nil || sum.Resources["messages"] != 9 || sum.Full || sum.Resources["contacts"] == 0 {
		t.Fatalf("archive: %v %+v", err, sum)
	}
	out, _, _ = tbRun(t, home, "export", "messages")
	if n := len(tbJSONLines(t, out)); n != 9 {
		t.Fatalf("store after archive has %d messages", n)
	}
	out, _, err = tbRun(t, home, "workflow", "archive", "--full", "--timeout", "0", "--json")
	if sum := tbDecode[tbSyncSummary](t, out); err != nil || !sum.Full || sum.NewMessages != 9 {
		t.Fatalf("archive --full: %v %+v", err, sum)
	}
	if _, _, err := tbRun(t, home, "workflow", "archive", "--timeout", "-1s"); ExitCode(err) != 2 {
		t.Fatalf("negative timeout: %v", err)
	}
	if out, _, _ := tbRun(t, home, "workflow", "archive", "--help"); strings.Contains(out, "from the API") || strings.Contains(out, "max-pages") || !strings.Contains(out, "sync") {
		t.Fatalf("archive help still mentions the API: %q", out)
	}
}

func TestTBDataSourceLiveRefused(t *testing.T) {
	home := tbLocalOnlySetup(t)
	for _, args := range [][]string{
		{"messages", "list"}, {"search", "kickoff"}, {"export", "messages"}, {"sync"},
		{"workflow", "archive"}, {"folders"}, {"awaiting-reply"},
	} {
		out, _, err := tbRun(t, home, append(args, "--data-source", "live", "--json")...)
		if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "no live source") || out != "" {
			t.Errorf("%v: %v (%d) %q", args, err, ExitCode(err), out)
		}
	}
	if _, err := cliutil.SetHomeOverride(home); err != nil {
		t.Fatal(err)
	}
	err := saveProfileStore(&profileStore{Profiles: map[string]Profile{"livep": {Name: "livep", Values: map[string]string{"data-source": "live"}}}})
	_, _ = cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("THUNDERBIRD_HOME", home)
	if out, _, err := tbRun(t, home, "folders", "--profile", "livep", "--json"); ExitCode(err) != 2 || out != "" {
		t.Errorf("run profile with data-source live: %v %q", err, out)
	}
	if _, err := os.Stat(filepath.Join(home, "data", "data.db")); !os.IsNotExist(err) {
		t.Fatalf("a refused live run touched the store: %v", err)
	}
	if _, _, err := tbRun(t, home, "sync", "--data-source", "local", "--json"); err != nil {
		t.Fatal(err)
	}
	out, errOut, err := tbRun(t, home, "search", "kickoff", "--json")
	if err != nil || !strings.Contains(out, "Project kickoff") || strings.Contains(errOut, "search endpoint") {
		t.Fatalf("search auto: %v %q %q", err, out, errOut)
	}
	var flags rootFlags
	usage := newRootCmd(&flags).PersistentFlags().Lookup("data-source").Usage
	if strings.Contains(usage, "API only") || strings.Contains(usage, "live with local fallback") || !strings.Contains(usage, "sync") {
		t.Fatalf("data-source usage: %q", usage)
	}
}

func TestTBAPICommandHidden(t *testing.T) {
	var flags rootFlags
	root := newRootCmd(&flags)
	api, _, err := root.Find([]string{"api"})
	if err != nil || api.Name() != "api" || !api.Hidden || api.Annotations["mcp:hidden"] != "true" {
		t.Fatalf("api: %v %+v", err, api)
	}
}

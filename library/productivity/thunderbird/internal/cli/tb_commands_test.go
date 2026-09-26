package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func tbRun(t *testing.T, home string, args ...string) (string, string, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append(args, "--home", home, "--no-learn"))
	err := root.Execute()
	_, _ = cliutil.SetHomeOverride("")
	return out.String(), errb.String(), err
}

func tbDecode[T any](t *testing.T, s string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return v
}

func TestTBCommandsWithoutProfileExitZero(t *testing.T) {
	t.Setenv(tbprofile.EnvRoot, t.TempDir())
	t.Setenv(tbprofile.EnvProfile, "")
	home := t.TempDir()
	out, _, err := tbRun(t, home, "sync", "--json")
	if err != nil || !tbDecode[tbSyncSummary](t, out).ProfileNotFound {
		t.Fatalf("sync: %v %q", err, out)
	}
	tests := []struct {
		args []string
		want string
		hint string
	}{
		{[]string{"folders", "--json"}, "[]", "run: thunderbird-pp-cli sync"},
		{[]string{"accounts", "--json"}, "[]", "no Thunderbird profile"},
		{[]string{"profiles", "--json"}, "[]", "no profiles.ini"},
	}
	for _, tt := range tests {
		out, errOut, err := tbRun(t, home, tt.args...)
		if err != nil || strings.TrimSpace(out) != tt.want || !strings.Contains(errOut, tt.hint) {
			t.Errorf("%v: err=%v out=%q stderr=%q", tt.args, err, out, errOut)
		}
	}
	out, _, err = tbRun(t, home, "stats", "--json")
	if rep := tbDecode[tbStatsReport](t, out); err != nil || rep.Overall.Messages != 0 || len(rep.Accounts) != 0 {
		t.Fatalf("stats: %v %q", err, out)
	}
	out, _, err = tbRun(t, home, "doctor", "--json")
	if err != nil || !strings.Contains(out, `"profile":"ERROR`) && !strings.Contains(out, `"profile": "ERROR`) {
		t.Fatalf("doctor: %v %q", err, out)
	}
}

func TestTBCommandsFixtureFlow(t *testing.T) {
	root, profile := tbtest.Fixture(t)
	t.Setenv(tbprofile.EnvRoot, root)
	t.Setenv(tbprofile.EnvProfile, "")
	home := t.TempDir()

	out, _, err := tbRun(t, home, "accounts", "--json")
	accs := tbDecode[[]tbAccountRow](t, out)
	if err != nil || len(accs) != 2 || accs[0].Source != "prefs" || strings.Join(accs[0].IdentityEmails, ",") != "bob@example.com,bob.work@example.com" {
		t.Fatalf("accounts before sync: %v %+v", err, accs)
	}

	out, _, err = tbRun(t, home, "sync", "--json")
	sum := tbDecode[tbSyncSummary](t, out)
	if err != nil || sum.Resources["messages"] != 9 || !strings.EqualFold(filepath.Clean(sum.Profile), filepath.Clean(profile)) {
		t.Fatalf("sync: %v %+v", err, sum)
	}

	out, _, _ = tbRun(t, home, "accounts", "--json")
	if accs = tbDecode[[]tbAccountRow](t, out); len(accs) != 2 || accs[0].Source != "store" || accs[1].Name != "Local Folders" {
		t.Fatalf("accounts after sync: %+v", accs)
	}

	out, _, err = tbRun(t, home, "folders", "--json", "--account", "bob@example.com")
	folders := tbDecode[[]tbFolderDoc](t, out)
	if err != nil || len(folders) != 5 {
		t.Fatalf("folders: %v %+v", err, folders)
	}
	for _, f := range folders {
		if f.Path == "Spam" && (f.Offline || f.Total != 0) || f.Path == "INBOX" && (f.Total != 7 || f.Unread != 2) {
			t.Errorf("folder row %+v", f)
		}
	}
	out, _, _ = tbRun(t, home, "folders", "--json", "--limit", "2")
	if len(tbDecode[[]tbFolderDoc](t, out)) != 2 {
		t.Errorf("--limit ignored: %s", out)
	}

	out, _, err = tbRun(t, home, "stats", "--json")
	rep := tbDecode[tbStatsReport](t, out)
	if err != nil || rep.Overall.Messages != 9 || rep.Overall.Unread != 2 || rep.Overall.Flagged != 1 || rep.Overall.Attachments != 1 ||
		rep.Overall.Folders != 7 || rep.Overall.OfflineFolders != 5 || rep.Overall.LastMessage != "2025-01-13T16:00:00Z" || rep.LastSync == "" {
		t.Fatalf("stats overall: %v %+v", err, rep.Overall)
	}
	if len(rep.Accounts) != 2 || rep.Accounts[0].Account != "account1" || rep.Accounts[0].Messages != 9 || rep.Accounts[1].Messages != 0 || rep.Accounts[1].Folders != 2 {
		t.Fatalf("stats accounts: %+v", rep.Accounts)
	}

	out, _, _ = tbRun(t, home, "profiles", "--json")
	profs := tbDecode[[]tbProfileRow](t, out)
	if len(profs) != 2 || !profs[1].Selected || !profs[1].IsDefault || profs[0].Exists {
		t.Fatalf("profiles: %+v", profs)
	}

	out, _, err = tbRun(t, home, "doctor", "--json")
	doc := tbDecode[map[string]any](t, out)
	if err != nil || doc["profile"] != "ok" || doc["accounts"] != "ok (2 accounts)" ||
		!strings.Contains(doc["folders"].(string), "7 folders, 5 stored offline, 2 server-only") || doc["store"] != "ok (9 messages)" {
		t.Fatalf("doctor: %v %v", err, doc)
	}
	if _, probed := doc["api"]; probed {
		t.Fatal("doctor still probes an API")
	}

	out, _, err = tbRun(t, home, "search", "kickoff", "--json")
	if err != nil || !strings.Contains(out, "Project kickoff") {
		t.Fatalf("framework search: %v %q", err, out)
	}
}

func TestTBProfileSelection(t *testing.T) {
	root, profile := tbtest.Fixture(t)
	home := t.TempDir()
	empty := t.TempDir()
	tests := []struct {
		name    string
		envRoot string
		envProf string
		args    []string
		wantErr string
	}{
		{"--profile directory", empty, "", []string{"--profile", profile}, ""},
		{"--profile name", root, "", []string{"--profile", "default-release"}, ""},
		{"THUNDERBIRD_PROFILE env", empty, profile, nil, ""},
		{"--profile beats env", empty, filepath.Join(empty, "nope"), []string{"--profile", profile}, ""},
		{"unknown --profile", root, "", []string{"--profile", "nope"}, "not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tbprofile.EnvRoot, tt.envRoot)
			t.Setenv(tbprofile.EnvProfile, tt.envProf)
			out, _, err := tbRun(t, home, append([]string{"accounts", "--json"}, tt.args...)...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil || len(tbDecode[[]tbAccountRow](t, out)) != 2 {
				t.Fatalf("err=%v out=%q", err, out)
			}
		})
	}
}

func TestTBSyncDryRunTouchesNothing(t *testing.T) {
	root, _ := tbtest.Fixture(t)
	t.Setenv(tbprofile.EnvRoot, root)
	home := t.TempDir()
	out, _, err := tbRun(t, home, "sync", "--dry-run", "--json")
	if err != nil || !strings.Contains(out, `"dry_run":true`) {
		t.Fatalf("dry-run: %v %q", err, out)
	}
	_ = filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Name() == "data.db" {
			t.Fatalf("dry-run created the store at %s", p)
		}
		return nil
	})
	if out, _, _ := tbRun(t, home, "sync", "--json"); tbDecode[tbSyncSummary](t, out).Resources["messages"] != 9 {
		t.Fatalf("control sync did not run: %q", out)
	}
	found := false
	_ = filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
		found = found || err == nil && info.Name() == "data.db"
		return nil
	})
	if !found {
		t.Fatal("control sync wrote no store under --home; dry-run check is vacuous")
	}
	if _, _, err := tbRun(t, home, "sync", "--resources", "bogus"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("bad --resources exit = %v", err)
	}
}

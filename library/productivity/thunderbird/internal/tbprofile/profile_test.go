package tbprofile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func TestRootDirEnvOverride(t *testing.T) {
	t.Setenv(EnvRoot, "/custom/tb")
	if got := RootDir(); got != "/custom/tb" {
		t.Fatalf("RootDir = %q", got)
	}
	t.Setenv(EnvRoot, "")
	if got := RootDir(); !strings.Contains(strings.ToLower(got), "thunderbird") {
		t.Fatalf("RootDir default %q lacks thunderbird", got)
	}
}

func TestParseINI(t *testing.T) {
	secs, order, err := ParseINI(filepath.Join(tbtest.TestdataRoot(), "profiles.ini"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "Profile1,Profile0,General" {
		t.Fatalf("order = %v", order)
	}
	if secs["Profile0"]["Default"] != "1" || secs["Profile1"]["Name"] != "old" {
		t.Fatalf("sections = %v", secs)
	}
	if _, _, err := ParseINI(filepath.Join(t.TempDir(), "missing.ini")); !os.IsNotExist(err) {
		t.Fatalf("missing file err = %v", err)
	}
}

func TestListProfiles(t *testing.T) {
	root, profile := tbtest.Fixture(t)
	got, err := ListProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d profiles", len(got))
	}
	byName := map[string]ProfileEntry{}
	for _, p := range got {
		byName[p.Name] = p
	}
	tests := []struct {
		name      string
		isDefault bool
		exists    bool
		path      string
	}{
		{"default-release", true, true, profile},
		{"old", false, false, filepath.Join(root, "Profiles", "missing.old")},
	}
	for _, tt := range tests {
		p := byName[tt.name]
		if p.IsDefault != tt.isDefault || p.Exists != tt.exists || !samePath(p.Path, tt.path) {
			t.Errorf("%s = %+v", tt.name, p)
		}
	}
	empty, err := ListProfiles(t.TempDir())
	if err != nil || len(empty) != 0 {
		t.Fatalf("no profiles.ini: %v %v", empty, err)
	}
}

func TestResolve(t *testing.T) {
	root, profile := tbtest.Fixture(t)
	tests := []struct {
		name     string
		selector string
		root     string
		want     string
		wantErr  string
	}{
		{"default from installs.ini", "", root, profile, ""},
		{"by name", "default-release", root, profile, ""},
		{"by dir basename", "abcd1234.default-release", root, profile, ""},
		{"by path", profile, "", profile, ""},
		{"named but missing dir", "old", root, "", "missing directory"},
		{"unknown selector", "nope", root, "", "not found"},
		{"no installation", "", t.TempDir(), "", ErrNoProfile.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.selector, tt.root)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || !samePath(got, tt.want) {
				t.Fatalf("Resolve = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	if _, err := Resolve("", t.TempDir()); !errors.Is(err, ErrNoProfile) {
		t.Fatalf("want ErrNoProfile, got %v", err)
	}
}

func TestResolveRelativePathIsAbsolute(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	t.Chdir(filepath.Dir(profile))
	for _, sel := range []string{filepath.Base(profile), "." + string(filepath.Separator) + filepath.Base(profile), filepath.Join("..", filepath.Base(filepath.Dir(profile)), filepath.Base(profile))} {
		got, err := Resolve(sel, "")
		if err != nil || !filepath.IsAbs(got) || !samePath(got, profile) {
			t.Fatalf("Resolve(%q) = %q, %v; want %q", sel, got, err, profile)
		}
	}
}

func TestBookName(t *testing.T) {
	for in, want := range map[string]string{
		filepath.Join("p", "abook.sqlite"):   "abook",
		filepath.Join("p", "abook-1.sqlite"): "abook-1",
		"history.sqlite":                     "history",
	} {
		if got := BookName(in); got != want {
			t.Errorf("BookName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveInstallsIniWinsOverProfilesIni(t *testing.T) {
	root, _ := tbtest.Fixture(t)
	other := filepath.Join(root, "Profiles", "zzzz.other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "installs.ini"), []byte("[ABC]\nDefault=Profiles/zzzz.other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve("", root)
	if err != nil || !samePath(got, other) {
		t.Fatalf("Resolve = %q, %v; want installs.ini default", got, err)
	}
}

func TestResolveFallsBackToProfilesIniDefault(t *testing.T) {
	root, profile := tbtest.Fixture(t)
	if err := os.Remove(filepath.Join(root, "installs.ini")); err != nil {
		t.Fatal(err)
	}
	got, err := Resolve("", root)
	if err != nil || !samePath(got, profile) {
		t.Fatalf("Resolve = %q, %v", got, err)
	}
}

func TestLockPresent(t *testing.T) {
	dir := t.TempDir()
	if LockPresent(dir) {
		t.Fatal("empty dir reported locked")
	}
	for _, name := range []string{"parent.lock", ".parentlock"} {
		d := t.TempDir()
		if err := os.WriteFile(filepath.Join(d, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if !LockPresent(d) {
			t.Errorf("%s not detected", name)
		}
	}
}

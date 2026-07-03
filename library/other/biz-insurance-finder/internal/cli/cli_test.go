package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/insurance"
)

// run executes the CLI with args and returns stdout. stdout is a buffer (not a
// terminal), so commands emit JSON by default.
func run(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := RootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if stdin != "" {
		cmd.SetIn(bytes.NewReader([]byte(stdin)))
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func writeImporterProfile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	p := insurance.DefaultProfile(time.Now())
	p.LegalName = "Acme Imports LLC"
	p.FormationState = "Delaware"
	p.BusinessAddress = "100 Commerce Way, Springfield, IL 62701"
	p.ContactName = "Jane Doe"
	p.ContactEmail = "owner@example.com"
	p.ContactPhone = "(312) 555-0100"
	p.YearStarted = 2023
	p.RevenueBand = "$100k-$175k"
	p.IndustryClass = "Consumer-goods importer"
	p.Importer, p.PrivateLabel, p.Manufacturer = true, true, true
	p.CountriesOfOrigin = []string{"China"}
	if err := insurance.SaveProfile(path, p); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	return path
}

func TestCLI_MatchRoutesImporterToSpecialty(t *testing.T) {
	path := writeImporterProfile(t)
	out, err := run(t, "", "--profile", path, "match", "--json")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	var recs []insurance.Recommendation
	if err := json.Unmarshal([]byte(out), &recs); err != nil {
		t.Fatalf("unmarshal match output: %v\n%s", err, out)
	}
	if recs[0].Provider.ID != "insurance-canopy" {
		t.Errorf("rank 1 = %q, want insurance-canopy", recs[0].Provider.ID)
	}
	byID := map[string]insurance.Recommendation{}
	for _, r := range recs {
		byID[r.Provider.ID] = r
	}
	if byID["biberk"].Tier != insurance.TierAvoid {
		t.Errorf("biberk tier = %q, want avoid for importer", byID["biberk"].Tier)
	}
}

func TestCLI_IntakeNoInputWritesProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	out, err := run(t, "", "--profile", path, "--no-input", "intake")
	if err != nil {
		t.Fatalf("intake: %v", err)
	}
	var res intakeResult
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("unmarshal intake output: %v\n%s", err, out)
	}
	if !res.Saved {
		t.Errorf("intake should have saved the profile")
	}
	if _, err := insurance.LoadProfile(path); err != nil {
		t.Errorf("profile not written: %v", err)
	}
}

// A profile file that exists but cannot be parsed must NOT be silently
// overwritten with defaults - in --no-input mode that would clobber the user's
// saved data with no warning. intake must error and leave the file untouched.
func TestCLI_IntakeRefusesToClobberUnreadableProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	corrupt := []byte("{ this is not valid json")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatalf("seed corrupt profile: %v", err)
	}

	_, err := run(t, "", "--profile", path, "--no-input", "intake")
	if err == nil {
		t.Fatalf("intake should error on an unreadable existing profile instead of overwriting it")
	}
	if ExitCode(err) != 5 {
		t.Errorf("exit code = %d, want 5 (data error)", ExitCode(err))
	}

	got, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatalf("read back profile: %v", rerr)
	}
	if !bytes.Equal(got, corrupt) {
		t.Errorf("intake overwrote the unreadable profile; file is now: %s", got)
	}
}

func TestCLI_GuideEmitsProvidersAndWarnings(t *testing.T) {
	path := writeImporterProfile(t)
	out, err := run(t, "", "--profile", path, "guide", "--json", "--top", "2")
	if err != nil {
		t.Fatalf("guide: %v", err)
	}
	var g guideOutput
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		t.Fatalf("unmarshal guide output: %v\n%s", err, out)
	}
	if !g.ImporterClass {
		t.Errorf("guide should mark importer class")
	}
	if len(g.Providers) != 2 {
		t.Errorf("guide --top 2 returned %d providers", len(g.Providers))
	}
	if len(g.Providers) > 0 && len(g.Providers[0].AnswerSheet.Fields) == 0 {
		t.Errorf("top provider should carry an answer sheet")
	}
	// The critical foreign-products warning must be present for an importer.
	found := false
	for _, w := range g.Warnings {
		if w.Title == "Foreign-products exclusion" {
			found = true
		}
	}
	if !found {
		t.Errorf("guide warnings should include the foreign-products exclusion")
	}
}

func TestCLI_DoctorOK(t *testing.T) {
	path := writeImporterProfile(t)
	out, err := run(t, "", "--profile", path, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	var rep doctorReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("unmarshal doctor output: %v\n%s", err, out)
	}
	if !rep.OK {
		t.Errorf("doctor should be OK with a complete profile: %+v", rep.Checks)
	}
}

func TestCLI_UnknownProviderIsNotFound(t *testing.T) {
	path := writeImporterProfile(t)
	_, err := run(t, "", "--profile", path, "answersheet", "no-such-provider")
	if err == nil {
		t.Fatalf("expected an error for an unknown provider")
	}
	if ExitCode(err) != 3 {
		t.Errorf("exit code = %d, want 3 (not found)", ExitCode(err))
	}
}

func TestCompactFields_RecursesIntoNested(t *testing.T) {
	in := json.RawMessage(`{"id":"x","notes":"top","provider":{"name":"P","notes":"nested"},"items":[{"name":"a","detail":"d"}]}`)
	out := compactFields(in)
	for _, gone := range []string{"top", "nested", "detail"} {
		if bytes.Contains(out, []byte(gone)) {
			t.Errorf("compactFields left verbose %q in output: %s", gone, out)
		}
	}
	for _, kept := range []string{`"id"`, `"name"`} {
		if !bytes.Contains(out, []byte(kept)) {
			t.Errorf("compactFields dropped non-verbose %q: %s", kept, out)
		}
	}
}

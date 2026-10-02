package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/cliutil/testenv"
	"strings"
	"testing"
)

func TestLuggageComputedSelectionAndUsage(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"parcel", "--length", "70", "--width", "45", "--height", "30", "--weight", "23", "--json", "--select", "results.chargeable_size", "--no-learn"})
	if e := cmd.Execute(); e != nil {
		t.Fatal(e)
	}
	var payload map[string]any
	if e := json.Unmarshal(out.Bytes(), &payload); e != nil {
		t.Fatal(e)
	}
	if len(payload) != 1 || payload["results"].(map[string]any)["chargeable_size"] != float64(160) {
		t.Fatalf("selection lost: %s", out.String())
	}
	for _, args := range [][]string{{"parcel", "--json"}, {"parcel", "--length", "-1", "--width", "2", "--height", "3", "--weight", "1", "--json"}, {"airports", "--limit", "0", "--json"}, {"products", "--data-source", "local", "--json"}, {"parcel", "--data-source", "live", "--json"}} {
		cmd = RootCmd()
		out.Reset()
		errOut.Reset()
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs(append(args, "--no-learn"))
		if e := cmd.Execute(); e == nil {
			t.Fatalf("invalid args accepted: %v", args)
		}
	}
}
func TestLuggageDryRunsAndLiveFixtures(t *testing.T) {
	testenv.Isolate(t)
	for _, leaf := range []string{"parcel", "quote", "airports", "counters", "same-day", "products"} {
		cmd := RootCmd()
		var out, errOut bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs([]string{leaf, "--dry-run", "--json", "--no-learn"})
		if e := cmd.Execute(); e != nil {
			t.Fatalf("%s: %v", leaf, e)
		}
		var p map[string]any
		if e := json.Unmarshal(out.Bytes(), &p); e != nil || p["dry_run"] != true {
			t.Fatalf("%s invalid dry run: %s %v", leaf, out.String(), e)
		}
		resolved, _, e := cmd.Find([]string{leaf})
		if e != nil {
			t.Fatal(e)
		}
		fixtures := resolved.Annotations["pp:happy-args"]
		if fixtures == "" || strings.Contains(fixtures, " --") || strings.Contains(fixtures, "--dry-run") {
			t.Fatalf("%s not a real tokenized fixture: %s", leaf, fixtures)
		}
	}
}

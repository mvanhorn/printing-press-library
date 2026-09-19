package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestHavenDryRunDoesNotCreateDatabase(t *testing.T) {
	for _, action := range []string{"refresh", "menu", "locations", "compare", "subtotal", "common", "changes", "nearby"} {
		t.Run(action, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "untouched.db")
			flags := &rootFlags{dryRun: true, asJSON: true}
			cmd := newHavenCommand(flags, action)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--db", path})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("invalid dry-run JSON: %s", out.String())
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("dry run touched database: %v", err)
			}
		})
	}
}

func TestHavenInputValidationBeforeDatabase(t *testing.T) {
	cases := []struct {
		action string
		args   []string
	}{
		{"menu", []string{}}, {"compare", []string{"--item", "The Sandwich", "--locations", "14208"}},
		{"subtotal", []string{"--location", "14208", "--item", "9656289=0"}},
		{"subtotal", []string{"--location", "14208", "--item", "9656289=9000", "--item", "9656289=2000"}},
		{"nearby", []string{"--lat", "41.3"}}, {"locations", []string{"--limit", "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "untouched.db")
			cmd := newHavenCommand(&rootFlags{}, tc.action)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(append(tc.args, "--db", path))
			if err := cmd.Execute(); err == nil {
				t.Fatal("invalid input accepted")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("invalid request touched database: %v", err)
			}
		})
	}
}

func TestHavenLimitPreservesObservationProvenance(t *testing.T) {
	v, err := limitHavenResult(map[string]any{"rows": []int{1, 2}, "observations": []int{3, 4}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	obj := v.(map[string]any)
	if len(obj["rows"].([]any)) != 1 || len(obj["observations"].([]any)) != 2 || obj["rows_total"] != 2 {
		t.Fatalf("wrong bounded result: %#v", obj)
	}
}

func TestParseHavenLocationIDs(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  int
		valid bool
	}{{"14208,14205", 2, true}, {"14208,14208", 1, true}, {"14208,bad", 0, false}, {"", 0, false}, {"-1", 0, false}} {
		ids, err := parseHavenIDs(tc.input)
		if (err == nil) != tc.valid || len(ids) != tc.want {
			t.Errorf("%q: %v %v", tc.input, ids, err)
		}
	}
}

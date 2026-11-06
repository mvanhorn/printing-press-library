// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
)

// TestNovelOnboardHelpWires smoke-tests that the onboard command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelOnboardHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"onboard", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("onboard --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "onboard [email...]", "--from-csv founding.csv"} {
		if !strings.Contains(help, want) {
			t.Fatalf("onboard --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestOnboardTagPageValidation(t *testing.T) {
	for _, tc := range []struct {
		count     int
		next, bad bool
	}{{0, true, true}, {0, false, false}, {100, true, false}, {1, false, false}} {
		if err := validateOnboardTagPage(tc.count, tc.next); (err != nil) != tc.bad {
			t.Errorf("count=%d next=%v: %v", tc.count, tc.next, err)
		}
	}
}

func TestOnboardPrivateTagBody(t *testing.T) {
	want := map[string]any{"name": "founding", "display_format": "label", "is_public": false}
	if got := onboardTagBody("founding"); !reflect.DeepEqual(got, want) {
		t.Fatalf("tag body = %#v", got)
	}
	cmd := newNovelOnboardCmd(&rootFlags{})
	if cmd.Flags().Lookup("csv") != nil || cmd.Flags().Lookup("from-csv") == nil || cmd.Annotations["mcp:read-flags"] != "from-csv" {
		t.Fatal("CSV input must use from-csv and leave global csv output available")
	}
}

func TestOnboardMemberSearchParams(t *testing.T) {
	for _, email := range []string{"ada@example.com", "Ada+test@Example.com"} {
		t.Run(email, func(t *testing.T) {
			want := map[string]string{"email": email, "status": "all"}
			if got := onboardMemberSearchParams(email); !reflect.DeepEqual(got, want) {
				t.Fatalf("search params = %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseOnboardTagID(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		want int64
		bad  bool
	}{
		{"top level", `{"id":42,"name":"founding"}`, 42, false},
		{"wrapped", `{"member_tag":{"id":43,"name":"founding"}}`, 43, false},
		{"missing ID", `{}`, 0, true},
		{"missing wrapped ID", `{"member_tag":{}}`, 0, true},
		{"zero ID", `{"id":0}`, 0, true},
		{"negative wrapped ID", `{"member_tag":{"id":-1}}`, 0, true},
		{"null response", `null`, 0, true},
		{"invalid JSON", `not JSON`, 0, true},
		{"invalid ID type", `{"id":"42"}`, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOnboardTagID(json.RawMessage(tt.raw))
			if (err != nil) != tt.bad || got != tt.want {
				t.Fatalf("got (%d, %v), want (%d, error=%v)", got, err, tt.want, tt.bad)
			}
		})
	}
}

func TestParseOnboardCSV(t *testing.T) {
	tests := []struct {
		name, csv string
		want      []onboardInput
		bad       bool
	}{
		{"header order and tags", "tags,name,email\nfounding; beta ;FOUNDING,Ada L,ada@example.com\n", []onboardInput{{Email: "ada@example.com", Name: "Ada L", Tags: []string{"founding", "beta"}}}, false},
		{"optional columns", "email\nada@example.com\n", []onboardInput{{Email: "ada@example.com", Tags: []string{}}}, false},
		{"blank lines", "email,name,tags\n\n,,\nada@example.com,,\n\n", []onboardInput{{Email: "ada@example.com", Tags: []string{}}}, false},
		{"quoted name", "email,name\nada@example.com,\"Lavery, Ada\"\n", []onboardInput{{Email: "ada@example.com", Name: "Lavery, Ada", Tags: []string{}}}, false},
		{"header only", "email,name\n", []onboardInput{}, false},
		{"BOM and normalized headers", "\ufeff Email , NAME\nada@example.com,Ada\n", []onboardInput{{Email: "ada@example.com", Name: "Ada", Tags: []string{}}}, false},
		{"missing email header", "name\nAda\n", nil, true},
		{"duplicate header", "email,email\nada@example.com,ada@example.com\n", nil, true},
		{"empty CSV", "", nil, true},
		{"missing email", "email,name\n,Ada\n", nil, true},
		{"bad email", "email\nada@example\n", nil, true},
		{"multiple at signs", "email\nada@@example.com\n", nil, true},
		{"malformed row", "email,name\nada@example.com\n", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOnboardCSV(strings.NewReader(tt.csv))
			if (err != nil) != tt.bad {
				t.Fatalf("parse error = %v, want error %v", err, tt.bad)
			}
			if !tt.bad && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDedupeOnboardPeople(t *testing.T) {
	tests := []struct {
		name        string
		input, want []onboardInput
		bad         bool
	}{
		{"merge duplicate email", []onboardInput{
			{Email: " Ada@Example.com ", Tags: []string{"founding"}},
			{Email: "ada@example.com", Name: "Ada", Tags: []string{"FOUNDING", "beta"}},
			{Email: "grace@example.com", Name: "Grace"},
		}, []onboardInput{
			{Email: "Ada@Example.com", Name: "Ada", Tags: []string{"founding", "beta"}},
			{Email: "grace@example.com", Name: "Grace", Tags: []string{}},
		}, false},
		{"first nonempty name wins", []onboardInput{{Email: "ada@example.com", Name: "Ada"}, {Email: "ADA@example.com", Name: "Other"}}, []onboardInput{{Email: "ada@example.com", Name: "Ada", Tags: []string{}}}, false},
		{"empty", nil, []onboardInput{}, false},
		{"bad positional email", []onboardInput{{Email: "bad"}}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dedupeOnboardPeople(tt.input)
			if (err != nil) != tt.bad {
				t.Fatalf("error = %v", err)
			}
			if !tt.bad && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestBuildOnboardPlan(t *testing.T) {
	for _, send := range []bool{false, true} {
		t.Run(map[bool]string{false: "silent", true: "invite new members"}[send], func(t *testing.T) {
			plan := buildOnboardPlan([]onboardInput{
				{Email: "Ada@example.com", Name: "Ada", Tags: []string{"founding", "New", "new"}},
				{Email: "grace@example.com", Tags: []string{"NEW", "beta"}},
			}, map[string]bool{"grace@example.com": true}, map[string]int64{"founding": 42}, []int64{1000969}, []int64{1234}, []int64{5678}, send)
			want := onboardPlan{People: []onboardPerson{
				{Email: "Ada@example.com", Name: "Ada", Action: "create", Spaces: []int64{1000969}, SpaceGroups: []int64{1234}, AccessGroups: []int64{5678}, Tags: []string{"founding", "New"}, SendInvitation: send},
				{Email: "grace@example.com", Action: "update-memberships", Spaces: []int64{1000969}, SpaceGroups: []int64{1234}, AccessGroups: []int64{5678}, Tags: []string{"NEW", "beta"}},
			}, TagsToCreate: []string{"New", "beta"}, TagsToCreateNote: "(created as private tags)"}
			if !reflect.DeepEqual(plan, want) {
				t.Fatalf("got %#v, want %#v", plan, want)
			}
		})
	}
	t.Run("empty arrays in JSON", func(t *testing.T) {
		plan := buildOnboardPlan([]onboardInput{{Email: "ada@example.com"}}, nil, nil, nil, nil, nil, false)
		raw, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "null") {
			t.Fatalf("nil list in %s", raw)
		}
	})
}

func TestValidateOnboardEmail(t *testing.T) {
	for _, email := range []string{"ada@example.com", "ada+test@example.co.uk"} {
		if err := validateOnboardEmail(email); err != nil {
			t.Fatal(err)
		}
	}
	for _, email := range []string{"", "@example.com", "ada@example", "ada@@example.com", "ada@.example", "ada@example.", "ada b@example.com"} {
		if err := validateOnboardEmail(email); err == nil {
			t.Errorf("accepted invalid email %q", email)
		}
	}
}

func TestCheckOnboardResponse(t *testing.T) {
	for _, tt := range []struct {
		raw string
		bad bool
	}{
		{`{"id":42}`, false}, {`{"success":true}`, false},
		{`{"success":false,"message":"Not allowed"}`, true}, {`not JSON`, true},
	} {
		if err := checkOnboardResponse(json.RawMessage(tt.raw)); (err != nil) != tt.bad {
			t.Errorf("%s: error = %v", tt.raw, err)
		}
	}
}

func TestOnboardInputAndHarnessGuards(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		harness string
		want    string
		bad     bool
	}{
		{"bare help", nil, "", "Usage:", false},
		{"missing input", []string{"--tag", "founding"}, "", "provide at least one email", true},
		{"dry run before CSV read", []string{"--from-csv", "/does-not-exist/founding.csv", "--dry-run"}, "", `"dry_run":true`, false},
		{"bad email before client", []string{"bad"}, "", "invalid email", true},
		{"invalid ID", []string{"ada@example.com", "--space", "-1"}, "", "must be positive", true},
		{"verify refuses send before CSV", []string{"--from-csv", "/does-not-exist/founding.csv", "--send", "--yes"}, "verify", `"refused":true`, true},
		{"dogfood refuses send", []string{"ada@example.com", "--send", "--yes"}, "dogfood", `"refused":true`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testenv.Isolate(t)
			t.Setenv("PRINTING_PRESS_VERIFY", "0")
			t.Setenv("PRINTING_PRESS_DOGFOOD", "0")
			if tt.harness == "verify" {
				t.Setenv("PRINTING_PRESS_VERIFY", "1")
			}
			if tt.harness == "dogfood" {
				t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
			}
			flags := &rootFlags{asJSON: true}
			cmd := newNovelOnboardCmd(flags)
			cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Preview only.")
			cmd.Flags().BoolVar(&flags.yes, "yes", false, "Apply the plan.")
			cmd.SetArgs(tt.args)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			if (err != nil) != tt.bad {
				t.Fatalf("error = %v, output = %s", err, out.String())
			}
			got := out.String()
			if err != nil {
				got += err.Error()
			}
			got = strings.ReplaceAll(strings.ReplaceAll(got, " ", ""), "\n", "")
			if !strings.Contains(got, strings.ReplaceAll(tt.want, " ", "")) {
				t.Fatalf("missing %q in %s", tt.want, out.String())
			}
		})
	}
}

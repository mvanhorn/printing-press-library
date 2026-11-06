// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/store"
)

// TestNovelFoundingHelpWires smoke-tests that the founding command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelFoundingHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"founding", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("founding --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "founding"} {
		if !strings.Contains(help, want) {
			t.Fatalf("founding --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestFoundingVerdict(t *testing.T) {
	for _, tc := range []struct {
		name         string
		paid, target int
		verdict      string
		shortBy      int
	}{
		{"below", 9, 10, "NO-GO", 1},
		{"equal", 10, 10, "GO", 0},
		{"above", 11, 10, "GO", 0},
		{"empty", 0, 10, "NO-GO", 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := foundingVerdict(tc.paid, tc.target, "local")
			if got.Verdict != tc.verdict || got.ShortBy != tc.shortBy || got.Paid != tc.paid || got.Target != tc.target || got.Source != "local" {
				t.Fatalf("unexpected verdict: %#v", got)
			}
		})
	}
}

func TestFoundingLocalExitCodes(t *testing.T) {
	testenv.Isolate(t)
	dbPath := circlePayerTestDB(t)
	for _, tc := range []struct {
		target, code int
		verdict      string
	}{
		{1, 0, "GO"}, {2, 8, "NO-GO"}, {0, 2, ""},
	} {
		t.Run(tc.verdict, func(t *testing.T) {
			flags := &rootFlags{asJSON: true, dataSource: "local"}
			cmd := newNovelFoundingCmd(flags)
			if cmd.Annotations["pp:typed-exit-codes"] != "0,8" || !strings.Contains(cmd.Long, "8 NO-GO") {
				t.Fatal("NO-GO exit code is not documented")
			}
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--db", dbPath, "--go-no-go", fmt.Sprint(tc.target)})
			err := cmd.Execute()
			code := 0
			if err != nil {
				code = ExitCode(err)
			}
			if code != tc.code {
				t.Fatalf("exit code %d, want %d; err=%v", code, tc.code, err)
			}
			if tc.target == 0 {
				return
			}
			// Sync hints are allowed; the verdict must not be repeated as a Cobra error.
			if strings.Contains(stderr.String(), "Error:") || strings.Contains(stderr.String(), "NO-GO") {
				t.Fatalf("unexpected error noise: %s", &stderr)
			}
			var result foundingResult
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Verdict != tc.verdict || result.Target != tc.target || result.Paid != 1 || result.Source != "local" {
				t.Fatalf("unexpected output: %s", &out)
			}
		})
	}
}

func TestFoundingMissingPaymentData(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			testenv.Isolate(t)
			dbPath := filepath.Join(t.TempDir(), "mirror.db")
			if existing {
				db, err := store.OpenWithContext(context.Background(), dbPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
			cmd := newNovelFoundingCmd(&rootFlags{asJSON: true, dataSource: "local"})
			cmd.SetArgs([]string{"--db", dbPath, "--go-no-go", "10"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "no synced payment data; run sync --resources community-member-subscriptions,community-member-charges") || ExitCode(err) == 2 || strings.Contains(out.String(), `"verdict"`) {
				t.Fatalf("missing payment data produced a verdict: output=%s error=%v", &out, err)
			}
		})
	}
}

func TestFoundingValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		args     []string
		code     int
		contains string
		dryRun   bool
	}{
		{"bare help", nil, 0, "Usage:", false},
		{"missing", []string{"--paywall", "1000969"}, 2, "", false},
		{"zero", []string{"--go-no-go", "0"}, 2, "", false},
		{"negative", []string{"--go-no-go", "-1"}, 2, "", false},
		{"dry run before validation", []string{"--db", filepath.Join(t.TempDir(), "missing.db")}, 0, "dry_run", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newNovelFoundingCmd(&rootFlags{asJSON: true, dataSource: "local", dryRun: tc.dryRun})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			code := 0
			if err != nil {
				code = ExitCode(err)
			}
			if code != tc.code || !strings.Contains(out.String(), tc.contains) {
				t.Fatalf("code=%d err=%v output=%s", code, err, &out)
			}
		})
	}
}

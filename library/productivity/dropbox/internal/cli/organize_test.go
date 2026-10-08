// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
)

// TestNovelOrganizeHelpWires smoke-tests that the organize command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelOrganizeHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"organize", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("organize --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "organize"} {
		if !strings.Contains(help, want) {
			t.Fatalf("organize --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestOrganizeCollisionsAndMissingAncestors(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/Camera Uploads", "folder", "", "", 0),
		fixtureRow("/Photos", "folder", "", "", 0),
		fixtureRow("/Photos/2019", "folder", "", "", 0),
		fixtureRow("/Photos/2019/03", "folder", "", "", 0),
		fixtureRow("/Camera Uploads/IMG_1.jpg", "file", "", "2019-03-04T00:00:00Z", 1),
		fixtureRow("/Camera Uploads/IMG_2.jpg", "file", "", "2019-03-20T00:00:00Z", 1),
		fixtureRow("/Camera Uploads/IMG_3.jpg", "file", "", "2020-07-01T00:00:00Z", 1),
		fixtureRow("/Camera Uploads/shot.png", "file", "", "2020-07-01T00:00:00Z", 1),
		fixtureRow("/Photos/2019/03/IMG_2.jpg", "file", "", "2019-03-20T00:00:00Z", 1),
	)
	data, err := runRead(t, "organize", "--match", "*.jpg", "--under", "/Camera Uploads", "--to", "/Photos/{year}/{month}", "--tz", "UTC", "--print-plan", "--db", db, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Matched      int                 `json:"matched"`
		PlannedMoves int                 `json:"planned_moves"`
		Mkdirs       []string            `json:"mkdirs"`
		Collisions   []organizeCollision `json:"collisions"`
		Plan         dropbox.Plan        `json:"plan"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Matched != 3 || got.PlannedMoves != 2 || len(got.Collisions) != 1 || strings.Join(got.Mkdirs, ",") != "/Photos/2020,/Photos/2020/07" {
		t.Fatalf("organize result: %+v", got)
	}
	if len(got.Plan.Ops) != 4 || got.Plan.Ops[2].From != "/Camera Uploads/IMG_1.jpg" || got.Plan.Ops[3].From != "/Camera Uploads/IMG_3.jpg" {
		t.Fatalf("plan ops: %+v", got.Plan.Ops)
	}
}

func TestOrganizeExcludesDevFilesByDefault(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/repo/photo.jpg", "file", "", "2020-01-01T00:00:00Z", 8),
		fixtureRow("/repo/node_modules/photo.jpg", "file", "", "2020-01-01T00:00:00Z", 9),
	)
	for _, tc := range []struct {
		include           bool
		matched, excluded int
	}{{false, 1, 1}, {true, 2, 0}} {
		args := []string{"organize", "--db", db, "--match", "*.jpg", "--under", "/repo", "--to", "/Photos/{year}", "--json"}
		if tc.include {
			args = append(args, "--include-dev-dirs")
		}
		data, err := runRead(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		var got organizeResult
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Matched != tc.matched || got.ExcludedDevDirs.Files != tc.excluded {
			t.Fatalf("include=%t organize=%+v", tc.include, got)
		}
	}
}

func TestOrganizeTimeZoneAndMissingDates(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/Inbox/midnight.jpg", "file", "", "2020-01-01T00:30:00Z", 1),
		fixtureRow("/Inbox/unknown.jpg", "file", "", "not-a-date", 1),
	)
	data, err := runRead(t, "organize", "--db", db, "--match", "*.jpg", "--under", "/Inbox", "--to", "/Photos/{year}/{month}", "--tz", "America/Los_Angeles", "--print-plan", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got organizeResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Matched != 2 || len(got.SkippedNoDate) != 1 || got.SkippedNoDate[0] != "/Inbox/unknown.jpg" || got.PlannedMoves != 1 || got.Plan == nil {
		t.Fatalf("organize=%+v", got)
	}
	last := got.Plan.Ops[len(got.Plan.Ops)-1]
	if last.To != "/Photos/2019/12/midnight.jpg" {
		t.Fatalf("timezone move=%+v", last)
	}
}

// A preview that mixes mkdir and move ops must keep each move's from/to under
// --agent; the compact key-frequency rule would otherwise drop them.
func TestOrganizeAgentPreviewKeepsMoveEndpoints(t *testing.T) {
	testenv.Isolate(t)
	db := seedIndex(t,
		fixtureRow("/Inbox", "folder", "", "", 0),
		fixtureRow("/Inbox/a.jpg", "file", "", "2020-07-01T00:00:00Z", 1),
	)
	data, err := runRead(t, "organize", "--match", "*.jpg", "--under", "/Inbox", "--to", "/Photos/{year}/{month}", "--tz", "UTC", "--db", db, "--agent")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Results struct {
			OpsPreview []dropbox.Op `json:"ops_preview"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	ops := got.Results.OpsPreview
	if len(ops) != 4 || ops[0].Op != "mkdir" || ops[0].Path != "/Photos" || ops[3].Op != "move" || ops[3].From != "/Inbox/a.jpg" || ops[3].To != "/Photos/2020/07/a.jpg" {
		t.Fatalf("agent preview ops: %+v", ops)
	}
}

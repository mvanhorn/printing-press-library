// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Live-dogfood fixture wiring for the generated endpoint commands.
//
// Every Tiimo resource except /api/profiles is scoped under
// /api/profiles/{profile_id}/..., so the generated commands all take a
// required <profile_id> positional. The live matrix has no way to invent one,
// so those commands were probed with no arguments, correctly returned exit 2,
// and were recorded as happy-path failures.
//
// Rather than weaken the commands' own argument validation to make a test
// pass, this supplies the matrix with a real profile id at startup. The id is
// read from the local mirror, so it is only present once the user has synced --
// which is exactly when a live happy path is meaningful anyway.
//
// This lives in its own file so a regen preserves it; the generated command
// files must not be hand-edited.

package cli

import (
	"database/sql"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/store"
)

// profileScopedCommands are command paths whose first positional is a profile
// id, keyed by the extra flags each needs beyond that.
var profileScopedCommands = map[string]string{
	"todo-tasks list": "",
	"todo-tasks get":  "",
	"todo-lists":      "",
	"tags":            "",
	"routines":        "--from=" + fixtureDate,
	"calendars list":  "",
	"profiles get":    "",
	"activities list": "--from-date=" + fixtureDate,
}

// fixtureDate is a stable, obviously-synthetic date used only for matrix
// fixtures. Any date the API accepts works; a fixed one keeps runs comparable.
const fixtureDate = "2026-08-14"

// novelCommandHappyArgs supplies live-matrix arguments for novel commands that
// are not profile-scoped and would otherwise be skipped.
//
// `export` earns its entry the hard way. Its documented example once carried
// `--output tiimo-profiles.json`, and `scorecard --live-check` executes
// examples verbatim -- which wrote a file holding a live profile id and a real
// account name into the CLI package root, where publish would have shipped it
// to a public repo. Dropping the arguments stopped the leak but left the
// command unexercised, so the live matrix reported hollow coverage for a
// headline feature.
//
// `--output` defaults to stdout, so naming no file exercises the real export
// path -- pagination, encoding, resource resolution -- while writing nothing to
// disk. `--limit=1` keeps the response to a single record: enough to prove the
// command works, small enough that nothing bulky lands in a harness log.
var novelCommandHappyArgs = map[string]string{
	"export": "resource=profiles;--limit=1",
}

// exampleOverrides REPLACE a generated command's Example text, unlike
// missingHelpExamples which only fills an empty one.
//
// Help prose is executable input. The live matrix harvests flags out of a
// command's Example and merges them into whatever pp:happy-args supplies, so a
// flag that appears only in documentation still runs. `export` shipped the
// generator's stock example, `export <resource> --format jsonl --output
// data.jsonl`; the matrix substituted the resource, kept `--output data.jsonl`,
// and wrote a file holding a live profile id and a real account name into the
// CLI package root -- the exact directory publish collects for a public repo.
// A safe pp:happy-args cannot prevent this, because the merge re-adds the flag.
//
// So no runnable line here may name an output path. `--output` still exists and
// is still documented in the README; the file case appears below as prose in a
// comment, which the harness does not execute.
var exampleOverrides = map[string]string{
	"export": `  # Stream profiles as JSONL on stdout (redirect it yourself for a file)
  tiimo-pp-cli export profiles --format jsonl

  # Cap the number of records
  tiimo-pp-cli export profiles --format jsonl --limit 1000

  # Pipe to another tool
  tiimo-pp-cli export profiles --format jsonl | jq '.profileId'`,
}

// missingHelpExamples supplies Example text for generated commands that ship
// without any.
//
// The live-dogfood help check requires a parent command's --help to carry an
// Examples section; `feedback` is generator-emitted (marked DO NOT EDIT) and
// has none, so it fails its own check on every print. Confirmed by fixing the
// identical failure on this CLI's hand-written `todo` parent purely by adding
// an Example.
//
// Set here rather than by editing the generated file, so a regen cannot drop
// it. Filed upstream as a generator defect.
var missingHelpExamples = map[string]string{
	"feedback": `  # Record friction you just hit
  tiimo-pp-cli feedback "drift returned nothing until I ran sync twice"

  # Record it and send it upstream
  tiimo-pp-cli feedback "todo schedule ignored --for" --send`,
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for path, example := range missingHelpExamples {
			if cmd, _, err := root.Find(splitCommandPath(path)); err == nil && cmd != nil && cmd.Example == "" {
				cmd.Example = example
			}
		}

		// Unconditional, unlike the fill above: the whole point is to displace
		// a generated example whose flags are unsafe to execute.
		for path, example := range exampleOverrides {
			if cmd, _, err := root.Find(splitCommandPath(path)); err == nil && cmd != nil {
				cmd.Example = example
			}
		}

		// `feedback` takes free-form prose: every string is valid input, so
		// there is no bad-argument case for the error-path probe to exercise.
		// Declare that honestly instead of inventing a rejection the command
		// does not and should not have.
		if cmd, _, err := root.Find([]string{"feedback"}); err == nil && cmd != nil {
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			cmd.Annotations["pp:no-error-path-probe"] = "true"
		}

		// Novel commands that take no profile id but still need arguments to be
		// exercised live. Without these the matrix skips their happy path and
		// the run is reported as hollow coverage for that feature -- a headline
		// command that is documented, shipped, and never actually run.
		for path, args := range novelCommandHappyArgs {
			cmd, _, err := root.Find(splitCommandPath(path))
			if err != nil || cmd == nil {
				continue
			}
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			if _, exists := cmd.Annotations["pp:happy-args"]; !exists {
				cmd.Annotations["pp:happy-args"] = args
			}
		}

		pid := fixtureProfileID()
		if pid == "" {
			// No mirror yet, so there is no real profile to offer. Leave the
			// commands unannotated rather than fabricating a UUID that would
			// turn a fixture gap into a confusing 404 against the live API.
			return
		}
		for path, extra := range profileScopedCommands {
			cmd, _, err := root.Find(splitCommandPath(path))
			if err != nil || cmd == nil {
				continue
			}
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			if _, exists := cmd.Annotations["pp:happy-args"]; exists {
				continue
			}
			args := "profile_id=" + pid
			// Commands that take a second positional need a real id for it
			// too; supplying only the profile id leaves them one argument
			// short and they correctly refuse.
			if second, ok := secondPositionalFixture(path); ok {
				if second == "" {
					// The mirror has no row to point at, so there is no
					// honest happy path to probe. Leave it unannotated.
					continue
				}
				args += ";" + second
			}
			if extra != "" {
				args += ";" + extra
			}
			cmd.Annotations["pp:happy-args"] = args
		}
	})
}

// secondPositionalFixture returns the extra positional a command needs beyond
// the profile id. The bool reports whether the command takes one at all.
//
// Values come from the local mirror so the probe hits a row that genuinely
// exists; an invented UUID would turn a fixture gap into a 404 and read as a
// command defect.
func secondPositionalFixture(path string) (string, bool) {
	switch path {
	case "todo-tasks get":
		// Test the resolved VALUE, not the assembled token: "task_id=" with an
		// empty right-hand side is a non-empty string that silently produces a
		// fixture with no argument in it.
		id := firstColumnValue("todo_tasks", "task_id")
		if id == "" {
			id = os.Getenv("TIIMO_TASK_ID")
		}
		if id == "" {
			return "", true
		}
		return "task_id=" + id, true
	default:
		return "", false
	}
}

// firstColumnValue reads one value from the local mirror, or "" when the
// mirror or row is absent.
func firstColumnValue(table, column string) string {
	dbPath := defaultDBPath("tiimo-pp-cli")
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}
	st, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return ""
	}
	defer st.Close()

	// Table and column are compile-time constants from the switch above, never
	// caller input, so the interpolation cannot be influenced externally.
	row := st.DB().QueryRow("SELECT " + column + " FROM " + table + " LIMIT 1")
	var v sql.NullString
	if err := row.Scan(&v); err != nil {
		return ""
	}
	return v.String
}

// splitCommandPath turns "todo-tasks list" into ["todo-tasks","list"].
func splitCommandPath(path string) []string {
	out := make([]string, 0, 2)
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == ' ' {
			if i > start {
				out = append(out, path[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// fixtureProfileID reads one profile id from the local mirror. It never opens
// the API: annotation wiring runs during command construction, long before
// flags are parsed, and must not perform network IO.
func fixtureProfileID() string {
	if v := os.Getenv("TIIMO_PROFILE_ID"); v != "" {
		return v
	}
	dbPath := defaultDBPath("tiimo-pp-cli")
	if _, err := os.Stat(dbPath); err != nil {
		return ""
	}
	st, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return ""
	}
	defer st.Close()

	items, err := st.List("profiles", 1)
	if err != nil || len(items) == 0 {
		return ""
	}
	var p profileRecord
	if err := json.Unmarshal(items[0], &p); err != nil {
		return ""
	}
	return p.ProfileID
}

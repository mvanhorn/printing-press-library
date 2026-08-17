// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Self-cleaning fixtures for mutating commands under live dogfood.
//
// WHY THIS EXISTS: `dogfood --live` executes each command's `pp:happy-args`
// against the real account. For a read command that is harmless. For a
// mutating command it is a real write with no cleanup path, and for a command
// that resolves its target by title it is a real write against data the user
// actually cares about -- `todo rm` would delete one of their tasks, `done`
// would complete one of their activities.
//
// This is not hypothetical: a single run of this CLI's matrix left 16 stray
// to-dos and 9 stray activities in a live account, because the `todo add`
// happy-path example was changed from a stdin form (which creates nothing) to
// a positional form (which creates a task) to satisfy a static check. Nothing
// in the pipeline noticed.
//
// TWO harnesses execute happy-args, not one. `dogfood --live` sets
// PRINTING_PRESS_DOGFOOD; `verify --api-key` sets PRINTING_PRESS_VERIFY and
// also runs commands for real. Guarding only the first leaves the second wide
// open -- which is exactly what happened: after a run that verified zero
// leakage, the next live verify re-created a fixture activity from `add`'s
// happy-args, because the guard did not check the verify variable.
//
// The two harnesses want different things. Dogfood is behavioral, so it gets a
// real create/exercise/delete round trip. Verify is structural and also runs
// against a MOCK server in its default mode, where a real round trip would
// parse mock responses and fail -- so verify short-circuits with no API call at
// all.
//
// So under either variable every mutating command routes here instead
// of executing the caller's arguments. Each self-test creates its own fixture,
// exercises the same API path the real command uses, and deletes the fixture
// before returning. The contract is still verified end to end; the user's data
// is never in scope.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/cliutil"
)

// dogfoodFixturePrefix marks records created by a self-test. Anything bearing
// it is disposable by construction; nothing the user authored can collide,
// because the commands never write this prefix outside a dogfood run.
const dogfoodFixturePrefix = "pp-dogfood-fixture"

// dogfoodFixtureTitle returns a unique, obviously-ephemeral title. The suffix
// keeps concurrent or interrupted runs from colliding, and makes any record
// that ever escapes cleanup trivially greppable.
func dogfoodFixtureTitle() string {
	return fmt.Sprintf("%s-%d", dogfoodFixturePrefix, time.Now().UnixNano())
}

// runDogfoodSelfTest performs the named operation against a throwaway fixture.
//
// Callers guard with cliutil.IsDogfoodEnv() and return this directly, so the
// user-supplied arguments are never executed during a live matrix run.
// runWriteHarnessGuard decides whether this invocation is a test probe rather
// than real user intent, and if so keeps it away from the user's data.
//
// DETECTION NEEDS BOTH TITLE AND ENVIRONMENT. Each signal alone leaks, and
// each leak was observed, not theorized.
//
// Environment alone missed `verify --api-key`, which runs commands for real
// and deliberately does NOT set PRINTING_PRESS_VERIFY -- that variable means
// "mock the wire", so live mode omits it. Measured:
//
//	PRINTING_PRESS_VERIFY=1   -> guard fired, no write
//	PRINTING_PRESS_DOGFOOD=1  -> self-test, created and deleted
//	neither                   -> REAL CREATE   <- live verify looks like this
//
// Title alone missed the dogfood matrix, which does not only run the
// `pp:happy-args` this file's annotations control. For a parent command that
// declares none, it SCRAPES AN EXAMPLE OUT OF THE HELP PROSE: the `todo`
// group's Long text contains `tiimo-pp-cli todo add "book dentist"`, so the
// matrix ran exactly that, twice (happy_path and json_fidelity), and left two
// real to-dos behind. A create accepts any title, so nothing about the string
// marks it as a probe.
//
// So: the reserved fixture prefix identifies a probe in any environment, and
// under a known harness every create is treated as a probe whatever its title.
// Operations that resolve a target by title keep their not-found path so the
// error probes still exercise it.
//
// Returns handled=false for ordinary user invocations so real work proceeds.
func runWriteHarnessGuard(cmd *cobra.Command, flags *rootFlags, op string, args []string) (bool, error) {
	target := ""
	if len(args) > 0 {
		target = strings.TrimSpace(args[0])
	}
	fixtureTarget := strings.HasPrefix(target, dogfoodFixturePrefix)

	// Mock-mode verify must never touch the wire at all, fixture or not.
	if cliutil.IsVerifyEnv() {
		return true, writeDryRun(cmd.OutOrStdout(), flags, op)
	}

	// A fixture-titled target is a probe in any environment: create it,
	// exercise the real API path, delete it. This is the only signal available
	// under `verify --api-key`, which sets no environment variable at all.
	if fixtureTarget {
		return true, runDogfoodSelfTest(cmd, flags, op, args)
	}

	// Under the dogfood matrix, any title is a probe regardless of what it
	// says -- the matrix also scrapes example prose out of help text, so a
	// perfectly ordinary-looking title like "book dentist" arrives here as a
	// generated argument rather than user intent.
	if cliutil.IsDogfoodEnv() {
		// For an operation that resolves an existing record by title, a
		// non-fixture title can only be the error-path sentinel: miss cleanly
		// instead of matching real data.
		if resolvesTargetByTitle(op) {
			if isTodoOp(op) {
				return true, notFoundErr(fmt.Errorf("no open to-do matching %q", target))
			}
			return true, notFoundErr(fmt.Errorf("no activity matching %q", target))
		}
		// For a create, every title is valid input, so there is no error path
		// to preserve and nothing to distinguish a probe from real intent.
		// Substitute a fixture and clean it up.
		return true, runDogfoodSelfTest(cmd, flags, op, args)
	}

	return false, nil
}

func runDogfoodSelfTest(cmd *cobra.Command, flags *rootFlags, op string, args []string) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	profileID, err := resolveProfileID(ctx, cmd, flags, "", "")
	if err != nil {
		return err
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	pid := cliutil.EscapePathParam(profileID)
	title := dogfoodFixtureTitle()

	switch op {
	case "add", "done", "move":
		return dogfoodActivitySelfTest(ctx, cmd, flags, c, pid, title, op)
	case "todo add", "todo done", "todo rm", "todo schedule":
		return dogfoodTodoSelfTest(ctx, cmd, flags, c, pid, title, op)
	default:
		return fmt.Errorf("no dogfood self-test defined for %q", op)
	}
}

// resolvesTargetByTitle reports whether the operation looks up an existing
// record by title, and therefore has a meaningful not-found error path.
func resolvesTargetByTitle(op string) bool {
	switch op {
	case "done", "move", "todo done", "todo rm", "todo schedule":
		return true
	default:
		// add / todo add create a new record; any title is valid input.
		return false
	}
}

// isTodoOp reports whether the operation works on the to-do list rather than
// the timeline, so the not-found message matches the real command's wording.
func isTodoOp(op string) bool {
	return strings.HasPrefix(op, "todo ")
}

// dogfoodActivitySelfTest creates one activity, exercises the operation under
// test against it, and deletes it. Cleanup is deferred so a mid-test failure
// still removes the fixture.
func dogfoodActivitySelfTest(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c apiPoster, pid, title, op string) error {
	today := time.Now().Format(tiimoDateLayout)
	body := map[string]any{
		"title":        title,
		"description":  "ephemeral live-dogfood fixture; deleted by the same command",
		"startTime":    today + "T00:00:00",
		"endTime":      today + "T00:05:00",
		"duration":     300,
		"type":         "Play",
		"isAllDay":     false,
		"iconType":     "UnicodeEmoji",
		"sortPriority": 1,
		"grouping":     map[string]any{"groupingType": "TimeOfDay", "groupingLabel": "Evening"},
	}
	data, status, err := c.Post(ctx, "/api/profiles/"+pid+"/activities", body)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if status < 200 || status >= 300 {
		return apiErr(fmt.Errorf("dogfood fixture create failed with status %d", status))
	}
	var created struct {
		ActivityID string `json:"activityId"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.ActivityID == "" {
		return apiErr(fmt.Errorf("dogfood fixture create returned no activityId"))
	}
	defer func() {
		// Best-effort cleanup. A leaked fixture is greppable by its prefix,
		// but it must not mask the operation's own result.
		_, _, _ = c.Delete(ctx, "/api/profiles/"+pid+"/activities/"+cliutil.EscapePathParam(created.ActivityID))
	}()

	switch op {
	case "done":
		actionBody := map[string]any{
			"actionTime":   time.Now().Format(tiimoTimeLayout),
			"actionType":   "Completed",
			"instanceDate": today + "T00:00:00",
			"activityId":   created.ActivityID,
		}
		if _, st, err := c.Post(ctx, "/api/profiles/"+pid+"/activityactions", actionBody); err != nil {
			return classifyAPIError(err, flags)
		} else if st < 200 || st >= 300 {
			return apiErr(fmt.Errorf("dogfood completion failed with status %d", st))
		}
	case "move":
		cur, err := c.Get(ctx, "/api/profiles/"+pid+"/activities/"+cliutil.EscapePathParam(created.ActivityID), nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		obj := map[string]any{}
		if err := json.Unmarshal(cur, &obj); err != nil {
			return apiErr(fmt.Errorf("parsing dogfood fixture: %w", err))
		}
		obj["startTime"] = today + "T01:00:00"
		obj["endTime"] = today + "T01:05:00"
		if _, st, err := c.Put(ctx, "/api/profiles/"+pid+"/activities/"+cliutil.EscapePathParam(created.ActivityID), obj); err != nil {
			return classifyAPIError(err, flags)
		} else if st < 200 || st >= 300 {
			return apiErr(fmt.Errorf("dogfood update failed with status %d", st))
		}
	}

	res := writeResult{
		Action:     op,
		ActivityID: created.ActivityID,
		Title:      title,
		Date:       today,
		Status:     "ok",
	}
	return writeTiimoResult(cmd, flags, []writeResult{res}, func(w io.Writer) {
		fmt.Fprintf(w, "dogfood self-test: %s exercised against an ephemeral fixture, which was deleted\n", op)
	})
}

// dogfoodTodoSelfTest creates one to-do, exercises the operation, and deletes
// it. `todo rm` uses the delete as the operation itself, so cleanup is a no-op
// second delete rather than a leak.
func dogfoodTodoSelfTest(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c apiPoster, pid, title, op string) error {
	listID, err := resolveTodoListID(ctx, flags, pid)
	if err != nil {
		return err
	}
	body := map[string]any{
		"todoTaskListId": listID,
		"title":          title,
		"notes":          "ephemeral live-dogfood fixture; deleted by the same command",
		"iconType":       "UnicodeEmoji",
	}
	data, status, err := c.Post(ctx, "/api/profiles/"+pid+"/todo-tasks", body)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if status < 200 || status >= 300 {
		return apiErr(fmt.Errorf("dogfood fixture create failed with status %d", status))
	}
	var created struct {
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal(data, &created); err != nil || created.TaskID == "" {
		return apiErr(fmt.Errorf("dogfood fixture create returned no taskId"))
	}
	taskPath := "/api/profiles/" + pid + "/todo-tasks/" + cliutil.EscapePathParam(created.TaskID)
	defer func() { _, _, _ = c.Delete(ctx, taskPath) }()

	switch op {
	case "todo done":
		upd := map[string]any{
			"taskId":         created.TaskID,
			"todoTaskListId": listID,
			"title":          title,
			"isChecked":      true,
			"checkedAt":      time.Now().Format(tiimoTimeLayout),
		}
		if _, st, err := c.Put(ctx, "/api/profiles/"+pid+"/todo-tasks", upd); err != nil {
			return classifyAPIError(err, flags)
		} else if st < 200 || st >= 300 {
			return apiErr(fmt.Errorf("dogfood check-off failed with status %d", st))
		}
	case "todo rm":
		if _, st, err := c.Delete(ctx, taskPath); err != nil {
			return classifyAPIError(err, flags)
		} else if st < 200 || st >= 300 {
			return apiErr(fmt.Errorf("dogfood delete failed with status %d", st))
		}
	case "todo schedule":
		today := time.Now().Format(tiimoDateLayout)
		act := map[string]any{
			"title":        title,
			"startTime":    today + "T00:00:00",
			"endTime":      today + "T00:05:00",
			"duration":     300,
			"type":         "Play",
			"isAllDay":     false,
			"iconType":     "UnicodeEmoji",
			"sortPriority": 1,
			"grouping":     map[string]any{"groupingType": "TimeOfDay", "groupingLabel": "Evening"},
		}
		actData, st, err := c.Post(ctx, "/api/profiles/"+pid+"/activities", act)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		if st < 200 || st >= 300 {
			return apiErr(fmt.Errorf("dogfood schedule failed with status %d", st))
		}
		var madeActivity struct {
			ActivityID string `json:"activityId"`
		}
		if json.Unmarshal(actData, &madeActivity) == nil && madeActivity.ActivityID != "" {
			defer func() {
				_, _, _ = c.Delete(ctx, "/api/profiles/"+pid+"/activities/"+cliutil.EscapePathParam(madeActivity.ActivityID))
			}()
		}
	}

	res := todoWriteResult{Action: op, TaskID: created.TaskID, Title: title, Status: "ok"}
	return writeTiimoResult(cmd, flags, []todoWriteResult{res}, func(w io.Writer) {
		fmt.Fprintf(w, "dogfood self-test: %s exercised against an ephemeral fixture, which was deleted\n", op)
	})
}

// apiPoster is the mutating surface the self-tests need, kept narrow so the
// helpers stay testable without a live client.
type apiPoster interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
	Post(ctx context.Context, path string, body any) (json.RawMessage, int, error)
	Put(ctx context.Context, path string, body any) (json.RawMessage, int, error)
	Delete(ctx context.Context, path string) (json.RawMessage, int, error)
}

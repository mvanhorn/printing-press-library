// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Writes go straight to the API; the local mirror is refreshed by `sync`.

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

// writeResult is the machine-readable envelope for a mutation.
type writeResult struct {
	Action     string `json:"action"`
	ActivityID string `json:"activity_id,omitempty"`
	Title      string `json:"title"`
	Date       string `json:"date,omitempty"`
	Start      string `json:"start,omitempty"`
	Bucket     string `json:"bucket,omitempty"`
	Duration   string `json:"duration,omitempty"`
	Status     string `json:"status"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTiimoAddCmd(flags))
		addNovelCommandIfAbsent(root, newTiimoDoneCmd(flags))
		addNovelCommandIfAbsent(root, newTiimoMoveCmd(flags))
	})
}

func newTiimoAddCmd(flags *rootFlags) *cobra.Command {
	var flagAt, flagFor, flagDate, flagIcon, flagProfile, flagDB, flagNote, flagBucket string
	var flagAllDay bool

	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Add an activity to the timeline",
		Long: `Put an activity on the timeline without opening the app.

Capture speed is the whole point for Tiimo's audience, so this resolves your
profile automatically and accepts human durations: --for 30m, --for 1h30m,
--for 90 (minutes).

ON TIMES: Tiimo activities are not pinned to the clock. They are ordered
inside a time-of-day bucket (Morning, Afternoon, Evening, Anytime), and the
API normalizes any start time you send to midnight -- verified against a real
account, where none of 1190 activities carried a clock time. --at is therefore
a convenience that picks the bucket for you (14:00 means Afternoon); pass
--bucket directly if you prefer. Only events imported from a linked external
calendar carry real times, and those are read-only.

For an unscheduled item that belongs on your to-do list rather than the
timeline, use 'todo add' instead.`,
		Example: "  tiimo-pp-cli add \"Deep work\" --at 14:00 --for 90m",
		Annotations: map[string]string{
			"pp:happy-args":       "title=pp-dogfood-fixture;--bucket=Morning;--for=15m",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "add")
			}
			if handled, err := runWriteHarnessGuard(cmd, flags, "add", args); handled {
				return err
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an activity title is required"))
			}
			title := strings.TrimSpace(args[0])

			day := time.Now()
			if strings.TrimSpace(flagDate) != "" {
				d, err := parseTiimoDate(flagDate)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(err)
				}
				day = d
			}
			startClock := 9 * 60
			if strings.TrimSpace(flagAt) != "" {
				m, err := parseClock(flagAt, 9, 0)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --at %q: want HH:MM", flagAt))
				}
				startClock = m
			}
			dur := 30 * time.Minute
			if strings.TrimSpace(flagFor) != "" {
				d, err := cliutil.ParseDurationLoose(flagFor)
				if err != nil || d <= 0 {
					// A bare number means minutes, which is how people speak.
					var n int
					if _, scanErr := fmt.Sscanf(strings.TrimSpace(flagFor), "%d", &n); scanErr == nil && n > 0 {
						d = time.Duration(n) * time.Minute
					} else {
						_ = cmd.Usage()
						return usageErr(fmt.Errorf("invalid --for %q: want 30m, 1h30m, or a number of minutes", flagFor))
					}
				}
				dur = d
			}

			start := time.Date(day.Year(), day.Month(), day.Day(), startClock/60, startClock%60, 0, 0, time.Local)
			end := start.Add(dur)

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			profileID, err := resolveProfileID(ctx, cmd, flags, flagProfile, flagDB)
			if err != nil {
				return err
			}

			bucket := bucketForHour(start.Hour())
			if b := strings.TrimSpace(flagBucket); b != "" {
				canonical, ok := canonicalBucket(b)
				if !ok {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --bucket %q: want Morning, Afternoon, Evening, or Anytime", flagBucket))
				}
				bucket = canonical
			}
			body := map[string]any{
				"title":        title,
				"description":  flagNote,
				"startTime":    start.Format(tiimoTimeLayout),
				"endTime":      end.Format(tiimoTimeLayout),
				"duration":     int(dur.Seconds()),
				"type":         "Play",
				"isAllDay":     flagAllDay,
				"iconType":     "UnicodeEmoji",
				"sortPriority": 100,
				"grouping": map[string]any{
					"groupingType":  "TimeOfDay",
					"groupingLabel": bucket,
				},
			}
			if flagIcon != "" {
				body["iconId"] = flagIcon
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, status, err := c.Post(ctx, "/api/profiles/"+cliutil.EscapePathParam(profileID)+"/activities", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("creating activity failed with status %d", status))
			}

			var created struct {
				ActivityID string `json:"activityId"`
			}
			_ = json.Unmarshal(data, &created)

			res := writeResult{
				Action:     "created",
				ActivityID: created.ActivityID,
				Title:      title,
				Date:       start.Format(tiimoDateLayout),
				Bucket:     bucket,
				Duration:   humanDuration(int(dur.Seconds())),
				Status:     "ok",
			}
			return writeTiimoResult(cmd, flags, []writeResult{res}, func(w io.Writer) {
				fmt.Fprintf(w, "Added %q to %s on %s for %s\n", title, bucket, res.Date, res.Duration)
				fmt.Fprintln(w, "Run 'tiimo-pp-cli sync' to refresh the local mirror.")
			})
		},
	}

	cmd.Flags().StringVar(&flagAt, "at", "", "Time of day used to pick the bucket (HH:MM); Tiimo does not store a clock time")
	cmd.Flags().StringVar(&flagBucket, "bucket", "", "Time-of-day bucket: Morning, Afternoon, Evening, or Anytime")
	cmd.Flags().StringVar(&flagFor, "for", "", "Duration (30m, 1h30m, or minutes), default 30m")
	cmd.Flags().StringVar(&flagDate, "date", "", "Date (YYYY-MM-DD), default today")
	cmd.Flags().StringVar(&flagIcon, "icon", "", "Emoji to show on the activity")
	cmd.Flags().StringVar(&flagNote, "note", "", "Longer description")
	cmd.Flags().BoolVar(&flagAllDay, "all-day", false, "Create as an all-day activity")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Profile name or UUID (auto-resolved when the account has one profile)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror used to resolve the profile")
	return cmd
}

func newTiimoDoneCmd(flags *rootFlags) *cobra.Command {
	var flagDate, flagProfile, flagDB string
	var flagUndo bool

	cmd := &cobra.Command{
		Use:   "done [title]",
		Short: "Mark an activity complete",
		Long: `Mark a timeline activity as completed, or undo that with --undo.

Completion is NOT a field on the activity. Writing completedAt through the
activity update endpoint returns 200 and silently discards it -- a trap worth
knowing about, because it makes a naive implementation report success while
changing nothing. Tiimo models completion as a separate action:

    POST /api/profiles/{profileId}/activityactions
    {actionTime, actionType: Completed|Reset, instanceDate, activityId}

instanceDate is what makes this per-occurrence: completing today's run of a
daily routine leaves every other day untouched.

The title is matched case-insensitively against activities mirrored for the
target date, so partial titles work; ambiguous matches are reported rather
than guessed at.`,
		Example: "  tiimo-pp-cli done \"Deep work\"",
		Annotations: map[string]string{
			"pp:happy-args":       "title=pp-dogfood-fixture",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "done")
			}
			if handled, err := runWriteHarnessGuard(cmd, flags, "done", args); handled {
				return err
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an activity title is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			profileID, err := resolveProfileID(ctx, cmd, flags, flagProfile, flagDB)
			if err != nil {
				return err
			}
			day := time.Now()
			if strings.TrimSpace(flagDate) != "" {
				d, parseErr := parseTiimoDate(flagDate)
				if parseErr != nil {
					_ = cmd.Usage()
					return usageErr(parseErr)
				}
				day = d
			}
			match, err := findActivityByTitle(ctx, cmd, flags, flagDB, args[0], day)
			if err != nil {
				return err
			}

			actionType := "Completed"
			action := "done"
			if flagUndo {
				actionType = "Reset"
				action = "reset"
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"actionTime":   time.Now().Format(tiimoTimeLayout),
				"actionType":   actionType,
				"instanceDate": match.Day() + "T00:00:00",
				"activityId":   match.ActivityID,
			}
			data, status, err := c.Post(ctx,
				"/api/profiles/"+cliutil.EscapePathParam(profileID)+"/activityactions", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("recording %s action failed with status %d", actionType, status))
			}

			// Confirm from the response rather than trusting the status code:
			// the update endpoint taught us that 200 does not imply the change
			// took effect.
			var after struct {
				State       *string `json:"state"`
				CompletedAt *string `json:"completedAt"`
			}
			_ = json.Unmarshal(data, &after)
			applied := (actionType == "Completed" && after.CompletedAt != nil) ||
				(actionType == "Reset" && after.CompletedAt == nil)
			if !applied {
				return apiErr(fmt.Errorf("the API accepted the %s action but the activity did not change state", actionType))
			}

			res := writeResult{
				Action:     action,
				ActivityID: match.ActivityID,
				Title:      match.Title,
				Date:       match.Day(),
				Status:     "ok",
			}
			return writeTiimoResult(cmd, flags, []writeResult{res}, func(w io.Writer) {
				if flagUndo {
					fmt.Fprintf(w, "Reset %q back to not-done (%s)\n", match.Title, match.Day())
				} else {
					fmt.Fprintf(w, "Marked %q complete (%s)\n", match.Title, match.Day())
				}
				fmt.Fprintln(w, "Run 'tiimo-pp-cli sync' to refresh the local mirror.")
			})
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Date to search (YYYY-MM-DD), default today")
	cmd.Flags().BoolVar(&flagUndo, "undo", false, "Undo a completion instead of recording one")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Profile name or UUID")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror")
	return cmd
}

func newTiimoMoveCmd(flags *rootFlags) *cobra.Command {
	var flagTo, flagDate, flagProfile, flagDB string

	cmd := &cobra.Command{
		Use:   "move [title]",
		Short: "Reschedule an activity to a different time",
		Long: `Shift a timeline activity to a new start time, keeping its duration.

Reviewers have asked Tiimo for drag-and-drop rescheduling; this is the
scriptable equivalent.`,
		Example: "  tiimo-pp-cli move \"Deep work\" --to 16:00",
		Annotations: map[string]string{
			"pp:happy-args":       "title=pp-dogfood-fixture;--to=10:00",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "move")
			}
			if handled, err := runWriteHarnessGuard(cmd, flags, "move", args); handled {
				return err
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an activity title is required"))
			}
			if strings.TrimSpace(flagTo) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to is required (the new start time, HH:MM)"))
			}
			newClock, err := parseClock(flagTo, 9, 0)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --to %q: want HH:MM", flagTo))
			}
			return mutateActivity(cmd, flags, args[0], flagDate, flagProfile, flagDB, "moved", func(obj map[string]any) error {
				startStr, _ := obj["startTime"].(string)
				start, ok := parseTiimoTime(startStr)
				if !ok {
					return apiErr(fmt.Errorf("activity has no usable start time to move"))
				}
				durSecs := 0
				if d, ok := obj["duration"].(float64); ok {
					durSecs = int(d)
				}
				newStart := time.Date(start.Year(), start.Month(), start.Day(), newClock/60, newClock%60, 0, 0, time.Local)
				obj["startTime"] = newStart.Format(tiimoTimeLayout)
				if durSecs > 0 {
					obj["endTime"] = newStart.Add(time.Duration(durSecs) * time.Second).Format(tiimoTimeLayout)
				}
				if g, ok := obj["grouping"].(map[string]any); ok {
					g["groupingLabel"] = bucketForHour(newStart.Hour())
				}
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&flagTo, "to", "", "New start time (HH:MM)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Date to search (YYYY-MM-DD), default today")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Profile name or UUID")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror")
	return cmd
}

// mutateActivity resolves a title to a single activity, fetches its current
// server-side representation, applies mutate, and PUTs the whole object back.
//
// Tiimo's activity update is a full replace -- PATCH returns 405 -- so
// sending a partial body would blank every field the caller did not name.
func mutateActivity(cmd *cobra.Command, flags *rootFlags, title, date, profile, dbPath, action string, mutate func(map[string]any) error) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	profileID, err := resolveProfileID(ctx, cmd, flags, profile, dbPath)
	if err != nil {
		return err
	}

	day := time.Now()
	if strings.TrimSpace(date) != "" {
		d, parseErr := parseTiimoDate(date)
		if parseErr != nil {
			_ = cmd.Usage()
			return usageErr(parseErr)
		}
		day = d
	}

	match, err := findActivityByTitle(ctx, cmd, flags, dbPath, title, day)
	if err != nil {
		return err
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	base := "/api/profiles/" + cliutil.EscapePathParam(profileID) + "/activities/" + cliutil.EscapePathParam(match.ActivityID)
	current, err := c.Get(ctx, base, nil)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	obj := map[string]any{}
	if err := json.Unmarshal(current, &obj); err != nil {
		return apiErr(fmt.Errorf("parsing activity %s: %w", match.ActivityID, err))
	}
	if err := mutate(obj); err != nil {
		return err
	}

	_, status, err := c.Put(ctx, base, obj)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if status < 200 || status >= 300 {
		return apiErr(fmt.Errorf("updating activity failed with status %d", status))
	}

	res := writeResult{
		Action:     action,
		ActivityID: match.ActivityID,
		Title:      match.Title,
		Date:       match.Day(),
		Status:     "ok",
	}
	if s, ok := parseTiimoTime(fmt.Sprint(obj["startTime"])); ok {
		res.Start = s.Format("15:04")
	}
	return writeTiimoResult(cmd, flags, []writeResult{res}, func(w io.Writer) {
		switch action {
		case "done":
			fmt.Fprintf(w, "Marked %q complete (%s)\n", match.Title, match.Day())
		case "moved":
			fmt.Fprintf(w, "Moved %q to %s on %s\n", match.Title, res.Start, match.Day())
		default:
			fmt.Fprintf(w, "Updated %q\n", match.Title)
		}
		fmt.Fprintln(w, "Run 'tiimo-pp-cli sync' to refresh the local mirror.")
	})
}

// findActivityByTitle resolves a human-typed title to exactly one activity on
// the given day, preferring an exact match before falling back to substring.
func findActivityByTitle(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath, title string, day time.Time) (activityRow, error) {
	st, ok, err := openLocalMirror(ctx, cmd, flags, dbPath)
	if err != nil {
		return activityRow{}, err
	}
	if !ok {
		return activityRow{}, notFoundErr(fmt.Errorf("no local mirror; run `tiimo-pp-cli sync` before referring to activities by title"))
	}
	defer st.Close()

	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
	to := from.AddDate(0, 0, 1).Add(-time.Second)
	acts, err := loadActivities(ctx, st.DB(), from, to)
	if err != nil {
		return activityRow{}, err
	}

	needle := strings.ToLower(strings.TrimSpace(title))
	exact := make([]activityRow, 0)
	partial := make([]activityRow, 0)
	for _, a := range acts {
		lt := strings.ToLower(a.Title)
		switch {
		case lt == needle:
			exact = append(exact, a)
		case strings.Contains(lt, needle):
			partial = append(partial, a)
		}
	}
	candidates := exact
	if len(candidates) == 0 {
		candidates = partial
	}

	switch len(candidates) {
	case 0:
		return activityRow{}, notFoundErr(fmt.Errorf("no activity matching %q on %s", title, from.Format(tiimoDateLayout)))
	case 1:
		if candidates[0].IsReadOnly {
			return activityRow{}, usageErr(fmt.Errorf("%q comes from a linked external calendar and is read-only in Tiimo; edit it in the source calendar", candidates[0].Title))
		}
		return candidates[0], nil
	default:
		names := make([]string, 0, len(candidates))
		for _, a := range candidates {
			at := "all day"
			if s, ok := a.Start(); ok && !a.IsAllDay {
				at = s.Format("15:04")
			}
			names = append(names, fmt.Sprintf("%s (%s)", a.Title, at))
		}
		return activityRow{}, usageErr(fmt.Errorf("%q matches %d activities on %s: %s; use a more specific title",
			title, len(candidates), from.Format(tiimoDateLayout), strings.Join(names, ", ")))
	}
}

// canonicalBucket normalizes a user-supplied bucket name to the exact casing
// Tiimo stores, so "morning" and "MORNING" both work.
func canonicalBucket(s string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "morning":
		return "Morning", true
	case "afternoon", "day":
		return "Afternoon", true
	case "evening", "night":
		return "Evening", true
	case "anytime", "unscheduled":
		return "Anytime", true
	default:
		return "", false
	}
}

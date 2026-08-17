// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// Capture-side commands for the to-do list, which is a separate resource from
// the timeline: to-dos have no start time and live in priority buckets.
//
// The API is asymmetric between the two and the difference is load-bearing:
// activities update via PUT on the item path, to-dos update via PUT on the
// COLLECTION path (item-level PUT returns 405). Both were verified live.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/cliutil"
)

// todoRow is one to-do task.
type todoRow struct {
	TaskID    string `json:"task_id"`
	ListID    string `json:"todo_task_list_id,omitempty"`
	Title     string `json:"title"`
	Notes     string `json:"notes,omitempty"`
	Duration  int    `json:"duration,omitempty"`
	IsChecked bool   `json:"is_checked"`
	CreatedAt string `json:"created_at,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

// todoWriteResult is the envelope for to-do mutations.
type todoWriteResult struct {
	Action string `json:"action"`
	TaskID string `json:"task_id,omitempty"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Count  int    `json:"count,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTiimoTodoCmd(flags))
	})
}

func newTiimoTodoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Capture and manage to-do tasks",
		Long: `Work with the to-do list -- the priority-bucketed list of unscheduled
tasks, which is a different resource from the timeline.

Use 'todo add' for a task with no fixed time, and the top-level 'add' for
something that belongs at a specific point in the day.`,
		Example: `  # Capture a task
  tiimo-pp-cli todo add "book dentist"

  # See what is still open
  tiimo-pp-cli todo list

  # Put one on today's timeline
  tiimo-pp-cli todo schedule "book dentist" --at 15:00`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(
		newTodoListCmd(flags),
		newTodoAddCmd(flags),
		newTodoDoneCmd(flags),
		newTodoRmCmd(flags),
		newTodoScheduleCmd(flags),
	)
	return cmd
}

func newTodoListCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List to-do tasks from the local mirror",
		Long: `Show outstanding to-do tasks.

Completed tasks are hidden unless --all is passed, because the point of the
list is what is still open.`,
		Example: "  tiimo-pp-cli todo list --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "todo list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rows := make([]todoRow, 0)
			st, ok, err := openLocalMirror(ctx, cmd, flags, flagDB)
			if err != nil {
				return err
			}
			if !ok {
				return writeNoMirror(cmd, flags, flagDB, rows)
			}
			defer st.Close()

			all, err := loadTodoTasks(ctx, st.DB())
			if err != nil {
				return err
			}
			for _, t := range all {
				if t.IsChecked && !flagAll {
					continue
				}
				rows = append(rows, t)
			}

			return writeTiimoResult(cmd, flags, rows, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintln(w, "No open to-do tasks.")
					return
				}
				tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "\tTASK\tEST\tADDED")
				for _, t := range rows {
					mark := " "
					if t.IsChecked {
						mark = "x"
					}
					est := "-"
					if t.Duration > 0 {
						est = humanDuration(t.Duration)
					}
					added := t.CreatedAt
					if len(added) >= 10 {
						added = added[:10]
					}
					fmt.Fprintf(tw, "[%s]\t%s\t%s\t%s\n", mark, t.Title, est, orDash(added))
				}
				_ = tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Include tasks already checked off")
	return cmd
}

func newTodoDoneCmd(flags *rootFlags) *cobra.Command {
	var flagProfile, flagDB string

	cmd := &cobra.Command{
		Use:   "done [title]",
		Short: "Check off a to-do task",
		Long: `Mark a to-do task as checked.

Tiimo updates to-dos through the collection path rather than the item path,
so this sends the whole task object back with its checked state set.`,
		Example: "  tiimo-pp-cli todo done \"book dentist\"",
		Annotations: map[string]string{
			"pp:happy-args":       "title=pp-dogfood-fixture",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "todo done")
			}
			if handled, err := runWriteHarnessGuard(cmd, flags, "todo done", args); handled {
				return err
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a task title is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			profileID, err := resolveProfileID(ctx, cmd, flags, flagProfile, flagDB)
			if err != nil {
				return err
			}
			task, err := findTodoByTitle(ctx, cmd, flags, flagDB, args[0])
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Collection path, not item path: item-level PUT is 405 here.
			body := map[string]any{
				"taskId":         task.TaskID,
				"todoTaskListId": task.ListID,
				"title":          task.Title,
				"notes":          task.Notes,
				"isChecked":      true,
				"checkedAt":      time.Now().Format(tiimoTimeLayout),
			}
			if task.Duration > 0 {
				body["duration"] = task.Duration
			}
			_, status, err := c.Put(ctx, "/api/profiles/"+cliutil.EscapePathParam(profileID)+"/todo-tasks", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("checking off task failed with status %d", status))
			}

			res := todoWriteResult{Action: "checked", TaskID: task.TaskID, Title: task.Title, Status: "ok"}
			return writeTiimoResult(cmd, flags, []todoWriteResult{res}, func(w io.Writer) {
				fmt.Fprintf(w, "Checked off: %s\n", task.Title)
				fmt.Fprintln(w, "Run 'tiimo-pp-cli sync' to refresh the local mirror.")
			})
		},
	}
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Profile name or UUID")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror")
	return cmd
}

func newTodoRmCmd(flags *rootFlags) *cobra.Command {
	var flagProfile, flagDB string

	cmd := &cobra.Command{
		Use:   "rm [title]",
		Short: "Delete a to-do task",
		Long: `Remove a to-do task entirely.

This deletes rather than checks off. Use 'todo done' to keep the record.`,
		Example: "  tiimo-pp-cli todo rm \"book dentist\"",
		Annotations: map[string]string{
			"pp:happy-args":       "title=pp-dogfood-fixture",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "todo rm")
			}
			if handled, err := runWriteHarnessGuard(cmd, flags, "todo rm", args); handled {
				return err
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a task title is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			profileID, err := resolveProfileID(ctx, cmd, flags, flagProfile, flagDB)
			if err != nil {
				return err
			}
			task, err := findTodoByTitle(ctx, cmd, flags, flagDB, args[0])
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, status, err := c.Delete(ctx,
				"/api/profiles/"+cliutil.EscapePathParam(profileID)+"/todo-tasks/"+cliutil.EscapePathParam(task.TaskID))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("deleting task failed with status %d", status))
			}

			res := todoWriteResult{Action: "deleted", TaskID: task.TaskID, Title: task.Title, Status: "ok"}
			return writeTiimoResult(cmd, flags, []todoWriteResult{res}, func(w io.Writer) {
				fmt.Fprintf(w, "Deleted to-do: %s\n", task.Title)
				fmt.Fprintln(w, "Run 'tiimo-pp-cli sync' to refresh the local mirror.")
			})
		},
	}
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Profile name or UUID")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror")
	return cmd
}

func newTodoScheduleCmd(flags *rootFlags) *cobra.Command {
	var flagAt, flagDate, flagFor, flagProfile, flagDB string
	var flagCheck bool

	cmd := &cobra.Command{
		Use:   "schedule [title]",
		Short: "Promote a to-do onto the timeline",
		Long: `Turn a to-do task into a scheduled activity.

The app lets you drag a to-do onto the timeline; this is the same move as one
command. The task's own estimated duration is reused unless --for overrides
it.

The to-do is left in place by default so nothing is lost if the scheduling was
a mistake; pass --check to tick it off at the same time.`,
		Example: "  tiimo-pp-cli todo schedule \"book dentist\" --at 15:00",
		Annotations: map[string]string{
			"pp:happy-args":       "title=pp-dogfood-fixture;--at=15:00",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "todo schedule")
			}
			if handled, err := runWriteHarnessGuard(cmd, flags, "todo schedule", args); handled {
				return err
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a task title is required"))
			}
			if strings.TrimSpace(flagAt) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--at is required (the start time, HH:MM)"))
			}
			startClock, err := parseClock(flagAt, 9, 0)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --at %q: want HH:MM", flagAt))
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

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			profileID, err := resolveProfileID(ctx, cmd, flags, flagProfile, flagDB)
			if err != nil {
				return err
			}
			task, err := findTodoByTitle(ctx, cmd, flags, flagDB, args[0])
			if err != nil {
				return err
			}

			dur := 30 * time.Minute
			if task.Duration > 0 {
				dur = time.Duration(task.Duration) * time.Second
			}
			if strings.TrimSpace(flagFor) != "" {
				d, err := cliutil.ParseDurationLoose(flagFor)
				if err != nil || d <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --for %q", flagFor))
				}
				dur = d
			}

			start := time.Date(day.Year(), day.Month(), day.Day(), startClock/60, startClock%60, 0, 0, time.Local)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"title":        task.Title,
				"description":  task.Notes,
				"startTime":    start.Format(tiimoTimeLayout),
				"endTime":      start.Add(dur).Format(tiimoTimeLayout),
				"duration":     int(dur.Seconds()),
				"type":         "Play",
				"isAllDay":     false,
				"iconType":     "UnicodeEmoji",
				"sortPriority": 100,
				"grouping": map[string]any{
					"groupingType":  "TimeOfDay",
					"groupingLabel": bucketForHour(start.Hour()),
				},
			}
			_, status, err := c.Post(ctx, "/api/profiles/"+cliutil.EscapePathParam(profileID)+"/activities", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("scheduling task failed with status %d", status))
			}

			checked := false
			if flagCheck {
				putBody := map[string]any{
					"taskId":         task.TaskID,
					"todoTaskListId": task.ListID,
					"title":          task.Title,
					"notes":          task.Notes,
					"isChecked":      true,
					"checkedAt":      time.Now().Format(tiimoTimeLayout),
				}
				if _, st, err := c.Put(ctx, "/api/profiles/"+cliutil.EscapePathParam(profileID)+"/todo-tasks", putBody); err == nil && st >= 200 && st < 300 {
					checked = true
				} else {
					// The activity already exists; a failed tick-off is a
					// warning, not a reason to claim the whole thing failed.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: activity created but the to-do could not be checked off\n")
				}
			}

			res := todoWriteResult{Action: "scheduled", TaskID: task.TaskID, Title: task.Title, Status: "ok"}
			return writeTiimoResult(cmd, flags, []todoWriteResult{res}, func(w io.Writer) {
				fmt.Fprintf(w, "Scheduled %q on %s at %s for %s\n",
					task.Title, start.Format(tiimoDateLayout), start.Format("15:04"), humanDuration(int(dur.Seconds())))
				if checked {
					fmt.Fprintln(w, "The to-do was checked off.")
				}
				fmt.Fprintln(w, "Run 'tiimo-pp-cli sync' to refresh the local mirror.")
			})
		},
	}
	cmd.Flags().StringVar(&flagAt, "at", "", "Start time (HH:MM)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Date (YYYY-MM-DD), default today")
	cmd.Flags().StringVar(&flagFor, "for", "", "Override the task's estimated duration")
	cmd.Flags().BoolVar(&flagCheck, "check", false, "Check the to-do off after scheduling it")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Profile name or UUID")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror")
	return cmd
}

// loadTodoTasks reads the mirrored to-do tasks with NULL-safe scans.
func loadTodoTasks(ctx context.Context, db *sql.DB) ([]todoRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT task_id, todo_task_list_id, title, notes, duration,
		       is_checked, created_at, checked_at
		FROM todo_tasks`)
	if err != nil {
		return nil, fmt.Errorf("querying to-do tasks: %w", err)
	}
	out := make([]todoRow, 0)
	for rows.Next() {
		var (
			taskID, listID, title, notes sql.NullString
			duration                     sql.NullInt64
			isChecked                    sql.NullBool
			createdAt, checkedAt         sql.NullString
		)
		if err := rows.Scan(&taskID, &listID, &title, &notes, &duration, &isChecked, &createdAt, &checkedAt); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning to-do task: %w", err)
		}
		out = append(out, todoRow{
			TaskID:    taskID.String,
			ListID:    listID.String,
			Title:     title.String,
			Notes:     notes.String,
			Duration:  int(duration.Int64),
			IsChecked: isChecked.Bool,
			CreatedAt: createdAt.String,
			CheckedAt: checkedAt.String,
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating to-do tasks: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing to-do rows: %w", err)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt < out[j].CreatedAt })
	return out, nil
}

// findTodoByTitle resolves a typed title to exactly one open to-do.
func findTodoByTitle(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath, title string) (todoRow, error) {
	st, ok, err := openLocalMirror(ctx, cmd, flags, dbPath)
	if err != nil {
		return todoRow{}, err
	}
	if !ok {
		return todoRow{}, notFoundErr(fmt.Errorf("no local mirror; run `tiimo-pp-cli sync` before referring to tasks by title"))
	}
	defer st.Close()

	all, err := loadTodoTasks(ctx, st.DB())
	if err != nil {
		return todoRow{}, err
	}

	needle := strings.ToLower(strings.TrimSpace(title))
	exact := make([]todoRow, 0)
	partial := make([]todoRow, 0)
	for _, t := range all {
		if t.IsChecked {
			continue
		}
		lt := strings.ToLower(t.Title)
		switch {
		case lt == needle:
			exact = append(exact, t)
		case strings.Contains(lt, needle):
			partial = append(partial, t)
		}
	}
	candidates := exact
	if len(candidates) == 0 {
		candidates = partial
	}
	switch len(candidates) {
	case 0:
		return todoRow{}, notFoundErr(fmt.Errorf("no open to-do matching %q", title))
	case 1:
		return candidates[0], nil
	default:
		names := make([]string, 0, len(candidates))
		for _, t := range candidates {
			names = append(names, t.Title)
		}
		return todoRow{}, usageErr(fmt.Errorf("%q matches %d open to-dos: %s; use a more specific title",
			title, len(candidates), strings.Join(names, ", ")))
	}
}

// resolveTodoListID finds the list new tasks should land in. Most accounts
// have exactly one.
func resolveTodoListID(ctx context.Context, flags *rootFlags, profileID string) (string, error) {
	c, err := flags.newClient()
	if err != nil {
		return "", err
	}
	data, err := c.Get(ctx, "/api/profiles/"+cliutil.EscapePathParam(profileID)+"/todo-task-lists", nil)
	if err != nil {
		return "", fmt.Errorf("listing to-do lists: %w", err)
	}
	var env struct {
		Lists []struct {
			TodoTaskListID string `json:"todoTaskListId"`
			Title          string `json:"title"`
		} `json:"lists"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", fmt.Errorf("parsing to-do lists: %w", err)
	}
	if len(env.Lists) == 0 {
		return "", notFoundErr(fmt.Errorf("this profile has no to-do lists; create one in the Tiimo app first"))
	}
	return env.Lists[0].TodoTaskListID, nil
}

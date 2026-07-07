// Copyright 2026 wwilson1017 and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-built ergonomic task writes. Toodledo's task add/edit/delete use a
// form-encoded `tasks=<JSON-array>` batch param that the generator cannot
// express as typed flags, so these are hand-authored: they resolve
// folder/context/goal names to ids, parse YYYY-MM-DD dates, and batch up to 50
// ids per call. Registered under the `tasks` parent in root.go.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/toodledo/internal/cliutil"
	"github.com/spf13/cobra"
)

// parseDueDate converts YYYY-MM-DD to a Toodledo GMT unix timestamp, anchored at
// noon UTC to avoid timezone date-shift (matching the toodledo-mcp convention).
func parseDueDate(s string) (int64, error) {
	s = strings.TrimSpace(s)
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, fmt.Errorf("invalid date %q (use YYYY-MM-DD)", s)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.UTC).Unix(), nil
}

// postTasksBatch POSTs a form-encoded `tasks=<json>` payload to a tasks endpoint.
func postTasksBatch(cmd *cobra.Command, flags *rootFlags, path string, payload any) (json.RawMessage, error) {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, status, err := c.PostForm(ctx, path, url.Values{"tasks": {string(b)}})
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	if status >= 400 {
		return nil, apiErr(fmt.Errorf("%s returned HTTP %d: %s", path, status, cliutil.SanitizeErrorBody(string(resp))))
	}
	return resp, nil
}

// taskFieldFlags holds the writable task fields shared by add and edit.
type taskFieldFlags struct {
	folder, context, goal        string
	priority, status, due, start string
	tag, note, repeat            string
	parent, length               int
	dbPath                       string
}

func (f *taskFieldFlags) register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.folder, "folder", "", "Folder (project) name or id")
	cmd.Flags().StringVar(&f.context, "context", "", "Context name or id, e.g. @work")
	cmd.Flags().StringVar(&f.goal, "goal", "", "Goal name or id")
	cmd.Flags().StringVar(&f.priority, "priority", "", "Priority: negative/low/medium/high/top")
	cmd.Flags().StringVar(&f.status, "status", "", "GTD status, e.g. next_action, waiting, someday")
	cmd.Flags().StringVar(&f.due, "due", "", "Due date YYYY-MM-DD (empty string clears on edit)")
	cmd.Flags().StringVar(&f.start, "start", "", "Start date YYYY-MM-DD (empty string clears on edit)")
	cmd.Flags().StringVar(&f.tag, "tag", "", "Comma-separated tags")
	cmd.Flags().StringVar(&f.note, "note", "", "Task note/description")
	cmd.Flags().StringVar(&f.repeat, "repeat", "", "Recurrence as an iCal RRULE, e.g. FREQ=WEEKLY;BYDAY=MO")
	cmd.Flags().IntVar(&f.parent, "parent", 0, "Parent task id (subtask; requires a Toodledo Pro subscription)")
	cmd.Flags().IntVar(&f.length, "length", 0, "Estimated length in minutes")
	cmd.Flags().StringVar(&f.dbPath, "db", "", "Local mirror path for name resolution")
}

// apply populates a task field map from the flags. onEdit enables empty-string
// date clearing and only includes flags the user actually changed.
func (f *taskFieldFlags) apply(cmd *cobra.Command, flags *rootFlags, fields map[string]any) error {
	changed := func(name string) bool { return cmd.Flags().Changed(name) }

	if f.priority != "" {
		p, ok := parsePriority(f.priority)
		if !ok {
			return usageErr(fmt.Errorf("invalid --priority %q (use negative/low/medium/high/top)", f.priority))
		}
		fields["priority"] = p
	}
	if f.status != "" {
		s, ok := parseStatus(f.status)
		if !ok {
			return usageErr(fmt.Errorf("invalid --status %q", f.status))
		}
		fields["status"] = s
	}
	if changed("star") {
		if v, _ := cmd.Flags().GetBool("star"); v {
			fields["star"] = 1
		} else {
			fields["star"] = 0
		}
	}
	if f.tag != "" {
		fields["tag"] = f.tag
	}
	if changed("note") {
		fields["note"] = f.note
	}
	if f.repeat != "" {
		fields["repeat"] = f.repeat
	}
	if f.parent != 0 {
		fields["parent"] = f.parent
	}
	if f.length != 0 {
		fields["length"] = f.length
	}
	if changed("due") {
		if strings.TrimSpace(f.due) == "" {
			fields["duedate"] = 0
		} else {
			d, err := parseDueDate(f.due)
			if err != nil {
				return usageErr(err)
			}
			fields["duedate"] = d
		}
	}
	if changed("start") {
		if strings.TrimSpace(f.start) == "" {
			fields["startdate"] = 0
		} else {
			d, err := parseDueDate(f.start)
			if err != nil {
				return usageErr(err)
			}
			fields["startdate"] = d
		}
	}

	if f.folder != "" || f.context != "" || f.goal != "" {
		db, ok, err := openLocalMirror(cmd, toodledoDBPath(f.dbPath))
		if err != nil {
			return err
		}
		if !ok {
			return notFoundErr(fmt.Errorf("cannot resolve --folder/--context/--goal names without a local mirror; run 'toodledo-pp-cli sync' first, or pass numeric ids"))
		}
		defer db.Close()
		for _, ref := range []struct {
			val, table, field string
		}{
			{f.folder, "folders", "folder"},
			{f.context, "contexts", "context"},
			{f.goal, "goals", "goal"},
		} {
			if ref.val == "" {
				continue
			}
			id, found, _ := resolveRefID(db, ref.table, ref.val)
			if !found {
				return usageErr(fmt.Errorf("%s %q not found. available: %s", ref.field, ref.val, strings.Join(availableNames(db, ref.table), ", ")))
			}
			fields[ref.field] = id
		}
	}
	return nil
}

// capitalizeFirst upper-cases the first byte of a single-word verb.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func newTasksAddCmd(flags *rootFlags) *cobra.Command {
	var ff taskFieldFlags
	var title string
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Create a task (resolves folder/context/goal names to ids)",
		Long: `Create a task. The title is the positional arg or --title. Folder, context,
and goal may be given by name (resolved via the local mirror) or numeric id.`,
		Example: strings.Trim(`
  toodledo-pp-cli tasks add "Email the team" --context @work --priority high --due 2026-06-20
  toodledo-pp-cli tasks add "Weekly review" --repeat FREQ=WEEKLY;BYDAY=SU --folder Routines`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitTaskDryRun(cmd, flags, "/tasks/add.php", "add")
			}
			if title == "" && len(args) > 0 {
				title = strings.Join(args, " ")
			}
			if strings.TrimSpace(title) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a task title is required (positional or --title)"))
			}
			fields := map[string]any{"title": title}
			if err := ff.apply(cmd, flags, fields); err != nil {
				return err
			}
			if ff.parent != 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: subtasks (--parent) require a Toodledo Pro subscription; on a free account the parent link is dropped silently.")
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would add task: %s\n", title)
				return nil
			}
			resp, err := postTasksBatch(cmd, flags, "/tasks/add.php", []map[string]any{fields})
			if err != nil {
				return err
			}
			return emitTaskWriteResult(cmd, flags, resp, "created")
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Task title (alternative to the positional arg)")
	cmd.Flags().Bool("star", false, "Star the task")
	ff.register(cmd)
	return cmd
}

func newTasksEditCmd(flags *rootFlags) *cobra.Command {
	var ff taskFieldFlags
	var title string
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task (only the fields you pass are changed)",
		Example: strings.Trim(`
  toodledo-pp-cli tasks edit 123456 --priority top --due 2026-07-01
  toodledo-pp-cli tasks edit 123456 --status waiting --note "blocked on vendor"`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitTaskDryRun(cmd, flags, "/tasks/edit.php", "edit")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a task id is required"))
			}
			id, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil {
				return usageErr(fmt.Errorf("invalid task id %q", args[0]))
			}
			fields := map[string]any{"id": id}
			if title != "" {
				fields["title"] = title
			}
			if err := ff.apply(cmd, flags, fields); err != nil {
				return err
			}
			if len(fields) == 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("nothing to change; pass at least one field flag"))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would edit task: %d\n", id)
				return nil
			}
			resp, err := postTasksBatch(cmd, flags, "/tasks/edit.php", []map[string]any{fields})
			if err != nil {
				return err
			}
			return emitTaskWriteResult(cmd, flags, resp, "updated")
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "New task title")
	cmd.Flags().Bool("star", false, "Set (true) or clear (false) the star")
	ff.register(cmd)
	return cmd
}

// Literal Use strings (not a shared dynamic builder) so the verify-skill static
// scanner can resolve `tasks complete` / `tasks reopen` from source.
func newTasksCompleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "complete <id> [id...]",
		Short:   "Mark one or more tasks complete by id (batched up to 50 per call)",
		Example: "  toodledo-pp-cli tasks complete 123456 234567",
		RunE:    taskStatusRunE(flags, "completed", func() any { return time.Now().Unix() }, "completed"),
	}
}

func newTasksReopenCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "reopen <id> [id...]",
		Short:   "Reopen one or more completed tasks by id, clearing their completion date (batched up to 50)",
		Example: "  toodledo-pp-cli tasks reopen 123456",
		RunE:    taskStatusRunE(flags, "completed", func() any { return 0 }, "reopened"),
	}
}

// taskStatusRunE sets the `completed` field on one or more task ids via
// tasks/edit.php (batched to 50). Shared by complete and reopen.
func taskStatusRunE(flags *rootFlags, field string, value func() any, verb string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return emitTaskDryRun(cmd, flags, "/tasks/edit.php", verb)
		}
		ids, err := parseIDArgs(args)
		if err != nil {
			return err
		}
		if cliutil.IsVerifyEnv() {
			fmt.Fprintf(cmd.OutOrStdout(), "would %s %d task(s)\n", verb, len(ids))
			return nil
		}
		done := 0
		for i := 0; i < len(ids); i += 50 {
			end := i + 50
			if end > len(ids) {
				end = len(ids)
			}
			batch := make([]map[string]any, 0, end-i)
			for _, id := range ids[i:end] {
				batch = append(batch, map[string]any{"id": id, field: value()})
			}
			resp, err := postTasksBatch(cmd, flags, "/tasks/edit.php", batch)
			if err != nil {
				return fmt.Errorf("%s tasks (%d already %s): %w", verb, done, verb, err)
			}
			var arr []json.RawMessage
			if json.Unmarshal(resp, &arr) == nil {
				done += len(arr)
			} else {
				done += len(batch)
			}
		}
		if flags.asJSON || flags.agent {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{verb: done}, flags)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %d task(s).\n", capitalizeFirst(verb), done)
		return nil
	}
}

func newTasksDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "delete <id> [id...]",
		Short:       "Permanently delete one or more tasks by id (irreversible; batched up to 50)",
		Example:     "  toodledo-pp-cli tasks delete 123456",
		Annotations: map[string]string{"mcp:destructive": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitTaskDryRun(cmd, flags, "/tasks/delete.php", "delete")
			}
			ids, err := parseIDArgs(args)
			if err != nil {
				return err
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would delete %d task(s)\n", len(ids))
				return nil
			}
			deleted := 0
			for i := 0; i < len(ids); i += 50 {
				end := i + 50
				if end > len(ids) {
					end = len(ids)
				}
				batch := ids[i:end]
				if _, err := postTasksBatch(cmd, flags, "/tasks/delete.php", batch); err != nil {
					return err
				}
				deleted += len(batch)
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"deleted": deleted}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %d task(s).\n", deleted)
			return nil
		},
	}
	return cmd
}

func parseIDArgs(args []string) ([]int, error) {
	if len(args) == 0 {
		return nil, usageErr(fmt.Errorf("at least one task id is required"))
	}
	ids := make([]int, 0, len(args))
	for _, a := range args {
		n, err := strconv.Atoi(strings.TrimSpace(a))
		if err != nil {
			return nil, usageErr(fmt.Errorf("invalid task id %q", a))
		}
		ids = append(ids, n)
	}
	return ids, nil
}

func emitTaskWriteResult(cmd *cobra.Command, flags *rootFlags, resp json.RawMessage, verb string) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printOutputWithFlags(cmd.OutOrStdout(), resp, flags)
	}
	var arr []map[string]any
	if json.Unmarshal(resp, &arr) == nil && len(arr) > 0 {
		if id, ok := arr[0]["id"]; ok {
			fmt.Fprintf(cmd.OutOrStdout(), "Task %v %s.\n", id, verb)
			return nil
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Task %s.\n", verb)
	return nil
}

// emitTaskDryRun renders a dry-run preview for the hand-built task write
// commands. Like the generated endpoint mirrors, --dry-run must still produce
// output — a JSON envelope under --json/--agent (or when piped), a one-line note
// at a terminal — rather than exiting silently, so a machine caller piping
// --json gets valid JSON back instead of an empty stream.
func emitTaskDryRun(cmd *cobra.Command, flags *rootFlags, path, verb string) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"action":   verb,
			"resource": "tasks",
			"path":     path,
			"dry_run":  true,
			"success":  false,
		}, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "(dry run) would %s; no request sent (%s)\n", verb, path)
	return nil
}

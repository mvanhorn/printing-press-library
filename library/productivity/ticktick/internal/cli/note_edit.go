// Copyright 2026 Harvey The AI Guy and contributors. Licensed under Apache-2.0. See LICENSE.
// Corruption-proof daily-note editing. The update payload is built from a
// field whitelist and can never include `kind` — sending kind on a TEXT-kind
// note converts it to NOTE and breaks childIds (real incident, 2026-07-07).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/ticktick/internal/client"
)

// noteUpdateWhitelist is the complete set of fields a note edit may send.
// `kind` is intentionally absent and must never be added.
var noteUpdateWhitelist = []string{
	"id", "projectId", "title", "content",
	"startDate", "dueDate", "timeZone", "isAllDay", "etag",
}

// pp:data-source live
func newNovelNoteEditCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	var flagAppend string
	var flagSetContent string
	var flagTaskID string
	var flagProjectID string

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a daily note's content via a corruption-proof field whitelist",
		Long: "Use this command for editing the daily note's content safely. Do NOT use it for generic task field updates; use 'tasks batch' instead.\n" +
			"The update payload is built from a strict field whitelist and never includes 'kind', so the note's TEXT kind and subtasks cannot be corrupted. The current etag is carried automatically, with one retry on conflict.",
		Example: "  ticktick-pp-cli note edit --date today --append \"20:15 wrapped the printing-press build\" --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run": true,
						"action":  "would edit the daily note via the whitelisted field set (never sends kind)",
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "would edit the daily note via the whitelisted field set (never sends kind)")
				return nil
			}
			if flagAppend == "" && flagSetContent == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--append or --set-content is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			taskID, projectID := flagTaskID, flagProjectID
			if projectID == "" {
				projectID = os.Getenv("TICKTICK_NOTES_PROJECT")
			}
			if taskID == "" {
				if projectID == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("provide --task-id, or --project-id (or TICKTICK_NOTES_PROJECT) so the note can be located by date"))
				}
				taskID, err = locateNoteTask(ctx, c, projectID, flagDate)
				if err != nil {
					return err
				}
			}
			if projectID == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--project-id is required when using --task-id"))
			}

			result, err := editNoteContent(ctx, c, taskID, projectID, flagAppend, flagSetContent, true)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Note %s updated (etag %v -> %v)\n", taskID, result["previous_etag"], result["new_etag"])
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "today", "Note date to locate: 'today', 'yesterday', or YYYY-MM-DD")
	cmd.Flags().StringVar(&flagAppend, "append", "", "Append this text as a new line at the end of the note content")
	cmd.Flags().StringVar(&flagSetContent, "set-content", "", "Replace the entire note content with this text")
	cmd.Flags().StringVar(&flagTaskID, "task-id", "", "Target task id directly (skips date-based lookup)")
	cmd.Flags().StringVar(&flagProjectID, "project-id", "", "Project id containing the note (or set TICKTICK_NOTES_PROJECT)")
	return cmd
}

// locateNoteTask finds the note task in projectID whose startDate matches the
// requested date, preferring TEXT-kind tasks when several match.
func locateNoteTask(ctx context.Context, c *client.Client, projectID, dateSpec string) (string, error) {
	target, err := resolveNoteDate(dateSpec)
	if err != nil {
		return "", err
	}
	data, err := c.Get(ctx, "/batch/check/0", nil)
	if err != nil {
		return "", apiErr(fmt.Errorf("listing tasks: %w", err))
	}
	var check struct {
		SyncTaskBean struct {
			Update []map[string]json.RawMessage `json:"update"`
		} `json:"syncTaskBean"`
	}
	if err := json.Unmarshal(data, &check); err != nil {
		return "", apiErr(fmt.Errorf("parsing task list: %w", err))
	}
	var matches []map[string]json.RawMessage
	for _, t := range check.SyncTaskBean.Update {
		if rawStr(t["projectId"]) != projectID {
			continue
		}
		if !strings.HasPrefix(rawStr(t["startDate"]), target) {
			continue
		}
		// Only note-kind tasks are eligible: a regular task or checklist with
		// today's startDate must never be selected as "the daily note".
		if !isNoteKind(rawStr(t["kind"])) {
			continue
		}
		matches = append(matches, t)
	}
	if len(matches) == 0 {
		return "", notFoundErr(fmt.Errorf("no note task in project %s with start date %s", projectID, target))
	}
	if len(matches) > 1 {
		for _, m := range matches {
			if rawStr(m["kind"]) == "TEXT" {
				return rawStr(m["id"]), nil
			}
		}
		titles := make([]string, 0, len(matches))
		for _, m := range matches {
			titles = append(titles, fmt.Sprintf("%s (%s)", rawStr(m["title"]), rawStr(m["id"])))
		}
		return "", usageErr(fmt.Errorf("multiple tasks match %s; use --task-id to pick one of: %s", target, strings.Join(titles, ", ")))
	}
	return rawStr(matches[0]["id"]), nil
}

// editNoteContent fetches the full task, mutates content, and sends a
// whitelist-filtered batch update. Retries once on per-item errors (etag races).
func editNoteContent(ctx context.Context, c *client.Client, taskID, projectID, appendText, setContent string, allowRetry bool) (map[string]any, error) {
	raw, err := c.GetNoCache(ctx, "/task/"+taskID, map[string]string{"projectId": projectID})
	if err != nil {
		return nil, apiErr(fmt.Errorf("fetching task %s: %w", taskID, err))
	}
	var task map[string]json.RawMessage
	if err := json.Unmarshal(raw, &task); err != nil {
		return nil, apiErr(fmt.Errorf("parsing task: %w", err))
	}

	// Central kind guard: every write path (date lookup, --task-id direct)
	// funnels through here, so a non-note target can never be overwritten.
	if kind := rawStr(task["kind"]); !isNoteKind(kind) {
		return nil, usageErr(fmt.Errorf("task %s has kind %q, not a note; refusing to edit — use 'tasks batch' for generic task updates", taskID, kind))
	}

	content := rawStr(task["content"])
	if setContent != "" {
		content = setContent
	}
	if appendText != "" {
		// Idempotent append: if the content already ends with the append text
		// (e.g. a retry after the server applied the update but returned a
		// per-item error), do not append it again.
		if strings.HasSuffix(strings.TrimRight(content, "\n"), appendText) {
			return map[string]any{
				"updated":         false,
				"already_applied": true,
				"task_id":         taskID,
				"project_id":      projectID,
				"previous_etag":   rawStr(task["etag"]),
				"new_etag":        rawStr(task["etag"]),
				"content_bytes":   len(content),
			}, nil
		}
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += appendText
	}

	update := map[string]any{}
	for _, key := range noteUpdateWhitelist {
		if v, ok := task[key]; ok {
			update[key] = json.RawMessage(v)
		}
	}
	update["content"] = content

	payload := map[string]any{"update": []any{update}}
	respRaw, status, err := c.Post(ctx, "/batch/task", payload)
	if err != nil {
		return nil, apiErr(fmt.Errorf("updating note: %w", err))
	}
	if status < 200 || status >= 300 {
		return nil, apiErr(fmt.Errorf("note update rejected (HTTP %d)", status))
	}
	var resp struct {
		ID2Etag  map[string]string          `json:"id2etag"`
		ID2Error map[string]json.RawMessage `json:"id2error"`
	}
	_ = json.Unmarshal(respRaw, &resp)
	if len(resp.ID2Error) > 0 {
		if allowRetry {
			return editNoteContent(ctx, c, taskID, projectID, appendText, setContent, false)
		}
		return nil, apiErr(fmt.Errorf("note update failed per-item: %v", resp.ID2Error))
	}
	return map[string]any{
		"updated":       true,
		"task_id":       taskID,
		"project_id":    projectID,
		"previous_etag": rawStr(task["etag"]),
		"new_etag":      resp.ID2Etag[taskID],
		"content_bytes": len(content),
	}, nil
}

// resolveNoteDate turns 'today'/'yesterday'/YYYY-MM-DD into a YYYY-MM-DD prefix.
func resolveNoteDate(spec string) (string, error) {
	now := time.Now()
	switch strings.ToLower(strings.TrimSpace(spec)) {
	case "", "today":
		return now.Format("2006-01-02"), nil
	case "yesterday":
		return now.AddDate(0, 0, -1).Format("2006-01-02"), nil
	case "tomorrow":
		return now.AddDate(0, 0, 1).Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", spec); err != nil {
		return "", usageErr(fmt.Errorf("--date must be today, yesterday, tomorrow, or YYYY-MM-DD (got %q)", spec))
	}
	return spec, nil
}

// rawStr unwraps a JSON string value; non-strings return their raw text.
func rawStr(r json.RawMessage) string {
	if len(r) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(r, &s); err == nil {
		return s
	}
	return string(r)
}

// isNoteKind reports whether a task kind is editable as a note. Daily notes
// are TEXT; legacy notes may report NOTE. Everything else (CHECKLIST, "" for
// plain tasks) is refused by the note-edit write path.
func isNoteKind(kind string) bool {
	return kind == "TEXT" || kind == "NOTE"
}

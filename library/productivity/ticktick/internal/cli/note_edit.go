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
	var flagSection string
	var flagSectionMap string

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a daily note's content via a corruption-proof field whitelist",
		Long: "Use this command for editing the daily note's content safely. Do NOT use it for generic task field updates; use 'tasks batch' instead.\n" +
			"The update payload is built from a strict field whitelist and never includes 'kind', so the note's TEXT kind and subtasks cannot be corrupted. The current etag is carried automatically, with one retry on conflict.\n\n" +
			"Bare --append adds the line to the END of the note. If your note is divided into headings, use --section \"<heading text>\" to insert at the end of that heading's block instead — an absent or ambiguous heading is an error, never a silent append at the end.\n\n" +
			"TickTick has no native daily note; any headings are your own convention. --section auto is a convenience for the common Morning/Afternoon/Evening journal: it picks a heading from the clock (before 12:00 / 12:00-17:59 / 18:00+). Remap it with --section-map or TICKTICK_NOTE_SECTIONS.",
		Example: "  ticktick-pp-cli note edit --date today --section \"## Log\" --append \"- shipped the section flag\"\n" +
			"  ticktick-pp-cli note edit --date today --section auto --append \"- *afternoon entry*\"\n" +
			"  ticktick-pp-cli note edit --date today --section auto --section-map \"afternoon=## Afternoon\" --append \"- entry\"\n" +
			"  ticktick-pp-cli note edit --date today --append \"20:15 wrapped the printing-press build\" --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			// Validate flags BEFORE the dry-run short-circuit. A --dry-run that
			// skips validation reports "would edit" for a command that could only
			// ever fail, which makes it useless as a pre-flight check.
			if flagAppend == "" && flagSetContent == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--append or --set-content is required"))
			}
			if flagSection != "" && flagSetContent != "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--section cannot be combined with --set-content (set-content replaces the whole note)"))
			}
			usedAuto := strings.EqualFold(strings.TrimSpace(flagSection), "auto")
			section, err := resolveSection(flagSection, flagSectionMap)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				action := "would edit the daily note via the whitelisted field set (never sends kind)"
				if section != "" {
					action = fmt.Sprintf("would insert the appended line at the end of the %q block (never sends kind); the block must exist or the real run errors", section)
				}
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"dry_run": true,
						"section": section,
						"action":  action,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), action)
				return nil
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

			result, err := editNoteContent(ctx, c, taskID, projectID, flagAppend, flagSetContent, section, true)
			if err != nil {
				if usedAuto {
					// TickTick has no native daily note, so `auto`'s default
					// headers are a convention, not a guarantee. Say so here
					// rather than leaving the user staring at a header they
					// never chose.
					return usageErr(fmt.Errorf("%w\n\n--section auto resolved the current hour to the heading %q, which this note does not contain.\nTickTick has no built-in daily-note headings — set your own with --section-map \"morning=<hdr>,afternoon=<hdr>,evening=<hdr>\"\nor the TICKTICK_NOTE_SECTIONS env var, or pass --section \"<heading text>\" directly", err, section))
				}
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
	cmd.Flags().StringVar(&flagSection, "section", "", "Insert --append text at the end of the note heading containing this text (e.g. \"~Afternoon~\", \"## Log\"). The reserved value \"auto\" picks a heading from the clock. Default: append at end of note")
	cmd.Flags().StringVar(&flagSectionMap, "section-map", "", "Headings --section auto maps to, e.g. \"morning=~Morning~,afternoon=~Afternoon~,evening=~Evening~\" (or set TICKTICK_NOTE_SECTIONS)")
	return cmd
}

// defaultSectionMap is the time-of-day → header mapping used by `--section
// auto`. TickTick has no native "daily note": the note, and any headings inside
// it, are a convention the user invents. These defaults describe one common
// journal layout, and are entirely overridable — see resolveAutoSection. They
// are a default, never an assumption: if the note has no matching header, auto
// fails loudly rather than filing the entry somewhere plausible.
var defaultSectionMap = [3]string{"~Morning~", "~Afternoon~", "~Evening~"}

// sectionMapFromSpec parses "morning=<hdr>,afternoon=<hdr>,evening=<hdr>".
// Missing keys keep their default.
func sectionMapFromSpec(spec string) ([3]string, error) {
	out := defaultSectionMap
	if strings.TrimSpace(spec) == "" {
		return out, nil
	}
	for _, part := range strings.Split(spec, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(v) == "" {
			return out, usageErr(fmt.Errorf("--section-map entries must look like morning=<header text> (got %q)", part))
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "morning":
			out[0] = strings.TrimSpace(v)
		case "afternoon":
			out[1] = strings.TrimSpace(v)
		case "evening":
			out[2] = strings.TrimSpace(v)
		default:
			return out, usageErr(fmt.Errorf("--section-map key must be morning, afternoon, or evening (got %q)", k))
		}
	}
	return out, nil
}

// resolveAutoSection turns `auto` into the header text for the current local
// hour. Precedence: --section-map flag, then TICKTICK_NOTE_SECTIONS, then the
// built-in default.
func resolveAutoSection(mapSpec string) (string, error) {
	if strings.TrimSpace(mapSpec) == "" {
		mapSpec = os.Getenv("TICKTICK_NOTE_SECTIONS")
	}
	m, err := sectionMapFromSpec(mapSpec)
	if err != nil {
		return "", err
	}
	switch h := time.Now().Hour(); {
	case h < 12:
		return m[0], nil
	case h < 18:
		return m[1], nil
	default:
		return m[2], nil
	}
}

// resolveSection normalizes --section into the header text to search for. ""
// means "append at the end of the note" (the original behavior). Any other
// value is matched literally against the note's headers, except the reserved
// word `auto`, which is resolved from the clock.
func resolveSection(spec, mapSpec string) (string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		if strings.TrimSpace(mapSpec) != "" {
			return "", usageErr(fmt.Errorf("--section-map has no effect without --section auto"))
		}
		return "", nil
	}
	if strings.EqualFold(spec, "auto") {
		return resolveAutoSection(mapSpec)
	}
	return spec, nil
}

// isHeadingLine reports whether a line reads as a section heading in a Markdown
// note: an ATX heading (`## Foo`), a fully-bold line (`**🌙 ~Evening~**`), or a
// horizontal rule. Deliberately structural — the CLI must not care what a
// user's headings are named.
func isHeadingLine(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return false
	}
	if t == "---" || t == "***" || t == "___" {
		return true
	}
	if strings.HasPrefix(t, "#") {
		return true
	}
	if len(t) >= 4 && strings.HasPrefix(t, "**") && strings.HasSuffix(t, "**") {
		return true
	}
	return false
}

// sectionHeaderIdxs returns the index of every *heading* line containing needle
// (case-insensitive). Only headings are candidates: a note's body freely
// mentions the same words as its headings — a checklist item reading
// "📥 Morning: pull agenda" must never be mistaken for the "~Morning~" header.
// More than one match is ambiguous and must not be silently resolved to the first.
func sectionHeaderIdxs(lines []string, needle string) []int {
	n := strings.ToLower(needle)
	var out []int
	for i, l := range lines {
		if isHeadingLine(l) && strings.Contains(strings.ToLower(l), n) {
			out = append(out, i)
		}
	}
	return out
}

// insertIntoSection places text at the end of the block whose header contains
// `header` — after the block's last non-blank line, before the next heading.
//
// It errors when the header is absent or ambiguous rather than falling back to
// an end-of-note append. That silent fallback is what quietly filed afternoon
// entries under Evening (real incident, 2026-07-09), and picking the first of
// several matches would be the same mistake wearing a different hat.
func insertIntoSection(content, header, text string) (string, error) {
	lines := strings.Split(content, "\n")
	hits := sectionHeaderIdxs(lines, header)
	if len(hits) == 0 {
		return "", usageErr(fmt.Errorf("note has no heading containing %q; refusing to append at the end of the note instead. Pass --section with text that matches one of this note's headings", header))
	}
	if len(hits) > 1 {
		found := make([]string, 0, len(hits))
		for _, i := range hits {
			found = append(found, strings.TrimSpace(lines[i]))
		}
		return "", usageErr(fmt.Errorf("%q matches %d headings (%s); pass a more specific --section", header, len(hits), strings.Join(found, " | ")))
	}
	start := hits[0]

	// Walk to the block's end: the next heading of any kind, or EOF.
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if isHeadingLine(lines[i]) {
			end = i
			break
		}
	}
	// Back off trailing blank lines so the insert lands after real content.
	last := end
	for last > start+1 && strings.TrimSpace(lines[last-1]) == "" {
		last--
	}

	// Match the block's own spacing. A block with fewer than two entries offers
	// no evidence, so fall back to the document's prevailing style rather than
	// silently defaulting to flush. (A note can carry both styles — plain
	// --append joins with a single newline, --set-content preserves blanks.)
	blank, decided := blankLineStyle(lines[start+1 : last])
	if !decided {
		blank, _ = blankLineStyle(lines)
	}
	ins := []string{text}
	if blank {
		ins = []string{"", text}
	}

	out := append([]string{}, lines[:last]...)
	out = append(out, ins...)
	out = append(out, lines[last:]...)
	return strings.Join(out, "\n"), nil
}

// blankLineStyle reports whether seg separates its list entries with blank
// lines, judged by the majority of the gaps between them. `decided` is false
// when seg holds fewer than two entries, i.e. there is no gap to sample and the
// caller should look at a wider scope rather than assume.
func blankLineStyle(seg []string) (blank bool, decided bool) {
	gaps, blanks := 0, 0
	for i := 1; i < len(seg)-1; i++ {
		if strings.HasPrefix(seg[i-1], "- ") && strings.HasPrefix(seg[i+1], "- ") {
			gaps++
			if strings.TrimSpace(seg[i]) == "" {
				blanks++
			}
		}
	}
	if gaps == 0 {
		return false, false
	}
	return blanks*2 > gaps, true
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
func editNoteContent(ctx context.Context, c *client.Client, taskID, projectID, appendText, setContent, section string, allowRetry bool) (map[string]any, error) {
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
		// Idempotent append. A plain append lands at the tail, so HasSuffix is
		// enough; a section insert lands mid-document, so it needs Contains.
		applied := strings.HasSuffix(strings.TrimRight(content, "\n"), appendText)
		if section != "" {
			applied = strings.Contains(content, appendText)
		}
		if applied {
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
		if section != "" {
			updated, err := insertIntoSection(content, section, appendText)
			if err != nil {
				return nil, err
			}
			content = updated
		} else {
			if content != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += appendText
		}
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
			return editNoteContent(ctx, c, taskID, projectID, appendText, setContent, section, false)
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

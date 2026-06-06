// Copyright 2026 Peter Yang and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/substack-notes/internal/client"
	"github.com/spf13/cobra"
)

type noteTextFlags struct {
	text      string
	file      string
	stdin     bool
	images    []string
	replyRole string
	surface   string
	tabID     string
}

func newNotesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "notes",
		Aliases: []string{"note", "drafts"},
		Short:   "Post, schedule, and manage Substack Notes",
		RunE:    parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newNotesPostCmd(flags))
	cmd.AddCommand(newNotesScheduleCmd(flags))
	cmd.AddCommand(newNotesDraftCmd(flags))
	cmd.AddCommand(newNotesListCmd(flags))
	cmd.AddCommand(newNotesRecentCmd(flags))
	cmd.AddCommand(newNotesDeleteCmd(flags))
	return cmd
}

func newNotesPostCmd(flags *rootFlags) *cobra.Command {
	var note noteTextFlags
	cmd := &cobra.Command{
		Use:   "post",
		Short: "Publish a note immediately",
		Example: strings.Join([]string{
			`  substack-notes-pp-cli notes post --text "Shipping a tiny thing today."`,
			`  substack-notes-pp-cli notes post --file note.txt`,
			`  substack-notes-pp-cli notes post --text "New demo" --image ./demo.png`,
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := noteRequestBody(cmd, note, nil)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, err = attachNoteImages(cmd.Context(), c, body, note.images)
			if err != nil {
				return err
			}
			data, _, err := c.PostWithParams(cmd.Context(), "/api/v1/comment/feed", map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	addNoteTextFlags(cmd, &note)
	return cmd
}

func newNotesScheduleCmd(flags *rootFlags) *cobra.Command {
	var note noteTextFlags
	var at string
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Schedule a note",
		Example: strings.Join([]string{
			`  substack-notes-pp-cli notes schedule --text "Tomorrow's note" --at "2026-07-15 09:00"`,
			`  substack-notes-pp-cli notes schedule --file note.txt --at "2026-07-15T09:00:00-07:00"`,
			`  substack-notes-pp-cli notes schedule --text "Image note" --image ./image.webp --at "2026-07-15T09:00:00-07:00"`,
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(at) == "" {
				return usageErr(fmt.Errorf("required flag %q not set", "at"))
			}
			triggerAt, err := parseNoteScheduleTime(at)
			if err != nil {
				return usageErr(err)
			}
			body, err := noteRequestBody(cmd, note, &triggerAt)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, err = attachNoteImages(cmd.Context(), c, body, note.images)
			if err != nil {
				return err
			}
			data, _, err := c.PostWithParams(cmd.Context(), "/api/v1/comment/draft", map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	addNoteTextFlags(cmd, &note)
	cmd.Flags().StringVar(&at, "at", "", "Schedule time, e.g. 2026-07-15 09:00 or RFC3339")
	return cmd
}

func newNotesDraftCmd(flags *rootFlags) *cobra.Command {
	var note noteTextFlags
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Save a note draft without scheduling it",
		Example: strings.Join([]string{
			`  substack-notes-pp-cli notes draft --text "Keep this for later."`,
			`  cat note.txt | substack-notes-pp-cli notes draft --stdin`,
			`  substack-notes-pp-cli notes draft --file note.txt --image ./cover.jpg`,
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := noteRequestBody(cmd, note, nil)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body, err = attachNoteImages(cmd.Context(), c, body, note.images)
			if err != nil {
				return err
			}
			data, _, err := c.PostWithParams(cmd.Context(), "/api/v1/comment/draft", map[string]string{}, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	addNoteTextFlags(cmd, &note)
	return cmd
}

func newNotesListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List note drafts and scheduled notes",
		Example: strings.Join([]string{
			`  substack-notes-pp-cli notes list --limit 20`,
			`  substack-notes-pp-cli notes list --limit 20 --cursor <cursor> --json`,
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"limit": strconv.Itoa(limit)}
			if cursor != "" {
				params["cursor"] = cursor
			}
			data, err := c.Get(cmd.Context(), "/api/v1/feed/drafts", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum drafts to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	return cmd
}

func newNotesRecentCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var cursor string
	var userID string
	cmd := &cobra.Command{
		Use:   "recent",
		Short: "Read recent published notes from a Substack profile",
		Example: strings.Join([]string{
			`  substack-notes-pp-cli notes recent --limit 5`,
			`  substack-notes-pp-cli notes recent --user-id <numeric-user-id> --json`,
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resolvedUserID := strings.TrimSpace(userID)
			if resolvedUserID == "" {
				resolvedUserID, err = detectCurrentSubstackUserID(cmd.Context(), c)
				if err != nil {
					return err
				}
			}
			if _, err := strconv.ParseInt(resolvedUserID, 10, 64); err != nil {
				return usageErr(fmt.Errorf("--user-id must be numeric: %w", err))
			}
			params := map[string]string{"limit": strconv.Itoa(limit)}
			if cursor != "" {
				params["cursor"] = cursor
			}
			data, err := c.Get(cmd.Context(), "/api/v1/reader/feed/profile/"+resolvedUserID, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			notes, err := extractRecentNotes(data, limit)
			if err != nil {
				return err
			}
			out, err := json.Marshal(notes)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum notes to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	cmd.Flags().StringVar(&userID, "user-id", "", "Substack user id; defaults to the authenticated user")
	return cmd
}

func newNotesDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <draft-id>",
		Short:   "Delete a note draft or scheduled note",
		Example: `  substack-notes-pp-cli notes delete <draft-id>`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := strconv.ParseInt(args[0], 10, 64); err != nil {
				return usageErr(fmt.Errorf("draft-id must be numeric: %w", err))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Delete(cmd.Context(), "/api/v1/comment/"+args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

func detectCurrentSubstackUserID(ctx context.Context, c *client.Client) (string, error) {
	data, err := c.Get(ctx, "/notes", map[string]string{})
	if err != nil {
		return "", classifyAPIError(err, nil)
	}
	text := string(data)
	for _, pattern := range []string{
		`\\\"user\\\":\{\\\"id\\\":(\d+)`,
		`"user":\{"id":(\d+)`,
	} {
		match := regexp.MustCompile(pattern).FindStringSubmatch(text)
		if len(match) == 2 {
			return match[1], nil
		}
	}
	return "", fmt.Errorf("could not detect current Substack user id; pass --user-id")
}

type recentNote struct {
	ID           any              `json:"id,omitempty"`
	Date         any              `json:"date,omitempty"`
	Type         any              `json:"type,omitempty"`
	Body         string           `json:"body,omitempty"`
	CanonicalURL any              `json:"canonical_url,omitempty"`
	URL          any              `json:"url,omitempty"`
	Attachments  []noteAttachment `json:"attachments,omitempty"`
}

type noteAttachment struct {
	ID     any `json:"id,omitempty"`
	Type   any `json:"type,omitempty"`
	URL    any `json:"url,omitempty"`
	Width  any `json:"width,omitempty"`
	Height any `json:"height,omitempty"`
}

func extractRecentNotes(data json.RawMessage, limit int) ([]recentNote, error) {
	var envelope struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing profile feed: %w", err)
	}
	notes := make([]recentNote, 0, limit)
	for _, item := range envelope.Items {
		if itemType, _ := item["type"].(string); itemType == "latest_post" {
			continue
		}
		obj := item
		for _, nested := range []string{"comment", "item", "entity"} {
			if candidate, ok := item[nested].(map[string]any); ok {
				obj = candidate
				break
			}
		}
		body := noteBodyFromFeedItem(obj)
		if body == "" {
			continue
		}
		notes = append(notes, recentNote{
			ID:           obj["id"],
			Date:         obj["date"],
			Type:         obj["type"],
			Body:         body,
			CanonicalURL: obj["canonical_url"],
			URL:          obj["url"],
			Attachments:  noteAttachmentsFromFeedItem(obj),
		})
		if len(notes) >= limit {
			break
		}
	}
	return notes, nil
}

func noteBodyFromFeedItem(item map[string]any) string {
	for _, key := range []string{"body", "text", "html", "body_html"} {
		if value, ok := item[key].(string); ok {
			return cleanNoteBody(value)
		}
	}
	return ""
}

func noteAttachmentsFromFeedItem(item map[string]any) []noteAttachment {
	values, ok := item["attachments"].([]any)
	if !ok {
		return nil
	}
	out := make([]noteAttachment, 0, len(values))
	for _, value := range values {
		attachment, ok := value.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, noteAttachment{
			ID:     attachment["id"],
			Type:   attachment["type"],
			URL:    attachment["url"],
			Width:  attachment["width"],
			Height: attachment["height"],
		})
	}
	return out
}

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

func cleanNoteBody(value string) string {
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

func addNoteTextFlags(cmd *cobra.Command, note *noteTextFlags) {
	cmd.Flags().StringVar(&note.text, "text", "", "Plain text note body")
	cmd.Flags().StringVar(&note.file, "file", "", "Read plain text note body from a file")
	cmd.Flags().BoolVar(&note.stdin, "stdin", false, "Read plain text note body from stdin")
	cmd.Flags().StringArrayVar(&note.images, "image", nil, "Attach an image file; repeat for multiple images")
	cmd.Flags().StringVar(&note.replyRole, "reply-role", "everyone", "Who can reply: everyone, free_subscriber, paid_subscriber")
	cmd.Flags().StringVar(&note.surface, "surface", "", "Optional Substack surface value")
	cmd.Flags().StringVar(&note.tabID, "tab-id", "", "Optional Substack feed tab id")
}

func noteRequestBody(cmd *cobra.Command, note noteTextFlags, triggerAt *time.Time) (map[string]any, error) {
	text, err := resolveNoteText(cmd, note)
	if err != nil {
		return nil, err
	}
	if note.replyRole != "everyone" && note.replyRole != "free_subscriber" && note.replyRole != "paid_subscriber" {
		return nil, usageErr(fmt.Errorf("--reply-role must be one of everyone, free_subscriber, paid_subscriber"))
	}
	body := map[string]any{
		"bodyJson":         proseMirrorDoc(text),
		"replyMinimumRole": note.replyRole,
	}
	if note.surface != "" {
		body["surface"] = note.surface
	}
	if note.tabID != "" {
		body["tabId"] = note.tabID
	}
	if triggerAt != nil {
		body["trigger_at"] = triggerAt.Format(time.RFC3339)
	}
	return body, nil
}

func resolveNoteText(cmd *cobra.Command, note noteTextFlags) (string, error) {
	sources := 0
	if note.text != "" {
		sources++
	}
	if note.file != "" {
		sources++
	}
	if note.stdin {
		sources++
	}
	if sources != 1 {
		return "", usageErr(fmt.Errorf("provide exactly one of --text, --file, or --stdin"))
	}
	var text string
	switch {
	case note.text != "":
		text = note.text
	case note.file != "":
		data, err := os.ReadFile(note.file)
		if err != nil {
			return "", fmt.Errorf("reading --file: %w", err)
		}
		text = string(data)
	case note.stdin:
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		text = string(data)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", usageErr(fmt.Errorf("note text is empty"))
	}
	return text, nil
}

func proseMirrorDoc(text string) map[string]any {
	blocks := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	content := make([]any, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		content = append(content, map[string]any{
			"type": "paragraph",
			"content": []any{
				map[string]any{
					"type": "text",
					"text": block,
				},
			},
		})
	}
	return map[string]any{
		"type": "doc",
		"attrs": map[string]any{
			"schemaVersion": "v1",
		},
		"content": content,
	}
}

func parseNoteScheduleTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
	}
	for _, layout := range formats {
		if layout == time.RFC3339 {
			if t, err := time.Parse(layout, raw); err == nil {
				return t, nil
			}
			continue
		}
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("--at must be RFC3339 or local time like 2026-07-15 09:00")
}

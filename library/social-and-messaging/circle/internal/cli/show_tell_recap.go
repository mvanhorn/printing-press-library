// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil"

	"github.com/spf13/cobra"
)

type showTellRecapEvent struct {
	ID       json.Number `json:"id"`
	Name     string      `json:"name"`
	StartsAt time.Time   `json:"starts_at"`
}

type showTellRecapPost struct {
	Applied           bool        `json:"applied"`
	ID                json.Number `json:"id,omitempty"`
	SpaceID           json.Number `json:"space_id"`
	Name              string      `json:"name"`
	Status            string      `json:"status"`
	SkipNotifications bool        `json:"skip_notifications"`
}

type showTellRecap struct {
	Event           showTellRecapEvent `json:"event"`
	Attendees       []circleAttendee   `json:"attendees"`
	RoomID          *json.Number       `json:"room_id"`
	TranscriptFound bool               `json:"transcript_found"`
	Markdown        string             `json:"markdown"`
	Post            *showTellRecapPost `json:"post"`
}

func showTellMarkdown(event circleEvent, duration time.Duration, attendees []circleAttendee, transcript string, hasRoom bool) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# %s recap\n\nDate: %s\n\nDuration: %s\n\nAttendees: %d\n", event.Name, event.StartsAt.UTC().Format(time.RFC3339), duration, len(attendees))
	for _, attendee := range attendees {
		fmt.Fprintf(&out, "- %s\n", strings.ReplaceAll(attendee.Name, "\n", " "))
	}
	out.WriteString("\n## Transcript\n\n")
	if transcript != "" {
		out.WriteString(transcript)
	} else if !hasRoom {
		out.WriteString("No live room recording found")
	} else {
		out.WriteString("No transcript available")
	}
	out.WriteByte('\n')
	return out.String()
}

func showTellPostBody(space, name, markdown string) map[string]any {
	content := make([]any, 0)
	paragraph := func(text string) map[string]any {
		return map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": text}}}
	}
	var lines []string
	flush := func() {
		if len(lines) > 0 {
			content = append(content, paragraph(strings.Join(lines, " ")))
			lines = nil
		}
	}
	var list map[string]any
	// shortcut: supports recap headings, paragraphs and flat bullets; use a Markdown AST for richer formatting.
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			list = nil
			continue
		}
		level := len(line) - len(strings.TrimLeft(line, "#"))
		if level >= 1 && level <= 6 && strings.HasPrefix(line[level:], " ") {
			flush()
			list = nil
			heading := paragraph(strings.TrimSpace(line[level:]))
			heading["type"], heading["attrs"] = "heading", map[string]any{"level": level}
			content = append(content, heading)
		} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
			flush()
			if list == nil {
				list = map[string]any{"type": "bulletList", "content": make([]any, 0)}
				content = append(content, list)
			}
			list["content"] = append(list["content"].([]any), map[string]any{"type": "listItem", "content": []any{paragraph(strings.TrimSpace(line[2:]))}})
		} else {
			list = nil
			lines = append(lines, line)
		}
	}
	flush()
	return map[string]any{
		"space_id": json.Number(space), "name": name + " recap", "status": "draft", "skip_notifications": true,
		"tiptap_body": map[string]any{"body": map[string]any{"type": "doc", "content": content}},
	}
}

func showTellPostID(raw json.RawMessage) (json.Number, error) {
	var created struct {
		ID   json.Number `json:"id"`
		Post struct {
			ID json.Number `json:"id"`
		} `json:"post"`
	}
	if err := circleDecode(raw, &created); err != nil {
		return "", err
	}
	id := created.Post.ID
	if id == "" {
		id = created.ID
	}
	if err := circlePositiveID(id.String()); err != nil {
		return "", fmt.Errorf("draft post response has no valid ID; inspect posts before retrying: %w", err)
	}
	return id, nil
}

func writeShowTellRecap(path string, data []byte) error {
	info, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink: %s", path)
	}
	return cliutil.AtomicWritePrivateFile(path, data, 0o600, 0o700)
}

func newNovelShowTellRecapCmd(flags *rootFlags) *cobra.Command {
	var room, out, space string
	var post bool
	cmd := &cobra.Command{
		Use:   "recap <event-id>",
		Short: "Pull a past session's attendees and live-room transcript into a markdown recap draft.",
		Long:  "Use this command to assemble attendees and transcript for a past Show & Tell into a draft. Do NOT use this command to create upcoming sessions; use 'show-tell schedule' instead.",
		Example: strings.Trim(`
  circle-pp-cli show-tell recap 3859021 --json
  circle-pp-cli show-tell recap 3859021 --post --space 1000969 --yes
`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "pp:preview-happy-path": "true", "pp:happy-args": "event-id=30000788", "pp:no-error-path-probe": "true", "mcp:write-flags": "out"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "show-tell recap")
			}
			if len(args) != 1 || (post && space == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("requires one event ID and --space when --post is used"))
			}
			if err := circlePositiveID(args[0]); err != nil {
				return usageErr(err)
			}
			if room != "" {
				if err := circlePositiveID(room); err != nil {
					return usageErr(fmt.Errorf("--room: %w", err))
				}
			}
			if space != "" {
				if err := circlePositiveID(space); err != nil {
					return usageErr(fmt.Errorf("--space: %w", err))
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var event circleEvent
			if err := circleGet(ctx, c, "/events/"+args[0], nil, &event); err != nil {
				return err
			}
			start, duration, err := event.schedule()
			if err != nil {
				return err
			}
			attendees, err := circleList[circleAttendee](ctx, c, "/event_attendees", map[string]string{"event_id": args[0]})
			if err != nil {
				return err
			}
			if room == "" {
				rooms, err := circleList[circleRoom](ctx, c, "/live/rooms", nil)
				if err != nil {
					return err
				}
				room = showTellRoom(rooms, start, start.Add(duration)).String()
			}
			transcripts := make([]string, 0)
			if room != "" {
				var response struct {
					Transcripts []string `json:"transcripts"`
				}
				if err := circleGet(ctx, c, "/live/rooms/"+room+"/transcripts", nil, &response); err != nil {
					return err
				}
				for _, entry := range response.Transcripts {
					text, err := circleTranscript(ctx, entry)
					if err != nil {
						return err
					}
					if text != "" {
						transcripts = append(transcripts, text)
					}
				}
			}
			text := strings.Join(transcripts, "\n\n")
			result := showTellRecap{Event: showTellRecapEvent{ID: event.ID, Name: event.Name, StartsAt: start}, Attendees: attendees, TranscriptFound: text != "", Markdown: showTellMarkdown(event, duration, attendees, text, room != "")}
			if room != "" {
				id := json.Number(room)
				result.RoomID = &id
			}
			if out != "" {
				if err := writeShowTellRecap(out, []byte(result.Markdown)); err != nil {
					return err
				}
			}
			if post {
				result.Post = &showTellRecapPost{SpaceID: json.Number(space), Name: event.Name + " recap", Status: "draft", SkipNotifications: true}
				if flags.yes {
					raw, _, err := c.Post(ctx, "/posts", showTellPostBody(space, event.Name, result.Markdown))
					if err != nil {
						return err
					}
					id, err := showTellPostID(raw)
					if err != nil {
						return err
					}
					result.Post.ID, result.Post.Applied = id, true
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if out == "" {
				_, err = fmt.Fprint(cmd.OutOrStdout(), result.Markdown)
			} else {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", out)
			}
			if err != nil {
				return err
			}
			if post {
				if result.Post.Applied {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Created draft post %s.\n", result.Post.ID)
				} else {
					_, err = fmt.Fprintf(cmd.OutOrStdout(), "Would create draft %q in space %s without notifications. Pass --yes to apply.\n", result.Post.Name, space)
				}
			}
			return err
		},
	}
	cmd.Flags().StringVar(&room, "room", "", "Live room ID; otherwise selects the closest room in the event window.")
	cmd.Flags().StringVar(&out, "out", "", "Write the Markdown recap to this file.")
	cmd.Flags().BoolVar(&post, "post", false, "Preview a draft recap post; use --yes to create it without notifications.")
	cmd.Flags().StringVar(&space, "space", "", "Destination space ID for the draft recap post.")
	return cmd
}

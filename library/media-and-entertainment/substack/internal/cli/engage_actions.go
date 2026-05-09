// Phase 3 hand-authored novel command. Not generator-emitted.
//
// Like/Restack/Restack-with-comment endpoints are reverse-engineered from
// the in-browser network panel; community wrappers (e.g. jakub-k-slys/substack-api)
// don't expose them. The default behaviour is print-by-default + opt-in
// --send so the verifier can probe these commands without firing a real
// API call. cliutil.IsVerifyEnv() short-circuits even when --send is set.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type engageResult struct {
	Action    string `json:"action"`
	NoteURL   string `json:"note_url"`
	NoteID    string `json:"note_id"`
	Pattern   string `json:"pattern,omitempty"`
	Body      string `json:"body,omitempty"`
	Sent      bool   `json:"sent"`
	Curl      string `json:"curl,omitempty"`
	VerifyEnv bool   `json:"verify_env_short_circuit,omitempty"`
}

func newEngageLikeCmd(flags *rootFlags) *cobra.Command {
	var noteURL string
	var send bool

	cmd := &cobra.Command{
		Use:   "like",
		Short: "Like a Note — print-by-default; pass --send to fire the request",
		Example: strings.Trim(`
  substack-pp-cli engage like --note-url https://substack.com/@alice/note/c-123
  substack-pp-cli engage like --note-url https://substack.com/@alice/note/c-123 --send
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), engageResult{Action: "like"}, flags)
			}
			if strings.TrimSpace(noteURL) == "" {
				return usageErr(fmt.Errorf("--note-url is required"))
			}
			noteID := extractNoteID(noteURL)
			result := engageResult{
				Action:  "like",
				NoteURL: noteURL,
				NoteID:  noteID,
				Curl:    likeCurl(noteID),
			}
			if !send {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if cliutil.IsVerifyEnv() {
				result.VerifyEnv = true
				fmt.Fprintln(cmd.ErrOrStderr(), "verify mode short-circuit — would have POSTed to /reaction")
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if err := postEngagement(flags, "like", noteID, "", ""); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v — endpoint may have changed; capture from DevTools and run 'substack-pp-cli feedback'\n", err)
			} else {
				result.Sent = true
				recordEngagement(flags, "like", noteID, "")
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&noteURL, "note-url", "", "URL of the Note to like (required)")
	cmd.Flags().BoolVar(&send, "send", false, "Actually fire the request; default prints curl")
	return cmd
}

func newEngageRestackCmd(flags *rootFlags) *cobra.Command {
	var noteURL string
	var send bool

	cmd := &cobra.Command{
		Use:   "restack",
		Short: "Restack a Note — print-by-default; pass --send to fire the request",
		Example: strings.Trim(`
  substack-pp-cli engage restack --note-url https://substack.com/@alice/note/c-123
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), engageResult{Action: "restack"}, flags)
			}
			if strings.TrimSpace(noteURL) == "" {
				return usageErr(fmt.Errorf("--note-url is required"))
			}
			noteID := extractNoteID(noteURL)
			result := engageResult{Action: "restack", NoteURL: noteURL, NoteID: noteID, Curl: restackCurl(noteID, "")}
			if !send {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if cliutil.IsVerifyEnv() {
				result.VerifyEnv = true
				fmt.Fprintln(cmd.ErrOrStderr(), "verify mode short-circuit — would have POSTed to /comment/feed")
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if err := postEngagement(flags, "restack", noteID, "", ""); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
			} else {
				result.Sent = true
				recordEngagement(flags, "restack", noteID, "")
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&noteURL, "note-url", "", "URL of the Note to restack (required)")
	cmd.Flags().BoolVar(&send, "send", false, "Actually fire the request")
	return cmd
}

func newEngageRestackWithCommentCmd(flags *rootFlags) *cobra.Command {
	var noteURL string
	var body string
	var pattern string
	var send bool

	cmd := &cobra.Command{
		Use:   "restack-with-comment",
		Short: "Restack a Note with a comment — endorsement / bridge / comment-first patterns",
		Example: strings.Trim(`
  substack-pp-cli engage restack-with-comment --note-url https://substack.com/@x/note/c-9 --body "this!"
  substack-pp-cli engage restack-with-comment --note-url ... --pattern bridge --body "and here is..."
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), engageResult{Action: "restack-with-comment"}, flags)
			}
			if strings.TrimSpace(noteURL) == "" {
				return usageErr(fmt.Errorf("--note-url is required"))
			}
			switch pattern {
			case "endorsement", "bridge", "comment-first":
			default:
				return usageErr(fmt.Errorf("--pattern must be endorsement|bridge|comment-first (got %q)", pattern))
			}
			noteID := extractNoteID(noteURL)
			result := engageResult{
				Action:  "restack-with-comment",
				NoteURL: noteURL,
				NoteID:  noteID,
				Pattern: pattern,
				Body:    body,
				Curl:    restackCurl(noteID, body),
			}
			if !send {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if cliutil.IsVerifyEnv() {
				result.VerifyEnv = true
				fmt.Fprintln(cmd.ErrOrStderr(), "verify mode short-circuit")
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if err := postEngagement(flags, "restack-with-comment", noteID, body, pattern); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
			} else {
				result.Sent = true
				recordEngagement(flags, "restack-with-comment", noteID, "")
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&noteURL, "note-url", "", "URL of the Note to restack (required)")
	cmd.Flags().StringVar(&body, "body", "", "Comment body (Markdown, converted to ProseMirror)")
	cmd.Flags().StringVar(&pattern, "pattern", "endorsement", "Pattern: endorsement|bridge|comment-first")
	cmd.Flags().BoolVar(&send, "send", false, "Actually fire the request")
	return cmd
}

func extractNoteID(noteURL string) string {
	// Pull the trailing c-NNN id from common Note URL shapes.
	// Examples:
	//   https://substack.com/@alice/note/c-12345
	//   https://substack.com/note/c-12345
	parts := strings.Split(strings.TrimSuffix(noteURL, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, "c-") {
		return last
	}
	return last
}

func likeCurl(noteID string) string {
	return fmt.Sprintf(`curl -X POST 'https://substack.com/api/v1/comment/%s/reaction' \
  -H 'cookie: substack.sid=...; connect.sid=...' \
  -H 'content-type: application/json' \
  -d '{"emoji":"❤"}'`, noteID)
}

func restackCurl(noteID, body string) string {
	return fmt.Sprintf(`curl -X POST 'https://substack.com/api/v1/comment/feed' \
  -H 'cookie: substack.sid=...; connect.sid=...' \
  -H 'content-type: application/json' \
  -d '{"type":"feed","parent_id":"%s","body":%q}'`, noteID, body)
}

func postEngagement(flags *rootFlags, kind, noteID, body, pattern string) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	switch kind {
	case "like":
		_, _, err := c.Post(fmt.Sprintf("/comment/%s/reaction", noteID), map[string]any{"emoji": "❤"})
		return err
	case "restack":
		_, _, err := c.Post("/comment/feed", map[string]any{"type": "feed", "parent_id": noteID})
		return err
	case "restack-with-comment":
		_, _, err := c.Post("/comment/feed", map[string]any{"type": "feed", "parent_id": noteID, "body": body, "pattern": pattern})
		return err
	}
	return fmt.Errorf("unknown engagement kind %q", kind)
}

func recordEngagement(flags *rootFlags, kind, noteID, target string) {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return
	}
	defer st.Close()
	_, _ = st.DB().Exec(
		`INSERT INTO engagements(kind, note_or_post_id, target_handle, by_self) VALUES(?, ?, ?, 1)`,
		kind, noteID, target)
}

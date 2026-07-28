// pp:data-source live

package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/docsbridge"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/notesparse"
)

// The `notes docs` group reads My Notes straight from the user's own Zoom web
// session. The sibling `notes summary` / `notes transcript` commands need
// Server-to-Server OAuth, which only an account admin can install; these need
// nothing but a signed-in browser.
//
// dogfoodNoteCap bounds the live-dogfood matrix, where every command shares a
// flat 30s budget and an unbounded sync would trip it.
const dogfoodNoteCap = 5

func newNotesDocsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Read My Notes and their transcripts using your own Zoom web session (no S2S OAuth, no admin)",
		Long: "Zoom exposes no public REST endpoint for My Notes, and the documented /docs/archives API that does " +
			"carry note transcripts requires docs:read:archive:admin plus account-level Docs archiving. These " +
			"commands use the same Zoom Docs calls the web app makes: browser cookies mint a short-lived bearer, " +
			"which then reads your notes list and each meeting's transcript. Capture a session once with " +
			"`press-auth login zoom.us --login-url https://zoom.us/signin`.",
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newNotesDocsListCmd(flags))
	cmd.AddCommand(newNotesDocsTranscriptCmd(flags))
	cmd.AddCommand(newNotesDocsSyncCmd(flags))
	return cmd
}

func newNotesDocsListCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List My Notes documents from the Zoom web session, newest first",
		Example: `  zoom-pp-cli notes docs list --json
  zoom-pp-cli notes docs list --since 30d --limit 20 --json --select title,meeting_id,updated_at`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceTime, err := parseSince(since)
			if err != nil {
				return err
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"would_call": "GET zoom docs my-notes folder children"})
			}
			effectiveLimit := limit
			if cliutil.IsDogfoodEnv() && (effectiveLimit == 0 || effectiveLimit > dogfoodNoteCap) {
				effectiveLimit = dogfoodNoteCap
			}
			client, err := docsbridge.New(30 * time.Second)
			if err != nil {
				// A listing with no session is an empty listing, not a crash:
				// same contract as querying a local mirror that was never
				// synced. The recovery instructions go to stderr so piped
				// consumers still get a well-formed empty array on stdout.
				var noSession *docsbridge.ErrNoSession
				if errors.As(err, &noSession) {
					fmt.Fprintf(cmd.ErrOrStderr(), "hint: %v\n", noSession)
					return flags.printJSON(cmd, []docsbridge.Note{})
				}
				return err
			}
			notes, err := client.Notes(cmd.Context(), sinceTime, effectiveLimit)
			if err != nil {
				return err
			}
			if notes == nil {
				notes = []docsbridge.Note{}
			}
			return flags.printJSON(cmd, notes)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only notes updated since this point (7d, 30d, ISO date)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max notes (0 = unlimited)")
	return cmd
}

func newNotesDocsTranscriptCmd(flags *rootFlags) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "transcript [note-id|meeting-id]",
		Short: "Fetch one meeting's transcript via the Zoom web session (vtt, md, or json)",
		Long: "Accepts either a note id (as shown by `notes docs list`, or the last path segment of a " +
			"docs.zoom.us/doc/... URL) or a meeting id. A note id is resolved to its meeting id first.",
		Example: `  zoom-pp-cli notes docs transcript J8FjimDWSWS0t4qXexE3cQ --format md
  zoom-pp-cli notes docs transcript J8FjimDWSWS0t4qXexE3cQ --format vtt > standup.vtt
  zoom-pp-cli notes docs transcript "KNnF+0YXT26CAnB3p8f9gw==" --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := strings.TrimSpace(args[0])
			format = strings.ToLower(strings.TrimSpace(format))
			switch format {
			case "", "json", "vtt", "md", "markdown":
			default:
				return fmt.Errorf("unsupported --format %q (want json, vtt, or md)", format)
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"would_call": "GET zoom docs transcript for " + id})
			}
			client, err := docsbridge.New(60 * time.Second)
			if err != nil {
				return err
			}
			meetingID, title, err := resolveNoteMeetingID(cmd, client, id)
			if err != nil {
				return err
			}
			transcript, err := client.Transcript(cmd.Context(), meetingID)
			if err != nil {
				return err
			}
			switch format {
			case "vtt":
				_, err := fmt.Fprint(cmd.OutOrStdout(), transcriptToVTT(transcript))
				return err
			case "md", "markdown":
				_, err := fmt.Fprint(cmd.OutOrStdout(), transcriptToMarkdown(transcript, title))
				return err
			default:
				return flags.printJSON(cmd, transcript)
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json, vtt, or md")
	return cmd
}

func newNotesDocsSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch My Notes transcripts and index them locally so `notes search` and `notes todos` work on them",
		Long: "Pulls each note's transcript over the web session and indexes it into the same local tables that " +
			"`notes ingest` populates, so `notes search` queries real transcripts instead of manually exported " +
			"PDFs. Re-syncing a note replaces its rows. Note that `notes todos` finds little here: its patterns " +
			"look for typed markers (TODO:, Action Item:, - [ ]) that spoken conversation does not contain.",
		Example: `  zoom-pp-cli notes docs sync --since 30d --json
  zoom-pp-cli notes docs sync --limit 5 --json --select synced,skipped`,
		Annotations: map[string]string{
			"mcp:local-write": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceTime, err := parseSince(since)
			if err != nil {
				return err
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"would_sync": "zoom docs my-notes transcripts"})
			}
			effectiveLimit := limit
			if cliutil.IsDogfoodEnv() && (effectiveLimit == 0 || effectiveLimit > dogfoodNoteCap) {
				effectiveLimit = dogfoodNoteCap
			}
			client, err := docsbridge.New(60 * time.Second)
			if err != nil {
				return err
			}
			notes, err := client.Notes(cmd.Context(), sinceTime, effectiveLimit)
			if err != nil {
				return err
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()

			var (
				synced   int
				skipped  int
				failures []map[string]string
			)
			for _, note := range notes {
				if note.MeetingID == "" {
					skipped++
					continue
				}
				transcript, err := client.Transcript(cmd.Context(), note.MeetingID)
				if err != nil {
					failures = append(failures, map[string]string{"note_id": note.ID, "title": note.Title, "error": err.Error()})
					continue
				}
				if len(transcript.Items) == 0 {
					skipped++
					continue
				}
				if _, err := localstore.IngestNote(cmd.Context(), db, transcriptToIngestedNote(note, transcript)); err != nil {
					failures = append(failures, map[string]string{"note_id": note.ID, "title": note.Title, "error": err.Error()})
					continue
				}
				synced++
			}
			if failures == nil {
				failures = []map[string]string{}
			}
			return flags.printJSON(cmd, map[string]any{
				"notes_considered": len(notes),
				"synced":           synced,
				"skipped":          skipped,
				"failures":         failures,
				"cookie_source":    client.CookieSource,
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only notes updated since this point (7d, 30d, ISO date)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max notes to sync (0 = unlimited)")
	return cmd
}

// resolveNoteMeetingID accepts a note id or a meeting id. Note ids are opaque
// file ids from the notes list; meeting ids are base64 and carry characters a
// file id never does, but the cheap discriminator is a lookup: if the id
// matches a note, use that note's meeting id.
func resolveNoteMeetingID(cmd *cobra.Command, client *docsbridge.Client, id string) (meetingID string, title string, err error) {
	if strings.ContainsAny(id, "+/=") {
		return id, "", nil
	}
	notes, err := client.Notes(cmd.Context(), time.Time{}, 0)
	if err != nil {
		return "", "", err
	}
	for _, note := range notes {
		if note.ID != id {
			continue
		}
		if note.MeetingID == "" {
			return "", "", fmt.Errorf("note %q (%s) has no meeting attached, so it has no transcript", note.Title, id)
		}
		return note.MeetingID, note.Title, nil
	}
	// Not a known note id — treat it as a meeting id and let the bridge answer.
	return id, "", nil
}

// transcriptToVTT renders WebVTT so transcripts drop into the same tooling that
// consumes Zoom's own recording .vtt files.
func transcriptToVTT(t *docsbridge.Transcript) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for i, item := range t.Items {
		fmt.Fprintf(&b, "%d\n", i+1)
		fmt.Fprintf(&b, "%s --> %s\n", vttTimestamp(item.StartTime), vttTimestamp(item.EndTime))
		fmt.Fprintf(&b, "%s: %s\n\n", t.SpeakerName(item.UserID), item.Text)
	}
	return b.String()
}

// vttTimestamp normalizes the bridge's HH:MM:SS.mmm to WebVTT's required
// three-digit fraction, tolerating a missing fraction.
func vttTimestamp(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "00:00:00.000"
	}
	if !strings.Contains(s, ".") {
		return s + ".000"
	}
	head, frac, _ := strings.Cut(s, ".")
	for len(frac) < 3 {
		frac += "0"
	}
	return head + "." + frac[:3]
}

// transcriptToMarkdown groups consecutive utterances by speaker so the result
// reads as dialogue rather than one line per cue.
func transcriptToMarkdown(t *docsbridge.Transcript, title string) string {
	var b strings.Builder
	if title == "" {
		title = "Zoom meeting transcript"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	if started := t.StartedAt(); !started.IsZero() {
		fmt.Fprintf(&b, "%s\n\n", started.Format(time.RFC1123))
	}
	if t.Uncompleted {
		b.WriteString("> This transcript is still being generated and may be incomplete.\n\n")
	}
	lastSpeaker := ""
	for _, item := range t.Items {
		speaker := t.SpeakerName(item.UserID)
		if speaker != lastSpeaker {
			fmt.Fprintf(&b, "\n**%s** (%s)\n\n", speaker, item.StartTime)
			lastSpeaker = speaker
		}
		fmt.Fprintf(&b, "%s\n", item.Text)
	}
	return b.String()
}

// transcriptToIngestedNote shapes a transcript for the local notes tables. One
// segment per speaker turn keeps FTS5 hits readable, and the todo regex runs
// over the same text `notes ingest` would have seen.
func transcriptToIngestedNote(note docsbridge.Note, t *docsbridge.Transcript) localstore.IngestedNote {
	var (
		segments []localstore.NoteSegment
		current  strings.Builder
		speaker  string
		ord      int
	)
	flush := func() {
		if current.Len() == 0 {
			return
		}
		segments = append(segments, localstore.NoteSegment{
			Ord:     ord,
			Heading: speaker,
			Text:    strings.TrimSpace(current.String()),
		})
		ord++
		current.Reset()
	}
	for _, item := range t.Items {
		if name := t.SpeakerName(item.UserID); name != speaker {
			flush()
			speaker = name
		}
		current.WriteString(item.Text)
		current.WriteString(" ")
	}
	flush()

	startTime := t.StartedAt()
	if startTime.IsZero() {
		startTime = note.UpdatedAt
	}
	return localstore.IngestedNote{
		SourceFile:   "zoom-docs://" + note.ID,
		FileFormat:   "zoom-docs-transcript",
		MeetingTopic: note.Title,
		MeetingID:    note.MeetingID,
		StartTime:    startTime,
		Segments:     segments,
		Todos:        notesparse.ExtractTodos(itemsAsLines(t.Items)),
	}
}

// itemsAsLines puts one utterance per line. The action-item regexes are
// line-anchored, so they must see the raw cues rather than the speaker-grouped
// segments above — a grouped turn buries "TODO: ..." mid-line where no pattern
// can match it.
func itemsAsLines(items []docsbridge.TranscriptItem) string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, strings.TrimSpace(item.Text))
	}
	return strings.Join(lines, "\n")
}

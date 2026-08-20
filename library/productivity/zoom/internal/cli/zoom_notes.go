package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/notesparse"
)

// newZoomNotesCmd: T9. The user-requested killer feature.
//
// Zoom has no public REST endpoint for the My Notes feature; this command
// stack combines:
//
//   - `notes web` — open https://zoom.us/notes in the user's browser
//   - `notes summary` / `notes transcript` — AI Companion documented endpoints
//   - `notes ingest` — parse exported PDF/DOCX into SQLite
//   - `notes search` — FTS5 across ingested notes
//   - `notes todos` — regex-extracted action items
func newZoomNotesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Zoom 'My Notes' integration (web open + AI Companion + ingest/search/todos)",
		Long: "Zoom does not expose a public REST endpoint for the My Notes feature (confirmed by Zoom devs " +
			"on the developer forum in 2024 and reconfirmed in late-2025 threads). This command group combines " +
			"three honest paths: open the notes web portal, fetch the documented AI Companion summary/transcript, " +
			"and ingest manually-exported notes PDF/DOCX into a local searchable corpus with auto-extracted action items.",
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newNotesWebCmd(flags))
	cmd.AddCommand(newNotesSummaryCmd(flags))
	cmd.AddCommand(newNotesTranscriptCmd(flags))
	cmd.AddCommand(newNotesIngestCmd(flags))
	cmd.AddCommand(newNotesListCmd(flags))
	cmd.AddCommand(newNotesSearchCmd(flags))
	cmd.AddCommand(newNotesTodosCmd(flags))
	return cmd
}

func newNotesWebCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "web [meeting-id]",
		Short: "Open https://zoom.us/notes in the default browser (the only live UI path to My Notes)",
		Example: `  zoom-pp-cli notes web --json
  zoom-pp-cli notes web 85123456789 --dry-run`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			url := "https://zoom.us/notes"
			if len(args) > 0 {
				url = "https://zoom.us/notes?meetingId=" + args[0]
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				if !flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "would open:", url)
					return nil
				}
				return flags.printJSON(cmd, map[string]any{"status": "would_open", "url": url, "platform": runtime.GOOS})
			}
			if err := openURL(cmd.Context(), url); err != nil {
				return fmt.Errorf("notes web: %w", err)
			}
			return flags.printJSON(cmd, map[string]any{"status": "opened", "url": url, "platform": runtime.GOOS})
		},
	}
}

func newNotesSummaryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary [meeting-uuid]",
		Short: "Fetch the AI Companion meeting summary for a meeting UUID",
		Long: "Calls the documented `/meetings/{meetingId}/meeting_summary` endpoint. Requires S2S OAuth + " +
			"the meeting:read:summary scope on the OAuth app.",
		Example: `  zoom-pp-cli notes summary 85123456789 --json
  zoom-pp-cli notes summary 85123456789 --select summary_title,summary_overview --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Zoom meeting UUIDs are base64 and commonly contain '/' and '=';
			// PathEscape keeps the UUID a single path segment instead of
			// splitting it into extra path components.
			return runCloudGET(cmd, flags, "/meetings/"+url.PathEscape(args[0])+"/meeting_summary")
		},
	}
	return cmd
}

func newNotesTranscriptCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transcript [meeting-uuid]",
		Short: "Fetch the AI Companion transcript for a meeting UUID",
		Long: "Calls the documented `/meetings/{meetingId}/transcript` endpoint. Requires S2S OAuth + " +
			"the meeting:read:transcript scope on the OAuth app.",
		Example: `  zoom-pp-cli notes transcript 85123456789 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// PathEscape so base64 UUIDs containing '/' or '=' stay a single
			// path segment.
			return runCloudGET(cmd, flags, "/meetings/"+url.PathEscape(args[0])+"/transcript")
		},
	}
	return cmd
}

func newNotesIngestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingest [file.pdf|file.docx|file.txt|file.md]",
		Short: "Parse an exported Notes file and index it (PDF/DOCX/TXT/MD)",
		Long: "Extracts text from the file, segments it by paragraph, indexes paragraphs into FTS5, and runs the " +
			"action-item regex over the full text to populate note_todos. Re-ingesting the same file replaces its rows.",
		Example: `  zoom-pp-cli notes ingest ~/Downloads/zoom-notes-2026-05-12.pdf --json
  zoom-pp-cli notes ingest ./notes/standup.docx --json`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			path := args[0]
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{"would_ingest": path})
			}
			note, err := notesparse.Parse(path)
			if err != nil {
				return err
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			id, err := localstore.IngestNote(cmd.Context(), db, note)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{
				"status":        "ingested",
				"note_id":       id,
				"source_file":   note.SourceFile,
				"file_format":   note.FileFormat,
				"meeting_topic": note.MeetingTopic,
				"meeting_id":    note.MeetingID,
				"start_time":    note.StartTime,
				"segment_count": len(note.Segments),
				"todo_count":    len(note.Todos),
			})
		},
	}
	return cmd
}

func newNotesListCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		limit int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List locally ingested notes, newest first, optionally filtered by recency (--since) and capped by --limit",
		Example: `  zoom-pp-cli notes list --json
  zoom-pp-cli notes list --since 30d --limit 20 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceTime, err := parseSince(since)
			if err != nil {
				return err
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			rows, err := localstore.ListNotes(cmd.Context(), db, sinceTime, limit)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []localstore.NoteRow{}
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Limit to notes since this point (7d, 30d, ISO date)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows (0 = unlimited)")
	return cmd
}

func newNotesSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		since     string
		meetingID string
		limit     int
	)
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "FTS5 search across ingested note segments",
		Example: `  zoom-pp-cli notes search "q2 launch plan" --json
  zoom-pp-cli notes search "pricing" --since 30d --limit 10 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			sinceTime, err := parseSince(since)
			if err != nil {
				return err
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			matches, err := localstore.SearchNotes(cmd.Context(), db, args[0], sinceTime, meetingID, limit)
			if err != nil {
				return err
			}
			if matches == nil {
				matches = []localstore.NoteSegmentMatch{}
			}
			return flags.printJSON(cmd, map[string]any{
				"query":   args[0],
				"matches": matches,
				"count":   len(matches),
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Limit to notes since this point (7d, 30d, ISO date)")
	cmd.Flags().StringVar(&meetingID, "meeting-id", "", "Scope to a specific meeting UUID/ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max matches")
	return cmd
}

func newNotesTodosCmd(flags *rootFlags) *cobra.Command {
	var (
		since     string
		meetingID string
		limit     int
	)
	cmd := &cobra.Command{
		Use:   "todos",
		Short: "List extracted action items from ingested notes (TODO:, Action:, [ ], Follow up:, Next:, Owner:)",
		Example: `  zoom-pp-cli notes todos --since 7d --json
  zoom-pp-cli notes todos --meeting-id 851234567 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceTime, err := parseSince(since)
			if err != nil {
				return err
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			rows, err := localstore.ListTodos(cmd.Context(), db, sinceTime, meetingID, limit)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []localstore.NoteTodoRow{}
			}
			return flags.printJSON(cmd, map[string]any{
				"count": len(rows),
				"todos": rows,
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Limit to notes since this point (7d, 30d, ISO date)")
	cmd.Flags().StringVar(&meetingID, "meeting-id", "", "Scope to a specific meeting UUID/ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows (0 = unlimited)")
	return cmd
}

// runCloudGET is a shared helper for notes summary/transcript and any other
// hand-coded GET that hits a documented endpoint outside the spec-emitted tree.
func runCloudGET(cmd *cobra.Command, flags *rootFlags, path string) error {
	// Dry-run / verify-env first — these never make a network call so they
	// don't need auth to succeed. Order matters: the previous shape errored
	// on missing auth before the user could test the command shape.
	if dryRunOK(flags) || cliutil.IsVerifyEnv() {
		return flags.printJSON(cmd, map[string]any{"would_call": "GET " + path})
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return err
	}
	if cfg.AuthHeader() == "" {
		return errors.New("notes: requires Zoom S2S OAuth — run `zoom-pp-cli auth set-token` first or export ZOOM_S2S_ACCESS_TOKEN")
	}
	url := strings.TrimRight(cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(cmd.Context(), "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", cfg.AuthHeader())
	req.Header.Set("Accept", "application/json")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("zoom GET: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("zoom GET %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		// Some transcript endpoints return text/vtt; surface raw.
		return flags.printJSON(cmd, map[string]any{"path": path, "raw": string(body)})
	}
	return flags.printJSON(cmd, out)
}

// Keep the bytes / context imports referenced in case they're inlined later.
var (
	_ = bytes.Buffer{}
	_ = context.Background
)

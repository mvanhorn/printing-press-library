package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/recordings"
)

// newZoomRecordingsCmd: parent for `recordings local`, `recordings recent`,
// `recordings drift`, `recordings analyze`, `recordings export`.
//
// Cloud recordings list/download remain on the spec-emitted `recordings ...`
// surface inside the api tree; we don't shadow them.
func newZoomRecordingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recordings",
		Short: "Local + cloud Zoom recording workflows (sync, search, drift, analyze, export)",
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newRecordingsLocalCmd(flags))
	cmd.AddCommand(newRecordingsRecentCmd(flags))
	cmd.AddCommand(newRecordingsSearchCmd(flags))
	cmd.AddCommand(newRecordingsDriftCmd(flags))
	cmd.AddCommand(newRecordingsAnalyzeCmd(flags))
	cmd.AddCommand(newRecordingsExportCmd(flags))
	return cmd
}

// recordings local {sync, list, search}
func newRecordingsLocalCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Operate on ~/Documents/Zoom/ local recordings",
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newRecordingsLocalSyncCmd(flags))
	cmd.AddCommand(newRecordingsLocalListCmd(flags))
	return cmd
}

func newRecordingsLocalSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		root  string
		since string
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Walk ~/Documents/Zoom/, parse VTT transcripts, index into SQLite",
		Long: "Idempotent. Each folder becomes one row in local_recordings; each VTT cue becomes a row in " +
			"local_transcript_segments (FTS5-indexed). --since 7d/30d/90d restricts to recently-recorded folders. " +
			"Required before `find`, `recordings search`, `storage`, `drift`, or `today --with-recordings` produce data.",
		Example: `  zoom-pp-cli recordings local sync --json
  zoom-pp-cli recordings local sync --since 30d --json`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			r := root
			if r == "" {
				r = recordings.DefaultLocalRoot()
			}
			sinceTime, err := parseSince(since)
			if err != nil {
				return err
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{
					"would_sync": true,
					"root":       r,
					"since":      since,
				})
			}
			if _, err := os.Stat(r); err != nil {
				// Missing root is honest "0 folders" not an error — many users
				// have never recorded locally. Return a structured response so
				// agents can branch on the count.
				return flags.printJSON(cmd, map[string]any{
					"root":            r,
					"folders_total":   0,
					"folders_synced":  0,
					"folders_skipped": 0,
					"transcript_cues": 0,
					"status":          "no_recordings_directory",
					"note":            fmt.Sprintf("%s does not exist (no local recordings yet)", r),
				})
			}
			folders, err := recordings.Scan(r)
			if err != nil {
				return err
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()

			var (
				foldersWritten int
				cuesWritten    int
				skipped        int
			)
			for _, f := range folders {
				if !sinceTime.IsZero() && f.Start.Before(sinceTime) {
					skipped++
					continue
				}
				id, err := localstore.UpsertRecording(cmd.Context(), db, f)
				if err != nil {
					skipped++
					continue
				}
				foldersWritten++
				if f.TranscriptPath != "" && strings.HasSuffix(strings.ToLower(f.TranscriptPath), ".vtt") {
					cues, perr := recordings.ParseVTTFile(f.TranscriptPath)
					if perr != nil {
						continue
					}
					for _, c := range cues {
						if err := localstore.UpsertSegment(cmd.Context(), db, id, c); err == nil {
							cuesWritten++
						}
					}
				}
			}
			return flags.printJSON(cmd, map[string]any{
				"root":            r,
				"folders_total":   len(folders),
				"folders_synced":  foldersWritten,
				"folders_skipped": skipped,
				"transcript_cues": cuesWritten,
				"status":          "synced",
			})
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "Override the local recordings folder (default ~/Documents/Zoom/)")
	cmd.Flags().StringVar(&since, "since", "", "Limit to recordings since this point (e.g. 7d, 30d, 2026-01-01)")
	return cmd
}

func newRecordingsLocalListCmd(flags *rootFlags) *cobra.Command {
	var (
		since       string
		partialOnly bool
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local recordings (run `recordings local sync` first)",
		Example: `  zoom-pp-cli recordings local list --json
  zoom-pp-cli recordings local list --since 30d --limit 20 --json
  zoom-pp-cli recordings local list --partial-only --json`,
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
			rows, err := localstore.ListLocalRecordings(cmd.Context(), db, localstore.ListLocalOpts{
				Since:       sinceTime,
				PartialOnly: partialOnly,
				Limit:       limit,
			})
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []localstore.LocalRecordingRow{}
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Limit to recordings since this point (7d, 30d, ISO date)")
	cmd.Flags().BoolVar(&partialOnly, "partial-only", false, "Only return folders with double_click_to_convert partials")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = unlimited)")
	return cmd
}

func newRecordingsRecentCmd(flags *rootFlags) *cobra.Command {
	var (
		limit  int
		source string
	)
	cmd := &cobra.Command{
		Use:   "recent",
		Short: "Recent recordings across local and cloud sources",
		Example: `  zoom-pp-cli recordings recent --json
  zoom-pp-cli recordings recent --source local --limit 5 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit == 0 {
				limit = 10
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()

			var out []map[string]any
			if source == "" || source == "local" || source == "both" {
				rows, err := localstore.ListLocalRecordings(cmd.Context(), db, localstore.ListLocalOpts{Limit: limit})
				if err != nil {
					return err
				}
				for _, r := range rows {
					out = append(out, map[string]any{
						"source": "local", "id": r.ID, "topic": r.Topic, "start": r.Start,
						"path": r.Path, "transcript": r.TranscriptPath, "total_bytes": r.TotalBytes,
					})
				}
			}
			// Cloud rows live in cloud_recordings table; populated only when
			// `recordings cloud list` was run. Honest empty list otherwise.
			if source == "" || source == "cloud" || source == "both" {
				rows, err := db.QueryContext(cmd.Context(), `SELECT meeting_uuid, meeting_id, COALESCE(topic, ''), start_time, duration_minutes, total_bytes FROM cloud_recordings ORDER BY start_time DESC LIMIT ?`, limit)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var uuid, mid, topic string
						var start time.Time
						var dur int
						var size int64
						if err := rows.Scan(&uuid, &mid, &topic, &start, &dur, &size); err == nil {
							out = append(out, map[string]any{
								"source": "cloud", "uuid": uuid, "meeting_id": mid, "topic": topic,
								"start": start, "duration_minutes": dur, "total_bytes": size,
							})
						}
					}
				}
			}
			// Sort newest first.
			sort.SliceStable(out, func(i, j int) bool {
				ti, _ := out[i]["start"].(time.Time)
				tj, _ := out[j]["start"].(time.Time)
				return ti.After(tj)
			})
			if len(out) > limit {
				out = out[:limit]
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows (default 10)")
	cmd.Flags().StringVar(&source, "source", "both", "local | cloud | both")
	return cmd
}

func newRecordingsSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		limit   int
		source  string
		since   string
		speaker string
	)
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search local + cloud transcript cues (alias for `find`)",
		Example: `  zoom-pp-cli recordings search "pricing" --json
  zoom-pp-cli recordings search "launch plan" --source local --since 30d --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runUnifiedFind(cmd, flags, args[0], speaker, since, source, 0, 0, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max matches")
	cmd.Flags().StringVar(&source, "source", "both", "local | cloud | both")
	cmd.Flags().StringVar(&since, "since", "", "Limit to recordings since this point (7d, 30d, ISO date)")
	cmd.Flags().StringVar(&speaker, "speaker", "", "Filter to one speaker label")
	return cmd
}

func newRecordingsDriftCmd(flags *rootFlags) *cobra.Command {
	var retentionDays int
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Compare local and cloud recordings — flags missing/expiring/partial",
		Long: "Joins local_recordings + cloud_recordings on meeting_id to surface: " +
			"(a) cloud recordings not yet on disk, (b) local recordings not in cloud, " +
			"(c) cloud recordings within --retention-days of the org's auto-delete, " +
			"(d) local double_click_to_convert partials whose cloud version is complete (safe to delete).",
		Example: `  zoom-pp-cli recordings drift --retention-days 90 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()
			if err := localstore.EnsureSchema(cmd.Context(), db); err != nil {
				return err
			}

			// Build maps keyed by meeting_id (best signal we have on both sides).
			local, _ := localstore.ListLocalRecordings(cmd.Context(), db, localstore.ListLocalOpts{})
			localByID := map[string]localstore.LocalRecordingRow{}
			for _, r := range local {
				if r.MeetingID != "" {
					localByID[r.MeetingID] = r
				}
			}

			cloud := map[string]map[string]any{}
			rows, err := db.QueryContext(cmd.Context(), `SELECT meeting_uuid, meeting_id, COALESCE(topic, ''), start_time, COALESCE(retention_at, ''), total_bytes FROM cloud_recordings`)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var uuid, mid, topic, retention string
					var start time.Time
					var size int64
					if err := rows.Scan(&uuid, &mid, &topic, &start, &retention, &size); err == nil && mid != "" {
						cloud[mid] = map[string]any{
							"uuid": uuid, "meeting_id": mid, "topic": topic, "start": start,
							"retention_at": retention, "total_bytes": size,
						}
					}
				}
			}

			cutoff := time.Time{}
			if retentionDays > 0 {
				cutoff = time.Now().Add(time.Duration(retentionDays) * 24 * time.Hour)
			}

			var missingLocally, missingInCloud, expiring, safeToCleanPartials []map[string]any
			for mid, c := range cloud {
				if _, ok := localByID[mid]; !ok {
					missingLocally = append(missingLocally, c)
				}
				if rt, _ := c["retention_at"].(string); rt != "" && retentionDays > 0 {
					if t, err := time.Parse(time.RFC3339, rt); err == nil && t.Before(cutoff) {
						expiring = append(expiring, c)
					}
				}
			}
			for mid, l := range localByID {
				if _, ok := cloud[mid]; !ok {
					missingInCloud = append(missingInCloud, map[string]any{
						"meeting_id": mid, "topic": l.Topic, "start": l.Start, "path": l.Path,
					})
				} else if l.HasPartial {
					safeToCleanPartials = append(safeToCleanPartials, map[string]any{
						"meeting_id": mid, "topic": l.Topic, "path": l.Path,
					})
				}
			}

			return flags.printJSON(cmd, map[string]any{
				"local_count":            len(localByID),
				"cloud_count":            len(cloud),
				"missing_locally":        missingLocally,
				"missing_in_cloud":       missingInCloud,
				"expiring_in_cloud":      expiring,
				"safe_to_clean_partials": safeToCleanPartials,
				"retention_days":         retentionDays,
				"note":                   "Cloud rows only present after `recordings cloud list` populates the cache (requires S2S OAuth).",
			})
		},
	}
	cmd.Flags().IntVar(&retentionDays, "retention-days", 0, "Flag cloud recordings expiring within this many days")
	return cmd
}

func newRecordingsAnalyzeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze [recording-id-or-name]",
		Short: "Per-speaker talk-time + interruption count from a local recording's VTT",
		Long: "Computes per-speaker total seconds, longest contiguous monologue, and cue-overlap count " +
			"(interruption proxy) from the cues in local_transcript_segments.",
		Example: `  zoom-pp-cli recordings analyze 2026-05-12 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{
					"would_analyze": query,
					"status":        "dry_run",
				})
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()

			// Resolve recording — match on id OR name LIKE.
			row := db.QueryRowContext(cmd.Context(),
				`SELECT id, path, COALESCE(topic, ''), COALESCE(name, '') FROM local_recordings
				 WHERE id = ? OR name LIKE ? OR path LIKE ? LIMIT 1`,
				query, "%"+query+"%", "%"+query+"%")
			var id, path, topic, name string
			if err := row.Scan(&id, &path, &topic, &name); err != nil {
				return fmt.Errorf("recordings analyze: no local recording matching %q (run `recordings local sync` first?)", query)
			}

			// Pull cues for analysis.
			rows, err := db.QueryContext(cmd.Context(),
				`SELECT cue_index, start_ms, end_ms, COALESCE(speaker, ''), text FROM local_transcript_segments WHERE recording_id = ? ORDER BY cue_index`, id)
			if err != nil {
				return err
			}
			defer rows.Close()

			type cue struct {
				start, end int64
				speaker    string
			}
			var cs []cue
			for rows.Next() {
				var idx int
				var s, e int64
				var sp, t string
				if err := rows.Scan(&idx, &s, &e, &sp, &t); err == nil {
					cs = append(cs, cue{s, e, sp})
				}
			}
			if len(cs) == 0 {
				return fmt.Errorf("recordings analyze: %q has no transcript cues — no VTT was indexed", query)
			}

			perSpeaker := map[string]*speakerStats{}
			for i, c := range cs {
				if c.speaker == "" {
					c.speaker = "(unknown)"
				}
				ss := perSpeaker[c.speaker]
				if ss == nil {
					ss = &speakerStats{Name: c.speaker}
					perSpeaker[c.speaker] = ss
				}
				dur := c.end - c.start
				if dur < 0 {
					dur = 0
				}
				ss.TalkSeconds += float64(dur) / 1000.0
				if float64(dur)/1000.0 > ss.LongestMonologueSec {
					ss.LongestMonologueSec = float64(dur) / 1000.0
				}
				ss.CueCount++
				// Interruption = overlap with previous cue from a different speaker.
				if i > 0 {
					prev := cs[i-1]
					if prev.speaker != c.speaker && c.start < prev.end {
						ss.Interruptions++
					}
				}
			}
			var list []speakerStats
			for _, ss := range perSpeaker {
				list = append(list, *ss)
			}
			sort.Slice(list, func(i, j int) bool { return list[i].TalkSeconds > list[j].TalkSeconds })

			return flags.printJSON(cmd, map[string]any{
				"recording_id":   id,
				"recording_path": path,
				"topic":          topic,
				"cue_count":      len(cs),
				"per_speaker":    list,
			})
		},
	}
	return cmd
}

type speakerStats struct {
	Name                string  `json:"name"`
	TalkSeconds         float64 `json:"talk_seconds"`
	LongestMonologueSec float64 `json:"longest_monologue_sec"`
	CueCount            int     `json:"cue_count"`
	Interruptions       int     `json:"interruptions"`
}

func newRecordingsExportCmd(flags *rootFlags) *cobra.Command {
	var (
		out            string
		withChat       bool
		withTranscript bool
	)
	cmd := &cobra.Command{
		Use:   "export [recording-id-or-name]",
		Short: "Package one recording's mp4 + transcript + chat + INDEX.md into a folder",
		Long: "Resolves the recording against local first. Copies the video file(s), the VTT transcript (with --with-transcript), " +
			"and the chat.txt (with --with-chat) into --out. Generates INDEX.md with a timestamped TOC derived from VTT cues.",
		Example: `  zoom-pp-cli recordings export 2026-05-12 --with-transcript --with-chat --out ~/Drive/q2-planning --json`,
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
			if out == "" {
				out = filepath.Join(os.TempDir(), "zoom-export-"+sanitize(query))
			}
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return flags.printJSON(cmd, map[string]any{
					"would_export":    query,
					"out":             out,
					"with_chat":       withChat,
					"with_transcript": withTranscript,
				})
			}
			db, closer, err := openLocalDB(cmd.Context())
			if err != nil {
				return err
			}
			defer closer()

			row := db.QueryRowContext(cmd.Context(),
				`SELECT id, path, COALESCE(topic, ''), COALESCE(name, ''), COALESCE(transcript_path, ''), COALESCE(chat_path, '')
				 FROM local_recordings WHERE id = ? OR name LIKE ? OR path LIKE ? LIMIT 1`,
				query, "%"+query+"%", "%"+query+"%")
			var id, path, topic, name, transcript, chat string
			if err := row.Scan(&id, &path, &topic, &name, &transcript, &chat); err != nil {
				return fmt.Errorf("recordings export: no local recording matching %q (run `recordings local sync` first?)", query)
			}

			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}

			// Copy all .mp4 files. A ReadDir failure (folder deleted from disk
			// while its SQLite row survives, or a permissions error) must be a
			// hard error — otherwise the export reports success with zero files.
			entries, err := os.ReadDir(path)
			if err != nil {
				return fmt.Errorf("recordings export: cannot read recording folder %s: %w (the folder may have been deleted; run `recordings local sync` to refresh the index)", path, err)
			}
			var copied []string
			for _, e := range entries {
				lower := strings.ToLower(e.Name())
				if !strings.HasSuffix(lower, ".mp4") {
					continue
				}
				dst := filepath.Join(out, e.Name())
				if err := copyFile(filepath.Join(path, e.Name()), dst); err == nil {
					copied = append(copied, dst)
				}
			}
			var copyErrors []string
			if withTranscript && transcript != "" {
				dst := filepath.Join(out, filepath.Base(transcript))
				if err := copyFile(transcript, dst); err == nil {
					copied = append(copied, dst)
				} else {
					copyErrors = append(copyErrors, fmt.Sprintf("transcript %s: %v", transcript, err))
				}
			}
			if withChat && chat != "" {
				dst := filepath.Join(out, filepath.Base(chat))
				if err := copyFile(chat, dst); err == nil {
					copied = append(copied, dst)
				} else {
					copyErrors = append(copyErrors, fmt.Sprintf("chat %s: %v", chat, err))
				}
			}

			// Generate INDEX.md from cues. writeIndex buffers the document and
			// surfaces any os.Create/write/close failure so a truncated index
			// is reported as an error rather than a false "exported" success.
			indexPath := filepath.Join(out, "INDEX.md")
			indexErr := writeExportIndex(cmd.Context(), db, indexPath, topic, path, id, copied)
			if indexErr != nil {
				return fmt.Errorf("recordings export: copied %d media file(s) to %s but INDEX.md write failed: %w", len(copied), out, indexErr)
			}
			// A requested transcript/chat copy that failed is a partial export —
			// surface it as an error so the caller can't mistake a truncated
			// bundle for a complete one.
			if len(copyErrors) > 0 {
				return fmt.Errorf("recordings export: exported %d file(s) to %s but %d requested file(s) failed to copy: %s",
					len(copied), out, len(copyErrors), strings.Join(copyErrors, "; "))
			}
			return flags.printJSON(cmd, map[string]any{
				"status":     "exported",
				"out":        out,
				"recording":  map[string]any{"id": id, "topic": topic, "path": path},
				"files":      copied,
				"index_path": indexPath,
			})
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Output directory (default: $TMPDIR/zoom-export-<id>)")
	cmd.Flags().BoolVar(&withChat, "with-chat", false, "Include the meeting_saved_chat.txt file")
	cmd.Flags().BoolVar(&withTranscript, "with-transcript", false, "Include the .vtt transcript")
	return cmd
}

// writeExportIndex builds INDEX.md in a buffer (so a mid-document failure can
// be detected before the file is declared complete), writes it atomically, and
// returns any create/write/close error. Callers must treat a non-nil return as
// a failed export.
func writeExportIndex(ctx context.Context, db *sql.DB, indexPath, topic, srcPath, recordingID string, copied []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", strings.TrimSpace(topic))
	fmt.Fprintf(&b, "Source recording: %s\n\n", srcPath)
	fmt.Fprintf(&b, "## Files\n")
	for _, c := range copied {
		fmt.Fprintf(&b, "- %s\n", filepath.Base(c))
	}
	fmt.Fprintf(&b, "\n## Timestamped Table of Contents\n\n")
	rows, err := db.QueryContext(ctx,
		`SELECT start_ms, COALESCE(speaker, ''), text FROM local_transcript_segments WHERE recording_id = ? ORDER BY cue_index`, recordingID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ms int64
			var sp, txt string
			if scanErr := rows.Scan(&ms, &sp, &txt); scanErr == nil {
				ts := formatHHMMSS(ms)
				if sp != "" {
					fmt.Fprintf(&b, "- **%s** _%s_: %s\n", ts, sp, txt)
				} else {
					fmt.Fprintf(&b, "- **%s**: %s\n", ts, txt)
				}
			}
		}
	}
	// Single atomic write; os.WriteFile reports create + write errors together.
	return os.WriteFile(indexPath, []byte(b.String()), 0o644)
}

// helpers

func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --since %q: %w", s, err)
		}
		return time.Now().AddDate(0, 0, -n), nil
	}
	if strings.HasSuffix(s, "h") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --since %q: %w", s, err)
		}
		return time.Now().Add(time.Duration(-n) * time.Hour), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q (try 7d, 30d, 2026-01-01, or ISO8601)", s)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func sanitize(s string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-")
	return r.Replace(s)
}

// formatHHMMSS mirrors the helper inside localstore.
func formatHHMMSS(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

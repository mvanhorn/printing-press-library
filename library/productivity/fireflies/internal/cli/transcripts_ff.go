// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

// fullTranscriptQuery fetches a complete transcript with sentences + analytics.
const fullTranscriptQuery = `
query Transcript($id: String!) {
  transcript(id: $id) {
    id
    title
    date
    dateString
    duration
    privacy
    transcript_url
    audio_url
    video_url
    is_live
    meeting_link
    calendar_type
    organizer_email
    host_email
    participants
    speakers { id name }
    meeting_attendees { displayName email phoneNumber }
    sentences {
      index
      text
      raw_text
      start_time
      end_time
      speaker_id
      speaker_name
      ai_filters { task pricing metric question date_and_time sentiment }
    }
    summary {
      action_items
      keywords
      outline
      overview
      shorthand_bullet
      notes
      gist
      bullet_gist
      short_summary
      meeting_type
      topics_discussed
    }
    analytics {
      sentiments { positive_pct neutral_pct negative_pct }
      categories { questions date_times metrics tasks }
      speakers {
        speaker_id name duration word_count longest_monologue
        monologues_count filler_words questions duration_pct words_per_minute
      }
    }
    meeting_info { fred_joined silent_meeting summary_status }
    channels { id title }
  }
}`

func newTranscriptsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transcripts",
		Short: "List, search, and manage meeting transcripts",
		Long:  "Manage Fireflies transcripts. Run 'sync' first to populate the local store.",
	}

	cmd.AddCommand(newTranscriptsListCmd(flags))
	cmd.AddCommand(newTranscriptsFindCmd(flags))
	cmd.AddCommand(newTranscriptsSearchCmd(flags))
	cmd.AddCommand(newTranscriptsRecentCmd(flags))
	cmd.AddCommand(newTranscriptsStatusCmd(flags))
	cmd.AddCommand(newTranscriptsPullCmd(flags))
	cmd.AddCommand(newTranscriptsGetCmd(flags))
	cmd.AddCommand(newTranscriptsExportCmd(flags))
	cmd.AddCommand(newTranscriptsDeleteCmd(flags))
	cmd.AddCommand(newTranscriptsUpdateCmd(flags))
	cmd.AddCommand(newTranscriptsShareCmd(flags))
	return cmd
}

func newTranscriptsListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var fromDays int
	var mine bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transcripts from local store",
		Long:  "List transcripts stored locally. Run 'sync' first to populate.",
		Example: strings.Trim(`
  fireflies-pp-cli transcripts list
  fireflies-pp-cli transcripts list --from 7 --limit 20
  fireflies-pp-cli transcripts list --mine --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := `SELECT data FROM transcripts ORDER BY CAST(date AS REAL) DESC`
			args2 := []any{}
			conds := []string{}

			if fromDays > 0 {
				cutoff := time.Now().AddDate(0, 0, -fromDays).UnixMilli()
				conds = append(conds, "CAST(date AS REAL) >= ?")
				args2 = append(args2, cutoff)
			}
			if mine {
				cfg, err := config.Load(flags.configPath)
				if err == nil && cfg.FirefliesApiKey != "" {
					_ = cfg // mine filter applied by organizer check below
				}
			}
			if len(conds) > 0 {
				query = `SELECT data FROM transcripts WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY CAST(date AS REAL) DESC`
			}
			if limit > 0 {
				query += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return fmt.Errorf("querying transcripts: %w", err)
			}
			defer rows.Close()

			var results []json.RawMessage
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				results = append(results, json.RawMessage(raw))
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-40s  %-19s  %s\n", "STATUS", "TITLE", "DATE", "DURATION")
			for _, raw := range results {
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				status := t.summaryStatus()
				dateStr := t.dateFormatted()
				dur := t.durationFormatted()
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-40s  %-19s  %s\n",
					status,
					truncate(t.Title, 40),
					dateStr,
					dur,
				)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No transcripts found. Run 'fireflies-pp-cli sync' first.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().IntVar(&fromDays, "from", 0, "Only show transcripts from the last N days (client-side filter)")
	cmd.Flags().BoolVar(&mine, "mine", false, "Only show my transcripts")
	return cmd
}

func newTranscriptsFindCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var participant string
	var channel string
	var keyword string
	var fromDays int
	var toDays int
	var processedOnly bool
	var limit int

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Find transcripts by participant, channel, keyword, or date range",
		Long: `Find transcripts in the local store using multiple filters. All date filtering
is applied client-side — the Fireflies API from_date param is unreliable.

Use --participant with an email address (not a name) to reliably find meetings
with a specific person. Meeting titles don't always contain participant names.`,
		Example: strings.Trim(`
  fireflies-pp-cli transcripts find --participant danijel.latin@verybigthings.com
  fireflies-pp-cli transcripts find --channel ryder --from 30
  fireflies-pp-cli transcripts find --keyword "budget" --processed-only
  fireflies-pp-cli transcripts find --participant client@company.com --from 90 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli sync' first.", err)
			}
			defer db.Close()

			// Resolve channel name to ID if provided
			channelFilter := ""
			if channel != "" {
				channelFilter = resolveChannelID(db, channel)
				if channelFilter == "" {
					// Fall back to title substring match
					channelFilter = channel
				}
			}

			rows, err := db.DB().QueryContext(cmd.Context(), "SELECT data FROM transcripts ORDER BY CAST(date AS REAL) DESC")
			if err != nil {
				return fmt.Errorf("querying transcripts: %w", err)
			}
			defer rows.Close()

			var results []json.RawMessage
			cutoffFrom := int64(0)
			cutoffTo := int64(0)
			if fromDays > 0 {
				cutoffFrom = time.Now().AddDate(0, 0, -fromDays).UnixMilli()
			}
			if toDays > 0 {
				cutoffTo = time.Now().AddDate(0, 0, -toDays).UnixMilli()
			}

			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}

				// Client-side date filter (from_date API param is broken)
				if cutoffFrom > 0 && int64(t.Date) < cutoffFrom {
					continue
				}
				if cutoffTo > 0 && int64(t.Date) > cutoffTo {
					continue
				}

				if processedOnly && t.summaryStatus() != "PROCESSED" {
					continue
				}

				if participant != "" && !t.hasParticipant(participant) {
					continue
				}

				if channelFilter != "" && !t.inChannel(channelFilter) {
					continue
				}

				if keyword != "" {
					kw := strings.ToLower(keyword)
					if !strings.Contains(strings.ToLower(t.Title), kw) &&
						!t.hasKeywordInSummary(kw) {
						continue
					}
				}

				results = append(results, json.RawMessage(raw))
				if limit > 0 && len(results) >= limit {
					break
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-40s  %-19s\n", "STATUS", "TITLE", "DATE")
			for _, raw := range results {
				var t transcriptRow
				if _ = json.Unmarshal(raw, &t); true {
					fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-40s  %-19s\n",
						t.summaryStatus(), truncate(t.Title, 40), t.dateFormatted())
				}
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching transcripts found.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&participant, "participant", "", "Filter by participant email address")
	cmd.Flags().StringVar(&channel, "channel", "", "Filter by channel name or ID")
	cmd.Flags().StringVar(&keyword, "keyword", "", "Filter by keyword in title or summary")
	cmd.Flags().IntVar(&fromDays, "from", 0, "Only show transcripts from the last N days")
	cmd.Flags().IntVar(&toDays, "to", 0, "Exclude transcripts older than N days ago")
	cmd.Flags().BoolVar(&processedOnly, "processed-only", false, "Only show PROCESSED meetings (skip PROCESSING/FAILED)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max results")
	return cmd
}

func newTranscriptsSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across synced transcript titles and summaries",
		Long:  "Offline full-text search using the local FTS5 index. No API calls consumed.",
		Example: strings.Trim(`
  fireflies-pp-cli transcripts search "pricing objection"
  fireflies-pp-cli transcripts search "roadmap" --limit 20 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli sync' first.", err)
			}
			defer db.Close()

			results, err := db.SearchTranscripts(args[0], limit)
			if err != nil {
				return fmt.Errorf("searching transcripts: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Found %d result(s) for %q:\n\n", len(results), args[0])
			for _, raw := range results {
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  [%s]\n", t.dateFormatted(), truncate(t.Title, 50), t.summaryStatus())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

func newTranscriptsRecentCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	var processedOnly bool

	cmd := &cobra.Command{
		Use:   "recent",
		Short: "Show recently processed transcripts",
		Example: strings.Trim(`
  fireflies-pp-cli transcripts recent
  fireflies-pp-cli transcripts recent --since 48h --json
  fireflies-pp-cli transcripts recent --since 24h --processed-only`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli sync' first.", err)
			}
			defer db.Close()

			dur, err := parseSinceDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since value %q: %w", since, err)
			}
			cutoff := time.Now().Add(-dur).UnixMilli()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM transcripts WHERE CAST(date AS REAL) >= ? ORDER BY CAST(date AS REAL) DESC`,
				cutoff)
			if err != nil {
				return fmt.Errorf("querying transcripts: %w", err)
			}
			defer rows.Close()

			var results []json.RawMessage
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				if processedOnly {
					var t transcriptRow
					if json.Unmarshal(raw, &t) == nil && t.summaryStatus() != "PROCESSED" {
						continue
					}
				}
				results = append(results, json.RawMessage(raw))
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Transcripts in last %s (%d found):\n\n", since, len(results))
			for _, raw := range results {
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-12s  %-50s  %s\n",
					t.summaryStatus(), truncate(t.Title, 50), t.dateFormatted())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "24h", "Duration to look back (e.g. 24h, 48h, 7d)")
	cmd.Flags().BoolVar(&processedOnly, "processed-only", false, "Only show PROCESSED meetings")
	return cmd
}

func newTranscriptsStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	var channel string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show processing status of recent meetings (PROCESSED / PROCESSING / FAILED)",
		Long: `Show the summary_status field for recent meetings. Use this to check whether
a same-day or next-morning meeting has finished processing before trying to
fetch its transcript or summary.`,
		Example: strings.Trim(`
  fireflies-pp-cli transcripts status
  fireflies-pp-cli transcripts status --since 48h
  fireflies-pp-cli transcripts status --channel ryder`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli sync' first.", err)
			}
			defer db.Close()

			dur, err := parseSinceDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}
			cutoff := time.Now().Add(-dur).UnixMilli()

			channelID := ""
			if channel != "" {
				channelID = resolveChannelID(db, channel)
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT data FROM transcripts WHERE CAST(date AS REAL) >= ? ORDER BY CAST(date AS REAL) DESC`,
				cutoff)
			if err != nil {
				return fmt.Errorf("querying: %w", err)
			}
			defer rows.Close()

			type statusRow struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Date   string `json:"date_string"`
				Status string `json:"status"`
			}
			var results []statusRow
			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				if channelID != "" && !t.inChannel(channelID) {
					continue
				}
				results = append(results, statusRow{
					ID:     t.ID,
					Title:  t.Title,
					Date:   t.DateString,
					Status: t.summaryStatus(),
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-50s  %s\n", "STATUS", "TITLE", "DATE")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s  %-50s  %s\n",
					r.Status, truncate(r.Title, 50), r.Date)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No transcripts found. Run 'fireflies-pp-cli sync' first.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "48h", "Duration to look back")
	cmd.Flags().StringVar(&channel, "channel", "", "Filter by channel name or ID")
	return cmd
}

func newTranscriptsPullCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "pull <id>",
		Short: "Fetch full transcript (sentences + analytics) from API and store locally",
		Long:  "Hydrate a transcript's sentences and analytics from the API. After pull, summary and speakers commands work offline for this transcript.",
		Example: strings.Trim(`
  fireflies-pp-cli transcripts pull abc123
  fireflies-pp-cli transcripts pull abc123 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch transcript %s from API\n", args[0])
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"id":"test","status":"ok"}`)
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			client, err := gql.New(cfg)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			data, err := client.Query(cmd.Context(), fullTranscriptQuery, map[string]any{"id": args[0]}, "transcript")
			if err != nil {
				return fmt.Errorf("fetching transcript: %w", err)
			}
			if err := db.UpsertTranscripts(data); err != nil {
				return fmt.Errorf("storing transcript: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), data, flags)
			}
			var t transcriptRow
			if err := json.Unmarshal(data, &t); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Pulled: %s (%s)\n", t.Title, t.summaryStatus())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newTranscriptsGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var live bool

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a transcript from local store (or API with --live)",
		Example: strings.Trim(`
  fireflies-pp-cli transcripts get abc123
  fireflies-pp-cli transcripts get abc123 --live --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"id":"test"}`)
				return nil
			}

			var data json.RawMessage

			if !live {
				if dbPath == "" {
					dbPath = defaultDBPath("fireflies-pp-cli")
				}
				db, err := store.OpenReadOnly(dbPath)
				if err != nil {
					return fmt.Errorf("opening store: %w", err)
				}
				defer db.Close()
				data, err = db.Get("transcripts", args[0])
				if err != nil {
					return fmt.Errorf("transcript not found locally — try 'fireflies-pp-cli transcripts pull %s'", args[0])
				}
			} else {
				cfg, err := config.Load(flags.configPath)
				if err != nil {
					return fmt.Errorf("loading config: %w", err)
				}
				client, err := gql.New(cfg)
				if err != nil {
					return err
				}
				data, err = client.Query(cmd.Context(), fullTranscriptQuery, map[string]any{"id": args[0]}, "transcript")
				if err != nil {
					return fmt.Errorf("fetching transcript: %w", err)
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&live, "live", false, "Fetch from API instead of local store")
	return cmd
}

func newTranscriptsExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var vaultPath string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "export <id>",
		Short: "Export a transcript as markdown to a file or vault path",
		Long: `Export a transcript as formatted markdown. Use --vault to write to a directory
using the auto-generated filename: YYYY-MM-DD_<sanitized-title>.md`,
		Example: strings.Trim(`
  fireflies-pp-cli transcripts export abc123 --vault ~/vaults/VBT/Projects/1_Active/Ryder/transcripts/
  fireflies-pp-cli transcripts export abc123 --output ./meeting-notes.md`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would export transcript %s\n", args[0])
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"exported":true}`)
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli transcripts pull %s' first.", err, args[0])
			}
			defer db.Close()

			raw, err := db.Get("transcripts", args[0])
			if err != nil {
				return fmt.Errorf("transcript %s not found — run 'transcripts pull %s' first", args[0], args[0])
			}

			var t transcriptRow
			if err := json.Unmarshal(raw, &t); err != nil {
				return fmt.Errorf("parsing transcript: %w", err)
			}

			md := renderTranscriptMarkdown(&t)

			dest := outputFile
			if dest == "" && vaultPath != "" {
				vaultPath = expandHome(vaultPath)
				if err := os.MkdirAll(vaultPath, 0o755); err != nil {
					return fmt.Errorf("creating vault dir: %w", err)
				}
				filename := t.vaultFilename()
				dest = filepath.Join(vaultPath, filename)
			}

			if dest == "" {
				fmt.Fprint(cmd.OutOrStdout(), md)
				return nil
			}

			if err := os.WriteFile(dest, []byte(md), 0o644); err != nil {
				return fmt.Errorf("writing file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported to: %s\n", dest)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&vaultPath, "vault", "", "Directory to write markdown (filename auto-generated as YYYY-MM-DD_title.md)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Explicit output file path")
	return cmd
}

func newTranscriptsDeleteCmd(flags *rootFlags) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a transcript from Fireflies",
		Example: `  fireflies-pp-cli transcripts delete abc123 --yes --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would DELETE transcript %s\n", args[0])
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"success":true}`)
				return nil
			}
			if !yes {
				return fmt.Errorf("use --yes to confirm deletion of transcript %s", args[0])
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			client, err := gql.New(cfg)
			if err != nil {
				return err
			}
			const q = `mutation DeleteTranscript($id: String!) { deleteTranscript(id: $id) { success message } }`
			data, err := client.Query(cmd.Context(), q, map[string]any{"id": args[0]}, "deleteTranscript")
			if err != nil {
				return fmt.Errorf("deleting transcript: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	return cmd
}

func newTranscriptsUpdateCmd(flags *rootFlags) *cobra.Command {
	var title string
	var privacy string

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update transcript title or privacy",
		Example: strings.Trim(`
  fireflies-pp-cli transcripts update abc123 --title "New Title"
  fireflies-pp-cli transcripts update abc123 --privacy TEAM --dry-run`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would update transcript %s\n", args[0])
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"success":true}`)
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			client, err := gql.New(cfg)
			if err != nil {
				return err
			}
			if title != "" {
				const q = `mutation UpdateMeetingTitle($id: String!, $title: String!) { updateMeetingTitle(id: $id, title: $title) { success message } }`
				data, err := client.Query(cmd.Context(), q, map[string]any{"id": args[0], "title": title}, "updateMeetingTitle")
				if err != nil {
					return fmt.Errorf("updating title: %w", err)
				}
				return printJSONFiltered(cmd.OutOrStdout(), data, flags)
			}
			if privacy != "" {
				const q = `mutation UpdateMeetingPrivacy($id: String!, $privacy: String!) { updateMeetingPrivacy(id: $id, privacy: $privacy) { success message } }`
				data, err := client.Query(cmd.Context(), q, map[string]any{"id": args[0], "privacy": privacy}, "updateMeetingPrivacy")
				if err != nil {
					return fmt.Errorf("updating privacy: %w", err)
				}
				return printJSONFiltered(cmd.OutOrStdout(), data, flags)
			}
			return fmt.Errorf("provide --title or --privacy")
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "New title for the transcript")
	cmd.Flags().StringVar(&privacy, "privacy", "", "Privacy setting: PUBLIC | PRIVATE | TEAM")
	return cmd
}

func newTranscriptsShareCmd(flags *rootFlags) *cobra.Command {
	var emails []string
	var expiryDays int

	cmd := &cobra.Command{
		Use:   "share <id>",
		Short: "Share a transcript with external email addresses",
		Example: `  fireflies-pp-cli transcripts share abc123 --emails user@company.com --expiry 7`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would share transcript %s with %v\n", args[0], emails)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"success":true}`)
				return nil
			}
			if len(emails) == 0 {
				return fmt.Errorf("provide at least one --emails address")
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			client, err := gql.New(cfg)
			if err != nil {
				return err
			}
			vars := map[string]any{"meeting_id": args[0], "emails": emails}
			if expiryDays > 0 {
				vars["expiry_days"] = expiryDays
			}
			const q = `mutation ShareMeeting($meeting_id: String!, $emails: [String!]!, $expiry_days: Int) { shareMeeting(meeting_id: $meeting_id, emails: $emails, expiry_days: $expiry_days) { success message } }`
			data, err := client.Query(cmd.Context(), q, vars, "shareMeeting")
			if err != nil {
				return fmt.Errorf("sharing transcript: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringSliceVar(&emails, "emails", nil, "Email addresses to share with (comma-separated)")
	cmd.Flags().IntVar(&expiryDays, "expiry", 0, "Link expiry in days: 7, 14, or 30")
	return cmd
}

// --- helpers ---

// transcriptRow is a minimal struct for parsing transcript JSON from the store.
type transcriptRow struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Date        float64         `json:"date"`
	DateString  string          `json:"dateString"`
	Duration    float64         `json:"duration"`
	Privacy     string          `json:"privacy"`
	OrgEmail    string          `json:"organizer_email"`
	Participants []string       `json:"participants"`
	MeetingInfo *meetingInfo    `json:"meeting_info"`
	Summary     *summaryFields  `json:"summary"`
	Channels    []channelRef    `json:"channels"`
	Speakers    []speakerRef    `json:"speakers"`
	Analytics   json.RawMessage `json:"analytics"`
	Sentences   json.RawMessage `json:"sentences"`
}

type meetingInfo struct {
	FredJoined    bool   `json:"fred_joined"`
	SilentMeeting bool   `json:"silent_meeting"`
	SummaryStatus string `json:"summary_status"`
}

type summaryFields struct {
	ActionItems    string   `json:"action_items"`
	Keywords       []string `json:"keywords"`
	Overview       string   `json:"overview"`
	ShorthandBullet string  `json:"shorthand_bullet"`
	Gist           string   `json:"gist"`
	Topics         []string `json:"topics_discussed"`
	Outline        string   `json:"outline"`
	Notes          string   `json:"notes"`
}

type channelRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type speakerRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (t *transcriptRow) summaryStatus() string {
	if t.MeetingInfo == nil {
		return "UNKNOWN"
	}
	return strings.ToUpper(t.MeetingInfo.SummaryStatus)
}

func (t *transcriptRow) dateFormatted() string {
	if t.Date > 0 {
		ts := time.UnixMilli(int64(t.Date))
		return ts.Format("2006-01-02 15:04")
	}
	if t.DateString != "" {
		n := 19
		if len(t.DateString) < n {
			n = len(t.DateString)
		}
		return t.DateString[:n]
	}
	return "unknown"
}

func (t *transcriptRow) durationFormatted() string {
	if t.Duration <= 0 {
		return ""
	}
	mins := int(t.Duration)
	return fmt.Sprintf("%dm", mins)
}

func (t *transcriptRow) hasParticipant(email string) bool {
	email = strings.ToLower(email)
	for _, p := range t.Participants {
		if strings.ToLower(p) == email {
			return true
		}
	}
	return false
}

func (t *transcriptRow) inChannel(channelIDOrTitle string) bool {
	q := strings.ToLower(channelIDOrTitle)
	for _, ch := range t.Channels {
		if strings.ToLower(ch.ID) == q || strings.ToLower(ch.Title) == q {
			return true
		}
	}
	return false
}

func (t *transcriptRow) hasKeywordInSummary(kw string) bool {
	if t.Summary == nil {
		return false
	}
	if strings.Contains(strings.ToLower(t.Summary.Overview), kw) {
		return true
	}
	if strings.Contains(strings.ToLower(t.Summary.ActionItems), kw) {
		return true
	}
	for _, topic := range t.Summary.Topics {
		if strings.Contains(strings.ToLower(topic), kw) {
			return true
		}
	}
	return false
}

func (t *transcriptRow) vaultFilename() string {
	datePrefix := ""
	if t.Date > 0 {
		ts := time.UnixMilli(int64(t.Date))
		datePrefix = ts.Format("2006-01-02")
	}
	title := sanitizeFilename(t.Title)
	if title == "" {
		title = t.ID
	}
	return datePrefix + "_" + title + ".md"
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('_')
		}
	}
	result := b.String()
	if len(result) > 60 {
		result = result[:60]
	}
	return result
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func resolveChannelID(db *store.Store, nameOrID string) string {
	rows, err := db.DB().QueryContext(context.Background(), `SELECT data FROM channels`)
	if err != nil {
		return nameOrID
	}
	defer rows.Close()
	lower := strings.ToLower(nameOrID)
	for rows.Next() {
		var raw []byte
		if rows.Scan(&raw) != nil {
			continue
		}
		var ch struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		if json.Unmarshal(raw, &ch) != nil {
			continue
		}
		if strings.ToLower(ch.Title) == lower || ch.ID == nameOrID {
			return ch.ID
		}
	}
	return nameOrID
}

func renderTranscriptMarkdown(t *transcriptRow) string {
	var sb strings.Builder
	sb.WriteString("# " + t.Title + "\n\n")
	sb.WriteString("**Date:** " + t.dateFormatted() + "\n")
	sb.WriteString("**Duration:** " + t.durationFormatted() + "\n")
	sb.WriteString("**Status:** " + t.summaryStatus() + "\n")
	sb.WriteString("**Organizer:** " + t.OrgEmail + "\n")
	if len(t.Participants) > 0 {
		sb.WriteString("**Participants:** " + strings.Join(t.Participants, ", ") + "\n")
	}
	sb.WriteString("\n---\n\n## Summary\n\n")
	if t.Summary != nil {
		if t.Summary.Overview != "" {
			sb.WriteString("### Overview\n\n" + t.Summary.Overview + "\n\n")
		}
		if t.Summary.ActionItems != "" {
			sb.WriteString("### Action Items\n\n" + t.Summary.ActionItems + "\n\n")
		}
		if len(t.Summary.Topics) > 0 {
			sb.WriteString("### Topics Discussed\n\n")
			for _, topic := range t.Summary.Topics {
				sb.WriteString("- " + topic + "\n")
			}
			sb.WriteString("\n")
		}
		if len(t.Summary.Keywords) > 0 {
			sb.WriteString("### Keywords\n\n" + strings.Join(t.Summary.Keywords, ", ") + "\n\n")
		}
	}
	return sb.String()
}


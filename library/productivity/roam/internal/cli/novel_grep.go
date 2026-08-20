package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/roam/internal/store"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newGrepCmd(flags *rootFlags) *cobra.Command {
	var since, fromUser, inMeeting, inGroup string
	var limit int

	cmd := &cobra.Command{
		Use:   "grep <query>",
		Short: "Search across every chat message and meeting transcript at once",
		Long: `FTS5 search across messages + transcripts in the local store.
Filters: --since 7d, --from-user <id>, --in-meeting <id>, --in-group <chat-id>.

Run 'roam-pp-cli sync' first to populate the local store.`,
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// pp:store-read — queries the local SQLite store (messages_fts, transcript_text_fts)
			query := strings.Join(args, " ")
			var _ store.Store
			var db *sql.DB
			db, closeDB, err := openNovelDB(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer closeDB()
			if err := ensureMessagesTables(cmd.Context(), db); err != nil {
				return apiErr(err)
			}

			cutoff := ""
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(err)
				}
				cutoff = ts.UTC().Format(time.RFC3339)
			}
			if limit <= 0 {
				limit = 50
			}

			type hit struct {
				Source    string `json:"source"`
				ID        string `json:"id"`
				ChatID    string `json:"chat_id,omitempty"`
				MeetingID string `json:"meeting_id,omitempty"`
				EventName string `json:"event_name,omitempty"`
				FromUser  string `json:"from_user,omitempty"`
				Ts        string `json:"ts,omitempty"`
				Snippet   string `json:"snippet"`
			}
			out := []hit{}

			// Messages
			msgSQL := `SELECT m.id, m.chat_id, m.from_user, m.ts, snippet(messages_fts, 0, '[', ']', '...', 16) AS snip
			           FROM messages m JOIN messages_fts ON messages_fts.rowid = m.rowid
			           WHERE messages_fts MATCH ?`
			msgArgs := []any{query}
			if cutoff != "" {
				msgSQL += ` AND m.ts >= ?`
				msgArgs = append(msgArgs, cutoff)
			}
			if fromUser != "" {
				msgSQL += ` AND m.from_user = ?`
				msgArgs = append(msgArgs, fromUser)
			}
			if inGroup != "" {
				msgSQL += ` AND m.chat_id = ?`
				msgArgs = append(msgArgs, inGroup)
			}
			msgSQL += fmt.Sprintf(` ORDER BY m.ts DESC LIMIT %d`, limit)
			rows, err := db.QueryContext(cmd.Context(), msgSQL, msgArgs...)
			if err == nil {
				for rows.Next() {
					var h hit
					h.Source = "message"
					_ = rows.Scan(&h.ID, &h.ChatID, &h.FromUser, &h.Ts, &h.Snippet)
					out = append(out, h)
				}
				rows.Close()
			}

			// Transcripts
			tSQL := `SELECT t.transcript_id, t.meeting_id, t.event_name, t.start, snippet(transcript_text_fts, 0, '[', ']', '...', 16) AS snip
			         FROM transcript_text t JOIN transcript_text_fts ON transcript_text_fts.rowid = t.rowid
			         WHERE transcript_text_fts MATCH ?`
			tArgs := []any{query}
			if cutoff != "" {
				tSQL += ` AND t.start >= ?`
				tArgs = append(tArgs, cutoff)
			}
			if inMeeting != "" {
				tSQL += ` AND t.meeting_id = ?`
				tArgs = append(tArgs, inMeeting)
			}
			tSQL += fmt.Sprintf(` ORDER BY t.start DESC LIMIT %d`, limit)
			rows2, err := db.QueryContext(cmd.Context(), tSQL, tArgs...)
			if err == nil {
				for rows2.Next() {
					var h hit
					h.Source = "transcript"
					_ = rows2.Scan(&h.ID, &h.MeetingID, &h.EventName, &h.Ts, &h.Snippet)
					out = append(out, h)
				}
				rows2.Close()
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				body, _ := json.Marshal(map[string]any{"query": query, "hits": out})
				fmt.Fprintln(w, string(body))
				return nil
			}
			if len(out) == 0 {
				fmt.Fprintln(w, "(no hits — run 'roam-pp-cli sync' to populate the store)")
				return nil
			}
			for _, h := range out {
				ctx := h.ChatID
				if ctx == "" {
					ctx = h.EventName
				}
				fmt.Fprintf(w, "[%s] %s  %s  %s\n", h.Source, h.Ts, ctx, h.Snippet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only consider items since (e.g. 7d, 24h, 30m)")
	cmd.Flags().StringVar(&fromUser, "from-user", "", "Restrict messages to a specific user/bot ID")
	cmd.Flags().StringVar(&inMeeting, "in-meeting", "", "Restrict transcripts to a specific meeting ID")
	cmd.Flags().StringVar(&inGroup, "in-group", "", "Restrict messages to a specific chat/group address ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max hits per source (default 50)")
	return cmd
}

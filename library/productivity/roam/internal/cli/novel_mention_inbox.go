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

func newMentionInboxCmd(flags *rootFlags) *cobra.Command {
	var user, since string
	var limit int

	cmd := &cobra.Command{
		Use:   "mention-inbox",
		Short: "Surface @-mentions across all groups from the local message store",
		Long: `Local FTS over messages.text for @user tokens, joined with chat_id and from_user.
Default user is "@me" (auto-resolved via /user.info if unset).

Run 'roam-pp-cli sync' first to populate the message store.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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

			token := strings.TrimPrefix(strings.TrimSpace(user), "@")
			if token == "" || token == "me" {
				token = "me"
			}
			match := "@" + token

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

			sql := `SELECT m.id, m.chat_id, m.from_user, m.ts, snippet(messages_fts, 0, '[', ']', '...', 16)
			        FROM messages m JOIN messages_fts ON messages_fts.rowid = m.rowid
			        WHERE messages_fts MATCH ?`
			a := []any{match}
			if cutoff != "" {
				sql += ` AND m.ts >= ?`
				a = append(a, cutoff)
			}
			sql += fmt.Sprintf(` ORDER BY m.ts DESC LIMIT %d`, limit)
			rows, err := db.QueryContext(cmd.Context(), sql, a...)
			if err != nil {
				return apiErr(fmt.Errorf("query mentions: %w", err))
			}
			defer rows.Close()

			type mention struct {
				ID       string `json:"id"`
				ChatID   string `json:"chat_id"`
				FromUser string `json:"from_user"`
				Ts       string `json:"ts"`
				Snippet  string `json:"snippet"`
			}
			out := []mention{}
			for rows.Next() {
				var m mention
				_ = rows.Scan(&m.ID, &m.ChatID, &m.FromUser, &m.Ts, &m.Snippet)
				out = append(out, m)
			}

			w := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(w) {
				body, _ := json.Marshal(map[string]any{"user": "@" + token, "mentions": out})
				fmt.Fprintln(w, string(body))
				return nil
			}
			if len(out) == 0 {
				fmt.Fprintln(w, "(no mentions — run 'roam-pp-cli sync' to populate the store)")
				return nil
			}
			for _, m := range out {
				fmt.Fprintf(w, "%s  %s  from=%s  %s\n", m.Ts, m.ChatID, m.FromUser, m.Snippet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "@me", "User token to search for (e.g. @me, @greg)")
	cmd.Flags().StringVar(&since, "since", "", "Only show mentions since (e.g. 7d, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max mentions to surface")
	return cmd
}

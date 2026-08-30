// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type threadMessage struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp,omitempty"`
	Seen      bool   `json:"seen"`
}

type threadView struct {
	ChatID   string          `json:"chat_id"`
	Provider string          `json:"provider,omitempty"`
	With     string          `json:"with,omitempty"`
	Count    int             `json:"message_count"`
	Messages []threadMessage `json:"messages"`
	Note     string          `json:"note,omitempty"`
}

func newNovelThreadCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		chatID string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "thread [chat-id]",
		Short: "Read one conversation end to end with names resolved",
		Long: strings.Trim(`
Renders a full conversation from the local mirror in chronological order, with
the counterpart resolved to a real name instead of an opaque provider id.

Use this command when you need the whole context of one conversation. Do NOT
use it to list conversations; use 'inbox' or 'chats list'.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli thread --chat example-chat-id --agent
  unipile-pp-cli thread example-chat-id --limit 50
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "chat=example-chat-id",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "thread")
			}
			if len(args) > 0 && chatID == "" {
				chatID = args[0]
			}
			if strings.TrimSpace(chatID) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a chat id is required (positional argument or --chat)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := threadView{ChatID: chatID, Messages: make([]threadMessage, 0)}
			db, ok, err := novelStore(cmd, flags, dbPath, view)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			chats, err := loadChats(ctx, db, false)
			if err != nil {
				return err
			}
			relations, err := loadRelations(ctx, db)
			if err != nil {
				return err
			}
			attendees, err := loadAttendees(ctx, db)
			if err != nil {
				return err
			}
			var match *chatRow
			for i := range chats {
				if chats[i].ID == chatID {
					match = &chats[i]
					break
				}
			}
			if match != nil {
				view.Provider = match.AccountType
				view.With = counterpartName(*match, relations, attendees)
			}

			rows, err := db.QueryContext(ctx, `
				SELECT COALESCE(id,''), COALESCE(text,''), COALESCE(timestamp,''),
				       COALESCE(is_sender,'0'), COALESCE(seen,'0'),
				       COALESCE(sender_attendee_id,'')
				FROM messages WHERE chat_id = ?`, chatID)
			if err != nil {
				return fmt.Errorf("reading thread: %w", err)
			}
			for rows.Next() {
				var id, text, ts, isSender, seen, senderAttendee sql.NullString
				if err := rows.Scan(&id, &text, &ts, &isSender, &seen, &senderAttendee); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning thread message: %w", err)
				}
				from := ""
				switch {
				case isSender.String == "1" || strings.EqualFold(isSender.String, "true"):
					from = "you"
				default:
					if n, ok := attendees.byAttendeeID[senderAttendee.String]; ok && n != "" {
						from = n
					} else {
						from = view.With
					}
				}
				if from == "" {
					from = "(unknown)"
				}
				view.Messages = append(view.Messages, threadMessage{
					ID:        id.String,
					From:      from,
					Text:      text.String,
					Timestamp: ts.String,
					Seen:      seen.String == "1" || strings.EqualFold(seen.String, "true"),
				})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating thread: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing thread rows: %w", err)
			}
			sort.Slice(view.Messages, func(i, j int) bool { return view.Messages[i].Timestamp < view.Messages[j].Timestamp })
			if limit > 0 && len(view.Messages) > limit {
				view.Messages = view.Messages[len(view.Messages)-limit:]
			}
			view.Count = len(view.Messages)
			synced := mirrorPopulated(ctx, db)
			if view.Count == 0 {
				if synced {
					view.Note = fmt.Sprintf("no messages for chat %q in the local mirror", chatID)
				} else {
					view.Note = "the local mirror is empty; run 'unipile-pp-cli sync --resources chats,messages' first"
				}
			}

			// An unknown chat id is a not-found (exit 3), not a successful
			// empty thread, so scripts can tell the two apart.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				if view.Count == 0 && synced {
					return notFoundErr(fmt.Errorf("no messages for chat %q in the local mirror", chatID))
				}
				return nil
			}
			if view.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				if synced {
					return notFoundErr(fmt.Errorf("no messages for chat %q in the local mirror", chatID))
				}
				return nil
			}
			header := view.With
			if header == "" {
				header = view.ChatID
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) - %d messages\n\n", header, view.Provider, view.Count)
			for _, m := range view.Messages {
				when := m.Timestamp
				if parsed, perr := time.Parse(time.RFC3339, m.Timestamp); perr == nil {
					when = parsed.Local().Format("2006-01-02 15:04")
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s\n", when, m.From, m.Text)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().StringVar(&chatID, "chat", "", "chat id to render (alternative to the positional argument)")
	cmd.Flags().IntVar(&limit, "limit", 200, "maximum messages to render, most recent last")
	return cmd
}

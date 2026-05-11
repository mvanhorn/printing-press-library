// `chats resolve` — novel #7. Resolves an @username to a numeric chat_id
// with local caching; falls back to getChat on cache miss.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newChatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chats",
		Short: "Local chat helpers (resolve handles, list cached chats)",
	}
	cmd.AddCommand(newChatsResolveCmd(flags))
	return cmd
}

func newChatsResolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve [@username]",
		Short: "Resolve an @username to a numeric chat_id (local cache, getChat fallback)",
		Long:  "Returns {chat_id, type, title} for the given @handle. Looks up the local cache first; on miss calls getChat and caches the result. Handles the case where a private group is upgraded to supergroup and the numeric chat_id changes.",
		Example: strings.Trim(`
  telegram-pp-cli chats resolve @mychan
  telegram-pp-cli chats resolve @support_bot --json --select chat_id,type
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,3,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			handle := args[0]
			if !strings.HasPrefix(handle, "@") {
				handle = "@" + handle
			}

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"username": handle, "dry_run": true}, flags)
			}

			s, err := openNovelStore(cmd.Context())
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			botID, err := resolveBotID(c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Local cache lookup.
			if entry, ok, lerr := lookupResolvedChat(cmd.Context(), s.DB(), botID, handle); lerr == nil && ok {
				entry["source"] = "cache"
				return printJSONFiltered(cmd.OutOrStdout(), entry, flags)
			}

			// Fallback: getChat.
			data, _, err := c.Post("/getChat", map[string]any{"chat_id": handle})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var envelope struct {
				Ok     bool            `json:"ok"`
				Result json.RawMessage `json:"result"`
			}
			var chat struct {
				ID    int64  `json:"id"`
				Type  string `json:"type"`
				Title string `json:"title"`
			}
			if err := json.Unmarshal(data, &envelope); err == nil && envelope.Ok && len(envelope.Result) > 0 {
				_ = json.Unmarshal(envelope.Result, &chat)
			} else {
				_ = json.Unmarshal(data, &chat)
			}
			if chat.ID == 0 {
				return apiErr(fmt.Errorf("getChat returned no chat id for %s", handle))
			}
			chatID := strconv.FormatInt(chat.ID, 10)
			if err := saveResolvedChat(cmd.Context(), s.DB(), botID, handle, chatID, chat.Type, chat.Title); err != nil {
				// Cache write failure is non-fatal — surface the resolved id.
				_ = err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"username": handle,
				"chat_id":  chatID,
				"type":     chat.Type,
				"title":    chat.Title,
				"source":   "api",
			}, flags)
		},
	}
	return cmd
}

func lookupResolvedChat(ctx context.Context, db *sql.DB, botID, username string) (map[string]any, bool, error) {
	row := db.QueryRowContext(ctx,
		`SELECT chat_id, COALESCE(chat_type,''), COALESCE(title,'') FROM telegram_chats_resolved
         WHERE bot_id = ? AND username = ?`,
		botID, username)
	var chatID, chatType, title string
	if err := row.Scan(&chatID, &chatType, &title); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return map[string]any{
		"username": username,
		"chat_id":  chatID,
		"type":     chatType,
		"title":    title,
	}, true, nil
}

func saveResolvedChat(ctx context.Context, db *sql.DB, botID, username, chatID, chatType, title string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO telegram_chats_resolved (bot_id, username, chat_id, chat_type, title)
         VALUES (?, ?, ?, ?, ?)`,
		botID, username, chatID, chatType, title)
	return err
}

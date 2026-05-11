// `publish` — novel #6. Sends a long-form post as one logical artifact
// (auto-split into chunks, recorded under a caller-chosen slug). The
// `publish edit` companion re-finds the slug and re-edits only the chunks
// whose content hash changed.

package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/internal/client"
)

func newPublishCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Send a long-form post tracked under a slug; later re-edit only changed chunks",
	}
	cmd.AddCommand(newPublishSendCmd(flags))
	cmd.AddCommand(newPublishEditCmd(flags))
	cmd.AddCommand(newPublishListCmd(flags))
	return cmd
}

func newPublishSendCmd(flags *rootFlags) *cobra.Command {
	var (
		channel   string
		bodyFile  string
		recordAs  string
		parseMode string
		htmlMode  bool
	)
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a multi-part post and record (slug -> message_ids) in the local store",
		Example: strings.Trim(`
  telegram-pp-cli publish send --channel @mychan --body release.md --record-as v1.2 --html
`, "\n"),
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2,3,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if channel == "" || bodyFile == "" || recordAs == "" {
				return cmd.Help()
			}
			body, err := os.ReadFile(bodyFile)
			if err != nil {
				return usageErr(fmt.Errorf("reading --body file %s: %w", bodyFile, err))
			}
			text := string(body)
			if htmlMode && parseMode == "" {
				parseMode = "HTML"
			}
			parts := splitMessage(text, maxMessageBytes)

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"channel":    channel,
					"slug":       recordAs,
					"parts":      len(parts),
					"parse_mode": parseMode,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			botID, err := resolveBotID(c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			s, err := openNovelStore(cmd.Context())
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			// Refuse to clobber an existing slug — `publish edit` is the
			// path for updates. This prevents accidentally re-sending a
			// duplicate when a script forgets `edit`.
			if exists, err := publishSlugExists(cmd.Context(), s.DB(), botID, recordAs); err != nil {
				return apiErr(err)
			} else if exists {
				return usageErr(fmt.Errorf("publish slug %q already exists; use `publish edit --record-as %s` to update", recordAs, recordAs))
			}

			sentIDs := make([]int64, 0, len(parts))
			for i, part := range parts {
				reqBody := map[string]any{
					"chat_id": channel,
					"text":    part,
				}
				if parseMode != "" {
					reqBody["parse_mode"] = parseMode
				}
				data, _, err := c.Post("/sendMessage", reqBody)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				msg, perr := parseTelegramMessage(data)
				if perr != nil {
					return apiErr(perr)
				}
				sentIDs = append(sentIDs, msg.MessageID)
				_ = recordPublishChunk(cmd.Context(), s.DB(), botID, channel, msg.MessageID, part, parseMode, msg.Date, recordAs, i)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"slug":        recordAs,
				"channel":     channel,
				"parts":       len(sentIDs),
				"message_ids": sentIDs,
				"ok":          true,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "Target channel @username or chat ID")
	cmd.Flags().StringVar(&bodyFile, "body", "", "Path to file containing the post body")
	cmd.Flags().StringVar(&recordAs, "record-as", "", "Caller-chosen slug to record this post under")
	cmd.Flags().StringVar(&parseMode, "parse-mode", "", "HTML | MarkdownV2 | Markdown")
	cmd.Flags().BoolVar(&htmlMode, "html", false, "Shorthand for --parse-mode HTML")
	return cmd
}

func newPublishEditCmd(flags *rootFlags) *cobra.Command {
	var (
		recordAs string
		bodyFile string
	)
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Re-edit a published post; only chunks whose content changed are sent to the API",
		Example: strings.Trim(`
  telegram-pp-cli publish edit --record-as v1.2 --body release-v2.md
`, "\n"),
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2,3,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if recordAs == "" || bodyFile == "" {
				return cmd.Help()
			}
			body, err := os.ReadFile(bodyFile)
			if err != nil {
				return usageErr(fmt.Errorf("reading --body file %s: %w", bodyFile, err))
			}
			newParts := splitMessage(string(body), maxMessageBytes)

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"slug":      recordAs,
					"new_parts": len(newParts),
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			botID, err := resolveBotID(c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			s, err := openNovelStore(cmd.Context())
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			existing, err := loadPublishChunks(cmd.Context(), s.DB(), botID, recordAs)
			if err != nil {
				return apiErr(err)
			}
			if len(existing) == 0 {
				return usageErr(fmt.Errorf("publish slug %q not found; use `publish send` first", recordAs))
			}

			edits, unchanged, appended, err := reconcilePublishChunks(cmd.Context(), c, s.DB(), botID, recordAs, existing, newParts)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"slug":      recordAs,
				"channel":   existing[0].ChatID,
				"edited":    edits,
				"unchanged": unchanged,
				"appended":  appended,
				"ok":        true,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&recordAs, "record-as", "", "Slug from a prior publish send")
	cmd.Flags().StringVar(&bodyFile, "body", "", "Path to the updated body file")
	return cmd
}

func newPublishListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recorded publish slugs and their chunk counts",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,10",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"slugs": []any{}}, flags)
			}
			s, err := openNovelStore(cmd.Context())
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(),
				`SELECT publish_slug, chat_id, COUNT(*) AS chunks, MAX(date) AS last_sent
                 FROM telegram_messages
                 WHERE publish_slug IS NOT NULL AND publish_slug != ''
                 GROUP BY publish_slug, chat_id
                 ORDER BY last_sent DESC`)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type row struct {
				Slug     string `json:"slug"`
				Channel  string `json:"channel"`
				Chunks   int    `json:"chunks"`
				LastSent int64  `json:"last_sent"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Slug, &r.Channel, &r.Chunks, &r.LastSent); err != nil {
					return apiErr(err)
				}
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

type publishChunk struct {
	ChatID      string
	MessageID   int64
	ChunkIndex  int
	ContentHash string
	Text        string
}

func loadPublishChunks(ctx context.Context, db *sql.DB, botID, slug string) ([]publishChunk, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT chat_id, message_id, publish_chunk_index, COALESCE(content_hash,''), COALESCE(text,'')
         FROM telegram_messages
         WHERE bot_id = ? AND publish_slug = ?
         ORDER BY publish_chunk_index ASC`,
		botID, slug)
	if err != nil {
		return nil, fmt.Errorf("load publish chunks: %w", err)
	}
	defer rows.Close()
	var out []publishChunk
	for rows.Next() {
		var c publishChunk
		if err := rows.Scan(&c.ChatID, &c.MessageID, &c.ChunkIndex, &c.ContentHash, &c.Text); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func reconcilePublishChunks(ctx context.Context, c *client.Client, db *sql.DB, botID, slug string, existing []publishChunk, newParts []string) (edited, unchanged, appended int, err error) {
	common := len(existing)
	if len(newParts) < common {
		common = len(newParts)
	}
	for i := 0; i < common; i++ {
		newHash := contentHash(newParts[i])
		if newHash == existing[i].ContentHash && existing[i].Text == newParts[i] {
			unchanged++
			continue
		}
		body := map[string]any{
			"chat_id":    existing[i].ChatID,
			"message_id": existing[i].MessageID,
			"text":       newParts[i],
		}
		_, _, perr := c.Post("/editMessageText", body)
		if perr != nil {
			// Treat 400 as a permanent reject (Telegram refuses edits >48h or
			// "message is not modified"); record-and-continue rather than
			// aborting the whole reconciliation.
			var apiE *client.APIError
			if errors.As(perr, &apiE) && apiE.StatusCode == 400 {
				continue
			}
			return edited, unchanged, appended, perr
		}
		_, _ = db.ExecContext(ctx,
			`UPDATE telegram_messages SET text = ?, content_hash = ? WHERE bot_id = ? AND message_id = ? AND publish_chunk_index = ?`,
			newParts[i], newHash, botID, existing[i].MessageID, existing[i].ChunkIndex)
		edited++
	}
	// Append new chunks beyond the existing length.
	for i := common; i < len(newParts); i++ {
		body := map[string]any{
			"chat_id": existing[0].ChatID,
			"text":    newParts[i],
		}
		data, _, perr := c.Post("/sendMessage", body)
		if perr != nil {
			return edited, unchanged, appended, perr
		}
		msg, mperr := parseTelegramMessage(data)
		if mperr != nil {
			return edited, unchanged, appended, mperr
		}
		_ = recordPublishChunk(ctx, db, botID, existing[0].ChatID, msg.MessageID, newParts[i], "", msg.Date, slug, i)
		appended++
	}
	return edited, unchanged, appended, nil
}

func recordPublishChunk(ctx context.Context, db *sql.DB, botID, chatID string, messageID int64, text, parseMode string, date int64, slug string, chunkIndex int) error {
	hash := contentHash(text)
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO telegram_messages
            (bot_id, chat_id, message_id, direction, text, parse_mode, date, publish_slug, publish_chunk_index, content_hash)
         VALUES (?, ?, ?, 'outbound', ?, ?, ?, ?, ?, ?)`,
		botID, chatID, messageID, text, parseMode, date, slug, chunkIndex, hash)
	return err
}

func contentHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:8])
}

func publishSlugExists(ctx context.Context, db *sql.DB, botID, slug string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM telegram_messages WHERE bot_id = ? AND publish_slug = ?`,
		botID, slug).Scan(&n)
	return n > 0, err
}

// Verify json import is used
var _ = json.Marshal

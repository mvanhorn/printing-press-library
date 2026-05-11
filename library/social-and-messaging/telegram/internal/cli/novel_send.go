// `send` — the headline command. Wraps sendMessage with:
//   - HTML/MarkdownV2 parse-mode shortcuts
//   - --idempotency-key (novel #1) — cache hits skip the API call
//   - --replace-last (novel #2) — edits the previous outbound message
//   - --html-escape (novel #5) — Telegram-safe HTML escape
//   - --chat repeatable for multi-recipient fan-out
//   - auto-split for text >4096 chars
// All non-dry-run paths record the result to telegram_messages.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/telegram/internal/client"
)

const maxMessageBytes = 4096

func newSendCmd(flags *rootFlags) *cobra.Command {
	var (
		chats          []string
		text           string
		readStdin      bool
		parseMode      string
		htmlMode       bool
		htmlEscape     bool
		disablePreview bool
		silent         bool
		protect        bool
		replyTo        int
		replyMarkup    string
		idempotencyKey string
		replaceLast    bool
		noAutoSplit    bool
	)

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a text message to one or more chats (idempotency, replace-last, HTML safety)",
		Long:  "Send a text message via Telegram's sendMessage. Adds idempotency keys, replace-last status messaging, HTML-safe escaping, and auto-split for messages >4096 chars. Records every successful send to the local store so 'messages list', 'audit', and 'publish' can use it later.",
		Example: strings.Trim(`
  telegram-pp-cli send --chat 1234567 --text "Heartbeat OK" --silent
  telegram-pp-cli send --chat @mychan --text "<b>Release v1.2</b>" --html
  echo "from stdin" | telegram-pp-cli send --chat 1234567 --stdin
  telegram-pp-cli send --chat 1234567 --idempotency-key release-v1.2 --text "shipped"
  telegram-pp-cli send --chat 1234567 --replace-last --text "Step 3/5 complete"
`, "\n"),
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2,3,4,5,7",
			"mcp:read-only":       "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("chat") && len(chats) == 0 && len(args) == 0 && text == "" && !readStdin {
				return cmd.Help()
			}

			body, err := resolveSendText(text, readStdin, args)
			if err != nil {
				return usageErr(err)
			}
			if len(chats) == 0 {
				return usageErr(fmt.Errorf("at least one --chat is required"))
			}

			if htmlEscape {
				body = telegramHTMLEscape(body)
				if parseMode == "" && !htmlMode {
					htmlMode = true
				}
			}
			if htmlMode && parseMode == "" {
				parseMode = "HTML"
			}

			parts := []string{body}
			if !noAutoSplit && len(body) > maxMessageBytes {
				parts = splitMessage(body, maxMessageBytes)
			}

			if dryRunOK(flags) {
				preview := map[string]any{
					"chats":           chats,
					"parts":           len(parts),
					"first_part_len":  len(parts[0]),
					"parse_mode":      parseMode,
					"idempotency_key": idempotencyKey,
					"replace_last":    replaceLast,
				}
				return printJSONFiltered(cmd.OutOrStdout(), preview, flags)
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
			db := s.DB()

			results := make([]map[string]any, 0, len(chats))
			anyErr := false
			for _, chatID := range chats {
				res, sendErr := sendOneChat(cmd.Context(), c, db, botID, chatID, parts, parseMode,
					disablePreview, silent, protect, replyTo, replyMarkup,
					idempotencyKey, replaceLast)
				if sendErr != nil {
					anyErr = true
					results = append(results, map[string]any{
						"chat":  chatID,
						"ok":    false,
						"error": sendErr.Error(),
					})
					continue
				}
				results = append(results, res)
			}

			out := any(results)
			if len(results) == 1 {
				out = results[0]
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
				return err
			}
			if anyErr {
				return apiErr(fmt.Errorf("one or more sends failed"))
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&chats, "chat", nil, "Target chat ID or @username (repeatable for fan-out)")
	cmd.Flags().StringVar(&text, "text", "", "Message text; mutually exclusive with --stdin and positional text")
	cmd.Flags().BoolVar(&readStdin, "stdin", false, "Read message text from stdin")
	cmd.Flags().StringVar(&parseMode, "parse-mode", "", "HTML | MarkdownV2 | Markdown (use --html as a shorthand for HTML)")
	cmd.Flags().BoolVar(&htmlMode, "html", false, "Shorthand for --parse-mode HTML")
	cmd.Flags().BoolVar(&htmlEscape, "html-escape", false, "Telegram-safe HTML escape (preserves <b>/<i>/<u>/<s>/<code>/<pre>/<a>; encodes everything else). Implies --html.")
	cmd.Flags().BoolVar(&disablePreview, "disable-web-page-preview", false, "Disable link previews")
	cmd.Flags().BoolVar(&silent, "silent", false, "Send without push notification (disable_notification)")
	cmd.Flags().BoolVar(&protect, "protect", false, "Protect content from forwarding (protect_content)")
	cmd.Flags().IntVar(&replyTo, "reply-to", 0, "Reply to message_id")
	cmd.Flags().StringVar(&replyMarkup, "reply-markup", "", "JSON reply_markup (inline keyboard, etc.)")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Skip the API call if a previous send with this key already succeeded for this bot+chat; returns the cached result")
	cmd.Flags().BoolVar(&replaceLast, "replace-last", false, "Edit the most recent outbound message in this chat (editMessageText); falls back to send-new on 400")
	cmd.Flags().BoolVar(&noAutoSplit, "no-auto-split", false, "Disable auto-splitting of messages >4096 chars")
	return cmd
}

func resolveSendText(textFlag string, readStdin bool, args []string) (string, error) {
	if textFlag != "" {
		if readStdin {
			return "", fmt.Errorf("--text and --stdin are mutually exclusive")
		}
		// Positional args are silently ignored when --text is set. This is
		// load-bearing: shell quote stripping can split "hello world" into
		// two args, and rejecting the call would break the common
		// `send --text "$VAR"` pattern when $VAR contains spaces and the
		// caller forgot to quote.
		return textFlag, nil
	}
	if readStdin {
		if len(args) > 0 {
			return "", fmt.Errorf("--stdin is mutually exclusive with positional text")
		}
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return strings.TrimRight(string(b), "\n\r"), nil
	}
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	return "", fmt.Errorf("provide text via --text, --stdin, or a positional argument")
}

// sendOneChat handles a single chat: idempotency cache hit, replace-last
// fallback, multi-part fan-out, and store recording.
func sendOneChat(ctx context.Context, c *client.Client, db *sql.DB,
	botID, chatID string, parts []string, parseMode string,
	disablePreview, silent, protect bool,
	replyTo int, replyMarkup string,
	idempotencyKey string, replaceLast bool,
) (map[string]any, error) {
	if idempotencyKey != "" {
		if cached, ok, err := lookupIdempotent(ctx, db, botID, chatID, idempotencyKey); err != nil {
			return nil, err
		} else if ok {
			var payloadOut any = string(cached.Payload)
			var parsed any
			if err := json.Unmarshal(cached.Payload, &parsed); err == nil {
				payloadOut = parsed
			}
			return map[string]any{
				"chat":            chatID,
				"ok":              true,
				"cached":          true,
				"idempotency_key": idempotencyKey,
				"message_id":      cached.MessageID,
				"payload":         payloadOut,
			}, nil
		}
	}

	if replaceLast && len(parts) == 1 {
		if priorMID, ok, err := lookupLastOutbound(ctx, db, botID, chatID); err != nil {
			return nil, err
		} else if ok {
			editBody := map[string]any{
				"chat_id":    chatID,
				"message_id": priorMID,
				"text":       parts[0],
			}
			if parseMode != "" {
				editBody["parse_mode"] = parseMode
			}
			if disablePreview {
				editBody["disable_web_page_preview"] = true
			}
			if replyMarkup != "" {
				editBody["reply_markup"] = json.RawMessage(replyMarkup)
			}
			data, _, err := c.Post("/editMessageText", editBody)
			if err == nil {
				edited, _ := parseTelegramMessage(data)
				return map[string]any{
					"chat":         chatID,
					"ok":           true,
					"replace_last": true,
					"message_id":   edited.MessageID,
				}, nil
			}
			// Edit failed (message too old / not modified). Fall through to
			// a fresh send. The API error already contains the status code.
			var apiE *client.APIError
			if errors.As(err, &apiE) && apiE.StatusCode == 400 {
				// continue to send
			} else {
				return nil, err
			}
		}
	}

	sent := make([]int64, 0, len(parts))
	var lastPayload json.RawMessage
	for i, part := range parts {
		body := map[string]any{
			"chat_id": chatID,
			"text":    part,
		}
		if parseMode != "" {
			body["parse_mode"] = parseMode
		}
		if disablePreview {
			body["disable_web_page_preview"] = true
		}
		if silent {
			body["disable_notification"] = true
		}
		if protect {
			body["protect_content"] = true
		}
		if i == 0 && replyTo != 0 {
			body["reply_to_message_id"] = replyTo
		}
		if i == 0 && replyMarkup != "" {
			body["reply_markup"] = json.RawMessage(replyMarkup)
		}
		data, _, err := c.Post("/sendMessage", body)
		if err != nil {
			return nil, err
		}
		lastPayload = data
		msg, err := parseTelegramMessage(data)
		if err != nil {
			return nil, fmt.Errorf("parsing sendMessage response: %w", err)
		}
		sent = append(sent, msg.MessageID)
		_ = recordOutbound(ctx, db, botID, chatID, msg.MessageID, part, "", parseMode,
			msg.Date, idempotencyKey, "", 0)
	}
	if idempotencyKey != "" && len(sent) > 0 {
		_ = saveIdempotent(ctx, db, botID, chatID, idempotencyKey, sent[0], lastPayload)
	}
	return map[string]any{
		"chat":            chatID,
		"ok":              true,
		"message_ids":     sent,
		"message_id":      sent[0],
		"parts":           len(sent),
		"idempotency_key": idempotencyKey,
	}, nil
}

// splitMessage breaks text into <=limit-byte chunks at the best natural
// boundary (paragraph > newline > sentence > space > raw cut).
func splitMessage(s string, limit int) []string {
	out := []string{}
	for len(s) > limit {
		cut := strings.LastIndex(s[:limit], "\n\n")
		if cut < limit/2 {
			cut = strings.LastIndex(s[:limit], "\n")
		}
		if cut < limit/2 {
			if dot := strings.LastIndex(s[:limit], ". "); dot > 0 {
				cut = dot + 1
			}
		}
		if cut < limit/2 {
			cut = strings.LastIndex(s[:limit], " ")
		}
		if cut <= 0 {
			cut = limit
		}
		out = append(out, strings.TrimSpace(s[:cut]))
		s = s[cut:]
	}
	if strings.TrimSpace(s) != "" {
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

// allowedHTMLTagRE matches Telegram's allowed HTML subset
// (https://core.telegram.org/bots/api#html-style). Anything else is escaped.
var allowedHTMLTagRE = regexp.MustCompile(`(?i)</?(?:b|strong|i|em|u|ins|s|strike|del|span|tg-spoiler|a|code|pre|br)(?:\s+[^>]*)?>`)

func telegramHTMLEscape(s string) string {
	var b strings.Builder
	last := 0
	for _, loc := range allowedHTMLTagRE.FindAllStringIndex(s, -1) {
		b.WriteString(html.EscapeString(s[last:loc[0]]))
		b.WriteString(s[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(html.EscapeString(s[last:]))
	return b.String()
}

// ----- DB helpers -----

type cachedSend struct {
	MessageID int64
	Payload   json.RawMessage
}

func lookupIdempotent(ctx context.Context, db *sql.DB, botID, chatID, key string) (cachedSend, bool, error) {
	row := db.QueryRowContext(ctx,
		`SELECT message_id, payload FROM telegram_idempotency
         WHERE bot_id = ? AND chat_id = ? AND idempotency_key = ?`,
		botID, chatID, key)
	var c cachedSend
	var payload []byte
	if err := row.Scan(&c.MessageID, &payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cachedSend{}, false, nil
		}
		return cachedSend{}, false, fmt.Errorf("idempotency lookup: %w", err)
	}
	c.Payload = json.RawMessage(payload)
	return c, true, nil
}

func saveIdempotent(ctx context.Context, db *sql.DB, botID, chatID, key string, messageID int64, payload json.RawMessage) error {
	if payload == nil {
		payload = json.RawMessage("{}")
	}
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO telegram_idempotency (bot_id, chat_id, idempotency_key, message_id, payload)
         VALUES (?, ?, ?, ?, ?)`,
		botID, chatID, key, messageID, []byte(payload))
	if err != nil {
		return fmt.Errorf("save idempotency: %w", err)
	}
	return nil
}

func lookupLastOutbound(ctx context.Context, db *sql.DB, botID, chatID string) (int64, bool, error) {
	row := db.QueryRowContext(ctx,
		`SELECT message_id FROM telegram_messages
         WHERE bot_id = ? AND chat_id = ? AND direction = 'outbound'
         ORDER BY date DESC, id DESC LIMIT 1`,
		botID, chatID)
	var mid int64
	if err := row.Scan(&mid); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("last-outbound lookup: %w", err)
	}
	return mid, true, nil
}

func recordOutbound(ctx context.Context, db *sql.DB, botID, chatID string, messageID int64, text, caption, parseMode string, date int64, idempotencyKey, publishSlug string, chunkIndex int) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO telegram_messages
            (bot_id, chat_id, message_id, direction, text, caption, parse_mode, date, idempotency_key, publish_slug, publish_chunk_index)
         VALUES (?, ?, ?, 'outbound', ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)`,
		botID, chatID, messageID, text, caption, parseMode, date, idempotencyKey, publishSlug, chunkIndex)
	return err
}

// ----- bot identity -----

type telegramMessage struct {
	MessageID int64 `json:"message_id"`
	Date      int64 `json:"date"`
	Chat      struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Type     string `json:"type"`
		Title    string `json:"title"`
	} `json:"chat"`
}

// Cached so multi-chat fan-out doesn't call getMe N times within a single
// process. The cached id is bound to the auth context that produced it;
// invocations with a different token start a fresh process and reset.
var cachedBotID string

func resolveBotID(c *client.Client) (string, error) {
	if cachedBotID != "" {
		return cachedBotID, nil
	}
	data, _, err := c.Post("/getMe", map[string]any{})
	if err != nil {
		return "", err
	}
	var envelope struct {
		Ok     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	var inner struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Ok && len(envelope.Result) > 0 {
		_ = json.Unmarshal(envelope.Result, &inner)
	} else {
		_ = json.Unmarshal(data, &inner)
	}
	if inner.ID == 0 {
		return "", fmt.Errorf("getMe returned no bot id")
	}
	cachedBotID = strconv.FormatInt(inner.ID, 10)
	return cachedBotID, nil
}

// parseTelegramMessage handles both envelope and unwrapped response shapes.
func parseTelegramMessage(data json.RawMessage) (telegramMessage, error) {
	var envelope struct {
		Ok     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Ok && len(envelope.Result) > 0 {
		var m telegramMessage
		if err := json.Unmarshal(envelope.Result, &m); err == nil && m.MessageID != 0 {
			return m, nil
		}
	}
	var m telegramMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return telegramMessage{}, err
	}
	return m, nil
}

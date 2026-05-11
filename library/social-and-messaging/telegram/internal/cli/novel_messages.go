// `messages list` — novel #3. Pure local-store read of every message
// the bot has sent or received. No API call.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newMessagesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Browse the local message history",
	}
	cmd.AddCommand(newMessagesListCmd(flags))
	return cmd
}

func newMessagesListCmd(flags *rootFlags) *cobra.Command {
	var (
		chatID    string
		since     string
		mineOnly  bool
		inbound   bool
		mediaType string
		limit     int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List messages from the local store (no API call)",
		Long:  "Reads from the telegram_messages table populated by every successful send and 'sync'. Filter by chat, time window, direction, or media type.",
		Example: strings.Trim(`
  telegram-pp-cli messages list --json --limit 20
  telegram-pp-cli messages list --chat 1234567 --since 1d --mine
  telegram-pp-cli messages list --inbound --since 1h
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,10",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"messages": []any{}}, flags)
			}
			s, err := openNovelStore(cmd.Context())
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			rows, err := queryMessages(cmd.Context(), s.DB(), chatID, since, mineOnly, inbound, mediaType, limit)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&chatID, "chat", "", "Filter to a single chat ID or @username")
	cmd.Flags().StringVar(&since, "since", "", "Time window: e.g. 1h, 6h, 1d, 7d, 30d, or today")
	cmd.Flags().BoolVar(&mineOnly, "mine", false, "Only outbound messages (sent by this bot)")
	cmd.Flags().BoolVar(&inbound, "inbound", false, "Only inbound messages")
	cmd.Flags().StringVar(&mediaType, "media-type", "", "Filter by media type: text|photo|video|document|audio|animation|voice|sticker|location")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum rows to return")
	return cmd
}

type messageRow struct {
	BotID          string `json:"bot_id"`
	ChatID         string `json:"chat_id"`
	MessageID      int64  `json:"message_id"`
	Direction      string `json:"direction"`
	Text           string `json:"text,omitempty"`
	Caption        string `json:"caption,omitempty"`
	MediaType      string `json:"media_type,omitempty"`
	ParseMode      string `json:"parse_mode,omitempty"`
	Date           int64  `json:"date"`
	DateISO        string `json:"date_iso"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	PublishSlug    string `json:"publish_slug,omitempty"`
}

func queryMessages(ctx context.Context, db *sql.DB, chatID, since string, mineOnly, inboundOnly bool, mediaType string, limit int) ([]messageRow, error) {
	var (
		wheres []string
		argv   []any
	)
	if chatID != "" {
		wheres = append(wheres, "chat_id = ?")
		argv = append(argv, chatID)
	}
	if mineOnly && inboundOnly {
		return nil, fmt.Errorf("--mine and --inbound are mutually exclusive")
	}
	if mineOnly {
		wheres = append(wheres, "direction = 'outbound'")
	}
	if inboundOnly {
		wheres = append(wheres, "direction = 'inbound'")
	}
	if mediaType != "" {
		if mediaType == "text" {
			wheres = append(wheres, "(media_type IS NULL OR media_type = '')")
		} else {
			wheres = append(wheres, "media_type = ?")
			argv = append(argv, mediaType)
		}
	}
	if since != "" {
		cutoff, err := parseSince(since)
		if err != nil {
			return nil, err
		}
		wheres = append(wheres, "date >= ?")
		argv = append(argv, cutoff.Unix())
	}
	q := "SELECT bot_id, chat_id, message_id, direction, COALESCE(text,''), COALESCE(caption,''), COALESCE(media_type,''), COALESCE(parse_mode,''), date, COALESCE(idempotency_key,''), COALESCE(publish_slug,'') FROM telegram_messages"
	if len(wheres) > 0 {
		q += " WHERE " + strings.Join(wheres, " AND ")
	}
	q += " ORDER BY date DESC, id DESC LIMIT ?"
	if limit <= 0 {
		limit = 100
	}
	argv = append(argv, limit)

	rows, err := db.QueryContext(ctx, q, argv...)
	if err != nil {
		return nil, fmt.Errorf("messages query: %w", err)
	}
	defer rows.Close()

	out := []messageRow{}
	for rows.Next() {
		var r messageRow
		if err := rows.Scan(&r.BotID, &r.ChatID, &r.MessageID, &r.Direction, &r.Text, &r.Caption, &r.MediaType, &r.ParseMode, &r.Date, &r.IdempotencyKey, &r.PublishSlug); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		r.DateISO = time.Unix(r.Date, 0).UTC().Format(time.RFC3339)
		out = append(out, r)
	}
	return out, rows.Err()
}

// parseSince accepts: today, 1h, 6h, 1d, 7d, 30d, or a number of hours by default.
func parseSince(s string) (time.Time, error) {
	now := time.Now().UTC()
	if s == "today" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	if len(s) < 2 {
		return time.Time{}, fmt.Errorf("invalid --since %q (try 1h, 6h, 1d, 7d, 30d, or today)", s)
	}
	unit := s[len(s)-1]
	num, err := strconv.Atoi(s[:len(s)-1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid --since %q: %w", s, err)
	}
	switch unit {
	case 'h':
		return now.Add(time.Duration(-num) * time.Hour), nil
	case 'd':
		return now.AddDate(0, 0, -num), nil
	case 'm':
		return now.Add(time.Duration(-num) * time.Minute), nil
	default:
		return time.Time{}, fmt.Errorf("invalid --since unit in %q (use h, m, d, or 'today')", s)
	}
}

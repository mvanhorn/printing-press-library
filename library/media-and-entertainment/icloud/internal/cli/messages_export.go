// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newMessagesExportCmd(f *rootFlags) *cobra.Command {
	var chatID, outPath string
	var sinceStr, untilStr string
	var includeTapbacks bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a chat or all chats to JSON",
		Long: `Export message history as JSON, including attachment paths and
file-existence flags. Tapbacks are excluded by default.

Use --chat <guid|chat_identifier> for one chat, or --chat all to export
every chat in one document. Date filters limit the exported window.`,
		Example: `  # Export one chat to stdout
  icloud-pp-cli messages export --chat +15551234567

  # Export all chats to a file
  icloud-pp-cli messages export --chat all --out /tmp/messages.json

  # Date-bounded export of one chat
  icloud-pp-cli messages export --chat <guid> --since 2026-01-01 --until 2026-05-01`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if chatID == "" {
				return usageErr(fmt.Errorf("--chat is required (use a GUID, chat_identifier, or 'all')"))
			}

			var since, until *time.Time
			if sinceStr != "" {
				t, err := parseDateFlag(sinceStr)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				since = &t
			}
			if untilStr != "" {
				t, err := parseDateFlag(untilStr)
				if err != nil {
					return usageErr(fmt.Errorf("--until: %w", err))
				}
				until = &t
			}

			db, err := openMessagesDB(f.messagesDBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			doc, err := buildExport(db, chatID, since, until, includeTapbacks)
			if err != nil {
				return err
			}

			out, closer, err := openExportSink(cmd, outPath)
			if err != nil {
				return err
			}
			defer closer()

			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			if err := enc.Encode(doc); err != nil {
				return fmt.Errorf("write export: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&chatID, "chat", "", "Chat GUID, chat_identifier, or 'all' for every chat (required)")
	cmd.Flags().StringVar(&outPath, "out", "", "Output file path (default: stdout)")
	cmd.Flags().StringVar(&sinceStr, "since", "", "Lower date bound (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&untilStr, "until", "", "Upper date bound (YYYY-MM-DD or RFC3339)")
	cmd.Flags().BoolVar(&includeTapbacks, "include-tapbacks", false, "Include tapback rows in the export")

	return cmd
}

type exportedAttachment struct {
	ROWID         int64  `json:"rowid"`
	Filename      string `json:"filename,omitempty"`
	ResolvedPath  string `json:"resolved_path,omitempty"`
	MIMEType      string `json:"mime_type,omitempty"`
	TransferState int    `json:"transfer_state"`
	Missing       bool   `json:"missing,omitempty"`
}

type exportedMessage struct {
	ROWID          int64                `json:"rowid"`
	GUID           string               `json:"guid"`
	Sender         string               `json:"sender,omitempty"`
	IsFromMe       bool                 `json:"is_from_me"`
	Date           string               `json:"date"`
	DateEdited     string               `json:"date_edited,omitempty"`
	Text           string               `json:"text"`
	TextSource     string               `json:"text_source"`
	HasAttachments bool                 `json:"has_attachments"`
	Attachments    []exportedAttachment `json:"attachments,omitempty"`
	AssociatedType *int                 `json:"associated_type,omitempty"`
}

type exportedChat struct {
	GUID           string             `json:"guid"`
	ChatIdentifier string             `json:"chat_identifier"`
	DisplayName    string             `json:"display_name,omitempty"`
	Participants   int                `json:"participants"`
	IsGroup        bool               `json:"is_group"`
	MessageCount   int                `json:"message_count"`
	Messages       []exportedMessage  `json:"messages"`
}

type exportDoc struct {
	GeneratedAt string         `json:"generated_at"`
	Chats       []exportedChat `json:"chats"`
}

func buildExport(db *sql.DB, chatID string, since, until *time.Time, includeTapbacks bool) (exportDoc, error) {
	doc := exportDoc{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	var targetChats []ChatRow
	if chatID == "all" {
		all, err := listChats(db, ChatListOpts{IncludeEmpty: false})
		if err != nil {
			return doc, fmt.Errorf("enumerate chats: %w", err)
		}
		targetChats = all
	} else {
		c, err := chatByIdentifier(db, chatID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return doc, usageErr(fmt.Errorf("no chat matched %q", chatID))
			}
			return doc, fmt.Errorf("resolve chat: %w", err)
		}
		targetChats = []ChatRow{c}
	}

	for _, c := range targetChats {
		msgs, err := messagesForChat(db, c.ROWID, MessageWindowOpts{
			Since:           since,
			Until:           until,
			IncludeTapbacks: includeTapbacks,
		})
		if err != nil {
			return doc, fmt.Errorf("messages for chat %s: %w", c.GUID, err)
		}

		ec := exportedChat{
			GUID:           c.GUID,
			ChatIdentifier: c.ChatIdentifier,
			DisplayName:    c.DisplayName,
			Participants:   c.ParticipantCount,
			IsGroup:        c.IsGroup,
			Messages:       make([]exportedMessage, 0, len(msgs)),
		}

		for _, m := range msgs {
			em := exportedMessage{
				ROWID:          m.ROWID,
				GUID:           m.GUID,
				Sender:         senderLabel(m),
				IsFromMe:       m.IsFromMe,
				Date:           m.Date.Format(time.RFC3339),
				Text:           m.Text,
				TextSource:     m.TextSource,
				HasAttachments: m.HasAttachments,
				AssociatedType: m.AssociatedType,
			}
			if m.DateEdited != nil {
				em.DateEdited = m.DateEdited.Format(time.RFC3339)
			}
			if m.HasAttachments {
				atts, err := attachmentsForMessage(db, m.ROWID)
				if err != nil {
					return doc, fmt.Errorf("attachments for message %d: %w", m.ROWID, err)
				}
				em.Attachments = make([]exportedAttachment, len(atts))
				for i, a := range atts {
					em.Attachments[i] = exportedAttachment{
						ROWID:         a.ROWID,
						Filename:      a.Filename,
						ResolvedPath:  a.ResolvedPath,
						MIMEType:      a.MIMEType,
						TransferState: a.TransferState,
						Missing:       a.Missing,
					}
				}
			}
			ec.Messages = append(ec.Messages, em)
		}
		ec.MessageCount = len(ec.Messages)
		doc.Chats = append(doc.Chats, ec)
	}

	return doc, nil
}

func openExportSink(cmd *cobra.Command, outPath string) (io.Writer, func(), error) {
	if outPath == "" {
		return cmd.OutOrStdout(), func() {}, nil
	}
	file, err := os.Create(outPath)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open output: %w", err)
	}
	return file, func() { _ = file.Close() }, nil
}

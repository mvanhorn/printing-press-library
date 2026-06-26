package cli

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"profilepress-pp-cli/internal/message"
)

const confirmSendText = "SEND-MESSAGE"

func newMessagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "messages", Short: "Draft and explicitly send LinkedIn messages"}
	cmd.AddCommand(newMessagesDraftCmd())
	cmd.AddCommand(newMessagesListCmd())
	cmd.AddCommand(newMessagesSendCmd())
	return cmd
}

func newMessagesDraftCmd() *cobra.Command {
	var dbPath, to, body, bodyFile, sourceNote string
	cmd := &cobra.Command{
		Use:   "draft",
		Short: "Create a local-only LinkedIn message draft",
		Example: "profilepress messages draft --to https://www.linkedin.com/in/example --body-file message.md\n" +
			"profilepress messages draft --to michael --body 'Great to reconnect.'",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(to) == "" {
				return errors.New("--to is required")
			}
			if bodyFile != "" {
				b, err := os.ReadFile(bodyFile)
				if err != nil {
					return err
				}
				body = string(b)
			}
			body = strings.TrimSpace(body)
			if body == "" {
				return errors.New("message body is required via --body or --body-file")
			}
			db, err := openStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			d, err := db.CreateMessageDraft(message.Draft{To: to, Body: body, SourceNote: sourceNote})
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"draft_id": d.ID, "to": d.To, "status": d.Status, "send_required": "profilepress messages send --draft " + d.ID + " --confirm-send " + confirmSendText})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().StringVar(&to, "to", "", "LinkedIn profile URL, member name, or thread identifier")
	cmd.Flags().StringVar(&body, "body", "", "message body")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "file containing message body")
	cmd.Flags().StringVar(&sourceNote, "source-note", "", "why this draft is appropriate")
	return cmd
}

func newMessagesListCmd() *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List local message drafts and send logs",
		Example: "profilepress messages list --limit 10",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			drafts, err := db.ListMessageDrafts(limit)
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"drafts": drafts})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum drafts to return")
	return cmd
}

func newMessagesSendCmd() *cobra.Command {
	var dbPath, draftID, confirm string
	var dryRun, simulateLive bool
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a message draft only with explicit confirmation",
		Example: "profilepress messages send --draft msg_123 --dry-run\n" +
			"profilepress messages send --draft msg_123 --confirm-send SEND-MESSAGE --simulate-live",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var d message.Draft
			if draftID == "" {
				d, err = db.LatestMessageDraft()
			} else {
				d, err = db.GetMessageDraft(draftID)
			}
			if err != nil {
				return err
			}
			if d.Status == "sent" {
				return errors.New("draft is already marked sent")
			}
			if dryRun {
				return printJSON(map[string]any{"draft_id": d.ID, "to": d.To, "status": "dry-run-passed", "would_send": true, "requires_confirmation": confirmSendText})
			}
			if confirm != confirmSendText {
				return errors.New("sending requires --confirm-send SEND-MESSAGE")
			}
			if !simulateLive {
				return errors.New("live LinkedIn send adapter is not enabled; rerun with --simulate-live for local workflow testing")
			}
			d, err = db.MarkMessageSent(d.ID, "simulate-live", confirm)
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"draft_id": d.ID, "to": d.To, "status": d.Status, "send_mode": d.SendMode})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	cmd.Flags().StringVar(&draftID, "draft", "", "draft ID; defaults to latest")
	cmd.Flags().StringVar(&confirm, "confirm-send", "", "must equal SEND-MESSAGE to send")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without marking sent")
	cmd.Flags().BoolVar(&simulateLive, "simulate-live", false, "test-only stand-in for a user-controlled browser send adapter")
	return cmd
}

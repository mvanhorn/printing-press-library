// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-outreach/internal/cliutil"
	"github.com/spf13/cobra"
)

func newMessageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message",
		Short: "Send messages, photos, or mark threads read",
		Long: `Message a host in an existing conversation. Writes are guarded: without
--confirm the command previews what it would send and does nothing. Real sends
require --confirm (or the global --yes).`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newMessageSendCmd(flags))
	cmd.AddCommand(newMessageSendImageCmd(flags))
	cmd.AddCommand(newMessageMarkReadCmd(flags))
	return cmd
}

// writeGuardActive reports whether a write should be held back (previewed only).
// Sends only happen with explicit --confirm (or --yes) and never under
// --dry-run or the verify harness.
func writeGuardActive(flags *rootFlags, confirm bool) bool {
	return flags.dryRun || cliutil.IsVerifyEnv() || !(confirm || flags.yes)
}

func newMessageSendCmd(flags *rootFlags) *cobra.Command {
	var text string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "send [thread-id]",
		Short: "Send a text message to an existing conversation",
		Example: "  airbnb-outreach-pp-cli message send 980001234567 --text \"Hi, is this available?\" --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			threadID := args[0]
			if text == "" {
				return usageErr(fmt.Errorf("--text is required"))
			}
			if writeGuardActive(flags, confirm) {
				return previewWrite(cmd, flags, "message.send", map[string]any{
					"thread_id": threadID,
					"text":      text,
				})
			}
			c := newAirbnbClient(flags)
			data, err := c.SendMessage(threadID, text, nil)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return reportWrite(cmd, flags, "sent", data)
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "Message text to send")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually send (without this, the message is only previewed)")
	return cmd
}

func newMessageSendImageCmd(flags *rootFlags) *cobra.Command {
	var files []string
	var text string
	var confirm bool
	cmd := &cobra.Command{
		Use:   "send-image [thread-id]",
		Short: "Send one or more photos (and optional text) to a conversation [experimental]",
		Long: `Upload and send photos to a host — the outreach use case for showing a
property owner what you have in mind. Uses Airbnb's signed-URL upload flow.

Experimental: the text-send path is validated, but the image upload chain
(GetSignedUrls -> CreateMediaItems) has not been confirmed against the live API;
if an upload step fails you'll get a clear error naming the failing step.`,
		Example: "  airbnb-outreach-pp-cli message send-image 980001234567 --file photo1.jpg --file photo2.jpg --text \"References\" --confirm",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			threadID := args[0]
			if len(files) == 0 {
				return usageErr(fmt.Errorf("at least one --file is required"))
			}
			if writeGuardActive(flags, confirm) {
				return previewWrite(cmd, flags, "message.send-image", map[string]any{
					"thread_id": threadID,
					"files":     files,
					"text":      text,
				})
			}
			c := newAirbnbClient(flags)
			var mediaIDs []string
			for _, f := range files {
				id, err := c.UploadImage(f)
				if err != nil {
					return apiErr(fmt.Errorf("uploading %s: %w", f, err))
				}
				mediaIDs = append(mediaIDs, id)
			}
			data, err := c.SendMessage(threadID, text, mediaIDs)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return reportWrite(cmd, flags, "sent", data)
		},
	}
	cmd.Flags().StringArrayVar(&files, "file", nil, "Image file to send (repeatable)")
	cmd.Flags().StringVar(&text, "text", "", "Optional caption text")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Actually send (without this, the action is only previewed)")
	return cmd
}

func newMessageMarkReadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mark-read [thread-id]",
		Short: "Mark a conversation as read [experimental]",
		Long: `Mark a thread's latest message as read. Experimental: Airbnb's web client
drives read-receipts over a realtime sync channel, so this REST mutation shape is
unconfirmed and may return an API error.`,
		Example: "  airbnb-outreach-pp-cli message mark-read 980001234567",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			if flags.dryRun || cliutil.IsVerifyEnv() {
				return previewWrite(cmd, flags, "message.mark-read", map[string]any{"thread_id": args[0]})
			}
			c := newAirbnbClient(flags)
			data, err := c.MarkRead(args[0])
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return reportWrite(cmd, flags, "marked_read", data)
		},
	}
	return cmd
}

// previewWrite shows what a guarded write would do without performing it.
func previewWrite(cmd *cobra.Command, flags *rootFlags, action string, payload map[string]any) error {
	preview := map[string]any{"action": action, "status": "preview", "would_send": payload, "hint": "add --confirm to actually send"}
	if flags.asJSON {
		return flags.printJSON(cmd, preview)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s would %s: %v\n", yellow("[preview]"), action, payload)
	fmt.Fprintln(cmd.OutOrStdout(), "  (add --confirm to actually send)")
	return nil
}

// reportWrite emits the result of a completed write.
func reportWrite(cmd *cobra.Command, flags *rootFlags, status string, data any) error {
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{"status": status, "result": data})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", green("✓"), status)
	return nil
}

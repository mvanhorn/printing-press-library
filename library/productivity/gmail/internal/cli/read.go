// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: read one message with full MIME decode and HTML-to-text —
// the conversion no other Gmail tool ships.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newReadCmd(flags))
	})
}

type readOutput struct {
	ID          string                     `json:"id"`
	ThreadID    string                     `json:"thread_id"`
	From        string                     `json:"from"`
	To          string                     `json:"to"`
	Cc          string                     `json:"cc,omitempty"`
	Date        string                     `json:"date"`
	Subject     string                     `json:"subject"`
	Labels      []string                   `json:"labels,omitempty"`
	Body        string                     `json:"body"`
	BodyIsHTML  bool                       `json:"body_converted_from_html"`
	Attachments []gmailmail.AttachmentInfo `json:"attachments,omitempty"`
}

func newReadCmd(flags *rootFlags) *cobra.Command {
	var htmlRaw bool

	cmd := &cobra.Command{
		Use:   "read <message-id>",
		Short: "Read a message with decoded body, converted from HTML when needed",
		Long: `Fetches the full message, walks the MIME tree, decodes base64url
content, and renders a plain-text body. HTML-only mail is converted to
readable text; pass --html to see the raw HTML instead.`,
		Example: strings.Trim(`
  gmail-pp-cli read 18c2f5a9b3d4e6f7
  gmail-pp-cli read 18c2f5a9b3d4e6f7 --json --select subject,body,attachments`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "message-id=example-message-id",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fetch and decode one message")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a message id argument is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			msg, err := gmailGetMessage(cmd.Context(), c, args[0], "full")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			text, html := gmailmail.ExtractBody(msg.Payload)
			body := strings.TrimSpace(text)
			fromHTML := false
			if body == "" && html != "" {
				fromHTML = true
				body = gmailmail.HTMLToText(html)
			}
			if htmlRaw && html != "" {
				body = html
			}
			out := readOutput{
				ID:          msg.ID,
				ThreadID:    msg.ThreadID,
				From:        msg.Header("From"),
				To:          msg.Header("To"),
				Cc:          msg.Header("Cc"),
				Date:        msg.Header("Date"),
				Subject:     msg.Header("Subject"),
				Labels:      msg.LabelIDs,
				Body:        body,
				BodyIsHTML:  fromHTML && !htmlRaw,
				Attachments: gmailmail.Attachments(msg.Payload),
			}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "From:    %s\n", out.From)
			fmt.Fprintf(w, "To:      %s\n", out.To)
			if out.Cc != "" {
				fmt.Fprintf(w, "Cc:      %s\n", out.Cc)
			}
			fmt.Fprintf(w, "Date:    %s\n", out.Date)
			fmt.Fprintf(w, "Subject: %s\n", out.Subject)
			if len(out.Attachments) > 0 {
				names := make([]string, 0, len(out.Attachments))
				for _, a := range out.Attachments {
					names = append(names, fmt.Sprintf("%s (%d bytes)", a.Filename, a.Size))
				}
				fmt.Fprintf(w, "Attachments: %s\n", strings.Join(names, ", "))
			}
			fmt.Fprintln(w)
			fmt.Fprintln(w, out.Body)
			return nil
		},
	}
	cmd.Flags().BoolVar(&htmlRaw, "html", false, "Print the raw HTML body instead of converted text")
	return cmd
}

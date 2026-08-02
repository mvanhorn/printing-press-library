// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: reply / reply-all with correct In-Reply-To, References, and
// threadId threading. Absorbs gws +reply / +reply-all.
package cli

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newReplyCmd(flags))
	})
}

// splitAddressList splits a To/Cc header into individual addresses. RFC 5322
// allows commas inside quoted display names (`"Doe, Jane" <jane@x.com>`), so
// parse properly first and fall back to a naive split only when the header is
// malformed enough that net/mail rejects it.
func splitAddressList(header string) []string {
	if strings.TrimSpace(header) == "" {
		return nil
	}
	if parsed, err := mail.ParseAddressList(header); err == nil {
		out := make([]string, 0, len(parsed))
		for _, a := range parsed {
			out = append(out, a.String())
		}
		return out
	}
	var out []string
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// replyRecipients computes reply / reply-all targets, dropping our own address.
func replyRecipients(msg *gmailmail.Message, self string, all bool) (to, cc []string) {
	replyTo := msg.Header("Reply-To")
	if replyTo == "" {
		replyTo = msg.Header("From")
	}
	to = splitAddressList(replyTo)
	if !all {
		return to, nil
	}
	selfLower := strings.ToLower(self)
	// Drop our own address, and de-duplicate: the From address usually
	// reappears in To, which would otherwise address the reply twice.
	seen := map[string]bool{}
	for _, a := range to {
		seen[gmailmail.ExtractEmail(a)] = true
	}
	appendNew := func(dst []string, addrs []string) []string {
		for _, a := range addrs {
			email := gmailmail.ExtractEmail(a)
			if email == "" || seen[email] || (selfLower != "" && email == selfLower) {
				continue
			}
			seen[email] = true
			dst = append(dst, a)
		}
		return dst
	}
	to = appendNew(to, splitAddressList(msg.Header("To")))
	cc = appendNew(nil, splitAddressList(msg.Header("Cc")))
	return to, cc
}

func newReplyCmd(flags *rootFlags) *cobra.Command {
	var f sendFlags
	var all bool
	var noQuote bool

	cmd := &cobra.Command{
		Use:   "reply <message-id>",
		Short: "Reply (or reply-all) with correct threading headers",
		Long: `Fetches the original message, derives recipients, subject, In-Reply-To,
References, and threadId, quotes the original body, and sends the reply so
Gmail threads it correctly.`,
		Example: strings.Trim(`
  gmail-pp-cli reply 18c2f5a9b3d4e6f7 --body "Sounds good."
  gmail-pp-cli reply 18c2f5a9b3d4e6f7 --all --body "Thanks everyone" --no-quote`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "message-id=example-message-id;--body=Example reply",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reply to a message with threading headers")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a message id argument is required"))
			}
			body, err := f.resolveBody()
			if err != nil {
				return usageErr(err)
			}
			if strings.TrimSpace(body) == "" && strings.TrimSpace(f.htmlBody) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--body, --body-file, or --html-body is required"))
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would reply to:", args[0])
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			orig, err := gmailGetMessage(cmd.Context(), c, args[0], "full")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			self, _, err := gmailProfile(cmd.Context(), c)
			if err != nil {
				self = "" // non-fatal: reply-all just may include our own address
			}
			to, cc := replyRecipients(orig, self, all)
			if len(f.to) > 0 {
				to = f.to // explicit override
			}
			if len(f.cc) > 0 {
				cc = f.cc
			}
			if !noQuote {
				body += gmailmail.QuoteOriginal(orig.Header("From"), orig.Header("Date"), orig.BodyText())
			}
			parentID := orig.Header("Message-ID")
			subject := f.subject
			if subject == "" {
				subject = gmailmail.ReplySubject(orig.Header("Subject"))
			}
			compose := gmailmail.Compose{
				To: to, Cc: cc, Bcc: f.bcc,
				Subject:     subject,
				Text:        body,
				HTML:        f.htmlBody,
				Attachments: f.attach,
				InReplyTo:   parentID,
				References:  gmailmail.ReferencesChain(orig.Header("References"), parentID),
			}
			raw, err := compose.BuildRaw()
			if err != nil {
				return usageErr(err)
			}
			id, err := gmailSendRaw(cmd.Context(), c, raw, orig.ThreadID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			res := sendResult{ID: id, ThreadID: orig.ThreadID, To: strings.Join(to, ", "), Subject: subject, Status: "sent"}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "replied in thread %s (message %s)\n", orig.ThreadID, id)
			return nil
		},
	}
	addComposeFlags(cmd, &f)
	cmd.Flags().BoolVar(&all, "all", false, "Reply to all recipients (reply-all)")
	cmd.Flags().BoolVar(&noQuote, "no-quote", false, "Do not quote the original message")
	return cmd
}

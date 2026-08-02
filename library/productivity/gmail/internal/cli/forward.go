// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: forward a message, carrying attachments over by re-downloading
// them through attachments.get. Absorbs gws +forward.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newForwardCmd(flags))
	})
}

// downloadAttachmentData fetches and decodes one attachment body.
func downloadAttachmentData(cmd *cobra.Command, c *client.Client, messageID, attachmentID string) ([]byte, error) {
	data, err := c.Get(cmd.Context(), "/gmail/v1/users/me/messages/"+messageID+"/attachments/"+attachmentID, nil)
	if err != nil {
		return nil, err
	}
	var att struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(data, &att); err != nil {
		return nil, fmt.Errorf("parsing attachment response: %w", err)
	}
	return gmailmail.DecodeB64URL(att.Data)
}

func newForwardCmd(flags *rootFlags) *cobra.Command {
	var f sendFlags
	var noAttachments bool

	cmd := &cobra.Command{
		Use:   "forward <message-id>",
		Short: "Forward a message, attachments included",
		Long: `Fetches the original message, prefixes the subject with Fwd:, includes
the original body below a forwarded-message header, and re-attaches the
original files (disable with --no-attachments).`,
		Example: strings.Trim(`
  gmail-pp-cli forward 18c2f5a9b3d4e6f7 --to alex@example.com
  gmail-pp-cli forward 18c2f5a9b3d4e6f7 --to alex@example.com --body "FYI" --no-attachments`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "message-id=example-message-id;--to=dest@example.com",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "forward a message")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a message id argument is required"))
			}
			if len(f.to) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to is required"))
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would forward:", args[0])
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
			note, err := f.resolveBody()
			if err != nil {
				return usageErr(err)
			}
			var body strings.Builder
			if strings.TrimSpace(note) != "" {
				body.WriteString(note + "\r\n\r\n")
			}
			body.WriteString("---------- Forwarded message ----------\r\n")
			body.WriteString("From: " + orig.Header("From") + "\r\n")
			body.WriteString("Date: " + orig.Header("Date") + "\r\n")
			body.WriteString("Subject: " + orig.Header("Subject") + "\r\n")
			body.WriteString("To: " + orig.Header("To") + "\r\n\r\n")
			body.WriteString(orig.BodyText())

			// Carry attachments over via a temp dir unless disabled.
			var attachPaths []string
			attachPaths = append(attachPaths, f.attach...)
			if !noAttachments {
				atts := gmailmail.Attachments(orig.Payload)
				if len(atts) > 0 {
					tmpDir, err := os.MkdirTemp("", "gmail-pp-cli-fwd-*")
					if err != nil {
						return fmt.Errorf("creating temp dir for attachments: %w", err)
					}
					defer os.RemoveAll(tmpDir)
					// Names repeat in real mail (image001.png, invoice.pdf);
					// without de-duplication the second write would clobber
					// the first and the forward would carry it twice.
					used := map[string]bool{}
					for _, a := range atts {
						if a.AttachmentID == "" {
							continue
						}
						data, err := downloadAttachmentData(cmd, c, orig.ID, a.AttachmentID)
						if err != nil {
							return classifyAPIError(fmt.Errorf("downloading attachment %s: %w", a.Filename, err), flags)
						}
						name := filepath.Base(filepath.Clean("/" + a.Filename))
						if name == "" || name == "." || name == ".." || name == string(os.PathSeparator) {
							name = "attachment"
						}
						unique := name
						for i := 1; used[unique]; i++ {
							ext := filepath.Ext(name)
							unique = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), i, ext)
						}
						used[unique] = true
						p := filepath.Join(tmpDir, unique)
						if err := os.WriteFile(p, data, 0o600); err != nil {
							return fmt.Errorf("writing attachment %s: %w", a.Filename, err)
						}
						attachPaths = append(attachPaths, p)
					}
				}
			}

			subject := f.subject
			if subject == "" {
				subject = gmailmail.ForwardSubject(orig.Header("Subject"))
			}
			compose := gmailmail.Compose{
				To: f.to, Cc: f.cc, Bcc: f.bcc,
				Subject:     subject,
				Text:        body.String(),
				HTML:        f.htmlBody,
				Attachments: attachPaths,
			}
			raw, err := compose.BuildRaw()
			if err != nil {
				return usageErr(err)
			}
			id, err := gmailSendRaw(cmd.Context(), c, raw, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			res := sendResult{ID: id, To: strings.Join(f.to, ", "), Subject: subject, Status: "sent"}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "forwarded as %s to %s\n", id, res.To)
			return nil
		},
	}
	addComposeFlags(cmd, &f)
	cmd.Flags().BoolVar(&noAttachments, "no-attachments", false, "Do not carry original attachments over")
	return cmd
}

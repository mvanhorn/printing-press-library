// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: send mail with attachments/HTML/cc/bcc. Absorbs gws +send and
// MCP send_email. The Anthropic hosted Gmail connector cannot send at all.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSendCmd(flags))
	})
}

// sendFlags carries every compose input shared by send/reply/forward/schedule.
type sendFlags struct {
	to, cc, bcc []string
	subject     string
	body        string
	bodyFile    string
	htmlBody    string
	attach      []string
	threadID    string
}

func addComposeFlags(cmd *cobra.Command, f *sendFlags) {
	cmd.Flags().StringSliceVar(&f.to, "to", nil, "Recipient address (repeatable or comma-separated)")
	cmd.Flags().StringSliceVar(&f.cc, "cc", nil, "Cc address (repeatable)")
	cmd.Flags().StringSliceVar(&f.bcc, "bcc", nil, "Bcc address (repeatable)")
	cmd.Flags().StringVar(&f.subject, "subject", "", "Subject line")
	cmd.Flags().StringVar(&f.body, "body", "", "Plain-text body")
	cmd.Flags().StringVar(&f.bodyFile, "body-file", "", "Read the plain-text body from a file (- for stdin)")
	cmd.Flags().StringVar(&f.htmlBody, "html-body", "", "HTML body (sent as multipart/alternative when --body is also set)")
	cmd.Flags().StringSliceVar(&f.attach, "attach", nil, "Attachment file path (repeatable)")
}

// resolveBody merges --body/--body-file, reading stdin for "-".
func (f *sendFlags) resolveBody() (string, error) {
	if f.bodyFile == "" {
		return f.body, nil
	}
	if f.body != "" {
		return "", fmt.Errorf("use either --body or --body-file, not both")
	}
	if f.bodyFile == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading body from stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(f.bodyFile)
	if err != nil {
		return "", fmt.Errorf("reading --body-file: %w", err)
	}
	return string(data), nil
}

type sendResult struct {
	ID       string `json:"id"`
	ThreadID string `json:"thread_id,omitempty"`
	To       string `json:"to"`
	Subject  string `json:"subject"`
	Status   string `json:"status"`
}

func newSendCmd(flags *rootFlags) *cobra.Command {
	var f sendFlags

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an email (plain text, HTML, attachments, cc/bcc)",
		Example: strings.Trim(`
  gmail-pp-cli send --to alex@example.com --subject "Status" --body "All green."
  gmail-pp-cli send --to alex@example.com --subject "Report" --body-file report.txt --attach chart.png
  gmail-pp-cli send --to team@example.com --subject "Update" --html-body "<p>Done.</p>" --dry-run`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "--to=dest@example.com;--subject=Example;--body=Example body",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "send an email via the Gmail API")
			}
			body, err := f.resolveBody()
			if err != nil {
				return usageErr(err)
			}
			if len(f.to) == 0 && len(f.cc) == 0 && len(f.bcc) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to (or --cc/--bcc) is required"))
			}
			if strings.TrimSpace(body) == "" && strings.TrimSpace(f.htmlBody) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--body, --body-file, or --html-body is required"))
			}
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would send:", strings.Join(f.to, ", "))
				return nil
			}
			compose := gmailmail.Compose{
				To: f.to, Cc: f.cc, Bcc: f.bcc,
				Subject: f.subject, Text: body, HTML: f.htmlBody,
				Attachments: f.attach,
			}
			raw, err := compose.BuildRaw()
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			id, err := gmailSendRaw(cmd.Context(), c, raw, f.threadID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			res := sendResult{ID: id, To: strings.Join(f.to, ", "), Subject: f.subject, Status: "sent"}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sent %s to %s\n", id, res.To)
			return nil
		},
	}
	addComposeFlags(cmd, &f)
	cmd.Flags().StringVar(&f.threadID, "thread-id", "", "Send inside an existing thread (see the thread_id field of find/read output)")
	return cmd
}

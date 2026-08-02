// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: download attachments from one message or harvest across a
// query. Absorbs ezgmail downloadAllAttachments with query-scoped harvesting.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

// senderSlugRe strips every character that could act as a path separator or
// traversal token. The From header is chosen by whoever sent the mail, so it
// must never reach the filesystem unfiltered.
var senderSlugRe = regexp.MustCompile(`[^a-zA-Z0-9._@+-]`)

func senderSlug(from string) string {
	s := senderSlugRe.ReplaceAllString(gmailmail.ExtractEmail(from), "_")
	// Collapse any remaining dot runs so no ".." survives, then trim the edges.
	for strings.Contains(s, "..") {
		s = strings.ReplaceAll(s, "..", "_")
	}
	s = strings.Trim(s, ".")
	if s == "" {
		return "unknown-sender"
	}
	return s
}

// assertWithin refuses any path that escapes root, so a hostile sender or
// attachment name cannot place files outside --out.
func assertWithin(root, path string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if absPath != absRoot && !strings.HasPrefix(absPath, absRoot+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to write outside the output directory: %q", path)
	}
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		attachmentsCmd := &cobra.Command{
			Use:   "attachments",
			Short: "Download and harvest message attachments",
		}
		attachmentsCmd.AddCommand(newAttachmentsSaveCmd(flags))
		addNovelCommandIfAbsent(root, attachmentsCmd)
	})
}

type savedAttachment struct {
	MessageID string `json:"message_id"`
	From      string `json:"from"`
	Filename  string `json:"filename"`
	Path      string `json:"path"`
	Size      int    `json:"size"`
}

// saveMessageAttachments downloads every attachment of one message into dir.
// root bounds every write; dir must already be inside it.
func saveMessageAttachments(cmd *cobra.Command, c *client.Client, msg *gmailmail.Message, dir, root string) ([]savedAttachment, error) {
	var saved []savedAttachment
	atts := gmailmail.Attachments(msg.Payload)
	for _, a := range atts {
		if a.AttachmentID == "" {
			continue
		}
		data, err := downloadAttachmentData(cmd, c, msg.ID, a.AttachmentID)
		if err != nil {
			return saved, fmt.Errorf("downloading %s from %s: %w", a.Filename, msg.ID, err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return saved, fmt.Errorf("creating output dir: %w", err)
		}
		// The attachment name is sender-chosen: keep only the base name and
		// reject traversal tokens outright.
		name := filepath.Base(filepath.Clean("/" + a.Filename))
		if name == "" || name == "." || name == ".." || name == string(os.PathSeparator) {
			suffix := a.AttachmentID
			if len(suffix) > 12 {
				suffix = suffix[:12]
			}
			name = "attachment-" + suffix
		}
		path := filepath.Join(dir, name)
		// Never clobber: add -1, -2, ... suffixes on collision.
		for i := 1; ; i++ {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				break
			}
			ext := filepath.Ext(name)
			path = filepath.Join(dir, fmt.Sprintf("%s-%d%s", strings.TrimSuffix(name, ext), i, ext))
		}
		if err := assertWithin(root, path); err != nil {
			return saved, err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return saved, fmt.Errorf("writing %s: %w", path, err)
		}
		saved = append(saved, savedAttachment{
			MessageID: msg.ID,
			From:      msg.Header("From"),
			Filename:  name,
			Path:      path,
			Size:      len(data),
		})
	}
	return saved, nil
}

func newAttachmentsSaveCmd(flags *rootFlags) *cobra.Command {
	var query string
	var outDir string
	var limit int
	var bySender bool

	cmd := &cobra.Command{
		Use:   "save [message-id]",
		Short: "Save attachments from a message, or harvest across a query",
		Long: `With a message id, downloads that message's attachments. With --query,
harvests attachments across every matching message (e.g. all PDFs from a
sender), organizing them under --out.`,
		Example: strings.Trim(`
  gmail-pp-cli attachments save 18c2f5a9b3d4e6f7 --out ./downloads
  gmail-pp-cli attachments save --query "from:billing@stripe.com filename:pdf" --out ./invoices --by-sender`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "message-id=example-message-id",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "download attachments to local files")
			}
			if len(args) == 0 && strings.TrimSpace(query) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a message id argument or --query is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var ids []string
			if len(args) > 0 {
				ids = []string{args[0]}
			} else {
				harvestQuery := query
				if !strings.Contains(harvestQuery, "has:attachment") && !strings.Contains(harvestQuery, "filename:") {
					harvestQuery += " has:attachment"
				}
				ids, err = gmailListIDs(cmd.Context(), c, harvestQuery, limit)
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			var all []savedAttachment
			for _, id := range ids {
				msg, err := gmailGetMessage(cmd.Context(), c, id, "full")
				if err != nil {
					return classifyAPIError(err, flags)
				}
				dir := outDir
				if bySender {
					dir = filepath.Join(outDir, senderSlug(msg.Header("From")))
					if err := assertWithin(outDir, dir); err != nil {
						return err
					}
				}
				saved, err := saveMessageAttachments(cmd, c, msg, dir, outDir)
				all = append(all, saved...)
				if err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning:", err)
				}
			}
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), all, flags)
			}
			if len(all) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no attachments found")
				return nil
			}
			for _, s := range all {
				fmt.Fprintf(cmd.OutOrStdout(), "saved %s (%d bytes) from %s\n", s.Path, s.Size, s.From)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Harvest attachments across all messages matching this Gmail query")
	cmd.Flags().StringVar(&outDir, "out", ".", "Output directory")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum messages to harvest from (with --query)")
	cmd.Flags().BoolVar(&bySender, "by-sender", false, "Organize files into per-sender subdirectories")
	return cmd
}

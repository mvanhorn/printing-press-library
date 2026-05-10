package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"

	"github.com/spf13/cobra"
)

type attachmentListRow struct {
	AttachmentID string `json:"attachment_id"`
	MessageID    string `json:"message_id"`
	Filename     string `json:"filename"`
	MimeType     string `json:"mime_type"`
	Size         int    `json:"size"`
	SenderEmail  string `json:"sender_email,omitempty"`
	Date         string `json:"date,omitempty"`
	Subject      string `json:"subject,omitempty"`
}

type attachmentExportResult struct {
	AttachmentID string `json:"attachment_id"`
	MessageID    string `json:"message_id"`
	Filename     string `json:"filename"`
	SavedPath    string `json:"saved_path"`
	Size         int    `json:"size"`
	Status       string `json:"status"` // "saved" | "skipped" | "error"
	Error        string `json:"error,omitempty"`
}

// gmailAttachmentPart is the MessagePart shape stored in attachments.data
type gmailAttachmentPart struct {
	PartID   string `json:"partId"`
	MimeType string `json:"mimeType"`
	Filename string `json:"filename"`
	Body     struct {
		AttachmentID string `json:"attachmentId"`
		Size         int    `json:"size"`
	} `json:"body"`
}

// gmailMessagePartBody is the response from the attachment download API
type gmailMessagePartBody struct {
	AttachmentID string `json:"attachmentId"`
	Size         int    `json:"size"`
	Data         string `json:"data"` // base64url-encoded bytes
}

func newAttachmentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attachments",
		Short: "Inventory and export Gmail attachments from local store",
	}
	cmd.AddCommand(newAttachmentsListCmd(flags))
	cmd.AddCommand(newAttachmentsExportCmd(flags))
	return cmd
}

func newAttachmentsListCmd(flags *rootFlags) *cobra.Command {
	var mimeType string
	var query string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all attachments in your synced mailbox filtered by MIME type or Gmail query — zero API calls after sync",
		Long: `Queries the local SQLite attachment index for attachments matching
optional MIME type and/or sender filters. No live API call needed after sync.

Use --type to filter by MIME type (e.g. application/pdf).
Use --query to filter by sender email substring.`,
		Example: `  gmail-pp-cli attachments list
  gmail-pp-cli attachments list --type application/pdf
  gmail-pp-cli attachments list --query from:accounting@ --agent
  gmail-pp-cli attachments list --agent --select filename,sender_email,size`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gmail-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\n\nRun 'gmail-pp-cli sync --full' first", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT a.id, a.messages_id, COALESCE(a.data,''), COALESCE(m.data,'')
				 FROM attachments a
				 LEFT JOIN messages m ON m.id = a.messages_id`)
			if err != nil {
				return fmt.Errorf("querying attachments: %w", err)
			}
			defer rows.Close()

			var result []attachmentListRow
			for rows.Next() {
				var attID, msgID, attDataJSON, msgDataJSON string
				if err := rows.Scan(&attID, &msgID, &attDataJSON, &msgDataJSON); err != nil {
					continue
				}
				if attDataJSON == "" {
					continue
				}
				var part gmailAttachmentPart
				if err := json.Unmarshal([]byte(attDataJSON), &part); err != nil {
					continue
				}
				if part.Filename == "" {
					continue
				}
				if mimeType != "" && !strings.EqualFold(part.MimeType, mimeType) {
					continue
				}

				row := attachmentListRow{
					AttachmentID: attID,
					MessageID:    msgID,
					Filename:     part.Filename,
					MimeType:     part.MimeType,
					Size:         part.Body.Size,
				}
				if msgDataJSON != "" {
					msg, parseErr := parseGmailMsg(msgDataJSON)
					if parseErr == nil {
						from := msg.header("From")
						_, _, domain := parseFrom(from)
						row.SenderEmail = from
						row.Date = msg.internalTime().Format("2006-01-02")
						row.Subject = msg.header("Subject")
						if query != "" {
							// simple substring filter on sender
							if !strings.Contains(strings.ToLower(from), strings.ToLower(query)) &&
								!strings.Contains(strings.ToLower(domain), strings.ToLower(query)) {
								continue
							}
						}
					}
				}
				result = append(result, row)
				if limit > 0 && len(result) >= limit {
					break
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading attachments: %w", err)
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(result) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No attachments found. Run 'gmail-pp-cli sync --full' to populate the local store.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "FILENAME\tTYPE\tSIZE\tSENDER\tDATE")
			for _, r := range result {
				sender := r.SenderEmail
				if len(sender) > 35 {
					sender = sender[:32] + "..."
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n", r.Filename, r.MimeType, r.Size, sender, r.Date)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&mimeType, "type", "", "Filter by MIME type (e.g. application/pdf, image/jpeg)")
	cmd.Flags().StringVar(&query, "query", "", "Filter by sender email substring")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum number of attachments to list")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}

func newAttachmentsExportCmd(flags *rootFlags) *cobra.Command {
	var mimeType string
	var query string
	var dir string
	var limit int
	var dbPath string
	var skipExisting bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Download every attachment matching a Gmail search query to a local directory — invoice extraction in one command",
		Long: `Finds attachments matching optional filters in the local store, then
downloads each one from the Gmail API and saves it to --dir with a
sane filename: <date>-<sender-domain>-<original-filename>.

Requires an active Gmail OAuth2 session. Run 'gmail-pp-cli auth login'
to authenticate.

Use --type to filter by MIME type (e.g. application/pdf).
Use --query to filter by sender email substring.
Use --skip-existing to avoid re-downloading files already present.`,
		Example: `  gmail-pp-cli attachments export --dir ~/invoices
  gmail-pp-cli attachments export --query "from:vendor@acme.com" --dir ~/invoices --agent
  gmail-pp-cli attachments export --type application/pdf --dir ~/invoices --skip-existing`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dir == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would export attachments to:", dir)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gmail-pp-cli")
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating output directory %s: %w", dir, err)
			}

			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\n\nRun 'gmail-pp-cli sync --full' first", err)
			}
			defer db.Close()

			// Build attachment list from local store
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT a.id, a.messages_id, COALESCE(a.data,''), COALESCE(m.data,'')
				 FROM attachments a
				 LEFT JOIN messages m ON m.id = a.messages_id`)
			if err != nil {
				return fmt.Errorf("querying attachments: %w", err)
			}

			type toDownload struct {
				attID    string
				msgID    string
				filename string
				mimeT    string
				size     int
				date     string
				domain   string
			}
			var candidates []toDownload
			for rows.Next() {
				var attID, msgID, attDataJSON, msgDataJSON string
				if err := rows.Scan(&attID, &msgID, &attDataJSON, &msgDataJSON); err != nil {
					continue
				}
				if attDataJSON == "" {
					continue
				}
				var part gmailAttachmentPart
				if err := json.Unmarshal([]byte(attDataJSON), &part); err != nil {
					continue
				}
				if part.Filename == "" {
					continue
				}
				if mimeType != "" && !strings.EqualFold(part.MimeType, mimeType) {
					continue
				}
				date := time.Now().Format("2006-01-02")
				domain := ""
				if msgDataJSON != "" {
					msg, parseErr := parseGmailMsg(msgDataJSON)
					if parseErr == nil {
						from := msg.header("From")
						if query != "" && !strings.Contains(strings.ToLower(from), strings.ToLower(query)) {
							continue
						}
						_, _, domain = parseFrom(from)
						date = msg.internalTime().Format("2006-01-02")
					}
				}
				candidates = append(candidates, toDownload{
					attID:    attID,
					msgID:    msgID,
					filename: part.Filename,
					mimeT:    part.MimeType,
					size:     part.Body.Size,
					date:     date,
					domain:   domain,
				})
				if limit > 0 && len(candidates) >= limit {
					break
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading attachments: %w", err)
			}

			if len(candidates) == 0 {
				msg := "No matching attachments found in local store."
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]string{"status": "no_attachments", "message": msg}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), msg)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			var results []attachmentExportResult
			for _, att := range candidates {
				// Build safe filename: date-domain-original
				safeDomain := strings.ReplaceAll(att.domain, ".", "-")
				if safeDomain == "" {
					safeDomain = "unknown"
				}
				saveName := att.date + "-" + safeDomain + "-" + sanitizeFilename(att.filename)
				savePath := filepath.Join(dir, saveName)

				res := attachmentExportResult{
					AttachmentID: att.attID,
					MessageID:    att.msgID,
					Filename:     att.filename,
					SavedPath:    savePath,
					Size:         att.size,
				}

				if skipExisting {
					if _, err := os.Stat(savePath); err == nil {
						res.Status = "skipped"
						results = append(results, res)
						continue
					}
				}

				apiPath := "/gmail/v1/users/me/messages/" + att.msgID + "/attachments/" + att.attID
				raw, apiErr := c.Get(apiPath, nil)
				if apiErr != nil {
					res.Status = "error"
					res.Error = apiErr.Error()
					results = append(results, res)
					continue
				}

				var body gmailMessagePartBody
				if jsonErr := json.Unmarshal(raw, &body); jsonErr != nil {
					res.Status = "error"
					res.Error = "decode response: " + jsonErr.Error()
					results = append(results, res)
					continue
				}

				decoded, decErr := base64.RawURLEncoding.DecodeString(body.Data)
				if decErr != nil {
					// try standard base64 with padding
					decoded, decErr = base64.URLEncoding.DecodeString(body.Data)
					if decErr != nil {
						res.Status = "error"
						res.Error = "base64 decode: " + decErr.Error()
						results = append(results, res)
						continue
					}
				}

				if writeErr := os.WriteFile(savePath, decoded, 0o644); writeErr != nil {
					res.Status = "error"
					res.Error = "write file: " + writeErr.Error()
					results = append(results, res)
					continue
				}
				res.Status = "saved"
				res.Size = len(decoded)
				results = append(results, res)

				if !flags.asJSON && !flags.agent {
					fmt.Fprintf(cmd.OutOrStdout(), "saved: %s\n", savePath)
				}
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			saved := 0
			for _, r := range results {
				if r.Status == "saved" {
					saved++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nDone: %d/%d attachments saved to %s\n", saved, len(results), dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Directory to save attachments to (required)")
	cmd.Flags().StringVar(&mimeType, "type", "", "Filter by MIME type (e.g. application/pdf)")
	cmd.Flags().StringVar(&query, "query", "", "Filter by sender email substring")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum number of attachments to export")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip files that already exist in the output directory")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}

// sanitizeFilename replaces characters that are unsafe in filenames.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
		" ", "_",
	)
	return replacer.Replace(name)
}

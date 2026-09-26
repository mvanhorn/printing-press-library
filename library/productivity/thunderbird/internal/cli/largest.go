// Copyright 2026 Edoardo Manco and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type tbLargestMessageRow struct {
	ID              string `json:"id"`
	Date            string `json:"date"`
	FromAddr        string `json:"from_addr"`
	FromName        string `json:"from_name"`
	Subject         string `json:"subject"`
	Account         string `json:"account"`
	Folder          string `json:"folder"`
	SizeBytes       int64  `json:"size_bytes"`
	AttachmentCount int    `json:"attachment_count"`
}

type tbLargestAttachmentRow struct {
	ID          string `json:"id"`
	MessageID   string `json:"message_id"`
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Account     string `json:"account"`
	Folder      string `json:"folder"`
	Date        string `json:"date"`
}

func tbScopeWhere(folder, account string) (string, []any) {
	var conds []string
	var args []any
	if f := strings.TrimSpace(folder); f != "" {
		conds = append(conds, "(lower(json_extract(data,'$.folder')) = lower(?) OR lower(json_extract(data,'$.folder_path')) = lower(?))")
		args = append(args, f, f)
	}
	if a := strings.TrimSpace(account); a != "" {
		conds = append(conds, "(lower(json_extract(data,'$.account')) = lower(?) OR lower(COALESCE(json_extract(data,'$.account_name'),'')) = lower(?))")
		args = append(args, a, a)
	}
	return strings.Join(conds, " AND "), args
}

func newNovelLargestCmd(flags *rootFlags) *cobra.Command {
	var attachments, includeInline bool
	var folder, account string
	var limit int

	cmd := &cobra.Command{
		Use:   "largest",
		Short: "Find the biggest messages or attachments across all accounts and folders.",
		Long: `Use this command to find the individual messages or attachments using the most space. Do NOT use it for per-folder totals; use 'folders' instead.

Messages are ranked by their size in the mbox (the raw RFC822 bytes);
with --attachments, attachments are ranked by their decoded size, skipping
inline parts (signature logos, cid: images) unless --include-inline.`,
		Example: strings.Trim(`
  thunderbird-pp-cli largest --attachments --limit 20 --agent
  thunderbird-pp-cli largest --folder INBOX --account account1 --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "largest")
			}
			resource := "messages"
			if attachments {
				resource = "attachments"
			}
			db, err := tbStoreFor(cmd, flags, resource)
			if err != nil || db == nil {
				return err
			}
			defer db.Close()
			where, qargs := tbScopeWhere(folder, account)
			order := "CAST(json_extract(data,'$.size_bytes') AS INTEGER) DESC, id"
			human := wantsHumanTable(cmd.OutOrStdout(), flags)
			if attachments {
				q := `SELECT data FROM resources WHERE resource_type = 'attachments'`
				if !includeInline {
					q += " AND " + tbSQLNotInline
				}
				if where != "" {
					q += " AND " + where
				}
				q += " ORDER BY " + order
				if limit > 0 {
					q += fmt.Sprintf(" LIMIT %d", limit)
				}
				docs, err := tbScanDocs[tbAttachmentDoc](db, q, qargs...)
				if err != nil {
					return err
				}
				rows := make([]tbLargestAttachmentRow, 0, len(docs))
				for _, d := range docs {
					rows = append(rows, tbLargestAttachmentRow{ID: d.ID, MessageID: d.MessageID, Index: d.Index, Filename: d.Filename,
						ContentType: d.ContentType, SizeBytes: d.SizeBytes, Account: d.Account, Folder: d.FolderPath, Date: d.Date})
				}
				if !human {
					return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
				}
				tw := newTabWriter(tbHumanOut(cmd))
				fmt.Fprintln(tw, "SIZE\tFILENAME\tTYPE\tMESSAGE\tFOLDER\tDATE")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", tbHumanBytes(r.SizeBytes), tbTrunc(r.Filename, 40), r.ContentType, r.MessageID, r.Folder, tbShortDate(r.Date))
				}
				return tw.Flush()
			}
			docs, err := tbQueryMessages(db, where, qargs, order, limit)
			if err != nil {
				return err
			}
			rows := make([]tbLargestMessageRow, 0, len(docs))
			for _, d := range docs {
				rows = append(rows, tbLargestMessageRow{ID: d.ID, Date: d.Date, FromAddr: d.FromAddr, FromName: d.FromName, Subject: d.Subject,
					Account: d.Account, Folder: d.FolderPath, SizeBytes: d.SizeBytes, AttachmentCount: d.AttachmentCount})
			}
			if !human {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "SIZE\tID\tDATE\tFROM\tSUBJECT\tFOLDER\tATT")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n", tbHumanBytes(r.SizeBytes), r.ID, tbShortDate(r.Date), tbTrunc(tbSender(r.FromName, r.FromAddr), 28), tbTrunc(r.Subject, 50), r.Folder, r.AttachmentCount)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&attachments, "attachments", false, "Rank attachments by decoded size instead of whole messages")
	cmd.Flags().BoolVar(&includeInline, "include-inline", false, "With --attachments, also rank inline parts (signature logos, cid: images)")
	cmd.Flags().StringVar(&folder, "folder", "", "Only this folder (leaf name like INBOX or path like Archives/2025, case-insensitive)")
	cmd.Flags().StringVar(&account, "account", "", "Only this account (key like account1 or account name)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum rows to show (0 = all)")
	return cmd
}

// Copyright 2026 Edoardo Manco and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

type tbAwaitingRow struct {
	ThreadID         string `json:"thread_id"`
	LastMessageID    string `json:"last_message_id"`
	Subject          string `json:"subject"`
	Counterpart      string `json:"counterpart"`
	CounterpartName  string `json:"counterpart_name"`
	LastDate         string `json:"last_date"`
	AgeDays          int    `json:"age_days"`
	MessagesInThread int    `json:"messages_in_thread"`
	Folder           string `json:"folder"`
	Account          string `json:"account"`
}

type tbThreadMsg struct {
	doc     tbMessageDoc
	dir     int
	replied bool
}

// tbAwaiting computes pending threads from window messages.
func tbAwaiting(docs []tbMessageDoc, mb *tbMailbox, theirs, includeBulk bool, account string, now time.Time) []tbAwaitingRow {
	bulk := tbNewBulk(docs, mb)
	threads := map[string]map[string]*tbThreadMsg{}
	for _, d := range docs {
		if tbprofile.IsTrashOrJunkFolderName(d.FolderPath) {
			continue
		}
		dir := mb.direction(d)
		if dir == tbDirOther {
			continue
		}
		msgs := threads[d.ThreadID]
		if msgs == nil {
			msgs = map[string]*tbThreadMsg{}
			threads[d.ThreadID] = msgs
		}
		key := tbDedupKey(d)
		if m := msgs[key]; m != nil {
			m.replied = m.replied || d.Replied
			if dir == tbDirSent {
				m.dir, m.doc = dir, d
			}
			continue
		}
		msgs[key] = &tbThreadMsg{doc: d, dir: dir, replied: d.Replied}
	}
	rows := make([]tbAwaitingRow, 0)
	for tid, msgs := range threads {
		var last *tbThreadMsg
		for _, m := range msgs {
			if last == nil || m.doc.Date > last.doc.Date || (m.doc.Date == last.doc.Date && m.dir == tbDirSent) {
				last = m
			}
		}
		d := last.doc
		if !tbAccountMatches(account, d.Account, d.AccountName) {
			continue
		}
		var addr, name string
		if theirs {
			if last.dir != tbDirSent {
				continue
			}
			for _, a := range tbRecipients(d) {
				if !mb.isOwn(a) {
					addr = a
					break
				}
			}
			if addr == "" {
				continue
			}
		} else {
			if last.dir != tbDirInbound || last.replied || (!includeBulk && bulk.kind(d) != "") {
				continue
			}
			addr, name = d.FromAddr, d.FromName
		}
		rows = append(rows, tbAwaitingRow{
			ThreadID: tid, LastMessageID: d.ID, Subject: d.Subject, Counterpart: addr, CounterpartName: name,
			LastDate: d.Date, AgeDays: int(now.Sub(tbMsgTime(d)).Hours() / 24), Folder: d.FolderPath, Account: d.Account,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].LastDate != rows[j].LastDate {
			return rows[i].LastDate < rows[j].LastDate
		}
		return rows[i].ThreadID < rows[j].ThreadID
	})
	return rows
}

func tbThreadSizes(db *store.Store, ids []string) (map[string]int, error) {
	out := map[string]int{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.DB().Query(`SELECT json_extract(data,'$.thread_id'), COUNT(DISTINCT COALESCE(NULLIF(json_extract(data,'$.message_id'),''), id))
		FROM resources WHERE resource_type = 'messages' AND json_extract(data,'$.thread_id') IN `+tbInClause(len(ids))+` GROUP BY 1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tid string
		var n int
		if err := rows.Scan(&tid, &n); err != nil {
			return nil, err
		}
		out[tid] = n
	}
	return out, rows.Err()
}

func newNovelAwaitingReplyCmd(flags *rootFlags) *cobra.Command {
	var days, limit int
	var since, account string
	var theirs, includeBulk bool

	cmd := &cobra.Command{
		Use:   "awaiting-reply",
		Short: "See every thread where someone is waiting on your answer, across all accounts, oldest first.",
		Long: `Use this command to find threads where a reply is pending, from you or (with --theirs) from the other side. Do NOT use it to list recent or unread mail; use 'messages list' instead.

A thread (References/In-Reply-To) is pending on you when its latest message
is inbound, dated after --since, not marked replied, and no message
sent by you follows it. "Sent by you" means stored in a sent folder (the
identities' fcc_folder from prefs.js, or a folder named Sent, Posta inviata,
Gesendet...); own-address mail in the Inbox counts as neither side. Bulk mail
is skipped unless --include-bulk: mailing lists (List-Id), automated mail
(Auto-Submitted, Precedence bulk/list/junk, X-Auto-Response-Suppress,
List-Unsubscribe), no-reply/notification senders, and one-way senders with
20+ messages in the window that you never wrote to. Trash, Junk and Spam
folders are always ignored. With
--theirs the latest message is one you sent and nobody has answered yet.`,
		Example: strings.Trim(`
  thunderbird-pp-cli awaiting-reply --since 14d --agent
  thunderbird-pp-cli awaiting-reply --theirs --since 30d --account account1`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "awaiting-reply")
			}
			now := tbNow()
			cutoff, err := tbWindowStart(cmd, since, days, now)
			if err != nil {
				return err
			}
			db, err := tbStoreFor(cmd, flags, "messages")
			if err != nil || db == nil {
				return err
			}
			defer db.Close()
			mb, err := tbLoadMailbox(db)
			if err != nil {
				return err
			}
			docs, err := tbWindowMessages(db, cutoff)
			if err != nil {
				return err
			}
			rows := tbAwaiting(docs, mb, theirs, includeBulk, account, now)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			ids := make([]string, 0, len(rows))
			for _, r := range rows {
				ids = append(ids, r.ThreadID)
			}
			sizes, err := tbThreadSizes(db, ids)
			if err != nil {
				return err
			}
			for i := range rows {
				rows[i].MessagesInThread = sizes[rows[i].ThreadID]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "AGE\tLAST\tCOUNTERPART\tSUBJECT\tMSGS\tFOLDER\tTHREAD")
			for _, r := range rows {
				fmt.Fprintf(tw, "%dd\t%s\t%s\t%s\t%d\t%s\t%s\n", r.AgeDays, tbShortDate(r.LastDate), tbTrunc(tbSender(r.CounterpartName, r.Counterpart), 28), tbTrunc(r.Subject, 60), r.MessagesInThread, r.Folder, r.ThreadID)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "14d", "Only threads whose latest message is newer than this (duration like 14d/2w or a date like 2025-01-31)")
	cmd.Flags().IntVar(&days, "days", 0, "Alias of --since in days")
	_ = cmd.Flags().MarkHidden("days")
	cmd.Flags().StringVar(&account, "account", "", "Only threads whose latest message is in this account (key like account1 or account name)")
	cmd.Flags().BoolVar(&theirs, "theirs", false, "Threads where you sent the latest message and are waiting on the other side")
	cmd.Flags().BoolVar(&includeBulk, "include-bulk", false, "Keep mailing-list, automated, no-reply and one-way high-volume senders")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum threads to show (0 = all)")
	return cmd
}

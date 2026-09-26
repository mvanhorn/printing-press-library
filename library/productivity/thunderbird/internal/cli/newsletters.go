// Copyright 2026 Edoardo Manco and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

type tbNewsletterRow struct {
	Key             string  `json:"key"`
	Kind            string  `json:"kind"`
	Name            string  `json:"name"`
	FromAddr        string  `json:"from_addr"`
	Messages        int     `json:"messages"`
	Unread          int     `json:"unread"`
	UnreadShare     float64 `json:"unread_share"`
	LastReceived    string  `json:"last_received"`
	LastRead        string  `json:"last_read"`
	ListUnsubscribe string  `json:"list_unsubscribe"`
}

var tbUnsubURLRE = regexp.MustCompile(`<\s*((?i:mailto:|https?://)[^>\s]+)\s*>`)

// tbUnsubscribeURL returns the first mailto: or http(s) URL of a List-Unsubscribe header.
func tbUnsubscribeURL(h string) string {
	if m := tbUnsubURLRE.FindStringSubmatch(h); m != nil {
		return m[1]
	}
	return ""
}

// tbListKey returns the list identifier inside <> of a List-Id header.
func tbListKey(listID string) string {
	if i := strings.LastIndex(listID, "<"); i >= 0 {
		if j := strings.Index(listID[i:], ">"); j > 1 {
			listID = listID[i+1 : i+j]
		}
	}
	return strings.ToLower(strings.TrimSpace(listID))
}

func tbMostCommon(counts map[string]int) string {
	best, n := "", 0
	for k, c := range counts {
		if c > n || (c == n && k < best) {
			best, n = k, c
		}
	}
	return best
}

// tbNewsletters groups inbound bulk mail by List-Id, and automated or
// one-way high-volume mail by sender address.
func tbNewsletters(docs []tbMessageDoc, mb *tbMailbox, minMessages int) []tbNewsletterRow {
	bulk := tbNewBulk(docs, mb)
	type group struct {
		row     tbNewsletterRow
		names   map[string]int
		addrs   map[string]int
		unsubAt string
	}
	groups := map[string]*group{}
	seen := map[string]bool{}
	for _, d := range docs {
		if tbprofile.IsTrashOrJunkFolderName(d.FolderPath) || mb.direction(d) != tbDirInbound {
			continue
		}
		kind := bulk.kind(d)
		var key string
		switch {
		case kind == tbBulkList:
			key = "list:" + tbListKey(d.ListID)
		case kind != "" && d.FromAddr != "":
			key = "sender:" + strings.ToLower(d.FromAddr)
		default:
			continue
		}
		if seen[key+"|"+tbDedupKey(d)] {
			continue
		}
		seen[key+"|"+tbDedupKey(d)] = true
		g := groups[key]
		if g == nil {
			g = &group{row: tbNewsletterRow{Key: strings.SplitN(key, ":", 2)[1], Kind: kind}, names: map[string]int{}, addrs: map[string]int{}}
			groups[key] = g
		}
		if kind == tbBulkAutomated {
			g.row.Kind = kind
		}
		g.row.Messages++
		if !d.Read {
			g.row.Unread++
		} else if d.Date > g.row.LastRead {
			g.row.LastRead = d.Date
		}
		if d.Date > g.row.LastReceived {
			g.row.LastReceived = d.Date
		}
		if d.FromName != "" {
			g.names[d.FromName]++
		}
		if d.FromAddr != "" {
			g.addrs[strings.ToLower(d.FromAddr)]++
		}
		if u := tbUnsubscribeURL(d.ListUnsubscribe); u != "" && d.Date >= g.unsubAt {
			g.row.ListUnsubscribe, g.unsubAt = u, d.Date
		}
	}
	rows := make([]tbNewsletterRow, 0, len(groups))
	for _, g := range groups {
		if g.row.Messages < minMessages {
			continue
		}
		r := g.row
		r.Name, r.FromAddr = tbMostCommon(g.names), tbMostCommon(g.addrs)
		r.UnreadShare = math.Round(float64(r.Unread)/float64(r.Messages)*1000) / 1000
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Messages != rows[j].Messages {
			return rows[i].Messages > rows[j].Messages
		}
		if rows[i].UnreadShare != rows[j].UnreadShare {
			return rows[i].UnreadShare > rows[j].UnreadShare
		}
		return rows[i].Key < rows[j].Key
	})
	return rows
}

func newNovelNewslettersCmd(flags *rootFlags) *cobra.Command {
	var since string
	var minMessages, limit int

	cmd := &cobra.Command{
		Use:   "newsletters",
		Short: "List mailing lists and bulk senders by volume and how much of their mail you never read.",
		Long: `Use this command for mailing lists and bulk senders ranked by volume and unread share. Do NOT use it for plain top-sender counts; use 'contacts top' instead.

Inbound mail is grouped by List-Id (kind "list"). Other bulk mail is grouped
by sender address: kind "automated" when any message carries Auto-Submitted,
Precedence bulk/list/junk, X-Auto-Response-Suppress or List-Unsubscribe, or
comes from a no-reply/notification address; kind "one_way" for senders with
20+ messages in the window that you never wrote to. unread_share is unread/messages, last_read is the newest read message, and
list_unsubscribe is the first mailto: or https: link of the newest message.
Trash and Junk are ignored, and copies of one Message-ID count once.`,
		Example: strings.Trim(`
  thunderbird-pp-cli newsletters --since 90d --min 3 --agent
  thunderbird-pp-cli newsletters --since 365d --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "newsletters")
			}
			from, err := tbParseTimeBound(since, tbNow(), false)
			if err != nil {
				return usageErr(err)
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
			docs, err := tbWindowMessages(db, from)
			if err != nil {
				return err
			}
			rows := tbNewsletters(docs, mb, minMessages)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "MESSAGES\tUNREAD\tSHARE\tNAME\tKEY\tLAST_RECEIVED\tLAST_READ")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%d\t%.0f%%\t%s\t%s\t%s\t%s\n", r.Messages, r.Unread, r.UnreadShare*100, tbTrunc(r.Name, 28), tbTrunc(r.Key, 40), tbShortDate(r.LastReceived), tbShortDate(r.LastRead))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "Only mail newer than this (duration like 90d/6w or a date like 2025-01-31)")
	cmd.Flags().IntVar(&minMessages, "min", 3, "Only lists or senders with at least this many messages in the window")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum rows to show (0 = all)")
	return cmd
}

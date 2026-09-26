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

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

type tbCorrespondentRow struct {
	Address       string `json:"address"`
	Name          string `json:"name"`
	Received      int    `json:"received"`
	SentTo        int    `json:"sent_to"`
	Total         int    `json:"total"`
	LastContact   string `json:"last_contact"`
	InAddressBook bool   `json:"in_address_book"`
	Collected     bool   `json:"collected"`
}

// tbCorrespondents ranks counterparts of window messages in both directions.
func tbCorrespondents(docs []tbMessageDoc, mb *tbMailbox, contacts []tbContactDoc, notInAbook, includeBulk bool) []tbCorrespondentRow {
	bulk := tbNewBulk(docs, mb)
	book, collected, cardName := map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, c := range contacts {
		for _, e := range c.Emails {
			e = strings.ToLower(e)
			if c.Collected {
				collected[e] = true
				continue
			}
			book[e] = true
			if cardName[e] == "" {
				cardName[e] = tbContactName(c)
			}
		}
	}
	type acc struct {
		row      tbCorrespondentRow
		nameDate string
	}
	byAddr := map[string]*acc{}
	get := func(addr string) *acc {
		a := byAddr[addr]
		if a == nil {
			a = &acc{row: tbCorrespondentRow{Address: addr}}
			byAddr[addr] = a
		}
		return a
	}
	seen := map[string]bool{}
	for _, d := range docs {
		if tbprofile.IsTrashOrJunkFolderName(d.FolderPath) {
			continue
		}
		dir := mb.direction(d)
		key := fmt.Sprint(dir, "|", tbDedupKey(d))
		if dir == tbDirOther || seen[key] {
			continue
		}
		seen[key] = true
		switch dir {
		case tbDirInbound:
			addr := strings.ToLower(d.FromAddr)
			if addr == "" || (!includeBulk && bulk.kind(d) != "") {
				continue
			}
			a := get(addr)
			a.row.Received++
			if d.Date > a.row.LastContact {
				a.row.LastContact = d.Date
			}
			if d.FromName != "" && d.Date >= a.nameDate {
				a.row.Name, a.nameDate = d.FromName, d.Date
			}
		case tbDirSent:
			done := map[string]bool{}
			for _, addr := range tbRecipients(d) {
				addr = strings.ToLower(addr)
				if addr == "" || mb.isOwn(addr) || done[addr] {
					continue
				}
				done[addr] = true
				a := get(addr)
				a.row.SentTo++
				if d.Date > a.row.LastContact {
					a.row.LastContact = d.Date
				}
			}
		}
	}
	rows := make([]tbCorrespondentRow, 0, len(byAddr))
	for addr, a := range byAddr {
		if mb.isOwn(addr) {
			continue
		}
		r := a.row
		r.Total = r.Received + r.SentTo
		r.InAddressBook, r.Collected = book[addr], collected[addr]
		if n := cardName[addr]; n != "" {
			r.Name = n
		}
		if notInAbook && r.InAddressBook {
			continue
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Total != rows[j].Total {
			return rows[i].Total > rows[j].Total
		}
		if rows[i].LastContact != rows[j].LastContact {
			return rows[i].LastContact > rows[j].LastContact
		}
		return rows[i].Address < rows[j].Address
	})
	return rows
}

func newNovelContactsTopCmd(flags *rootFlags) *cobra.Command {
	var since string
	var notInAbook, includeBulk bool
	var limit int

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Rank the people you exchange mail with by volume and recency, and spot frequent ones missing from your address book.",
		Long: `Use this command to rank people by how much and how recently you exchanged mail. Do NOT use it to look up a known person's card; use 'contacts search' / 'contacts show' instead.

received counts messages from the person; bulk senders (mailing lists,
automated or no-reply mail, one-way senders with 20+ messages you never wrote
to) are left out unless --include-bulk. sent_to counts messages you sent to them in To or Cc, where
"sent" means stored in a sent folder. Your own identities, Trash and Junk
are excluded, and copies of the same Message-ID count once.
in_address_book matches a card of a real address book; collected reports a
Collected Addresses entry separately.`,
		Example: strings.Trim(`
  thunderbird-pp-cli contacts top --since 180d --agent
  thunderbird-pp-cli contacts top --since 365d --not-in-abook --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contacts top")
			}
			now := tbNow()
			from, err := tbParseTimeBound(since, now, false)
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
			contacts, err := tbLoadDocs[tbContactDoc](db, "contacts")
			if err != nil {
				return err
			}
			docs, err := tbWindowMessages(db, from)
			if err != nil {
				return err
			}
			rows := tbCorrespondents(docs, mb, contacts, notInAbook, includeBulk)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "ADDRESS\tNAME\tRECEIVED\tSENT_TO\tTOTAL\tLAST\tABOOK")
			for _, r := range rows {
				ab := "no"
				if r.InAddressBook {
					ab = "yes"
				} else if r.Collected {
					ab = "collected"
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%s\t%s\n", r.Address, tbTrunc(r.Name, 28), r.Received, r.SentTo, r.Total, tbShortDate(r.LastContact), ab)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "180d", "Only mail newer than this (duration like 90d/6w or a date like 2025-01-31)")
	cmd.Flags().BoolVar(&notInAbook, "not-in-abook", false, "Only people without an address-book card (collected addresses still count as missing)")
	cmd.Flags().BoolVar(&includeBulk, "include-bulk", false, "Keep mailing-list, automated, no-reply and one-way high-volume senders")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum people to show (0 = all)")
	return cmd
}

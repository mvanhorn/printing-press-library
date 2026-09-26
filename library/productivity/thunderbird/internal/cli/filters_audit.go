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

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

type tbFilterAuditRow struct {
	ID               string                   `json:"id"`
	Account          string                   `json:"account"`
	AccountName      string                   `json:"account_name"`
	Index            int                      `json:"index"`
	Name             string                   `json:"name"`
	Enabled          bool                     `json:"enabled"`
	Actions          []tbprofile.FilterAction `json:"actions"`
	Target           string                   `json:"target"`
	TargetFolder     string                   `json:"target_folder"`
	TargetExists     *bool                    `json:"target_exists"`
	MatchType        string                   `json:"match_type"`
	Summary          string                   `json:"condition_summary"`
	Status           string                   `json:"status"`
	Hits             *int                     `json:"hits"`
	ScannedMessages  int                      `json:"scanned_messages"`
	UnsupportedTerms []string                 `json:"unsupported_terms"`
	DuplicateOf      string                   `json:"duplicate_of"`
	Issues           []string                 `json:"issues"`
}

func tbIsFolderAction(t string) bool {
	return strings.EqualFold(t, "Move to folder") || strings.EqualFold(t, "Copy to folder")
}

func tbFilterSignature(d tbFilterDoc) string {
	var b strings.Builder
	b.WriteString(d.Account + "|" + strings.ToUpper(d.MatchType))
	for _, t := range d.Conditions {
		fmt.Fprintf(&b, "|%s,%s,%s", strings.ToLower(t.Field), strings.ToLower(t.Op), t.Value)
	}
	b.WriteString("|>")
	for _, a := range d.Actions {
		fmt.Fprintf(&b, "|%s=%s", strings.ToLower(a.Type), a.Value)
	}
	return b.String()
}

func tbFilterSubject(d tbMessageDoc) tbprofile.FilterSubject {
	return tbprofile.FilterSubject{
		FromAddr: d.FromAddr, FromName: d.FromName, To: d.To, Cc: d.Cc, Subject: d.Subject, Body: d.BodyText, ListID: d.ListID,
		Date: tbMsgTime(d), SizeBytes: d.SizeBytes, Read: d.Read, Replied: d.Replied, Flagged: d.Flagged, Forwarded: d.Forwarded,
		HasAttachments: d.HasAttachments,
	}
}

// tbAuditFilters expects filters in account/index order.
func tbAuditFilters(filters []tbFilterDoc, msgs []tbMessageDoc, mb *tbMailbox, accounts []tbAccountRow, folders map[string]bool, now time.Time) []tbFilterAuditRow {
	accs := make([]tbprofile.Account, 0, len(accounts))
	for _, a := range accounts {
		accs = append(accs, tbprofile.Account{Key: a.ID, Server: tbprofile.Server{Hostname: a.Hostname, UserName: a.UserName}})
	}
	byAccount := map[string][]tbprofile.FilterSubject{}
	seen := map[string]bool{}
	for _, m := range msgs {
		key := m.Account + "|" + tbDedupKey(m)
		if mb.sentByMe(m) || seen[key] {
			continue
		}
		seen[key] = true
		byAccount[m.Account] = append(byAccount[m.Account], tbFilterSubject(m))
	}
	firstBySig := map[string]string{}
	rows := make([]tbFilterAuditRow, 0, len(filters))
	for _, f := range filters {
		r := tbFilterAuditRow{
			ID: f.ID, Account: f.Account, AccountName: f.AccountName, Index: f.Index, Name: f.Name, Enabled: f.Enabled,
			Actions: f.Actions, MatchType: f.MatchType, Summary: tbConditionSummary(f.MatchType, f.Conditions),
			UnsupportedTerms: []string{}, Issues: []string{},
		}
		if r.Actions == nil {
			r.Actions = []tbprofile.FilterAction{}
		}
		if !f.Enabled {
			r.Issues = append(r.Issues, "disabled")
		}
		for _, a := range f.Actions {
			if !tbIsFolderAction(a.Type) {
				continue
			}
			exists, key := false, ""
			if acc, path, ok := tbprofile.MatchFolderURI(a.Value, accs); ok {
				key = tbFolderKey(acc, path)
				exists = folders[strings.ToLower(key)]
			}
			if r.TargetExists == nil || !exists {
				r.Target, r.TargetFolder, r.TargetExists = a.Value, key, &exists
			}
			if !exists {
				r.Issues = append(r.Issues, "missing_target_folder")
				break
			}
		}
		for _, t := range f.Conditions {
			if !tbprofile.TermSupported(t) {
				r.UnsupportedTerms = append(r.UnsupportedTerms, strings.TrimSpace(t.Field+" "+t.Op+" "+t.Value))
			}
		}
		subjects := byAccount[f.Account]
		r.ScannedMessages = len(subjects)
		if len(r.UnsupportedTerms) > 0 {
			r.Status = "unevaluated"
			r.Issues = append(r.Issues, "unevaluated")
		} else {
			r.Status = "evaluated"
			hits := 0
			for _, s := range subjects {
				if m, _ := tbprofile.EvalFilter(f.MatchType, f.Conditions, s, now); m {
					hits++
				}
			}
			r.Hits = &hits
			if hits == 0 {
				r.Issues = append(r.Issues, "no_hits")
			}
		}
		sig := tbFilterSignature(f)
		if first, ok := firstBySig[sig]; ok {
			r.DuplicateOf = first
			r.Issues = append(r.Issues, "duplicate")
		} else {
			firstBySig[sig] = f.ID
		}
		rows = append(rows, r)
	}
	return rows
}

func tbFiltersNeedBody(filters []tbFilterDoc) bool {
	for _, f := range filters {
		for _, t := range f.Conditions {
			if strings.EqualFold(strings.TrimSpace(t.Field), "body") {
				return true
			}
		}
	}
	return false
}

func newNovelFiltersAuditCmd(flags *rootFlags) *cobra.Command {
	var days int
	var since, account string

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Check every message filter: how often it matches, whether it is disabled, and whether its target folder still exists.",
		Long: `Evaluate every message filter rule (msgFilterRules.dat) against the local
store and flag the ones that need attention.

For each filter: enabled, actions, target folder of Move/Copy actions and
whether it still exists, and hits = how many messages of the filter's account
since --since (sent folders excluded, copies counted once) match its
condition; scanned_messages is how many were tested. Terms on from, to, cc,
"to or cc", subject, body, List-Id, date, age in days, size, status and
attachment status are evaluated from stored fields; any other term (Bcc or
"all addresses", which covers Bcc and is not stored, custom headers, tags,
junk score, regex) makes the filter "unevaluated" with
hits null rather than a guess. duplicate_of names an earlier filter of the same
account with identical conditions and actions. issues lists disabled,
missing_target_folder, unevaluated, no_hits and duplicate.`,
		Example: strings.Trim(`
  thunderbird-pp-cli filters audit --since 90d --agent
  thunderbird-pp-cli filters audit --account account1 --since 365d`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "filters audit")
			}
			now := tbNow()
			cutoff, err := tbWindowStart(cmd, since, days, now)
			if err != nil {
				return err
			}
			db, err := tbStoreFor(cmd, flags, "filters")
			if err != nil || db == nil {
				return err
			}
			defer db.Close()
			all, err := tbLoadDocs[tbFilterDoc](db, "filters")
			if err != nil {
				return err
			}
			filters := make([]tbFilterDoc, 0, len(all))
			for _, f := range all {
				if tbAccountMatches(account, f.Account, f.AccountName) {
					filters = append(filters, f)
				}
			}
			sort.Slice(filters, func(i, j int) bool {
				if filters[i].Account != filters[j].Account {
					return filters[i].Account < filters[j].Account
				}
				return filters[i].Index < filters[j].Index
			})
			mb, err := tbLoadMailbox(db)
			if err != nil {
				return err
			}
			accounts, err := tbLoadDocs[tbAccountRow](db, "accounts")
			if err != nil {
				return err
			}
			folderDocs, err := tbLoadDocs[tbFolderDoc](db, "folders")
			if err != nil {
				return err
			}
			folders := map[string]bool{}
			for _, f := range folderDocs {
				folders[strings.ToLower(f.ID)] = true
			}
			var msgs []tbMessageDoc
			if len(filters) > 0 {
				accKeys := map[string]bool{}
				var inArgs []any
				for _, f := range filters {
					if !accKeys[f.Account] {
						accKeys[f.Account] = true
						inArgs = append(inArgs, f.Account)
					}
				}
				msgs, err = tbQueryMessagesBody(db, `json_extract(data,'$.date') >= ? AND json_extract(data,'$.account') IN `+tbInClause(len(inArgs)),
					append([]any{tbFormatTime(cutoff)}, inArgs...), "", 0, tbFiltersNeedBody(filters))
				if err != nil {
					return err
				}
			}
			rows := tbAuditFilters(filters, msgs, mb, accounts, folders, now)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "ACCOUNT\t#\tNAME\tENABLED\tHITS\tSCANNED\tTARGET_OK\tISSUES")
			for _, r := range rows {
				hits, target := "-", "-"
				if r.Hits != nil {
					hits = fmt.Sprint(*r.Hits)
				}
				if r.TargetExists != nil {
					target = fmt.Sprint(*r.TargetExists)
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%v\t%s\t%d\t%s\t%s\n", r.AccountName, r.Index, tbTrunc(r.Name, 30), r.Enabled, hits, r.ScannedMessages, target, strings.Join(r.Issues, ","))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "90d", "Count hits among messages newer than this (duration like 90d/12w or a date like 2025-01-31)")
	cmd.Flags().IntVar(&days, "days", 0, "Alias of --since in days")
	_ = cmd.Flags().MarkHidden("days")
	cmd.Flags().StringVar(&account, "account", "", "Only filters of this account (key like account1 or account name)")
	return cmd
}

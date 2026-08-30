// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: followup — meetings with no subsequent follow-up signal.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clarify/internal/cliutil"

	"github.com/spf13/cobra"
)

type followupMeetingView struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	EndedAt string `json:"ended_at,omitempty"`
	Company string `json:"company,omitempty"`
	Reason  string `json:"reason"`
}

type followupCompanyView struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Meetings int    `json:"meetings_in_window"`
}

type followupView struct {
	Since           string                `json:"since"`
	Gaps            []followupMeetingView `json:"gaps"`
	NoDealCompanies []followupCompanyView `json:"no_deal_companies,omitempty"`
	ScannedMeetings int                   `json:"scanned_meetings"`
	Note            string                `json:"note,omitempty"`
}

func newNovelFollowupCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagNoDeal bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "followup",
		Short: "The dropped-ball list: meetings with no subsequent activity, comment, or task on the linked deal or company.",
		Long: `Finds meetings in the window that show no follow-up signal: the linked
deal and company records have not been updated since the meeting ended, and no
task was created after it. With --no-deal it also lists companies you met with
that have no open deal.

Reads the local mirror: sync meetings, deals, companies, and tasks first.`,
		Example: `  clarify-pp-cli followup --since 7d --json
  clarify-pp-cli followup --since 14d --no-deal`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list meetings in the window with no follow-up signal on their linked records")
				return nil
			}
			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since must be a duration like 7d, 24h, or 1w: %w", err))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, ok, err := clarifyMirrorGuard(cmd, flags, ctx, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "resources") {
				hintIfStale(cmd, db, "resources", flags.maxAge)
			}

			meetings, err := loadClarifyObjects(ctx, db, "meeting")
			if err != nil {
				return err
			}
			companies, err := loadClarifyObjects(ctx, db, "company")
			if err != nil {
				return err
			}
			deals, err := loadClarifyObjects(ctx, db, "deal")
			if err != nil {
				return err
			}
			tasks, err := loadClarifyObjects(ctx, db, "task")
			if err != nil {
				return err
			}
			people, err := loadClarifyObjects(ctx, db, "person")
			if err != nil {
				return err
			}
			companyByID := indexByID(companies)
			dealByID := indexByID(deals)
			personByID := indexByID(people)

			// Task follow-up signals: creation time plus the record IDs the
			// task references, so only tasks linked to the meeting's own deal
			// or company suppress a gap.
			type taskSignal struct {
				created time.Time
				refs    map[string]bool
			}
			var taskSignals []taskSignal
			for _, t := range tasks {
				created, ok := attrTime(t.Attrs, clarifyCreatedKeys...)
				if !ok {
					continue
				}
				refs := map[string]bool{}
				for _, ids := range t.Rels {
					for _, id := range ids {
						refs[id] = true
					}
				}
				for _, id := range attrLinkIDs(t, "company_id", "deal_id", "meeting_id", "person_id") {
					refs[id] = true
				}
				taskSignals = append(taskSignals, taskSignal{created: created, refs: refs})
			}

			byEmail := emailIndex(people)
			openDealCompanies := map[string]bool{}
			for _, d := range deals {
				if clarifyStageClosed(attrString(d.Attrs, clarifyStageKeys...)) {
					continue
				}
				for _, cid := range linkedCompanyIDs(d) {
					openDealCompanies[cid] = true
				}
			}

			now := time.Now()
			windowStart := now.Add(-window)
			view := followupView{Since: flagSince, Gaps: []followupMeetingView{}, ScannedMeetings: 0}
			meetingsPerCompany := map[string]int{}
			skippedNoTime := 0

			for _, m := range meetings {
				end, ok := attrTime(m.Attrs, clarifyEndKeys...)
				if !ok {
					if start, sok := attrTime(m.Attrs, clarifyStartKeys...); sok {
						end = start.Add(time.Hour)
					} else {
						skippedNoTime++
						continue
					}
				}
				if end.Before(windowStart) || end.After(now) {
					continue
				}
				view.ScannedMeetings++

				companyIDs := meetingCompanyIDs(m, byEmail, personByID)
				for _, cid := range companyIDs {
					meetingsPerCompany[cid]++
				}

				// Follow-up signals: any linked deal or company updated after
				// the meeting ended, or any task created after it.
				followedUp := false
				for _, did := range linkedDealIDs(m) {
					if d, ok := dealByID[did]; ok {
						if updated, uok := objUpdatedAt(d); uok && updated.After(end) {
							followedUp = true
						}
					}
				}
				if !followedUp {
					for _, cid := range companyIDs {
						if c, ok := companyByID[cid]; ok {
							if updated, uok := objUpdatedAt(c); uok && updated.After(end) {
								followedUp = true
							}
						}
					}
				}
				if !followedUp {
					linked := map[string]bool{}
					for _, cid := range companyIDs {
						linked[cid] = true
					}
					for _, did := range linkedDealIDs(m) {
						linked[did] = true
					}
					linked[m.ID] = true // tasks carry meeting_id links
					for _, ts := range taskSignals {
						if !ts.created.After(end) || !ts.created.Before(end.Add(window)) {
							continue
						}
						for id := range ts.refs {
							if linked[id] {
								followedUp = true
								break
							}
						}
						if followedUp {
							break
						}
					}
				}
				if followedUp {
					continue
				}
				entry := followupMeetingView{
					ID:      m.ID,
					Name:    attrString(m.Attrs, clarifyNameKeys...),
					EndedAt: end.Format(time.RFC3339),
					Reason:  "no update to the linked deal or company and no task created after the meeting",
				}
				for _, cid := range companyIDs {
					if c, ok := companyByID[cid]; ok && entry.Company == "" {
						entry.Company = attrString(c.Attrs, clarifyNameKeys...)
					}
				}
				view.Gaps = append(view.Gaps, entry)
			}
			sort.Slice(view.Gaps, func(i, j int) bool { return view.Gaps[i].EndedAt > view.Gaps[j].EndedAt })

			if flagNoDeal {
				view.NoDealCompanies = []followupCompanyView{}
				for cid, count := range meetingsPerCompany {
					if openDealCompanies[cid] {
						continue
					}
					entry := followupCompanyView{ID: cid, Meetings: count}
					if c, ok := companyByID[cid]; ok {
						entry.Name = attrString(c.Attrs, clarifyNameKeys...)
					}
					view.NoDealCompanies = append(view.NoDealCompanies, entry)
				}
				sort.Slice(view.NoDealCompanies, func(i, j int) bool {
					return view.NoDealCompanies[i].Meetings > view.NoDealCompanies[j].Meetings
				})
			}

			if len(meetings) == 0 {
				view.Note = "no meetings in the local mirror; run: clarify-pp-cli sync --resources resources --path-context object=meeting"
			} else if view.ScannedMeetings == 0 {
				view.Note = fmt.Sprintf("no meetings ended in the last %s (%d meetings mirrored, %d without parseable times)", flagSince, len(meetings), skippedNoTime)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(view.Gaps) == 0 {
				fmt.Fprintf(out, "No follow-up gaps found across %d meetings in the last %s.\n", view.ScannedMeetings, flagSince)
			} else {
				fmt.Fprintf(out, "%d meetings with no follow-up signal (last %s):\n\n", len(view.Gaps), flagSince)
				for _, g := range view.Gaps {
					name := g.Name
					if name == "" {
						name = g.ID
					}
					fmt.Fprintf(out, "  %s  %s", g.EndedAt, name)
					if g.Company != "" {
						fmt.Fprintf(out, "  @ %s", g.Company)
					}
					fmt.Fprintln(out)
				}
			}
			if flagNoDeal && len(view.NoDealCompanies) > 0 {
				fmt.Fprintf(out, "\nCompanies met with but no open deal (%d):\n", len(view.NoDealCompanies))
				for _, c := range view.NoDealCompanies {
					fmt.Fprintf(out, "  %-40s  %d meetings\n", c.Name, c.Meetings)
				}
			}
			if view.Note != "" {
				fmt.Fprintln(out, view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Window to scan for meetings, as a duration like 7d, 24h, or 1w")
	cmd.Flags().BoolVar(&flagNoDeal, "no-deal", false, "Also list companies with meetings in the window but no open deal")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the standard local mirror)")
	return cmd
}

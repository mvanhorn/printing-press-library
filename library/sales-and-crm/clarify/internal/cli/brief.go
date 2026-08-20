// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: brief — start-of-day overview from the local mirror.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type briefMeetingView struct {
	ID        string   `json:"id"`
	Name      string   `json:"name,omitempty"`
	StartsAt  string   `json:"starts_at,omitempty"`
	Attendees []string `json:"attendees,omitempty"`
	Company   string   `json:"company,omitempty"`
	OpenDeals []string `json:"open_deals,omitempty"`
}

type briefTaskView struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Due  string `json:"due,omitempty"`
}

type briefDealView struct {
	ID      string  `json:"id"`
	Name    string  `json:"name,omitempty"`
	Stage   string  `json:"stage,omitempty"`
	Amount  float64 `json:"amount,omitempty"`
	Updated string  `json:"updated_at,omitempty"`
}

type briefView struct {
	Date              string             `json:"date"`
	TodaysMeetings    []briefMeetingView `json:"todays_meetings"`
	TasksDueToday     []briefTaskView    `json:"tasks_due_today"`
	DealsMovedRecently []briefDealView   `json:"deals_moved_recently"`
	OpenDealCount     int                `json:"open_deal_count"`
	Note              string             `json:"note,omitempty"`
}

func newNovelBriefCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "brief",
		Short: "Start-of-day overview: today's meetings joined to their companies, open deals, and yesterday's record activity",
		Long: `Use this command for a start-of-day overview across all meetings and deals.
Do NOT use it to prepare for one specific meeting; use 'prep' instead.
Do NOT use it to list stalled deals only; use 'stale' instead.

Reads the local mirror: sync the built-in objects first
(clarify-pp-cli sync --resources resources --path-context object=meeting, then deal, person, company, task).`,
		Example: `  clarify-pp-cli brief --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build the start-of-day overview from the local mirror")
				return nil
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
			people, err := loadClarifyObjects(ctx, db, "person")
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
			personByID := indexByID(people)
			companyByID := indexByID(companies)

			now := time.Now()
			dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			dayEnd := dayStart.AddDate(0, 0, 1)
			yesterday := dayStart.AddDate(0, 0, -1)

			// Open deals grouped by their related company for the meeting join.
			openDealsByCompany := map[string][]string{}
			openDealCount := 0
			var moved []briefDealView
			for _, d := range deals {
				stage := attrString(d.Attrs, clarifyStageKeys...)
				if clarifyStageClosed(stage) {
					continue
				}
				openDealCount++
				label := attrString(d.Attrs, clarifyNameKeys...)
				if label == "" {
					label = d.ID
				}
				if stage != "" {
					label = label + " (" + stage + ")"
				}
				for _, companyID := range linkedCompanyIDs(d) {
					openDealsByCompany[companyID] = append(openDealsByCompany[companyID], label)
				}
				if updated, ok := objUpdatedAt(d); ok && updated.After(yesterday) {
					entry := briefDealView{
						ID:      d.ID,
						Name:    attrString(d.Attrs, clarifyNameKeys...),
						Stage:   stage,
						Updated: updated.Format(time.RFC3339),
					}
					if amount, ok := attrNumber(d.Attrs, clarifyAmountKeys...); ok {
						entry.Amount = amount
					}
					moved = append(moved, entry)
				}
			}
			sort.Slice(moved, func(i, j int) bool { return moved[i].Updated > moved[j].Updated })

			byEmail := emailIndex(people)
			view := briefView{
				Date:               dayStart.Format("2006-01-02"),
				TodaysMeetings:     []briefMeetingView{},
				TasksDueToday:      []briefTaskView{},
				DealsMovedRecently: moved,
				OpenDealCount:      openDealCount,
			}
			if view.DealsMovedRecently == nil {
				view.DealsMovedRecently = []briefDealView{}
			}

			skippedNoStart := 0
			for _, m := range meetings {
				start, ok := attrTime(m.Attrs, clarifyStartKeys...)
				if !ok {
					skippedNoStart++
					continue
				}
				if start.Before(dayStart) || !start.Before(dayEnd) {
					continue
				}
				entry := briefMeetingView{
					ID:       m.ID,
					Name:     attrString(m.Attrs, clarifyNameKeys...),
					StartsAt: start.Format(time.RFC3339),
				}
				for _, part := range meetingParticipants(m) {
					label := part.Name
					if label == "" {
						label = part.Email
					}
					if label != "" {
						entry.Attendees = append(entry.Attendees, label)
					}
				}
				if len(entry.Attendees) == 0 {
					for _, pid := range relIDsAny(m, clarifyPeopleRelKeys) {
						if p, ok := personByID[pid]; ok {
							if n := attrString(p.Attrs, clarifyNameKeys...); n != "" {
								entry.Attendees = append(entry.Attendees, n)
							}
						}
					}
				}
				companyIDs := meetingCompanyIDs(m, byEmail, personByID)
				for _, cid := range companyIDs {
					if c, ok := companyByID[cid]; ok && entry.Company == "" {
						entry.Company = attrString(c.Attrs, clarifyNameKeys...)
					}
					entry.OpenDeals = append(entry.OpenDeals, openDealsByCompany[cid]...)
				}
				view.TodaysMeetings = append(view.TodaysMeetings, entry)
			}
			sort.Slice(view.TodaysMeetings, func(i, j int) bool {
				return view.TodaysMeetings[i].StartsAt < view.TodaysMeetings[j].StartsAt
			})

			for _, t := range tasks {
				if taskDone(attrString(t.Attrs, "status", "state")) {
					continue
				}
				due, ok := attrTime(t.Attrs, clarifyDueKeys...)
				if !ok || due.Before(dayStart) || !due.Before(dayEnd) {
					continue
				}
				view.TasksDueToday = append(view.TasksDueToday, briefTaskView{
					ID:   t.ID,
					Name: attrString(t.Attrs, clarifyNameKeys...),
					Due:  due.Format(time.RFC3339),
				})
			}

			if len(meetings) == 0 && len(deals) == 0 {
				view.Note = "mirror is empty: sync meetings and deals first (clarify-pp-cli sync --resources resources --path-context object=meeting)"
			} else if skippedNoStart > 0 && len(view.TodaysMeetings) == 0 {
				view.Note = fmt.Sprintf("%d meetings had no parseable start time; today's list may be incomplete", skippedNoStart)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Briefing for %s\n\n", view.Date)
			if len(view.TodaysMeetings) == 0 {
				fmt.Fprintln(out, "No meetings today in the local mirror.")
			} else {
				fmt.Fprintf(out, "Today's meetings (%d):\n", len(view.TodaysMeetings))
				for _, m := range view.TodaysMeetings {
					fmt.Fprintf(out, "  %s  %s", m.StartsAt, m.Name)
					if m.Company != "" {
						fmt.Fprintf(out, "  @ %s", m.Company)
					}
					fmt.Fprintln(out)
					if len(m.OpenDeals) > 0 {
						fmt.Fprintf(out, "      open deals: %v\n", m.OpenDeals)
					}
				}
			}
			if len(view.TasksDueToday) > 0 {
				fmt.Fprintf(out, "\nTasks due today (%d):\n", len(view.TasksDueToday))
				for _, t := range view.TasksDueToday {
					fmt.Fprintf(out, "  %s\n", t.Name)
				}
			}
			if len(view.DealsMovedRecently) > 0 {
				fmt.Fprintf(out, "\nDeals updated since yesterday (%d):\n", len(view.DealsMovedRecently))
				for _, d := range view.DealsMovedRecently {
					fmt.Fprintf(out, "  %-40s  %s\n", d.Name, d.Stage)
				}
			}
			fmt.Fprintf(out, "\nOpen deals in pipeline: %d\n", view.OpenDealCount)
			if view.Note != "" {
				fmt.Fprintln(out, view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the standard local mirror)")
	return cmd
}

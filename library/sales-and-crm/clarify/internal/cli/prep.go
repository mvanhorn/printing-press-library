// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: prep — one-shot pre-call pack for a meeting.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/clarify/internal/cliutil"

	"github.com/spf13/cobra"
)

type prepAttendeeView struct {
	ID     string   `json:"id"`
	Name   string   `json:"name,omitempty"`
	Emails []string `json:"emails,omitempty"`
	Title  string   `json:"title,omitempty"`
}

type prepDealView struct {
	ID     string  `json:"id"`
	Name   string  `json:"name,omitempty"`
	Stage  string  `json:"stage,omitempty"`
	Amount float64 `json:"amount,omitempty"`
}

type prepTranscriptView struct {
	MeetingID   string `json:"meeting_id"`
	MeetingName string `json:"meeting_name,omitempty"`
	HeldAt      string `json:"held_at,omitempty"`
	Excerpt     string `json:"excerpt"`
	Source      string `json:"source"`
}

type prepView struct {
	Meeting         map[string]any       `json:"meeting"`
	StartsAt        string               `json:"starts_at,omitempty"`
	Attendees       []prepAttendeeView   `json:"attendees"`
	Company         string               `json:"company,omitempty"`
	CompanyID       string               `json:"company_id,omitempty"`
	OpenDeals       []prepDealView       `json:"open_deals"`
	PastTranscripts []prepTranscriptView `json:"past_transcripts"`
	Note            string               `json:"note,omitempty"`
}

func newNovelPrepCmd(flags *rootFlags) *cobra.Command {
	var flagNext bool
	var dbPath string
	var maxTranscripts int

	cmd := &cobra.Command{
		Use:   "prep [meeting-id]",
		Short: "One command before a call: the meeting's attendees, their company, open deals",
		Long: `Use this command to prepare for one upcoming meeting.
Do NOT use it for a general record background bundle; use 'dossier' instead.

Builds a pre-call pack from the local mirror: the meeting's attendees with
their person records, the company, its open deals, and transcript excerpts
from past meetings with the same company (fetched live once and cached).`,
		Example: `  clarify-pp-cli prep --next --agent
  clarify-pp-cli prep 5f8b7d2e-9c4a-4e1b-8f3d-2a6c9e0b4d71`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !flags.asJSON && !flags.agent {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would assemble the pre-call pack for the meeting from the local mirror plus cached transcripts")
				return nil
			}
			if len(args) == 0 && !flagNext {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("pass a meeting ID or --next"))
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
			personByID := indexByID(people)
			companyByID := indexByID(companies)

			var target clarifyObj
			found := false
			if flagNext {
				now := time.Now()
				var best time.Time
				for _, m := range meetings {
					start, ok := attrTime(m.Attrs, clarifyStartKeys...)
					if !ok || start.Before(now) {
						continue
					}
					if !found || start.Before(best) {
						best, target, found = start, m, true
					}
				}
				if !found {
					if flags.asJSON || flags.agent {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					}
					return notFoundErr(fmt.Errorf("no upcoming meeting in the local mirror; run: clarify-pp-cli sync --resources resources --path-context object=meeting"))
				}
			} else {
				for _, m := range meetings {
					if m.ID == args[0] {
						target, found = m, true
						break
					}
				}
				if !found {
					if flags.asJSON || flags.agent {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					}
					return notFoundErr(fmt.Errorf("meeting %s not found in the local mirror", args[0]))
				}
			}

			view := prepView{
				Meeting:         map[string]any{"id": target.ID, "name": attrString(target.Attrs, clarifyNameKeys...)},
				Attendees:       []prepAttendeeView{},
				OpenDeals:       []prepDealView{},
				PastTranscripts: []prepTranscriptView{},
			}
			if start, ok := attrTime(target.Attrs, clarifyStartKeys...); ok {
				view.StartsAt = start.Format(time.RFC3339)
			}

			byEmail := emailIndex(people)
			for _, part := range meetingParticipants(target) {
				attendee := prepAttendeeView{Name: part.Name}
				if part.Email != "" {
					attendee.Emails = []string{part.Email}
				}
				if part.Email != "" {
					for _, p := range byEmail[strings.ToLower(part.Email)] {
						attendee.ID = p.ID
						if n := attrString(p.Attrs, clarifyNameKeys...); n != "" {
							attendee.Name = n
						}
						if t := attrString(p.Attrs, "job_title", "title_role", "role"); t != "" {
							attendee.Title = t
						}
						if len(linkedCompanyIDs(p)) > 0 {
							break // prefer the duplicate that carries the company link
						}
					}
				}
				if attendee.Name != "" || len(attendee.Emails) > 0 {
					view.Attendees = append(view.Attendees, attendee)
				}
			}
			for _, pid := range relIDsAny(target, clarifyPeopleRelKeys) {
				if p, ok := personByID[pid]; ok {
					view.Attendees = append(view.Attendees, prepAttendeeView{
						ID:     p.ID,
						Name:   attrString(p.Attrs, clarifyNameKeys...),
						Emails: attrItems(p.Attrs, clarifyEmailKeys...),
						Title:  attrString(p.Attrs, "job_title", "title_role", "role"),
					})
				}
			}
			companyIDs := meetingCompanyIDs(target, byEmail, personByID)
			if len(companyIDs) > 0 {
				view.CompanyID = companyIDs[0]
				if c, ok := companyByID[view.CompanyID]; ok {
					view.Company = attrString(c.Attrs, clarifyNameKeys...)
				}
			}

			companySet := map[string]bool{}
			for _, cid := range companyIDs {
				companySet[cid] = true
			}
			for _, d := range deals {
				if clarifyStageClosed(attrString(d.Attrs, clarifyStageKeys...)) {
					continue
				}
				match := false
				for _, cid := range linkedCompanyIDs(d) {
					if companySet[cid] {
						match = true
					}
				}
				if !match {
					continue
				}
				entry := prepDealView{
					ID:    d.ID,
					Name:  attrString(d.Attrs, clarifyNameKeys...),
					Stage: attrString(d.Attrs, clarifyStageKeys...),
				}
				if amount, ok := attrNumber(d.Attrs, clarifyAmountKeys...); ok {
					entry.Amount = amount
				}
				view.OpenDeals = append(view.OpenDeals, entry)
			}

			// Past meetings with the same company, most recent first.
			type pastMeeting struct {
				obj  clarifyObj
				held time.Time
			}
			var past []pastMeeting
			now := time.Now()
			for _, m := range meetings {
				if m.ID == target.ID {
					continue
				}
				start, ok := attrTime(m.Attrs, clarifyStartKeys...)
				if !ok || start.After(now) {
					continue
				}
				match := false
				for _, cid := range meetingCompanyIDs(m, byEmail, personByID) {
					if companySet[cid] {
						match = true
					}
				}
				if match {
					past = append(past, pastMeeting{obj: m, held: start})
				}
			}
			sort.Slice(past, func(i, j int) bool { return past[i].held.After(past[j].held) })

			limit := maxTranscripts
			if cliutil.IsDogfoodEnv() && limit > 1 {
				limit = 1
			}
			if err := db.EnsureClarifySideTables(ctx); err != nil {
				return err
			}
			var transcriptErr string
			for _, pm := range past {
				if len(view.PastTranscripts) >= limit {
					break
				}
				content, cached, cerr := db.CachedTranscript(ctx, pm.obj.ID)
				if cerr != nil && transcriptErr == "" {
					transcriptErr = cerr.Error()
				}
				source := "cache"
				if cerr == nil && !cached && flags.dataSource != "local" {
					if c, clientErr := flags.newClient(); clientErr == nil {
						path := fmt.Sprintf("/workspaces/{workspace}/meetings/%s/transcript", url.PathEscape(pm.obj.ID))
						if data, ferr := c.Get(ctx, path, nil); ferr == nil {
							content = extractTranscriptText(data)
							source = "live"
							if content != "" {
								_ = db.CacheTranscript(ctx, pm.obj.ID, content, time.Now())
							}
						} else if transcriptErr == "" {
							transcriptErr = ferr.Error()
						}
					} else if transcriptErr == "" {
						transcriptErr = clientErr.Error()
					}
				}
				if content == "" {
					continue
				}
				excerpt := content
				if len(excerpt) > 700 {
					runes := []rune(excerpt)
					if len(runes) > 700 {
						runes = runes[:700]
					}
					excerpt = string(runes) + "…"
				}
				view.PastTranscripts = append(view.PastTranscripts, prepTranscriptView{
					MeetingID:   pm.obj.ID,
					MeetingName: attrString(pm.obj.Attrs, clarifyNameKeys...),
					HeldAt:      pm.held.Format(time.RFC3339),
					Excerpt:     excerpt,
					Source:      source,
				})
			}
			if len(view.PastTranscripts) == 0 {
				switch {
				case len(past) == 0:
					view.Note = "no past meetings with this company in the local mirror"
				case transcriptErr != "":
					view.Note = fmt.Sprintf("transcripts unavailable: %s", transcriptErr)
				case flags.dataSource == "local":
					view.Note = "no cached transcripts; rerun without --data-source local to fetch them once"
				default:
					view.Note = "past meetings found but no transcripts were available"
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Prep: %v", view.Meeting["name"])
			if view.StartsAt != "" {
				fmt.Fprintf(out, "  (%s)", view.StartsAt)
			}
			fmt.Fprintln(out)
			if view.Company != "" {
				fmt.Fprintf(out, "Company: %s\n", view.Company)
			}
			if len(view.Attendees) > 0 {
				fmt.Fprintln(out, "\nAttendees:")
				for _, a := range view.Attendees {
					fmt.Fprintf(out, "  %-30s %-25s %s\n", a.Name, a.Title, strings.Join(a.Emails, ", "))
				}
			}
			if len(view.OpenDeals) > 0 {
				fmt.Fprintln(out, "\nOpen deals:")
				for _, d := range view.OpenDeals {
					fmt.Fprintf(out, "  %-40s  %s\n", d.Name, d.Stage)
				}
			}
			for _, t := range view.PastTranscripts {
				fmt.Fprintf(out, "\nFrom %q (%s):\n  %s\n", t.MeetingName, t.HeldAt, t.Excerpt)
			}
			if view.Note != "" {
				fmt.Fprintf(out, "\n%s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagNext, "next", false, "Prep for the soonest upcoming meeting in the local mirror")
	cmd.Flags().IntVar(&maxTranscripts, "max-transcripts", 2, "Maximum past-meeting transcripts to include (each may cost one live fetch before caching)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the standard local mirror)")
	return cmd
}

// extractTranscriptText flattens a transcript payload to plain text. Handles a
// JSON:API envelope with segment lists as well as plain text bodies.
func extractTranscriptText(data json.RawMessage) string {
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		return cliutil.CleanText(strings.TrimSpace(string(data)))
	}
	var sb strings.Builder
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for _, key := range []string{"text", "content", "transcript", "speech"} {
				if s, ok := t[key].(string); ok && strings.TrimSpace(s) != "" {
					sb.WriteString(strings.TrimSpace(s))
					sb.WriteString(" ")
				}
			}
			for _, val := range t {
				switch val.(type) {
				case map[string]any, []any:
					walk(val)
				}
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(envelope)
	return cliutil.CleanText(strings.TrimSpace(sb.String()))
}

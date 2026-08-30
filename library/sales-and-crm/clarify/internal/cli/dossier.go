// Copyright 2026 Isaac Marks and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: dossier — one compact background bundle for any record.
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

	"github.com/spf13/cobra"
)

type dossierRelatedView struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Name  string `json:"name,omitempty"`
	Stage string `json:"stage,omitempty"`
}

type dossierMeetingView struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	StartsAt string `json:"starts_at,omitempty"`
}

type dossierView struct {
	Record     map[string]any       `json:"record"`
	Type       string               `json:"type"`
	Related    []dossierRelatedView `json:"related"`
	Meetings   []dossierMeetingView `json:"meetings"`
	Activities []map[string]any     `json:"activities,omitempty"`
	Source     string               `json:"source"`
	Note       string               `json:"note,omitempty"`
}

func newNovelDossierCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var flagActivities bool

	cmd := &cobra.Command{
		Use:   "dossier <record-id>",
		Short: "A complete background bundle on any record: fields, relationships, activities, comments",
		Long: `Use this command for a complete background bundle on any record.
Do NOT use it to prepare for a specific meeting; use 'prep' instead.

Finds the record in the local mirror by ID (any object type), attaches every
related mirrored record and meeting, and — unless --data-source local — pulls
the record's live activity feed (which includes comments).`,
		Example: `  clarify-pp-cli dossier 5f8b7d2e-9c4a-4e1b-8f3d-2a6c9e0b4d71 --agent
  clarify-pp-cli dossier 5f8b7d2e-9c4a-4e1b-8f3d-2a6c9e0b4d71 --agent --select record,related`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !flags.asJSON && !flags.agent {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would assemble a background bundle for the record from the local mirror plus its live activity feed")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a record ID is required"))
			}
			recordID := args[0]
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

			objTypes := []string{"person", "company", "deal", "meeting", "task"}
			byType := map[string][]clarifyObj{}
			var allPeople []clarifyObj
			var target clarifyObj
			found := false
			for _, t := range objTypes {
				objs, err := loadClarifyObjects(ctx, db, t)
				if err != nil {
					return err
				}
				byType[t] = objs
				if t == "person" {
					allPeople = objs
				}
				for _, o := range objs {
					if o.ID == recordID {
						target = o
						found = true
					}
				}
			}
			byEmail := emailIndex(allPeople)
			personByID := indexByID(allPeople)
			if !found {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return notFoundErr(fmt.Errorf("record %s not found in the local mirror; sync its object type first (clarify-pp-cli sync --resources resources --path-context object=<type>)", recordID))
			}

			view := dossierView{
				Record:   map[string]any{"id": target.ID, "attributes": target.Attrs},
				Type:     target.Type,
				Related:  []dossierRelatedView{},
				Meetings: []dossierMeetingView{},
				Source:   "local",
			}

			// Directly related records via the target's own relationships.
			related := map[string]bool{}
			seenMeetings := map[string]bool{}
			addMeeting := func(obj clarifyObj) {
				if seenMeetings[obj.ID] {
					return
				}
				seenMeetings[obj.ID] = true
				start, _ := attrTime(obj.Attrs, clarifyStartKeys...)
				mv := dossierMeetingView{ID: obj.ID, Name: attrString(obj.Attrs, clarifyNameKeys...)}
				if !start.IsZero() {
					mv.StartsAt = start.Format(time.RFC3339)
				}
				view.Meetings = append(view.Meetings, mv)
			}
			addRelated := func(obj clarifyObj) {
				if obj.ID == target.ID || related[obj.ID] {
					return
				}
				related[obj.ID] = true
				entry := dossierRelatedView{
					ID:   obj.ID,
					Type: obj.Type,
					Name: attrString(obj.Attrs, clarifyNameKeys...),
				}
				if obj.Type == "deal" {
					entry.Stage = attrString(obj.Attrs, clarifyStageKeys...)
				}
				view.Related = append(view.Related, entry)
			}
			wanted := map[string]bool{}
			for _, ids := range target.Rels {
				for _, id := range ids {
					wanted[id] = true
				}
			}
			// Reverse edges: any mirrored record that references the target.
			for _, t := range objTypes {
				for _, o := range byType[t] {
					if o.ID == target.ID {
						continue
					}
					if wanted[o.ID] {
						if o.Type == "meeting" {
							addMeeting(o)
						} else {
							addRelated(o)
						}
						continue
					}
					refsTarget := false
					for _, ids := range o.Rels {
						for _, id := range ids {
							if id == target.ID {
								refsTarget = true
							}
						}
					}
					if !refsTarget {
						for _, id := range attrLinkIDs(o, "company_id", "deal_id", "person_id", "meeting_id", "assignee_id") {
							if id == target.ID {
								refsTarget = true
							}
						}
					}
					if !refsTarget && o.Type == "meeting" && target.Type == "person" {
						// meetings reference people by participant email
						for _, part := range meetingParticipants(o) {
							for _, e := range attrItems(target.Attrs, clarifyEmailKeys...) {
								if part.Email != "" && strings.EqualFold(part.Email, e) {
									refsTarget = true
								}
							}
						}
					}
					if !refsTarget && o.Type == "meeting" && target.Type == "company" {
						// meetings reach companies through participants' person records
						for _, cid := range meetingCompanyIDs(o, byEmail, personByID) {
							if cid == target.ID {
								refsTarget = true
							}
						}
					}
					if refsTarget {
						if o.Type == "meeting" {
							addMeeting(o)
						} else {
							addRelated(o)
						}
					}
				}
			}
			sort.Slice(view.Meetings, func(i, j int) bool { return view.Meetings[i].StartsAt > view.Meetings[j].StartsAt })

			// Live activity feed (includes comments) unless local-only.
			if flagActivities && flags.dataSource != "local" {
				if c, cerr := flags.newClient(); cerr == nil {
					path := fmt.Sprintf("/workspaces/{workspace}/objects/%s/records/%s/activities", target.Type, url.PathEscape(target.ID))
					if data, aerr := c.Get(ctx, path, map[string]string{"page[size]": "25"}); aerr == nil {
						var envelope struct {
							Data []map[string]any `json:"data"`
						}
						if json.Unmarshal(data, &envelope) == nil && len(envelope.Data) > 0 {
							view.Activities = envelope.Data
							view.Source = "local+live"
						}
					} else {
						view.Note = fmt.Sprintf("activity feed unavailable: %v", aerr)
					}
				} else {
					view.Note = fmt.Sprintf("activity feed skipped: %v", cerr)
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			name := attrString(target.Attrs, clarifyNameKeys...)
			fmt.Fprintf(out, "Dossier: %s (%s %s)\n\n", name, target.Type, target.ID)
			if len(view.Related) > 0 {
				fmt.Fprintf(out, "Related records (%d):\n", len(view.Related))
				for _, r := range view.Related {
					fmt.Fprintf(out, "  %-8s  %-40s  %s\n", r.Type, r.Name, r.Stage)
				}
			}
			if len(view.Meetings) > 0 {
				fmt.Fprintf(out, "\nMeetings (%d):\n", len(view.Meetings))
				for _, m := range view.Meetings {
					fmt.Fprintf(out, "  %s  %s\n", m.StartsAt, m.Name)
				}
			}
			if len(view.Activities) > 0 {
				fmt.Fprintf(out, "\nRecent activity: %d events (use --json for detail)\n", len(view.Activities))
			}
			if view.Note != "" {
				fmt.Fprintln(out, view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to the standard local mirror)")
	cmd.Flags().BoolVar(&flagActivities, "activities", true, "Fetch the record's live activity feed (includes comments); disabled by --data-source local")
	return cmd
}

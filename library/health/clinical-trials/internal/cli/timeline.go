// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// timeline.go implements the `timeline` command: outputs a trial's key dates
// in chronological order. It fetches a date-only field list directly from the
// /studies/{nctId} endpoint, since the command needs nothing but dates and
// identity.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// timelineEvent is one date entry in the chronological sequence.
type timelineEvent struct {
	Label string `json:"label"`
	Date  string `json:"date"`
}

// timelineView is the JSON shape `timeline` emits.
type timelineView struct {
	NCTID  string          `json:"id"`
	Title  string          `json:"title,omitempty"`
	Status string          `json:"status,omitempty"`
	Events []timelineEvent `json:"events"`
	Note   string          `json:"note,omitempty"`
}

// timelineFields is the fields request sent to /studies/{nctId}. Every field
// here is now also in normalizedFields, but this const stays local and stays a
// strict subset: `timeline` renders dates only, so requesting the sponsor,
// condition, intervention, and location arrays would inflate the response for
// data the command discards. Switching to normalizedFields would also couple
// this command to changes made for the intelligence commands' benefit.
const timelineFields = "NCTId,BriefTitle,OverallStatus,StartDate,PrimaryCompletionDate,CompletionDate,LastUpdatePostDate,WhyStopped"

// rawTimelineStudy captures only the date-related portions of the CT.gov
// protocolSection. It parallels rawStudy in intel.go but is intentionally
// narrower — timeline.go only needs dates and identity fields.
type rawTimelineStudy struct {
	ProtocolSection struct {
		IdentificationModule struct {
			NCTID      string `json:"nctId"`
			BriefTitle string `json:"briefTitle"`
		} `json:"identificationModule"`
		StatusModule struct {
			OverallStatus   string `json:"overallStatus"`
			WhyStopped      string `json:"whyStopped"`
			StartDateStruct struct {
				Date string `json:"date"`
			} `json:"startDateStruct"`
			PrimaryCompletionDateStruct struct {
				Date string `json:"date"`
			} `json:"primaryCompletionDateStruct"`
			CompletionDateStruct struct {
				Date string `json:"date"`
			} `json:"completionDateStruct"`
			LastUpdatePostDateStruct struct {
				Date string `json:"date"`
			} `json:"lastUpdatePostDateStruct"`
		} `json:"statusModule"`
	} `json:"protocolSection"`
}

// ctgovFetchTimeline fetches one trial's date fields from /studies/{nctId}
// using the expanded timelineFields list.
func ctgovFetchTimeline(ctx context.Context, c ctgovClient, nctID string) (rawTimelineStudy, bool, error) {
	raw, err := c.Get(ctx, "/studies/"+nctID, map[string]string{
		"format": "json",
		"fields": timelineFields,
	})
	if err != nil {
		return rawTimelineStudy{}, false, err
	}
	var s rawTimelineStudy
	if err := json.Unmarshal(raw, &s); err != nil {
		return rawTimelineStudy{}, false, err
	}
	if s.ProtocolSection.IdentificationModule.NCTID == "" {
		return rawTimelineStudy{}, false, nil
	}
	return s, true, nil
}

// buildTimeline converts the raw study into an ordered slice of events,
// omitting any dates that are empty.
func buildTimeline(s rawTimelineStudy) timelineView {
	sm := s.ProtocolSection.StatusModule
	im := s.ProtocolSection.IdentificationModule

	type candidate struct {
		label string
		date  string
	}
	candidates := []candidate{
		{"Start", sm.StartDateStruct.Date},
		{"Primary completion", sm.PrimaryCompletionDateStruct.Date},
		{"Completion", sm.CompletionDateStruct.Date},
		{"Last update posted", sm.LastUpdatePostDateStruct.Date},
	}

	// Sort by date string. CT.gov dates are "YYYY-MM-DD" or "YYYY-MM" or
	// "YYYY" — lexicographic sort is chronologically correct for all three.
	// Events with empty dates are excluded.
	var events []timelineEvent
	for _, c := range candidates {
		if strings.TrimSpace(c.date) != "" {
			events = append(events, timelineEvent{Label: c.label, Date: c.date})
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Date < events[j].Date
	})

	note := ""
	if sm.WhyStopped != "" {
		note = "stopped: " + sm.WhyStopped
	}
	if len(events) == 0 {
		note = "no dates posted for this trial"
		if sm.WhyStopped != "" {
			note += "; stopped: " + sm.WhyStopped
		}
	}

	return timelineView{
		NCTID:  im.NCTID,
		Title:  im.BriefTitle,
		Status: sm.OverallStatus,
		Events: events,
		Note:   note,
	}
}

// pp:data-source live
func newNovelTimelineCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timeline <nct-id>",
		Short: "Show a trial's key dates in chronological order: start, primary completion, completion, last-update posted.",
		Long: "Fetches a single trial and outputs its posted dates as an ordered sequence:\n" +
			"start date, primary completion date, completion date, and last-update posted.\n" +
			"Events with no posted date are omitted. Dates are sorted chronologically;\n" +
			"CT.gov date strings (YYYY-MM-DD, YYYY-MM, YYYY) sort correctly lexicographically.",
		Example: "  clinical-trials-pp-cli timeline NCT04280705\n" +
			"  clinical-trials-pp-cli timeline NCT04280705 --json",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show key trial dates from ClinicalTrials.gov")
				return nil
			}
			nctID := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			s, ok, err := ctgovFetchTimeline(ctx, c, nctID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !ok {
				return usageErr(fmt.Errorf("no trial found for %q (check the NCT id)", nctID))
			}

			view := buildTimeline(s)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderTimelineHuman(cmd, view)
		},
	}
	return cmd
}

func renderTimelineHuman(cmd *cobra.Command, v timelineView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s  %s\n", v.NCTID, truncate(v.Title, 80))
	fmt.Fprintf(w, "  status=%s\n\n", v.Status)
	if len(v.Events) == 0 {
		fmt.Fprintln(w, "No dates posted for this trial.")
	} else {
		fmt.Fprintln(w, "Timeline:")
		for _, e := range v.Events {
			fmt.Fprintf(w, "  %-26s %s\n", e.Label, e.Date)
		}
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}

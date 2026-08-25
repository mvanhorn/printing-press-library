// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written absorbed feature: merged cross-account calendar list (manifest row #1).
// Invoked by the promoted `calendars` command when no calendarId is given.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/gauth"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/verdict"
	"github.com/spf13/cobra"
)

type calendarListRow struct {
	Account    string `json:"account"`
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	AccessRole string `json:"access_role"`
	Primary    bool   `json:"primary,omitempty"`
	TimeZone   string `json:"time_zone,omitempty"`
}

type calendarListOutput struct {
	Calendars []calendarListRow `json:"calendars"`
	Coverage  verdict.Coverage  `json:"coverage"`
}

func runMergedCalendarList(cmd *cobra.Command, flags *rootFlags) error {
	dir := gauth.ConfigDir(flags.authDir)
	profiles, err := gauth.LoadProfiles(dir)
	if err != nil {
		return err
	}
	var rows []calendarListRow
	var sources []verdict.Source
	for _, p := range profiles {
		src := verdict.Source{Account: p.Name, Calendar: "calendarList", FetchedAt: time.Now().UTC().Format(time.RFC3339)}
		c, err := flags.clientFor(p.Name)
		if err != nil {
			src.Error = err.Error()
			sources = append(sources, src)
			continue
		}
		pageToken := ""
		for {
			params := map[string]string{"maxResults": "250"}
			if pageToken != "" {
				params["pageToken"] = pageToken
			}
			data, err := c.GetNoCache(cmd.Context(), "/users/me/calendarList", params)
			if err != nil {
				src.Error = err.Error()
				break
			}
			var body struct {
				Etag  string `json:"etag"`
				Items []struct {
					ID         string `json:"id"`
					Summary    string `json:"summary"`
					AccessRole string `json:"accessRole"`
					Primary    bool   `json:"primary"`
					TimeZone   string `json:"timeZone"`
				} `json:"items"`
				NextPageToken string `json:"nextPageToken"`
			}
			if err := json.Unmarshal(data, &body); err != nil {
				src.Error = fmt.Sprintf("unparseable calendarList body: %v", err)
				break
			}
			src.EtagPresent = src.EtagPresent || body.Etag != ""
			for _, it := range body.Items {
				rows = append(rows, calendarListRow{Account: p.Name, ID: it.ID, Summary: it.Summary, AccessRole: it.AccessRole, Primary: it.Primary, TimeZone: it.TimeZone})
			}
			if body.NextPageToken == "" {
				break
			}
			pageToken = body.NextPageToken
		}
		sources = append(sources, src)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Account != rows[j].Account {
			return rows[i].Account < rows[j].Account
		}
		return rows[i].Summary < rows[j].Summary
	})
	out := calendarListOutput{Calendars: rows, Coverage: verdict.BuildCoverage(sources)}
	if err := emitVerdict(cmd, flags, out, func(w io.Writer) {
		for _, r := range out.Calendars {
			p := ""
			if r.Primary {
				p = " (primary)"
			}
			fmt.Fprintf(w, "%s  %s  [%s]%s  %s\n", r.Account, r.Summary, r.AccessRole, p, r.ID)
		}
		fmt.Fprintln(w, coverageSummary(out.Coverage))
		coverageErrorLines(w, out.Coverage)
	}); err != nil {
		return err
	}
	if !out.Coverage.Complete {
		return exitDegraded("calendar list incomplete: %d/%d accounts", out.Coverage.Checked, out.Coverage.Of)
	}
	return nil
}

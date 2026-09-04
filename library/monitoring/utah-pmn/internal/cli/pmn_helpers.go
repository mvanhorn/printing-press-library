// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored land-use monitoring helpers for the Utah PMN CLI.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/internal/client"
)

// pmnNotice is one entry from getUpcomingNotices.json's noticeDtoList.
type pmnNotice struct {
	EntityName       string `json:"entityName"`
	PublicBodyName   string `json:"publicBodyName"`
	NoticeID         int64  `json:"noticeId"`
	MeetingTitle     string `json:"meetingTitle"`
	MeetingAddress1  string `json:"meetingAddress1"`
	MeetingAddress2  string `json:"meetingAddress2"`
	MeetingState     string `json:"meetingState"`
	MeetingZip       string `json:"meetingZip"`
	MeetingCity      string `json:"meetingCity"`
	MeetingStartTime string `json:"meetingStartTime"`
	MeetingAgenda    string `json:"meetingAgenda"`
	GovernmentType   string `json:"governmentType"`
	NoticeURL        string `json:"noticeUrl,omitempty"`
}

// millardLocation is one curated Millard County town the sweep queries.
type millardLocation struct {
	City string `json:"city"`
	Zip  string `json:"zip"`
}

// millardLocations is the curated town/ZIP set the county sweep covers.
// County bodies surface under the nearest town, so the union + dedup gives
// full Millard County coverage (city councils, planning commissions, the
// county commission, RDAs, and boards).
var millardLocations = []millardLocation{
	{"Delta", "84624"},
	{"Fillmore", "84631"},
	{"Hinckley", "84635"},
	{"Oak City", "84649"},
	{"Holden", "84636"},
	{"Scipio", "84656"},
	{"Kanosh", "84637"},
	{"Meadow", "84644"},
	{"Lynndyl", "84640"},
	{"Leamington", "84638"},
}

// landUseBodyTerms identify public bodies that decide land-use/development
// approvals. Matched as lowercase substrings against the body/entity name.
var landUseBodyTerms = []string{
	"planning commission",
	"planning",
	"city council",
	"town council",
	"county commission",
	"commission",
	"board of adjustment",
	"redevelopment",
	"community reinvestment",
	"zoning",
	"design review",
	"land use",
}

// nonLandUseBodyTerms veto bodies that would otherwise match a broad term
// like "board" or "council" but never handle land-use approvals.
var nonLandUseBodyTerms = []string{
	"board of education",
	"school",
	"cemetery",
	"library",
	"recreation",
	"water conservancy",
	"mosquito",
	"fire district",
}

// landUseKeywords are agenda phrases that signal a development/zoning action.
var landUseKeywords = []string{
	"rezone", "rezoning", "zone change", "zoning",
	"conditional use", "cup", "subdivision", "subdivide", "plat",
	"variance", "annex", "site plan", "ordinance",
	"development agreement", "general plan", "easement", "setback",
	"parcel", "land use", "lot line", "right-of-way", "right of way",
	"preliminary plat", "final plat", "boundary adjustment",
}

// bodyLooksLandUse reports whether a notice's body decides land-use matters.
func bodyLooksLandUse(n pmnNotice) bool {
	hay := strings.ToLower(n.PublicBodyName + " " + n.EntityName)
	for _, veto := range nonLandUseBodyTerms {
		if strings.Contains(hay, veto) {
			return false
		}
	}
	for _, term := range landUseBodyTerms {
		if strings.Contains(hay, term) {
			return true
		}
	}
	return false
}

// agendaHasLandUse reports whether the agenda or title mentions a land-use action.
func agendaHasLandUse(n pmnNotice) (bool, string) {
	hay := strings.ToLower(n.MeetingAgenda + " " + n.MeetingTitle)
	for _, kw := range landUseKeywords {
		if strings.Contains(hay, kw) {
			return true, kw
		}
	}
	return false, ""
}

// fetchNotices calls getUpcomingNotices.json for one location and date window.
// returnFormattedDateValues is always true so meetingStartTime is a string.
func fetchNotices(ctx context.Context, c *client.Client, location, start, end string, limit int) ([]pmnNotice, error) {
	params := map[string]string{
		"zipOrCity":                 location,
		"returnFormattedDateValues": "true",
		"listSize":                  strconv.Itoa(limit),
	}
	if start != "" {
		params["startDate"] = start
	}
	if end != "" {
		params["endDate"] = end
	}
	raw, err := c.Get(ctx, "/getUpcomingNotices.json", params)
	if err != nil {
		return nil, err
	}
	var env struct {
		NoticeDtoList []pmnNotice `json:"noticeDtoList"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("parsing notices: %w", err)
	}
	for i := range env.NoticeDtoList {
		env.NoticeDtoList[i].NoticeURL = noticeURL(env.NoticeDtoList[i].NoticeID)
	}
	return env.NoticeDtoList, nil
}

// sweepLocations fetches across multiple locations concurrently and dedups by
// noticeId. Locations are fetched in parallel (bounded) so a multi-town county
// sweep takes about as long as the slowest single request, not their sum.
func sweepLocations(ctx context.Context, c *client.Client, locs []string, start, end string, limit int) ([]pmnNotice, error) {
	type result struct {
		idx     int
		notices []pmnNotice
		err     error
	}
	const maxConcurrent = 6
	sem := make(chan struct{}, maxConcurrent)
	results := make(chan result, len(locs))
	var wg sync.WaitGroup
	for i, loc := range locs {
		wg.Add(1)
		go func(idx int, location string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			notices, err := fetchNotices(ctx, c, location, start, end, limit)
			results <- result{idx: idx, notices: notices, err: err}
		}(i, loc)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	ordered := make([][]pmnNotice, len(locs))
	var failed []string
	var firstErr error
	for r := range results {
		if r.err != nil {
			failed = append(failed, locs[r.idx])
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		ordered[r.idx] = r.notices
	}
	// Fail closed on any location error so millard/landuse/agenda/watch
	// do not present incomplete county coverage as a successful sweep, and
	// since does not record a partial snapshot as the seen-set baseline.
	if firstErr != nil {
		sort.Strings(failed)
		return nil, fmt.Errorf("location sweep incomplete: %d/%d locations failed (%s): %w", len(failed), len(locs), strings.Join(failed, ", "), firstErr)
	}

	seen := map[int64]bool{}
	var out []pmnNotice
	for _, notices := range ordered {
		for _, n := range notices {
			if seen[n.NoticeID] {
				continue
			}
			seen[n.NoticeID] = true
			out = append(out, n)
		}
	}
	sortNoticesByDate(out)
	return out, nil
}

func sortNoticesByDate(ns []pmnNotice) {
	sort.SliceStable(ns, func(i, j int) bool {
		return ns[i].MeetingStartTime < ns[j].MeetingStartTime
	})
}

func noticeURL(id int64) string {
	return fmt.Sprintf("https://www.utah.gov/pmn/sitemap/notice/%d.html", id)
}

// dateWindow returns today and today+days as YYYY-MM-DD strings.
func dateWindow(days int) (string, string) {
	now := time.Now()
	start := now.Format("2006-01-02")
	end := now.AddDate(0, 0, days).Format("2006-01-02")
	return start, end
}

// millardCityNames returns just the city names for sweeping.
func millardCityNames() []string {
	out := make([]string, 0, len(millardLocations))
	for _, l := range millardLocations {
		out = append(out, l.City)
	}
	return out
}

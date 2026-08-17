// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written support layer for the verdict command family (conflicts,
// slots, changes, events exceptions, events update, manifest check).
//
// Design: this file owns flag parsing, manifest+profile loading, and the
// thin HTTP fetch layer; all verdict semantics (busy classification, overlap
// pairing, mirror detection, interval algebra, coverage) live in
// internal/verdict, where the unit tests feed fixtures directly.
//
// Freshness rule: every read feeding a verdict uses the client's NoCache
// variants (GetNoCache / PostQueryWithParams) — a verdict computed from a
// warm response cache would assert confidence over stale data.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/gauth"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/manifest"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/verdict"
)

// exitFindings wraps a verdict "findings present" outcome as exit code 3
// (conflicts found / manifest drift found). The JSON or table output has
// already been printed when this fires; the error is the exit-code carrier.
func exitFindings(format string, a ...any) error {
	return &cliError{code: 3, err: fmt.Errorf(format, a...)}
}

// exitDegraded wraps a verdict "incomplete coverage" outcome as exit code 4:
// at least one manifest calendar could not be read, so any all-clear or
// open-slot claim is explicitly downgraded rather than silently confident.
func exitDegraded(format string, a ...any) error {
	return &cliError{code: 4, err: fmt.Errorf(format, a...)}
}

// parseTimeFlag accepts the two documented forms for --from/--to/--since:
// YYYY-MM-DD (interpreted as local midnight) and RFC3339. Returns the zero
// time for an empty string so callers can apply defaults.
func parseTimeFlag(name, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, usageErr(fmt.Errorf("invalid %s value %q: use YYYY-MM-DD (local midnight) or RFC3339 (e.g. 2026-08-18T09:00:00-06:00)", name, value))
}

// relDays parses the "+Nd" relative form ("+2d" → 2, true).
func relDays(s string) (int, bool) {
	if len(s) < 3 || s[0] != '+' || s[len(s)-1] != 'd' {
		return 0, false
	}
	n := 0
	for _, r := range s[1 : len(s)-1] {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// resolveWindow parses --from/--to with defaults: --from defaults to today
// (local midnight), --to defaults to from+7d. Literal "today" and relative
// "+Nd" forms are accepted ("+Nd" on --from means today+N; on --to it means
// from+N). to must be after from.
func resolveWindow(fromFlag, toFlag string) (time.Time, time.Time, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	var from, to time.Time
	switch {
	case fromFlag == "today":
		from = today
	default:
		if n, ok := relDays(fromFlag); ok {
			from = today.AddDate(0, 0, n)
		} else {
			var err error
			from, err = parseTimeFlag("--from", fromFlag)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
		}
	}
	if from.IsZero() {
		from = today
	}
	switch {
	case toFlag == "today":
		to = today
	default:
		if n, ok := relDays(toFlag); ok {
			to = from.AddDate(0, 0, n)
		} else {
			var err error
			to, err = parseTimeFlag("--to", toFlag)
			if err != nil {
				return time.Time{}, time.Time{}, err
			}
		}
	}
	if to.IsZero() {
		to = from.AddDate(0, 0, 7)
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, usageErr(fmt.Errorf("--to (%s) must be after --from (%s)", to.Format(time.RFC3339), from.Format(time.RFC3339)))
	}
	return from.UTC(), to.UTC(), nil
}

// parseBetween parses a daily HH:MM-HH:MM wall-clock window into minutes
// after local midnight.
func parseBetween(value string) (startMin, endMin int, err error) {
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return 0, 0, usageErr(fmt.Errorf("invalid --between value %q: use HH:MM-HH:MM (e.g. 09:00-17:00)", value))
	}
	parse := func(s string) (int, error) {
		hm := strings.SplitN(strings.TrimSpace(s), ":", 2)
		if len(hm) != 2 {
			return 0, fmt.Errorf("bad clock value %q", s)
		}
		h, herr := strconv.Atoi(hm[0])
		m, merr := strconv.Atoi(hm[1])
		if herr != nil || merr != nil || h < 0 || h > 24 || m < 0 || m > 59 {
			return 0, fmt.Errorf("bad clock value %q", s)
		}
		return h*60 + m, nil
	}
	startMin, serr := parse(parts[0])
	endMin, eerr := parse(parts[1])
	if serr != nil || eerr != nil || endMin <= startMin {
		return 0, 0, usageErr(fmt.Errorf("invalid --between value %q: use HH:MM-HH:MM with end after start", value))
	}
	return startMin, endMin, nil
}

// resolveLocation maps --tz to a *time.Location: "local" (default) or an
// IANA zone name.
func resolveLocation(tz string) (*time.Location, error) {
	if tz == "" || strings.EqualFold(tz, "local") {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, usageErr(fmt.Errorf("invalid --tz value %q: use 'local' or an IANA zone like America/Denver", tz))
	}
	return loc, nil
}

// verdictContext is the loaded multi-account ground truth every verdict
// command starts from: gauth profiles + the approved calendar manifest, plus
// a lazily-built per-account client cache.
type verdictContext struct {
	dir      string
	profiles []gauth.Profile
	manifest *manifest.Manifest

	flags   *rootFlags
	clients map[string]*client.Client
	// clientErrs remembers a failed client build (usually a missing or
	// expired token) so every calendar of that account degrades coverage
	// with the same actionable error instead of retrying per calendar.
	clientErrs map[string]error
}

// loadVerdictContext loads profiles.yaml and calendars.yaml from the gauth
// config dir. A missing profiles.yaml or calendars.yaml surfaces the layer's
// own actionable error — never a panic — which matters because test and
// fresh-install environments routinely lack both.
func loadVerdictContext(flags *rootFlags) (*verdictContext, error) {
	dir := gauth.ConfigDir(flags.authDir)
	profiles, err := gauth.LoadProfiles(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	m, err := manifest.LoadValidated(dir, names)
	if err != nil {
		return nil, err
	}
	return &verdictContext{
		dir:        dir,
		profiles:   profiles,
		manifest:   m,
		flags:      flags,
		clients:    map[string]*client.Client{},
		clientErrs: map[string]error{},
	}, nil
}

// clientFor returns (building on first use) the authenticated client for a
// profile name. Build failures are cached and returned on every subsequent
// call so one bad account degrades coverage exactly once per calendar.
func (vc *verdictContext) clientFor(account string) (*client.Client, error) {
	if c, ok := vc.clients[account]; ok {
		return c, nil
	}
	if err, ok := vc.clientErrs[account]; ok {
		return nil, err
	}
	c, err := vc.flags.clientFor(account)
	if err != nil {
		vc.clientErrs[account] = err
		return nil, err
	}
	vc.clients[account] = c
	return c, nil
}

// verdictGetter is the slice of client.Client the events fetch layer needs;
// narrow so tests could substitute a fake without touching gauth.
type verdictGetter interface {
	GetNoCache(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// eventsListPage mirrors the fields of an events.list response the verdict
// layer consumes.
type eventsListPage struct {
	Etag          string            `json:"etag"`
	Updated       string            `json:"updated"`
	NextPageToken string            `json:"nextPageToken"`
	Items         []json.RawMessage `json:"items"`
}

// verdictMaxPages caps events.list pagination per calendar as a loop guard;
// at maxResults=2500 this is 125k events per calendar per window.
const verdictMaxPages = 50

// fetchCalendarEvents pulls every event of one calendar for the given query
// params (paginating on nextPageToken, always via the no-cache read path)
// and parses them into typed engine events. The returned Source carries the
// upstream freshness evidence for the coverage block; on any failure the
// Source records the error and no events are returned — a half-read calendar
// must not half-count.
func fetchCalendarEvents(ctx context.Context, g verdictGetter, account, calendarID string, baseParams map[string]string) ([]verdict.Event, verdict.Source) {
	src := verdict.Source{
		Account:   account,
		Calendar:  calendarID,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	params := map[string]string{"maxResults": "2500"}
	for k, v := range baseParams {
		params[k] = v
	}
	path := "/calendars/" + url.PathEscape(calendarID) + "/events"

	var events []verdict.Event
	var updatedMax time.Time
	for page := 0; ; page++ {
		if page >= verdictMaxPages {
			src.Error = fmt.Sprintf("pagination exceeded %d pages for calendar %s", verdictMaxPages, calendarID)
			return nil, src
		}
		raw, err := g.GetNoCache(ctx, path, params)
		if err != nil {
			src.Error = err.Error()
			return nil, src
		}
		var pageBody eventsListPage
		if err := json.Unmarshal(raw, &pageBody); err != nil {
			src.Error = fmt.Sprintf("unparseable events.list response: %v", err)
			return nil, src
		}
		if page == 0 {
			src.EtagPresent = pageBody.Etag != ""
		}
		if pageBody.Updated != "" {
			if u, err := time.Parse(time.RFC3339, pageBody.Updated); err == nil && u.After(updatedMax) {
				updatedMax = u
			}
		}
		for _, item := range pageBody.Items {
			ev, err := verdict.ParseEvent(account, calendarID, item)
			if err != nil {
				src.Error = err.Error()
				return nil, src
			}
			if ev.Updated.After(updatedMax) {
				updatedMax = ev.Updated
			}
			events = append(events, ev)
		}
		if pageBody.NextPageToken == "" {
			break
		}
		params["pageToken"] = pageBody.NextPageToken
	}
	if !updatedMax.IsZero() {
		src.UpstreamUpdatedMax = updatedMax.UTC().Format(time.RFC3339)
	}
	return events, src
}

// fetchManifestEvents fans out fetchCalendarEvents over every manifest
// calendar, in manifest order, and returns the merged event set plus one
// coverage source per calendar. Per-calendar failures (including a failed
// account client build) land in the sources — they never abort the fan-out.
func fetchManifestEvents(ctx context.Context, vc *verdictContext, baseParams map[string]string) ([]verdict.Event, []verdict.Source) {
	var events []verdict.Event
	sources := make([]verdict.Source, 0, len(vc.manifest.Calendars))
	for _, entry := range vc.manifest.Calendars {
		c, err := vc.clientFor(entry.Account)
		if err != nil {
			sources = append(sources, verdict.Source{
				Account:   entry.Account,
				Calendar:  entry.ID,
				FetchedAt: time.Now().UTC().Format(time.RFC3339),
				Error:     err.Error(),
			})
			continue
		}
		evs, src := fetchCalendarEvents(ctx, c, entry.Account, entry.ID, baseParams)
		events = append(events, evs...)
		sources = append(sources, src)
	}
	return events, sources
}

// verdictPoster is the freebusy slice of client.Client: a read-only query
// that rides POST on the wire (never cached, never verify-gated).
type verdictPoster interface {
	PostQueryWithParams(ctx context.Context, path string, params map[string]string, body any) (json.RawMessage, int, error)
}

type freeBusyCalendar struct {
	Errors []struct {
		Domain string `json:"domain"`
		Reason string `json:"reason"`
	} `json:"errors"`
	Busy []struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"busy"`
}

type freeBusyResponse struct {
	Calendars map[string]freeBusyCalendar `json:"calendars"`
}

// fetchFreeBusy queries /freeBusy for one account's manifest calendars in a
// single POST and returns merged busy intervals plus one coverage source per
// calendar. freebusy responses carry no etag and no updated stamps, so those
// evidence fields stay honestly empty.
func fetchFreeBusy(ctx context.Context, p verdictPoster, account string, calendarIDs []string, from, to time.Time) ([]verdict.Interval, []verdict.Source) {
	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	newSource := func(id string) verdict.Source {
		return verdict.Source{Account: account, Calendar: id, FetchedAt: fetchedAt}
	}
	items := make([]map[string]string, 0, len(calendarIDs))
	for _, id := range calendarIDs {
		items = append(items, map[string]string{"id": id})
	}
	body := map[string]any{
		"timeMin": from.UTC().Format(time.RFC3339),
		"timeMax": to.UTC().Format(time.RFC3339),
		"items":   items,
	}
	sources := make([]verdict.Source, 0, len(calendarIDs))
	raw, _, err := p.PostQueryWithParams(ctx, "/freeBusy", nil, body)
	if err != nil {
		for _, id := range calendarIDs {
			src := newSource(id)
			src.Error = err.Error()
			sources = append(sources, src)
		}
		return nil, sources
	}
	var resp freeBusyResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		for _, id := range calendarIDs {
			src := newSource(id)
			src.Error = fmt.Sprintf("unparseable freeBusy response: %v", err)
			sources = append(sources, src)
		}
		return nil, sources
	}
	var busy []verdict.Interval
	for _, id := range calendarIDs {
		src := newSource(id)
		cal, ok := resp.Calendars[id]
		switch {
		case !ok:
			src.Error = "calendar missing from freeBusy response"
		case len(cal.Errors) > 0:
			reasons := make([]string, 0, len(cal.Errors))
			for _, e := range cal.Errors {
				reasons = append(reasons, e.Reason)
			}
			src.Error = "freeBusy error: " + strings.Join(reasons, ", ")
		default:
			for _, b := range cal.Busy {
				start, serr := time.Parse(time.RFC3339, b.Start)
				end, eerr := time.Parse(time.RFC3339, b.End)
				if serr != nil || eerr != nil {
					src.Error = fmt.Sprintf("unparseable busy interval [%s, %s]", b.Start, b.End)
					break
				}
				busy = append(busy, verdict.Interval{Start: start.UTC(), End: end.UTC()})
			}
		}
		sources = append(sources, src)
	}
	return busy, sources
}

// wantsVerdictJSON reports whether verdict output should go through the
// machine pipeline (printJSONFiltered) instead of the concise human text.
// Any machine-format flag opts in; --agent already implies --json.
func wantsVerdictJSON(flags *rootFlags) bool {
	return flags.asJSON || flags.csv || flags.plain || flags.quiet || flags.selectFields != ""
}

// emitVerdict prints v through the machine pipeline, or calls human for
// terminal-facing output.
func emitVerdict(cmd *cobra.Command, flags *rootFlags, v any, human func(w io.Writer)) error {
	if wantsVerdictJSON(flags) {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		// Verdict commands always fetch live through the manifest clients
		// (pp:data-source live); the generic printer's "local" default was
		// mislabeling the evidence trail (operator's assistant bug report,
		// 2026-08-17). Coverage per-source stamps remain the fine-grained truth.
		return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(raw), flags, map[string]any{"source": "live"})
	}
	human(cmd.OutOrStdout())
	return nil
}

// coverageSummary renders the one-line human summary of a coverage block.
func coverageSummary(cov verdict.Coverage) string {
	state := "COMPLETE"
	if !cov.Complete {
		state = "DEGRADED"
	}
	return fmt.Sprintf("coverage: %d/%d calendars read (%s)", cov.Checked, cov.Of, state)
}

// coverageErrorLines lists each failed source as an indented human line.
func coverageErrorLines(w io.Writer, cov verdict.Coverage) {
	for _, s := range cov.Sources {
		if s.Error != "" {
			fmt.Fprintf(w, "  ! %s/%s: %s\n", s.Account, s.Calendar, s.Error)
		}
	}
}

// resolveUpdateAccount picks the gauth profile for `events update`: the
// --account flag when set, otherwise the manifest entry list for calendarId
// when exactly one account claims it. Ambiguity and absence are hard errors —
// a write must never guess its account.
func resolveUpdateAccount(flags *rootFlags, calendarID string) (string, error) {
	dir := gauth.ConfigDir(flags.authDir)
	profiles, err := gauth.LoadProfiles(dir)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		names = append(names, p.Name)
	}
	if flags.account != "" {
		found := false
		for _, n := range names {
			if n == flags.account {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("no profile %q (have: %s)", flags.account, strings.Join(names, ", "))
		}
		if m, merr := manifest.Load(dir); merr == nil {
			if err := refuseReadOnlyManifestRole(m, flags.account, calendarID); err != nil {
				return "", err
			}
		}
		return flags.account, nil
	}
	m, err := manifest.LoadValidated(dir, names)
	if err != nil {
		return "", fmt.Errorf("--account not set and manifest lookup failed: %w", err)
	}
	entries := m.Find(calendarID)
	switch len(entries) {
	case 0:
		return "", usageErr(fmt.Errorf("calendar %q is not in %s — pass --account <profile> explicitly", calendarID, manifest.FileName))
	case 1:
		if err := refuseReadOnlyManifestRole(m, entries[0].Account, calendarID); err != nil {
			return "", err
		}
		return entries[0].Account, nil
	default:
		accounts := make([]string, 0, len(entries))
		for _, e := range entries {
			accounts = append(accounts, e.Account)
		}
		sort.Strings(accounts)
		return "", usageErr(fmt.Errorf("calendar %q is manifested under multiple accounts (%s) — pass --account to disambiguate", calendarID, strings.Join(accounts, ", ")))
	}
}

// refuseReadOnlyManifestRole blocks writes to calendars the manifest declares
// read-only. The OAuth scope is the hard barrier; this is the earlier, more
// actionable one.
func refuseReadOnlyManifestRole(m *manifest.Manifest, account, calendarID string) error {
	for _, e := range m.Calendars {
		if e.Account == account && e.ID == calendarID && e.Role == manifest.RoleRead {
			return fmt.Errorf("calendar %q is declared role: read for account %q in %s — refusing to write (change the manifest role to 'write' if this is intended)", calendarID, account, manifest.FileName)
		}
	}
	return nil
}

// errIsThirdPartyBarrier reports whether err is the client safety barrier's
// refusal, which commands surface verbatim (it is already actionable).
func errIsThirdPartyBarrier(err error) bool {
	return errors.Is(err, client.ErrThirdPartyBarrier)
}

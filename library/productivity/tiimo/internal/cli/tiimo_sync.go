// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-written sync that replaces the generated one.
//
// WHY THIS EXISTS: the generated syncer resolves resource paths through
// syncResourcePath(), which only registers flat, non-parameterized paths. For
// this API that is exactly one resource -- /api/profiles. Every other Tiimo
// resource lives under /api/profiles/{profile_id}/..., so the generated sync
// reports "unknown sync resource" for all of them and can never hydrate the
// local mirror. Since every offline read in this CLI is backed by that mirror,
// the generated sync leaves the whole CLI empty.
//
// This version enumerates profiles first, then fans out over each profile's
// nested resources, and writes through the typed store helpers so the domain
// tables the novel commands query are actually populated.
//
// A regen must preserve this file or re-fix the same gap.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/store"
)

// activityWindowDays bounds one activities request. The endpoint returns a
// map keyed by every date in the range, so a multi-year pull in one call
// would be a single enormous response; chunking keeps each request small and
// lets partial progress survive a mid-run failure.
const activityWindowDays = 30

// syncResourceResult is one resource's outcome.
type syncResourceResult struct {
	Resource string `json:"resource"`
	Records  int    `json:"records"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		// Remove the generated sync before installing this one. It is not
		// additive: leaving both would mean the working command name is taken
		// by the one that cannot reach nested resources.
		if existing, _, err := root.Find([]string{"sync"}); err == nil && existing != nil && existing.Name() == "sync" {
			root.RemoveCommand(existing)
		}
		addNovelCommandIfAbsent(root, newTiimoSyncCmd(flags))
	})
}

func newTiimoSyncCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagFrom, flagTo, flagDB, flagProfile string
	var flagAhead int
	var flagResources []string
	var flagFull, flagLatestOnly, flagStrict bool
	var flagMaxPages, flagConcurrency int
	var flagParams, flagGlobalParams, flagResourceParams, flagPathContext []string
	var flagDates string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Tiimo data to local SQLite for offline search and analysis",
		Long: `Mirror your Tiimo data locally.

Every offline read in this CLI -- today, agenda, drift, stalls, adherence,
gaps, overlaps, capacity, rolling, feed, backup -- is served from this mirror,
so this is the command that makes the rest useful.

Activities and routines are windowed because their endpoints require a date
range. --since sets how far back to pull and --ahead how far forward, since a
planner is as much about next week as last week.

This sync has no incremental cursor -- every run re-fetches the whole window,
because Tiimo's activity endpoint is a date-range read with no change feed.
--full is therefore accepted for contract compatibility and additionally
clears recorded sync state so freshness checks re-evaluate from scratch.`,
		Example: "  tiimo-pp-cli sync --since 90d",
		Annotations: map[string]string{
			"pp:happy-args":       "--since=7d",
			"pp:typed-exit-codes": "0,4,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync")
			}

			back := 30 * 24 * time.Hour
			if strings.TrimSpace(flagSince) != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil || d <= 0 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: want 7d, 4w, or 24h", flagSince))
				}
				back = d
			}
			now := time.Now()
			from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Add(-back)
			to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, flagAhead)
			if strings.TrimSpace(flagFrom) != "" {
				t, err := parseTiimoDate(flagFrom)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(err)
				}
				from = t
			}
			if strings.TrimSpace(flagTo) != "" {
				t, err := parseTiimoDate(flagTo)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(err)
				}
				to = t
			}
			if d := strings.TrimSpace(flagDates); d != "" {
				parts := strings.SplitN(d, "..", 2)
				if len(parts) != 2 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --dates %q: want FROM..TO", flagDates))
				}
				a, aErr := parseTiimoDate(parts[0])
				b, bErr := parseTiimoDate(parts[1])
				if aErr != nil || bErr != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --dates %q: both sides must be YYYY-MM-DD", flagDates))
				}
				from, to = a, b
			}
			// --path-context is the framework's way to fill a path placeholder.
			// The only one this API has is profile_id, so map it onto --profile.
			for _, kv := range flagPathContext {
				k, v, ok := strings.Cut(kv, "=")
				if !ok {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --path-context %q: want key=value", kv))
				}
				if strings.TrimSpace(k) == "profile_id" {
					flagProfile = strings.TrimSpace(v)
				}
			}
			if flagLatestOnly {
				// Narrow to today only. Useful for a cheap catch-up run that
				// does not re-walk months of history.
				today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
				from, to = today, today
			}
			if to.Before(from) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to is before --from"))
			}

			want := map[string]bool{}
			for _, r := range flagResources {
				for _, part := range strings.Split(r, ",") {
					if p := strings.TrimSpace(part); p != "" {
						want[p] = true
					}
				}
			}
			selected := func(name string) bool { return len(want) == 0 || want[name] }

			extraParams := map[string]string{}
			perResourceParams := map[string]map[string]string{}
			for _, kv := range append(append([]string{}, flagParams...), flagGlobalParams...) {
				k, v, ok := strings.Cut(kv, "=")
				if !ok {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid param %q: want key=value", kv))
				}
				extraParams[strings.TrimSpace(k)] = v
			}
			for _, spec := range flagResourceParams {
				res, kv, ok := strings.Cut(spec, ":")
				if !ok {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --resource-param %q: want resource:key=value", spec))
				}
				k, v, ok2 := strings.Cut(kv, "=")
				if !ok2 {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --resource-param %q: want resource:key=value", spec))
				}
				res = strings.TrimSpace(res)
				if perResourceParams[res] == nil {
					perResourceParams[res] = map[string]string{}
				}
				perResourceParams[res][strings.TrimSpace(k)] = v
			}
			paramsFor := func(resource string, base map[string]string) map[string]string {
				out := map[string]string{}
				for k, v := range base {
					out[k] = v
				}
				for k, v := range extraParams {
					out[k] = v
				}
				for k, v := range perResourceParams[resource] {
					out[k] = v
				}
				if len(out) == 0 {
					return nil
				}
				return out
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			dbPath := flagDB
			if strings.TrimSpace(dbPath) == "" {
				dbPath = defaultDBPath("tiimo-pp-cli")
			}
			st, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local mirror at %s: %w", dbPath, err)
			}
			defer st.Close()

			if flagFull {
				// No incremental cursor exists to reset, but clearing the
				// recorded state makes freshness checks treat the next read as
				// a cold one rather than trusting a stale timestamp.
				for _, res := range backupResources {
					_ = st.SaveSyncState(res, "", 0)
				}
			}

			results := make([]syncResourceResult, 0, 8)
			record := func(name string, n int, err error) {
				r := syncResourceResult{Resource: name, Records: n, Status: "ok"}
				if err != nil {
					r.Status = "error"
					r.Error = err.Error()
				}
				results = append(results, r)
			}

			// Profiles first: everything else is scoped by profile id.
			profiles, err := profilesFromAPI(ctx, flags)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if selected("profiles") {
				n := 0
				for _, p := range profiles {
					raw, mErr := json.Marshal(p)
					if mErr != nil {
						continue
					}
					keyed, kErr := canonicalID(raw, "profileId")
					if kErr != nil {
						continue
					}
					if uErr := st.UpsertProfiles(keyed); uErr == nil {
						n++
					}
				}
				record("profiles", n, nil)
			}

			if strings.TrimSpace(flagProfile) != "" {
				id, rErr := resolveProfileID(ctx, cmd, flags, flagProfile, dbPath)
				if rErr != nil {
					return rErr
				}
				filtered := make([]profileRecord, 0, 1)
				for _, p := range profiles {
					if p.ProfileID == id {
						filtered = append(filtered, p)
					}
				}
				profiles = filtered
			}

			for _, p := range profiles {
				pid := cliutil.EscapePathParam(p.ProfileID)

				if selected("activities") {
					n, aErr := syncActivities(ctx, c, st, pid, from, to, flagMaxPages, paramsFor("activities", nil))
					record("activities", n, aErr)
				}
				if selected("todo_tasks") {
					n, tErr := syncFlatList(ctx, c, st, "/api/profiles/"+pid+"/todo-tasks", paramsFor("todo_tasks", nil), []string{"taskId"},
						func(raw json.RawMessage) error { return st.UpsertTodoTasks(raw) })
					record("todo_tasks", n, tErr)
				}
				if selected("tags") {
					n, tErr := syncFlatList(ctx, c, st, "/api/profiles/"+pid+"/tags", paramsFor("tags", nil), []string{"tagId"},
						func(raw json.RawMessage) error { return st.UpsertTags(raw) })
					record("tags", n, tErr)
				}
				if selected("routines") {
					params := map[string]string{
						"from": from.Format(tiimoDateLayout),
						"to":   to.Format(tiimoDateLayout),
					}
					n, rErr := syncFlatList(ctx, c, st, "/api/profiles/"+pid+"/routines", params, []string{"routineId", "id"},
						func(raw json.RawMessage) error { return st.UpsertRoutines(raw) })
					record("routines", n, rErr)
				}
				if selected("calendars") {
					n, cErr := syncFlatList(ctx, c, st,
						"/api/externalCalendar/profiles/"+pid+"/linkedCalendars", nil, []string{"calendarId"},
						func(raw json.RawMessage) error { return st.UpsertCalendars(raw) })
					record("calendars", n, cErr)
				}
				if selected("todo_lists") {
					n, lErr := syncTodoLists(ctx, c, st, pid)
					record("todo_lists", n, lErr)
				}
				if selected("calendar_events") {
					// Events imported from linked calendars live behind a
					// different endpoint from native activities, and they are
					// the ONLY ones that carry real clock times -- native Tiimo
					// activities are bucket-scheduled at midnight. Omitting
					// them made `today` silently miss every real meeting.
					n, eErr := syncCalendarEvents(ctx, c, st, pid, from, to)
					record("calendar_events", n, eErr)
				}
			}

			total := 0
			failed := 0
			for _, r := range results {
				total += r.Records
				if r.Status == "error" {
					failed++
				}
			}
			for _, r := range results {
				if r.Status == "ok" {
					_ = st.SaveSyncState(r.Resource, to.Format(tiimoDateLayout), r.Records)
				}
			}

			if failed > 0 && total == 0 {
				for _, r := range results {
					if r.Status == "error" {
						fmt.Fprintf(cmd.ErrOrStderr(), "sync error: %s: %s\n", r.Resource, r.Error)
					}
				}
				return apiErr(fmt.Errorf("sync failed: %d resource(s) errored and nothing was written", failed))
			}
			if failed > 0 && flagStrict {
				for _, r := range results {
					if r.Status == "error" {
						fmt.Fprintf(cmd.ErrOrStderr(), "sync error: %s: %s\n", r.Resource, r.Error)
					}
				}
				return apiErr(fmt.Errorf("--strict: %d resource(s) failed", failed))
			}
			if failed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d resource(s) failed; %d record(s) written from the rest\n", failed, total)
			}

			return writeTiimoResult(cmd, flags, results, func(w io.Writer) {
				fmt.Fprintf(w, "Synced %d record(s) from %s to %s\n",
					total, from.Format(tiimoDateLayout), to.Format(tiimoDateLayout))
				for _, r := range results {
					status := ""
					if r.Status == "error" {
						status = "  ERROR: " + r.Error
					}
					fmt.Fprintf(w, "  %-12s %5d%s\n", r.Resource, r.Records, status)
				}
			})
		},
	}

	cmd.Flags().BoolVar(&flagFull, "full", false, "Full resync: clear recorded sync state and re-fetch the whole window")
	cmd.Flags().IntVar(&flagMaxPages, "max-pages", 0, "Maximum date windows to fetch per resource (0 = unlimited)")
	cmd.Flags().BoolVar(&flagStrict, "strict", false, "Treat any resource-level failure as fatal instead of a warning")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 1, "Accepted for contract compatibility; this sync is sequential by design (the API is per-profile and small)")
	cmd.Flags().StringArrayVar(&flagParams, "param", nil, "Extra query param for flat-list requests (repeatable, key=value)")
	cmd.Flags().StringArrayVar(&flagGlobalParams, "global-param", nil, "Extra query param injected into every request (repeatable, key=value)")
	cmd.Flags().StringArrayVar(&flagResourceParams, "resource-param", nil, "Per-resource query param (repeatable, resource:key=value)")
	cmd.Flags().StringArrayVar(&flagPathContext, "path-context", nil, "Fill a path placeholder (repeatable, key=value); profile_id selects the profile")
	cmd.Flags().StringVar(&flagDates, "dates", "", "Explicit window as FROM..TO (YYYY-MM-DD..YYYY-MM-DD)")
	cmd.Flags().BoolVar(&flagLatestOnly, "latest-only", false, "Sync only the current day rather than the configured window")
	cmd.Flags().StringVar(&flagSince, "since", "", "How far back to pull (7d, 4w, 24h); default 30d")
	cmd.Flags().IntVar(&flagAhead, "ahead", 30, "How many days forward to pull")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Explicit window start (YYYY-MM-DD), overrides --since")
	cmd.Flags().StringVar(&flagTo, "to", "", "Explicit window end (YYYY-MM-DD), overrides --ahead")
	cmd.Flags().StringSliceVar(&flagResources, "resources", nil, "Comma-separated resources to sync (default all): profiles, activities, todo_tasks, todo_lists, tags, routines, calendars")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Sync only this profile (name or UUID)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror")
	return cmd
}

// canonicalID copies the resource's own identifier into a plain "id" field.
//
// The store derives a row key with extractObjectID(), which only recognizes
// id / Id / ID / _id / uuid / slug / name. Tiimo names every identifier after
// its resource -- activityId, taskId, tagId, calendarId -- so without this
// every activity upsert fails with "missing id", and tags and calendars
// silently fall back to being keyed by their *name*, which collides as soon
// as two tags share one. Injecting the real id fixes both.
func canonicalID(raw json.RawMessage, idFields ...string) (json.RawMessage, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("parsing record: %w", err)
	}
	if existing, ok := obj["id"]; ok {
		if s := store.ResourceIDString(existing); s != "" && s != "<nil>" {
			return raw, nil
		}
	}
	for _, field := range idFields {
		if v, ok := obj[field]; ok {
			if s := store.ResourceIDString(v); s != "" && s != "<nil>" {
				obj["id"] = s
				out, err := json.Marshal(obj)
				if err != nil {
					return nil, fmt.Errorf("re-encoding record: %w", err)
				}
				return out, nil
			}
		}
	}
	return nil, fmt.Errorf("record has none of the expected id fields %v", idFields)
}

// occurrenceID keys an activity row by activity id plus the date it falls on,
// so each occurrence of a repeating activity is stored separately.
func occurrenceID(raw json.RawMessage, date string) (json.RawMessage, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("parsing activity: %w", err)
	}
	activityID := store.ResourceIDString(obj["activityId"])
	if activityID == "" || activityID == "<nil>" {
		return nil, fmt.Errorf("activity record has no activityId")
	}
	day := strings.TrimSpace(date)
	if day == "" {
		// Fall back to the start timestamp's date when the response map key
		// is unusable, so a row is still stored rather than dropped.
		if s := store.ResourceIDString(obj["startTime"]); len(s) >= 10 {
			day = s[:10]
		}
	}
	if day == "" {
		return nil, fmt.Errorf("activity %s has no resolvable occurrence date", activityID)
	}
	// NUL is the store's own composite-key delimiter (see resourceStorageID
	// and BareResourceID). Using it means framework helpers strip the
	// occurrence suffix correctly; an ad-hoc separator like "@" is invisible
	// to them and leaks the composite into API calls as a bogus activity id.
	obj["id"] = activityID + string([]byte{0}) + day
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("re-encoding activity: %w", err)
	}
	return out, nil
}

// syncActivities walks the window in chunks and flattens the date-keyed
// response into individual activity rows.
//
// The endpoint returns {"2026-08-14": [ ... ], "2026-08-15": [ ... ]} rather
// than a flat array, which is why the generic list path cannot handle it.
func syncActivities(ctx context.Context, c apiGetter, st *store.Store, pid string, from, to time.Time, maxPages int, extra map[string]string) (int, error) {
	total := 0
	failures := 0
	pages := 0
	var firstErr error
	for start := from; !start.After(to); start = start.AddDate(0, 0, activityWindowDays) {
		if maxPages > 0 && pages >= maxPages {
			break
		}
		pages++
		end := start.AddDate(0, 0, activityWindowDays-1)
		if end.After(to) {
			end = to
		}
		params := map[string]string{
			"fromDate": start.Format(tiimoDateLayout),
			"toDate":   end.Format(tiimoDateLayout),
		}
		for k, v := range extra {
			params[k] = v
		}
		data, err := c.Get(ctx, "/api/profiles/"+pid+"/activities", params)
		if err != nil {
			return total, fmt.Errorf("fetching activities %s..%s: %w",
				start.Format(tiimoDateLayout), end.Format(tiimoDateLayout), err)
		}
		byDate := map[string][]json.RawMessage{}
		if err := json.Unmarshal(data, &byDate); err != nil {
			return total, fmt.Errorf("parsing activities window %s: %w", start.Format(tiimoDateLayout), err)
		}
		for date, items := range byDate {
			for _, raw := range items {
				// Key each row by activityId + occurrence date.
				//
				// A repeating activity is returned once per day in the window
				// but carries the SAME activityId every time. Keying on the
				// bare id would collapse every occurrence onto one row --
				// 1190 records became 14 before this was fixed -- and destroy
				// exactly the per-occurrence history that drift, adherence and
				// stalls exist to read. The activity_id column still holds the
				// real id, so grouping by activity is unaffected.
				keyed, err := occurrenceID(raw, date)
				if err == nil {
					err = st.UpsertActivities(keyed)
				}
				if err != nil {
					// Count and surface rather than swallow: a window that
					// writes nothing must not report success.
					failures++
					if firstErr == nil {
						firstErr = err
					}
					continue
				}
				total++
			}
		}
	}
	if total == 0 && failures > 0 {
		return 0, fmt.Errorf("all %d activity record(s) failed to store: %w", failures, firstErr)
	}
	if failures > 0 {
		return total, fmt.Errorf("%d of %d activity record(s) failed to store: %w", failures, failures+total, firstErr)
	}
	return total, nil
}

// syncCalendarEvents mirrors the activities imported from each linked
// external calendar.
//
// These share the activity shape and the date-keyed response, so they land in
// the same table and are distinguished by isReadOnly / origin. A calendar the
// user has hidden or disconnected is skipped rather than fetched.
func syncCalendarEvents(ctx context.Context, c apiGetter, st *store.Store, pid string, from, to time.Time) (int, error) {
	data, err := c.Get(ctx, "/api/externalCalendar/profiles/"+pid+"/linkedCalendars", nil)
	if err != nil {
		return 0, fmt.Errorf("listing linked calendars: %w", err)
	}
	var cals []struct {
		CalendarID string `json:"calendarId"`
		Hidden     bool   `json:"hidden"`
		Connected  bool   `json:"connected"`
	}
	if err := json.Unmarshal(data, &cals); err != nil {
		return 0, fmt.Errorf("parsing linked calendars: %w", err)
	}

	total := 0
	attempted := 0
	succeeded := 0
	var firstErr error
	for _, cal := range cals {
		if cal.CalendarID == "" || cal.Hidden || !cal.Connected {
			continue
		}
		attempted++
		calOK := true
		calPath := "/api/externalCalendar/profiles/" + pid + "/externalCalendars/" +
			cliutil.EscapePathParam(cal.CalendarID) + "/activities"
		for start := from; !start.After(to); start = start.AddDate(0, 0, activityWindowDays) {
			end := start.AddDate(0, 0, activityWindowDays-1)
			if end.After(to) {
				end = to
			}
			body, err := c.Get(ctx, calPath, map[string]string{
				"fromDate": start.Format(tiimoDateLayout),
				"toDate":   end.Format(tiimoDateLayout),
			})
			if err != nil {
				calOK = false
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			byDate := map[string][]json.RawMessage{}
			if err := json.Unmarshal(body, &byDate); err != nil {
				calOK = false
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			for date, items := range byDate {
				for _, raw := range items {
					keyed, err := occurrenceID(raw, date)
					if err != nil {
						continue
					}
					if err := st.UpsertActivities(keyed); err != nil {
						continue
					}
					total++
				}
			}
		}
		if calOK {
			succeeded++
		}
	}
	// One broken calendar should not mask the others. Tiimo currently returns
	// HTTP 500 (ArgumentNullException) for some linked calendars -- an upstream
	// server fault, not a client error -- so treat it as a partial result when
	// any other calendar was readable.
	if attempted > 0 && succeeded == 0 && firstErr != nil {
		return total, fmt.Errorf("every linked calendar failed: %w", firstErr)
	}
	if firstErr != nil {
		return total, fmt.Errorf("%d of %d linked calendar(s) failed (upstream): %w",
			attempted-succeeded, attempted, firstErr)
	}
	return total, nil
}

// syncTodoLists unwraps the {"lists": [...]} envelope.
func syncTodoLists(ctx context.Context, c apiGetter, st *store.Store, pid string) (int, error) {
	data, err := c.Get(ctx, "/api/profiles/"+pid+"/todo-task-lists", nil)
	if err != nil {
		return 0, fmt.Errorf("fetching to-do lists: %w", err)
	}
	var env struct {
		Lists []json.RawMessage `json:"lists"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return 0, fmt.Errorf("parsing to-do lists: %w", err)
	}
	n := 0
	for _, raw := range env.Lists {
		var probe struct {
			TodoTaskListID string `json:"todoTaskListId"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil || probe.TodoTaskListID == "" {
			continue
		}
		if err := st.Upsert("todo_lists", probe.TodoTaskListID, raw); err != nil {
			continue
		}
		n++
	}
	return n, nil
}

// syncFlatList handles the resources that really do return a flat array.
//
// idFields names the resource's own identifier so canonicalID can promote it
// to "id" before the store derives a row key from it.
func syncFlatList(ctx context.Context, c apiGetter, st *store.Store, path string, params map[string]string, idFields []string, upsert func(json.RawMessage) error) (int, error) {
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return 0, fmt.Errorf("fetching %s: %w", path, err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return 0, fmt.Errorf("parsing %s: %w", path, err)
	}
	n := 0
	failures := 0
	var firstErr error
	for _, raw := range items {
		keyed, err := canonicalID(raw, idFields...)
		if err == nil {
			err = upsert(keyed)
		}
		if err != nil {
			failures++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		n++
	}
	if n == 0 && failures > 0 {
		return 0, fmt.Errorf("all %d record(s) from %s failed to store: %w", failures, path, firstErr)
	}
	if failures > 0 {
		return n, fmt.Errorf("%d of %d record(s) from %s failed to store: %w", failures, failures+n, path, firstErr)
	}
	return n, nil
}

// apiGetter is the read surface these helpers need, kept narrow so they can be
// exercised without a live client.
type apiGetter interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

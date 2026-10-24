// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// Change kinds, in the feed's tie-break order.
var changeKinds = []string{"version", "title", "subtitle", "price", "iap", "privacy", "event", "editors_choice", "featured"}

const changesNotesLimit = 500

type changeRow struct {
	Date               string `json:"date"`
	PreviousCapturedAt string `json:"previousCapturedAt"`
	AppID              string `json:"appId"`
	Title              string `json:"title"`
	Storefront         string `json:"storefront"`
	Kind               string `json:"kind"`
	Change             string `json:"change"`
	Field              string `json:"field"`
	From               string `json:"from"`
	To                 string `json:"to"`
	Detail             string `json:"detail"`

	at time.Time
}

type changesMeta struct {
	Since           *string  `json:"since"`
	Window          string   `json:"window"`
	Apps            []string `json:"apps"`
	AppsWithoutData []string `json:"apps_without_data"`
	Kinds           []string `json:"kinds"`
	Storefronts     []string `json:"storefronts"`
	Comparisons     int      `json:"comparisons"`
	TotalChanges    int      `json:"total_changes"`
	Limit           int      `json:"limit"`
	Truncated       bool     `json:"truncated"`
	Note            string   `json:"note"`
}

type changesView struct {
	Meta    changesMeta `json:"meta"`
	Results []changeRow `json:"results"`
}

type changesOpts struct {
	appRefs []appstore.AppRef
	kinds   map[string]bool
	scope   s3bScope
	since   time.Time
	window  string
	limit   int
}

func newNovelChangesCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagApps, flagKind, flagCountries, flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "changes",
		Short: "List stored changes across tracked apps in one dated feed: versions, title/subtitle, prices, IAP prices, privacy, events, featuring",
		Long:  "Use this for a dated feed of every change across your tracked apps inside a window: new versions and their notes, title and subtitle edits, price and in-app purchase price changes, privacy-label changes, new in-app events, and featuring or Editor's Choice gained or lost. Do NOT use this for a field-by-field diff of one app's two latest snapshots; use 'watch-listing' instead, and to measure whether a change moved rank or ratings use 'impact'.",
		Example: strings.Trim(`
  app-store-pp-cli changes --since 7d --apps 553834731,850417475 --agent
  app-store-pp-cli changes --since 30d --kind version,iap,privacy --json
  app-store-pp-cli changes --since 2026-10-01 --countries us,gb`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--since=7d;--apps=324684580,553834731",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "changes")
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("changes takes no positional arguments (got %q); pass apps with --apps 553834731,850417475", strings.Join(args, " ")))
			}
			now := s3bNow()
			since, err := s3bParseSince("since", flagSince, now)
			if err != nil {
				return err
			}
			kinds, err := parseChangeKinds(flagKind)
			if err != nil {
				return err
			}
			if flagLimit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (no limit) or more, got %d", flagLimit))
			}
			var refs []appstore.AppRef
			if strings.TrimSpace(flagApps) != "" {
				parts := make([]string, 0)
				for _, p := range strings.Split(flagApps, ",") {
					if p = strings.TrimSpace(p); p != "" {
						parts = append(parts, p)
					}
				}
				if len(parts) == 0 {
					return usageErr(fmt.Errorf("--apps is empty; pass ids such as 553834731,850417475"))
				}
				if refs, err = s3bParseAppArgs(parts); err != nil {
					return err
				}
			}
			scope, err := s3bStorefrontScope(cmd, flags, flagCountries)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			opts := changesOpts{appRefs: refs, kinds: kinds, scope: scope, since: since, window: strings.TrimSpace(flagSince), limit: flagLimit}
			hint := "capture listings first with 'app-store-pp-cli app 553834731' (plus iap, privacy, events and featured), then again on a later day so there are consecutive snapshots to compare"
			st, ok, err := s3bOpenStore(cmd, flags, hint, func(note string) any {
				meta := newChangesMeta(opts)
				meta.Note = note
				return changesView{Meta: meta, Results: make([]changeRow, 0)}
			})
			if err != nil || !ok {
				return err
			}
			defer func() { _ = st.Close() }()

			view, err := buildChanges(ctx, st, opts)
			if err != nil {
				return s3bLocalReadErr(err)
			}
			if len(view.Meta.Apps) > 0 {
				s3bWarnStaleApps(ctx, cmd, flags, st, view.Meta.Apps, scope, "app-store-pp-cli app <app> for each app (plus iap, privacy and events)")
			}
			return renderChanges(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Window of changes to list (7d, 4w, a date such as 2026-10-01, or all)")
	cmd.Flags().StringVar(&flagApps, "apps", "", "Comma-separated apps to include (ids, id123, store URLs or bundle ids); default every app with stored listing history")
	cmd.Flags().StringVar(&flagKind, "kind", "", "Comma-separated change kinds: version,title,subtitle,price,iap,privacy,event,editors_choice,featured (default all)")
	cmd.Flags().StringVar(&flagCountries, "countries", "", "Storefronts to include (us,gb or all); default every storefront with stored data, or --country when set")
	cmd.Flags().IntVar(&flagLimit, "limit", 200, "Maximum changes to return, newest first (0 = no limit)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Local snapshot store path (default: the CLI data directory)")
	return cmd
}

func parseChangeKinds(raw string) (map[string]bool, error) {
	out := map[string]bool{}
	if strings.TrimSpace(raw) == "" {
		for _, k := range changeKinds {
			out[k] = true
		}
		return out, nil
	}
	valid := map[string]bool{}
	for _, k := range changeKinds {
		valid[k] = true
	}
	for _, p := range strings.Split(raw, ",") {
		k := strings.ToLower(strings.TrimSpace(p))
		k = strings.ReplaceAll(k, "-", "_")
		switch k {
		case "":
			continue
		case "events":
			k = "event"
		case "editorschoice", "editors_choice_badge":
			k = "editors_choice"
		case "versions":
			k = "version"
		}
		if !valid[k] {
			return nil, usageErr(fmt.Errorf("--kind %q: use any of %s", p, strings.Join(changeKinds, ",")))
		}
		out[k] = true
	}
	if len(out) == 0 {
		return nil, usageErr(fmt.Errorf("--kind is empty; use any of %s", strings.Join(changeKinds, ",")))
	}
	return out, nil
}

func newChangesMeta(opts changesOpts) changesMeta {
	kinds := make([]string, 0, len(opts.kinds))
	for _, k := range changeKinds {
		if opts.kinds[k] {
			kinds = append(kinds, k)
		}
	}
	apps := make([]string, 0, len(opts.appRefs))
	for _, r := range opts.appRefs {
		apps = append(apps, s3bRefLabel(r))
	}
	return changesMeta{
		Since:           s3bTimePtr(opts.since),
		Window:          opts.window,
		Apps:            apps,
		AppsWithoutData: make([]string, 0),
		Kinds:           kinds,
		Storefronts:     make([]string, 0),
		Limit:           opts.limit,
	}
}

// changesCollector gathers rows and counts the consecutive capture pairs
// compared inside the window.
type changesCollector struct {
	opts        changesOpts
	rows        []changeRow
	comparisons int
	storefronts map[string]bool
	// pairsSeen counts each (storefront, later capture) once: the Lookup-field
	// and page-field streams both walk a page+lookup capture.
	pairsSeen map[string]bool
	// releases names stored releases dated inside the window that no capture
	// pair showed as a version row ("<app> <version> (<date>)").
	releases []string
}

func (c *changesCollector) inWindow(t time.Time) bool {
	return c.opts.since.IsZero() || !t.Before(c.opts.since)
}

// compared records one consecutive pair whose later capture is in the window.
func (c *changesCollector) compared(sf string, later time.Time) bool {
	if !c.opts.scope.has(sf) || !c.inWindow(later) {
		return false
	}
	if k := sf + "|" + later.UTC().Format(time.RFC3339Nano); !c.pairsSeen[k] {
		if c.pairsSeen == nil {
			c.pairsSeen = map[string]bool{}
		}
		c.pairsSeen[k] = true
		c.comparisons++
	}
	c.storefronts[sf] = true
	return true
}

func (c *changesCollector) add(r changeRow) {
	if !c.opts.kinds[r.Kind] || !c.opts.scope.has(r.Storefront) || !c.inWindow(r.at) {
		return
	}
	r.Date = s3bFormat(r.at)
	c.rows = append(c.rows, r)
}

func buildChanges(ctx context.Context, st *store.Store, opts changesOpts) (changesView, error) {
	view := changesView{Meta: newChangesMeta(opts), Results: make([]changeRow, 0)}
	var ids []string
	if len(opts.appRefs) > 0 {
		resolved, missing, err := s3bResolveLocalApps(ctx, st, opts.appRefs)
		if err != nil {
			return view, err
		}
		ids = resolved
		view.Meta.AppsWithoutData = append(view.Meta.AppsWithoutData, missing...)
	} else {
		// Every app with any stored listing history, including apps captured
		// only through commands that write no listing snapshot (IAP lists,
		// privacy labels, events, version entries).
		all, err := store.AppsWithHistory(ctx, st, time.Time{})
		if err != nil {
			return view, err
		}
		ids = all
	}
	view.Meta.Apps = append(make([]string, 0, len(ids)), ids...)
	titles, err := s3bAppTitles(ctx, st)
	if err != nil {
		return view, err
	}
	var featured []s3bFeaturedCapture
	if opts.kinds["featured"] {
		rows, err := store.FeaturedSince(ctx, st, "", time.Time{})
		if err != nil {
			return view, err
		}
		featured = s3bFeaturedCaptures(rows)
	}

	c := &changesCollector{opts: opts, rows: make([]changeRow, 0), storefronts: map[string]bool{}}
	noData := 0 // requested apps that resolved but have no stored history
	for _, id := range ids {
		before := c.comparisons
		hadHistory, err := collectAppChanges(ctx, st, c, id, titles[id], featured)
		if err != nil {
			return view, err
		}
		if !hadHistory && c.comparisons == before && len(opts.appRefs) > 0 {
			view.Meta.AppsWithoutData = append(view.Meta.AppsWithoutData, id)
			noData++
		}
	}

	kindOrder := map[string]int{}
	for i, k := range changeKinds {
		kindOrder[k] = i
	}
	sort.SliceStable(c.rows, func(i, j int) bool {
		a, b := c.rows[i], c.rows[j]
		if !a.at.Equal(b.at) {
			return a.at.After(b.at)
		}
		if a.AppID != b.AppID {
			return a.AppID < b.AppID
		}
		if a.Storefront != b.Storefront {
			return a.Storefront < b.Storefront
		}
		if kindOrder[a.Kind] != kindOrder[b.Kind] {
			return kindOrder[a.Kind] < kindOrder[b.Kind]
		}
		return a.Field < b.Field
	})
	view.Meta.Comparisons = c.comparisons
	view.Meta.TotalChanges = len(c.rows)
	view.Meta.Storefronts = s3bSortedKeys(c.storefronts)
	view.Results = c.rows
	if opts.limit > 0 && len(view.Results) > opts.limit {
		view.Results = view.Results[:opts.limit]
		view.Meta.Truncated = true
	}
	window := "in the stored history"
	if !opts.since.IsZero() {
		window = "since " + s3bFormat(opts.since)
	}
	switch {
	case len(ids) == 0:
		view.Meta.Note = "No tracked apps in the local store yet; capture listings with 'app-store-pp-cli app 553834731' (and again on a later day) to build a change history."
	case len(ids) > 0 && noData == len(ids):
		view.Meta.Note = fmt.Sprintf("None of the %d requested app(s) has been captured yet (%s); capture each listing with 'app-store-pp-cli app <id>', and again on a later day, to build a change history.", len(view.Meta.AppsWithoutData), strings.Join(view.Meta.AppsWithoutData, ", "))
	case c.comparisons == 0:
		view.Meta.Note = fmt.Sprintf("No consecutive captures to compare %s: each of the %d captured app(s) has at most one capture per storefront there, so there is no capture pair to diff yet. Capture again later to build a change history.", window, len(ids)-noData)
		if len(view.Meta.AppsWithoutData) > 0 {
			view.Meta.Note += fmt.Sprintf(" Never captured: %s; capture each with 'app-store-pp-cli app <id>'.", strings.Join(view.Meta.AppsWithoutData, ", "))
		}
	case len(c.rows) == 0:
		view.Meta.Note = fmt.Sprintf("Compared %d consecutive capture pairs %s; nothing changed.", c.comparisons, window)
	default:
		view.Meta.Note = fmt.Sprintf("%d change(s) from %d consecutive capture pairs %s, newest first; each row is dated by the capture that first showed it (previousCapturedAt is the capture before).", len(c.rows), c.comparisons, window)
		if view.Meta.Truncated {
			view.Meta.Note += fmt.Sprintf(" Showing the newest %d; raise --limit to see all.", opts.limit)
		}
	}
	if len(c.releases) > 0 {
		named, more := c.releases, ""
		if len(named) > 5 {
			named, more = named[:5], fmt.Sprintf(" and %d more", len(c.releases)-5)
		}
		view.Meta.Note += fmt.Sprintf(" The stored version history dates %d release(s) %s that no capture pair showed: %s%s.", len(c.releases), window, strings.Join(named, ", "), more)
	}
	return view, nil
}

// collectAppChanges diffs one app's stored history. hadHistory reports
// whether the app has any stored listing, IAP, privacy or event rows.
func collectAppChanges(ctx context.Context, st *store.Store, c *changesCollector, appID, title string, featured []s3bFeaturedCapture) (bool, error) {
	// Capture pairs are counted per app: a batched Lookup gives several apps the
	// same capture time.
	c.pairsSeen = nil
	versions, snaps, err := s3bKnownVersions(ctx, st, appID)
	if err != nil {
		return false, err
	}
	hadHistory := len(snaps) > 0
	if title == "" {
		for i := len(snaps) - 1; i >= 0 && title == ""; i-- {
			title = snaps[i].Title
		}
	}
	notes := map[string]s3bVersion{}
	for _, v := range versions {
		notes[v.Version] = v
	}
	base := func(sf string, at, prev time.Time, kind, change, field, from, to, detail string) changeRow {
		return changeRow{PreviousCapturedAt: s3bFormat(prev), AppID: appID, Title: title, Storefront: sf, Kind: kind,
			Change: change, Field: field, From: from, To: to, Detail: detail, at: at}
	}

	// Streams whose kinds were not requested are skipped entirely, so
	// meta.comparisons only counts comparisons that could yield a row.
	wants := func(kinds ...string) bool {
		for _, k := range kinds {
			if c.opts.kinds[k] {
				return true
			}
		}
		return false
	}
	var common, pages []s3bSnapshotPair
	isPage := func(s store.AppSnapshot) bool { return s.Source == store.SourcePageLookup }
	if wants("version", "title", "price") {
		common = s3bConsecutivePairs(snaps, c.opts.scope, false, nil)
	}
	if wants("subtitle", "editors_choice") {
		pages = s3bConsecutivePairs(snaps, c.opts.scope, false, isPage)
	}

	// Fields every snapshot carries (from Lookup): version, title, price.
	// A Lookup CDN copy can lag the product page by hours, so consecutive
	// captures from the two sources can step back to an older version and
	// forward again; a version is reported only when it is newer than every
	// version already seen in that storefront.
	newest := map[string]string{}
	for _, p := range common {
		sf := p.Storefront
		if _, ok := newest[sf]; !ok {
			newest[sf] = p.Prev.Version
		}
		fromVersion, versionUp := "", false
		if p.Cur.Version != "" && s3aCompareVersions(p.Cur.Version, newest[sf]) > 0 {
			fromVersion, versionUp = newest[sf], newest[sf] != ""
			newest[sf] = p.Cur.Version
		}
		if !c.compared(sf, p.Cur.CapturedAt) {
			continue
		}
		at, prev := p.Cur.CapturedAt, p.Prev.CapturedAt
		if versionUp {
			c.add(base(sf, at, prev, "version", "changed", "version", fromVersion, p.Cur.Version, versionDetail(notes[p.Cur.Version], p.Cur)))
		}
		if p.Prev.Title != "" && p.Cur.Title != "" && p.Prev.Title != p.Cur.Title {
			c.add(base(p.Storefront, at, prev, "title", "changed", "title", p.Prev.Title, p.Cur.Title, ""))
		}
		if p.Prev.Price != p.Cur.Price || p.Prev.Currency != p.Cur.Currency {
			c.add(base(p.Storefront, at, prev, "price", "changed", "price", s3bPriceLabel(p.Prev), s3bPriceLabel(p.Cur), ""))
		}
	}
	// A release the stored version history dates inside the window, but that
	// no capture pair showed (one listing capture, or captures on either side
	// of several releases), is named in the note: rows stay dated by captures.
	if wants("version") {
		shown := map[string]bool{}
		for _, r := range c.rows {
			if r.AppID == appID && r.Kind == "version" {
				shown[r.To] = true
			}
		}
		for _, v := range versions {
			if v.ReleasedAt.IsZero() || !c.inWindow(v.ReleasedAt) || shown[v.Version] {
				continue
			}
			c.releases = append(c.releases, fmt.Sprintf("%s %s (%s)", appID, v.Version, v.ReleasedAt.UTC().Format("2006-01-02")))
		}
	}
	// Page-only fields: subtitle and Editor's Choice.
	for _, p := range pages {
		if !c.compared(p.Storefront, p.Cur.CapturedAt) {
			continue
		}
		at, prev := p.Cur.CapturedAt, p.Prev.CapturedAt
		if p.Prev.Subtitle != p.Cur.Subtitle {
			change := "changed"
			switch {
			case p.Prev.Subtitle == "":
				change = "added"
			case p.Cur.Subtitle == "":
				change = "removed"
			}
			c.add(base(p.Storefront, at, prev, "subtitle", change, "subtitle", p.Prev.Subtitle, p.Cur.Subtitle, ""))
		}
		if p.Prev.EditorsChoice != p.Cur.EditorsChoice {
			change := "lost"
			if p.Cur.EditorsChoice {
				change = "gained"
			}
			c.add(base(p.Storefront, at, prev, "editors_choice", change, "editors_choice", fmt.Sprint(p.Prev.EditorsChoice), fmt.Sprint(p.Cur.EditorsChoice), ""))
		}
	}

	// In-app purchase price lists (stored only when the list changed).
	iaps, err := store.IAPHistory(ctx, st, appID, "")
	if err != nil {
		return false, err
	}
	hadHistory = hadHistory || len(iaps) > 0
	if !wants("iap") {
		iaps = nil
	}
	for _, pair := range iapPairs(iaps) {
		prevList, curList := pair[0], pair[1]
		if !c.compared(curList.Storefront, curList.CapturedAt) {
			continue
		}
		for _, d := range diffIAPLists(prevList.Items, curList.Items) {
			c.add(base(curList.Storefront, curList.CapturedAt, prevList.CapturedAt, "iap", d.change, d.field, d.from, d.to, d.detail))
		}
	}

	// Privacy labels (stored only when the label changed).
	privacy, err := store.PrivacyHistory(ctx, st, appID, "")
	if err != nil {
		return false, err
	}
	hadHistory = hadHistory || len(privacy) > 0
	if !wants("privacy") {
		privacy = nil
	}
	bySF := map[string][]store.PrivacySnapshot{}
	for _, p := range privacy {
		bySF[p.Storefront] = append(bySF[p.Storefront], p)
	}
	for _, sf := range s3bSortedKeys(bySF) {
		list := bySF[sf]
		for i := 1; i < len(list); i++ {
			if !c.compared(sf, list[i].CapturedAt) {
				continue
			}
			for _, d := range diffPrivacy(list[i-1].Privacy, list[i].Privacy) {
				c.add(base(sf, list[i].CapturedAt, list[i-1].CapturedAt, "privacy", d.change, d.field, d.from, d.to, d.detail))
			}
		}
	}

	// New in-app events: first seen after the storefront's first capture.
	events, err := store.EventsHistory(ctx, st, appID, "")
	if err != nil {
		return false, err
	}
	hadHistory = hadHistory || len(events) > 0
	captureTimes := map[string][]time.Time{}
	baseline := map[string]time.Time{}
	for _, s := range snaps {
		if isPage(s) {
			captureTimes[s.Storefront] = append(captureTimes[s.Storefront], s.CapturedAt)
			baseline[s.Storefront] = s3bMinTime(baseline[s.Storefront], s.CapturedAt)
		}
	}
	for _, e := range events {
		captureTimes[e.Storefront] = append(captureTimes[e.Storefront], e.FirstSeenAt, e.LastSeenAt)
		baseline[e.Storefront] = s3bMinTime(baseline[e.Storefront], e.FirstSeenAt)
	}
	for _, e := range events {
		if !wants("event") || !e.FirstSeenAt.After(baseline[e.Storefront]) {
			continue
		}
		prev := latestBefore(captureTimes[e.Storefront], e.FirstSeenAt)
		if !c.compared(e.Storefront, e.FirstSeenAt) {
			continue
		}
		c.add(base(e.Storefront, e.FirstSeenAt, prev, "event", "new", e.Title, "", e.Title, eventDetail(e)))
	}

	// Featuring on the Today tab, gained or lost between consecutive captures.
	groups := map[string][]s3bFeaturedCapture{}
	for _, fc := range featured {
		k := fc.Storefront + "|" + fc.Platform
		groups[k] = append(groups[k], fc)
	}
	for _, k := range s3bSortedKeys(groups) {
		g := groups[k]
		appeared := false
		for _, fc := range g {
			if len(fc.Apps[appID]) > 0 {
				appeared = true
				break
			}
		}
		if !appeared {
			continue
		}
		for i := 1; i < len(g); i++ {
			prevStories, curStories := g[i-1].Apps[appID], g[i].Apps[appID]
			if !c.compared(g[i].Storefront, g[i].At) {
				continue
			}
			field := g[i].Platform
			if field == "" {
				field = "today"
			}
			switch {
			case len(curStories) > 0 && len(prevStories) == 0:
				c.add(base(g[i].Storefront, g[i].At, g[i-1].At, "featured", "gained", field, "not featured", "featured", "featured in: "+strings.Join(curStories, "; ")))
			case len(curStories) == 0 && len(prevStories) > 0:
				c.add(base(g[i].Storefront, g[i].At, g[i-1].At, "featured", "lost", field, "featured", "not featured", "no longer featured (was in: "+strings.Join(prevStories, "; ")+")"))
			}
		}
	}
	return hadHistory, nil
}

func versionDetail(v s3bVersion, cur store.AppSnapshot) string {
	notes := strings.TrimSpace(v.Notes)
	if notes == "" {
		notes = strings.TrimSpace(cur.Fields.ReleaseNotes)
	}
	released := v.ReleasedAt
	if released.IsZero() {
		released, _ = s3bParseStoredTime(cur.Fields.VersionReleasedAt)
	}
	parts := make([]string, 0, 2)
	if !released.IsZero() {
		parts = append(parts, "released "+s3bFormat(released))
	}
	if notes != "" {
		parts = append(parts, s3bTruncate(strings.Join(strings.Fields(notes), " "), changesNotesLimit))
	}
	return strings.Join(parts, ": ")
}

func eventDetail(e store.StoredEvent) string {
	parts := make([]string, 0, 3)
	if e.Kind != "" {
		parts = append(parts, e.Kind)
	}
	if e.StartDate != "" || e.EndDate != "" {
		parts = append(parts, strings.TrimSpace(e.StartDate+" to "+e.EndDate))
	}
	if e.Badge != "" {
		parts = append(parts, e.Badge)
	}
	return strings.Join(parts, "; ")
}

func latestBefore(times []time.Time, t time.Time) time.Time {
	var best time.Time
	for _, x := range times {
		if x.Before(t) && x.After(best) {
			best = x
		}
	}
	return best
}

// iapPairs returns consecutive IAP lists per storefront (time order).
func iapPairs(lists []store.IAPSnapshot) [][2]store.IAPSnapshot {
	bySF := map[string][]store.IAPSnapshot{}
	for _, l := range lists {
		bySF[l.Storefront] = append(bySF[l.Storefront], l)
	}
	out := make([][2]store.IAPSnapshot, 0)
	for _, sf := range s3bSortedKeys(bySF) {
		g := bySF[sf]
		sort.SliceStable(g, func(i, j int) bool { return g[i].CapturedAt.Before(g[j].CapturedAt) })
		for i := 1; i < len(g); i++ {
			out = append(out, [2]store.IAPSnapshot{g[i-1], g[i]})
		}
	}
	return out
}

type fieldDiff struct {
	change, field, from, to, detail string
}

// diffIAPLists compares two top-IAP lists by name: price changes, additions
// and removals; a list whose only change is order is reported as reordered.
func diffIAPLists(prev, cur []appstore.IAPItem) []fieldDiff {
	key := func(items []appstore.IAPItem) ([]string, map[string]string) {
		order := make([]string, 0, len(items))
		prices := map[string]string{}
		seen := map[string]int{}
		for _, it := range items {
			name := strings.TrimSpace(it.Name)
			seen[name]++
			if seen[name] > 1 {
				name = fmt.Sprintf("%s #%d", name, seen[name])
			}
			order = append(order, name)
			prices[name] = it.Price
		}
		return order, prices
	}
	prevOrder, prevPrices := key(prev)
	curOrder, curPrices := key(cur)
	out := make([]fieldDiff, 0)
	for _, name := range curOrder {
		old, ok := prevPrices[name]
		switch {
		case !ok:
			out = append(out, fieldDiff{change: "added", field: name, to: curPrices[name]})
		case old != curPrices[name]:
			out = append(out, fieldDiff{change: "changed", field: name, from: old, to: curPrices[name]})
		}
	}
	for _, name := range prevOrder {
		if _, ok := curPrices[name]; !ok {
			out = append(out, fieldDiff{change: "removed", field: name, from: prevPrices[name]})
		}
	}
	if len(out) == 0 && strings.Join(prevOrder, "\x00") != strings.Join(curOrder, "\x00") {
		out = append(out, fieldDiff{change: "reordered", field: "order", from: strings.Join(prevOrder, " > "), to: strings.Join(curOrder, " > "), detail: "same items and prices in a new order"})
	}
	return out
}

// privacyShape maps a privacy label to type -> category -> "title|data types"
// (categories merged across the type's purposes, data types sorted).
func privacyShape(p appstore.Privacy) (map[string]string, map[string]map[string]string) {
	titles := map[string]string{}
	shape := map[string]map[string]string{}
	for _, t := range p.Types {
		id := t.Identifier
		if id == "" {
			id = t.Title
		}
		titles[id] = t.Title
		labels := map[string]string{}
		types := map[string]map[string]bool{}
		addCats := func(list []appstore.PrivacyCategory) {
			for _, c := range list {
				cid := c.Identifier
				if cid == "" {
					cid = c.Title
				}
				if labels[cid] == "" {
					labels[cid] = c.Title
					if labels[cid] == "" {
						labels[cid] = cid
					}
				}
				if types[cid] == nil {
					types[cid] = map[string]bool{}
				}
				for _, dt := range c.DataTypes {
					types[cid][dt] = true
				}
			}
		}
		addCats(t.Categories)
		for _, pu := range t.Purposes {
			addCats(pu.Categories)
		}
		cats := map[string]string{}
		for cid, label := range labels {
			cats[cid] = label + "|" + strings.Join(s3bSortedKeys(types[cid]), ",")
		}
		shape[id] = cats
	}
	return titles, shape
}

// diffPrivacy compares two privacy labels type by type.
func diffPrivacy(prev, cur appstore.Privacy) []fieldDiff {
	prevTitles, prevShape := privacyShape(prev)
	curTitles, curShape := privacyShape(cur)
	catLabels := func(cats map[string]string) string {
		labels := make([]string, 0, len(cats))
		for _, cid := range s3bSortedKeys(cats) {
			labels = append(labels, strings.SplitN(cats[cid], "|", 2)[0])
		}
		return strings.Join(labels, ", ")
	}
	out := make([]fieldDiff, 0)
	for _, id := range s3bSortedKeys(curShape) {
		title := curTitles[id]
		if title == "" {
			title = id
		}
		old, ok := prevShape[id]
		if !ok {
			out = append(out, fieldDiff{change: "added", field: title, to: catLabels(curShape[id]), detail: "new privacy label type"})
			continue
		}
		added, removed, retyped := make([]string, 0), make([]string, 0), make([]string, 0)
		for _, cid := range s3bSortedKeys(curShape[id]) {
			label := strings.SplitN(curShape[id][cid], "|", 2)[0]
			if prevVal, ok := old[cid]; !ok {
				added = append(added, "+"+label)
			} else if prevVal != curShape[id][cid] {
				retyped = append(retyped, label)
			}
		}
		for _, cid := range s3bSortedKeys(old) {
			if _, ok := curShape[id][cid]; !ok {
				removed = append(removed, "-"+strings.SplitN(old[cid], "|", 2)[0])
			}
		}
		if len(added)+len(removed)+len(retyped) == 0 {
			continue
		}
		parts := append(append([]string{}, added...), removed...)
		if len(retyped) > 0 {
			parts = append(parts, "data types changed in "+strings.Join(retyped, ", "))
		}
		out = append(out, fieldDiff{change: "changed", field: title, from: catLabels(old), to: catLabels(curShape[id]), detail: strings.Join(parts, "; ")})
	}
	for _, id := range s3bSortedKeys(prevShape) {
		if _, ok := curShape[id]; ok {
			continue
		}
		title := prevTitles[id]
		if title == "" {
			title = id
		}
		out = append(out, fieldDiff{change: "removed", field: title, from: catLabels(prevShape[id]), detail: "privacy label type removed"})
	}
	return out
}

func renderChanges(cmd *cobra.Command, flags *rootFlags, view changesView) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, view, flags)
	}
	if len(view.Results) == 0 {
		_, _ = fmt.Fprintln(w, view.Meta.Note)
		return nil
	}
	rows := make([][]string, 0, len(view.Results))
	for _, r := range view.Results {
		what := r.From + " -> " + r.To
		switch {
		case r.From == "" && r.To != "":
			what = r.To
		case r.To == "" && r.From != "":
			what = r.From
		}
		rows = append(rows, []string{r.Date, r.AppID, s3bTruncate(r.Title, 28), r.Storefront, r.Kind, r.Change, s3bTruncate(r.Field, 30), s3bTruncate(what, 60)})
	}
	if err := s3bPrintTable(w, []string{"date", "app", "title", "storefront", "kind", "change", "field", "value"}, rows); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(w, "\n"+view.Meta.Note)
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelChangesCmd(flags))
	})
}

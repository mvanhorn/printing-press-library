// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

// watchListingRef identifies one compared snapshot.
type watchListingRef struct {
	CapturedAt  time.Time `json:"capturedAt"`
	Version     string    `json:"version,omitempty"`
	ContentHash string    `json:"contentHash"`
}

// watchListingChange is one listing field that differs between the two
// snapshots; Detail breaks list changes down (added, removed, repriced).
type watchListingChange struct {
	Field  string `json:"field"`
	From   any    `json:"from"`
	To     any    `json:"to"`
	Detail any    `json:"detail,omitempty"`
}

type watchListingView struct {
	AppID      string               `json:"appId"`
	Title      string               `json:"title,omitempty"`
	Storefront string               `json:"storefront"`
	Source     string               `json:"source,omitempty"`
	Snapshots  int                  `json:"snapshots"`
	Previous   *watchListingRef     `json:"previous"`
	Latest     *watchListingRef     `json:"latest"`
	Changed    bool                 `json:"changed"`
	Changes    []watchListingChange `json:"changes"`
	Note       string               `json:"note,omitempty"`
}

// watchListingRepriced is one in-app purchase whose price changed.
type watchListingRepriced struct {
	Name string `json:"name"`
	From string `json:"from"`
	To   string `json:"to"`
}

// watchListingListDetail lists the items added to and removed from a list.
type watchListingListDetail struct {
	Added    any                    `json:"added"`
	Removed  any                    `json:"removed"`
	Repriced []watchListingRepriced `json:"repriced,omitempty"`
	Note     string                 `json:"note,omitempty"`
}

// watchListingEvent is an in-app event named in an events change.
type watchListingEvent struct {
	EventID   string `json:"eventId"`
	Title     string `json:"title,omitempty"`
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newWatchListingCmd(flags))
	})
}

func newWatchListingCmd(flags *rootFlags) *cobra.Command {
	var source string
	cmd := &cobra.Command{
		Use:   "watch-listing <app>",
		Short: "Diff the two latest stored snapshots of an app listing",
		Long: strings.TrimSpace(`
Field-by-field diff of the two most recent stored snapshots of one app's
listing in one storefront (--country, or the storefront of a store URL):
title, subtitle, version (with its release date and notes), price, Editor's
Choice, the in-app purchase price list (added, removed, repriced), the
privacy label, the screenshot set, in-app events, description, seller,
genre, age rating and minimum OS. Rating counts and chart badges are not
listing changes and are left out.

Snapshots are compared like with like: 'app' writes full page+lookup
snapshots, 'prices' and 'compare' write lookup-only ones (no subtitle, IAP,
privacy or events). The full pair is preferred; --source pins one. When the
two snapshots have the same content hash the result has no changes and a
note says so; with fewer than two snapshots it explains what to run.

Use this for one app's latest listing diff. For a dated change feed across
many apps use 'changes'; to measure whether a change moved rank or ratings
use 'impact'.`),
		Example: strings.Trim(`
  app-store-pp-cli watch-listing 324684580 --country us --agent
  app-store-pp-cli watch-listing 553834731 --country gb --json --select changes
  app-store-pp-cli watch-listing https://apps.apple.com/jp/app/id324684580 --source lookup`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "app=324684580",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch-listing")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<app> is required: a numeric App Store id such as 324684580 (or id123, a store URL or a stored bundle id)"))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("watch-listing takes one app, got %d arguments", len(args)))
			}
			ref, appID, cc, err := s3aLocalApp(cmd, flags, args[0])
			if err != nil {
				return err
			}
			src, err := s3aSnapshotSource(source)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := watchListingView{AppID: appID, Storefront: cc, Changes: make([]watchListingChange, 0)}
			hint := fmt.Sprintf("%s app %s --country %s   (twice, at different times)", appStoreCLIName, appID, cc)
			if s3aMissingMirror(cmd, hint) {
				view.Note = "no local listing snapshots yet; run: " + hint
				return emitWatchListing(cmd, flags, view)
			}
			st, err := openAppStoreStoreRead(cmd, flags)
			if err != nil {
				return err
			}
			defer s3aCloseStore(st)
			if appID, view.Note, err = s3aResolveLocalApp(ctx, st, ref); err != nil || appID == "" {
				if err != nil {
					return err
				}
				return emitWatchListing(cmd, flags, view)
			}
			view.AppID = appID

			snaps, err := store.AppSnapshots(ctx, st, appID, cc, time.Time{})
			if err != nil {
				return apiErr(fmt.Errorf("reading app snapshots: %w", err))
			}
			if len(snaps) > 0 {
				warnStale(cmd, flags, snaps[len(snaps)-1].CapturedAt, time.Now(), "listing snapshot of app "+appID, fmt.Sprintf("%s app %s --country %s", appStoreCLIName, appID, cc))
			}
			if len(snaps) == 0 {
				view.Note = fmt.Sprintf("no stored snapshots of app %s in %s; run: %s", appID, cc, hint)
				if others, err := s3aOtherStorefronts(ctx, st, appID, cc); err == nil && others != "" {
					view.Note += "; snapshots exist in " + others + " (pass --country)"
				}
				return emitWatchListing(cmd, flags, view)
			}
			view.Title = snaps[len(snaps)-1].Title
			group, chosen, pickNote := s3aPickSnapshotPair(snaps, src)
			view.Source, view.Snapshots = chosen, len(group)
			if len(group) == 0 {
				view.Note = fmt.Sprintf("no %s snapshots of app %s in %s (other sources exist; drop --source)", src, appID, cc)
				return emitWatchListing(cmd, flags, view)
			}
			latest := group[len(group)-1]
			view.Latest = &watchListingRef{CapturedAt: latest.CapturedAt.UTC(), Version: latest.Version, ContentHash: latest.ContentHash}
			if len(group) < 2 {
				view.Note = fmt.Sprintf("only one %s snapshot of app %s in %s (%s); watch-listing needs two; run: %s",
					chosen, appID, cc, latest.CapturedAt.UTC().Format(time.RFC3339), hint)
				if pickNote != "" {
					view.Note += "; " + pickNote
				}
				return emitWatchListing(cmd, flags, view)
			}
			prev := group[len(group)-2]
			view.Previous = &watchListingRef{CapturedAt: prev.CapturedAt.UTC(), Version: prev.Version, ContentHash: prev.ContentHash}
			events := map[string]store.StoredEvent{}
			if prev.EventsHash != latest.EventsHash {
				stored, err := store.EventsHistory(ctx, st, appID, cc)
				if err != nil {
					return apiErr(fmt.Errorf("reading app events: %w", err))
				}
				for _, e := range stored {
					events[e.EventID] = e
				}
			}
			view.Changes = s3aDiffListing(prev, latest, events)
			view.Changed = len(view.Changes) > 0
			switch {
			case prev.ContentHash != "" && prev.ContentHash == latest.ContentHash:
				view.Note = fmt.Sprintf("no listing change between %s and %s (same content hash)", prev.CapturedAt.UTC().Format(time.RFC3339), latest.CapturedAt.UTC().Format(time.RFC3339))
			case !view.Changed:
				view.Note = "the content hash changed but no compared field differs (the snapshot format may have changed)"
			}
			if pickNote != "" {
				view.Note = strings.TrimPrefix(view.Note+"; "+pickNote, "; ")
			}
			return emitWatchListing(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Compare only snapshots of this source: page (full listing from 'app') or lookup (from 'prices'/'compare'); default: the full pair when stored")
	cmd.Flags().String("db", "", "Local snapshot database path (default: the CLI data directory)")
	return cmd
}

// s3aSnapshotSource maps --source for listing snapshots.
func s3aSnapshotSource(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "any", "auto":
		return "", nil
	case "page", "page+lookup", "full":
		return store.SourcePageLookup, nil
	case "lookup":
		return store.SourceLookup, nil
	}
	return "", usageErr(fmt.Errorf("--source: unknown snapshot source %q; use page (full listing) or lookup", s))
}

// s3aPickSnapshotPair chooses which snapshots to compare (oldest first):
// the pinned source, else the full page+lookup series when it has two
// snapshots, else the lookup series, else the latest snapshot's source.
// The note explains a fallback to lookup-only snapshots.
func s3aPickSnapshotPair(snaps []store.AppSnapshot, pinned string) ([]store.AppSnapshot, string, string) {
	bySource := map[string][]store.AppSnapshot{}
	for _, s := range snaps {
		bySource[s.Source] = append(bySource[s.Source], s)
	}
	if pinned != "" {
		return bySource[pinned], pinned, ""
	}
	if len(bySource[store.SourcePageLookup]) >= 2 {
		return bySource[store.SourcePageLookup], store.SourcePageLookup, ""
	}
	if len(bySource[store.SourceLookup]) >= 2 {
		return bySource[store.SourceLookup], store.SourceLookup,
			"compared lookup-only snapshots (no subtitle, IAP, privacy or events data); run 'app' twice for a full listing diff"
	}
	latest := snaps[len(snaps)-1].Source
	return bySource[latest], latest, ""
}

// s3aOtherStorefronts lists the other storefronts holding snapshots of the
// app, e.g. "gb (2), jp (1)".
func s3aOtherStorefronts(ctx context.Context, st *store.Store, appID, exclude string) (string, error) {
	tracked, err := store.TrackedApps(ctx, st, time.Time{})
	if err != nil {
		return "", err
	}
	parts := []string{}
	for _, t := range tracked {
		if t.AppID == appID && t.Storefront != exclude {
			parts = append(parts, fmt.Sprintf("%s (%d)", t.Storefront, t.Snapshots))
		}
	}
	return strings.Join(parts, ", "), nil
}

// s3aDiffListing compares two snapshots of the same source. Equal content
// hashes mean no listing change, so nothing is compared.
func s3aDiffListing(prev, latest store.AppSnapshot, events map[string]store.StoredEvent) []watchListingChange {
	changes := make([]watchListingChange, 0)
	if prev.ContentHash != "" && prev.ContentHash == latest.ContentHash {
		return changes
	}
	pf, lf := prev.Fields, latest.Fields
	add := func(field string, from, to, detail any) {
		changes = append(changes, watchListingChange{Field: field, From: from, To: to, Detail: detail})
	}
	if prev.Title != latest.Title {
		add("title", prev.Title, latest.Title, nil)
	}
	if prev.Subtitle != latest.Subtitle {
		add("subtitle", prev.Subtitle, latest.Subtitle, nil)
	}
	if prev.Version != latest.Version {
		add("version", prev.Version, latest.Version, map[string]any{
			"previousReleasedAt": pf.VersionReleasedAt,
			"releasedAt":         lf.VersionReleasedAt,
			"releaseNotes":       lf.ReleaseNotes,
		})
	} else if pf.ReleaseNotes != lf.ReleaseNotes {
		add("releaseNotes", pf.ReleaseNotes, lf.ReleaseNotes, nil)
	}
	if prev.Price != latest.Price || prev.Currency != latest.Currency {
		add("price", prev.Price, latest.Price, map[string]any{
			"fromFormatted": prev.FormattedPrice, "toFormatted": latest.FormattedPrice,
			"fromCurrency": prev.Currency, "toCurrency": latest.Currency,
		})
	}
	if prev.EditorsChoice != latest.EditorsChoice {
		add("editorsChoice", prev.EditorsChoice, latest.EditorsChoice, nil)
	}
	if prev.HasIAP != latest.HasIAP {
		add("hasInAppPurchases", prev.HasIAP, latest.HasIAP, nil)
	}
	if prev.IAPHash != latest.IAPHash {
		add("inAppPurchases", s3aIAPList(pf.InAppPurchases), s3aIAPList(lf.InAppPurchases), s3aIAPDetail(pf.InAppPurchases, lf.InAppPurchases))
	}
	if prev.PrivacyHash != latest.PrivacyHash {
		added, removed := s3aSetDiff(pf.PrivacyTypes, lf.PrivacyTypes)
		d := watchListingListDetail{Added: added, Removed: removed}
		if len(added) == 0 && len(removed) == 0 {
			d.Note = "the same label types changed their data categories or data types; see 'privacy' for the full label"
		}
		add("privacy", s3aStrList(pf.PrivacyTypes), s3aStrList(lf.PrivacyTypes), d)
	}
	if prev.ScreenshotsHash != latest.ScreenshotsHash {
		add("screenshots", pf.ScreenshotCount, lf.ScreenshotCount, map[string]any{
			"note": "the screenshot set changed (values are screenshot counts)",
		})
	}
	if prev.EventsHash != latest.EventsHash {
		addedIDs, removedIDs := s3aSetDiff(pf.EventIDs, lf.EventIDs)
		named := func(ids []string) []watchListingEvent {
			out := make([]watchListingEvent, 0, len(ids))
			for _, id := range ids {
				e := events[id]
				out = append(out, watchListingEvent{EventID: id, Title: e.Title, StartDate: e.StartDate, EndDate: e.EndDate})
			}
			return out
		}
		d := watchListingListDetail{Added: named(addedIDs), Removed: named(removedIDs)}
		if len(addedIDs) == 0 && len(removedIDs) == 0 {
			d.Note = "the same events changed their title or dates"
		}
		add("events", s3aStrList(pf.EventIDs), s3aStrList(lf.EventIDs), d)
	}
	if pf.DescriptionHash != lf.DescriptionHash {
		add("description", pf.DescriptionHash, lf.DescriptionHash, map[string]any{
			"note": "the description text changed (values are text hashes)",
		})
	}
	for _, f := range []struct{ name, from, to string }{
		{"sellerName", pf.SellerName, lf.SellerName},
		{"primaryGenre", strings.TrimSpace(pf.PrimaryGenreID + " " + pf.PrimaryGenre), strings.TrimSpace(lf.PrimaryGenreID + " " + lf.PrimaryGenre)},
		{"contentRating", pf.ContentRating, lf.ContentRating},
		{"minimumOsVersion", pf.MinimumOSVersion, lf.MinimumOSVersion},
	} {
		if f.from != f.to {
			add(f.name, f.from, f.to, nil)
		}
	}
	return changes
}

// s3aIAPList renders an IAP list as "name: price" strings (empty list, not null).
func s3aIAPList(items []appstore.IAPItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Name+": "+it.Price)
	}
	return out
}

// s3aIAPDetail matches IAP items by name: added, removed and repriced.
func s3aIAPDetail(before, after []appstore.IAPItem) watchListingListDetail {
	was := map[string]string{}
	for _, it := range before {
		was[it.Name] = it.Price
	}
	now := map[string]string{}
	for _, it := range after {
		now[it.Name] = it.Price
	}
	added, removed := make([]appstore.IAPItem, 0), make([]appstore.IAPItem, 0)
	repriced := make([]watchListingRepriced, 0)
	for _, it := range after {
		price, ok := was[it.Name]
		switch {
		case !ok:
			added = append(added, it)
		case price != it.Price:
			repriced = append(repriced, watchListingRepriced{Name: it.Name, From: price, To: it.Price})
		}
	}
	for _, it := range before {
		if _, ok := now[it.Name]; !ok {
			removed = append(removed, it)
		}
	}
	d := watchListingListDetail{Added: added, Removed: removed, Repriced: repriced}
	if len(added) == 0 && len(removed) == 0 && len(repriced) == 0 {
		d.Note = "same items and prices in a different order"
	}
	return d
}

// s3aSetDiff returns the items only in b (added) and only in a (removed), sorted.
func s3aSetDiff(a, b []string) ([]string, []string) {
	inA, inB := map[string]bool{}, map[string]bool{}
	for _, x := range a {
		inA[x] = true
	}
	for _, x := range b {
		inB[x] = true
	}
	added, removed := make([]string, 0), make([]string, 0)
	for x := range inB {
		if !inA[x] {
			added = append(added, x)
		}
	}
	for x := range inA {
		if !inB[x] {
			removed = append(removed, x)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// s3aStrList returns a non-nil copy so an empty list marshals as [].
func s3aStrList(in []string) []string {
	return append(make([]string, 0, len(in)), in...)
}

func emitWatchListing(cmd *cobra.Command, flags *rootFlags, v watchListingView) error {
	return s3aEmit(cmd, flags, v, func(w *bytes.Buffer) {
		name := v.AppID
		if v.Title != "" {
			name += " " + s3aCell(v.Title, 60)
		}
		fmt.Fprintf(w, "%s in %s", name, v.Storefront)
		if v.Source != "" {
			fmt.Fprintf(w, " (%s snapshots: %d)", v.Source, v.Snapshots)
		}
		fmt.Fprintln(w)
		if v.Previous != nil && v.Latest != nil {
			fmt.Fprintf(w, "%s -> %s\n", s3aTime(v.Previous.CapturedAt), s3aTime(v.Latest.CapturedAt))
		}
		for _, c := range v.Changes {
			fmt.Fprintf(w, "  %s: %s -> %s\n", c.Field, s3aChangeText(c.From), s3aChangeText(c.To))
		}
		if v.Note != "" {
			fmt.Fprintln(w, v.Note)
		}
	})
}

// s3aChangeText renders a change value compactly for human output.
func s3aChangeText(v any) string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return `""`
		}
		return s3aCell(x, 70)
	case []string:
		if len(x) == 0 {
			return "(none)"
		}
		return s3aCell(strings.Join(x, ", "), 90)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return s3aCell(string(b), 90)
}

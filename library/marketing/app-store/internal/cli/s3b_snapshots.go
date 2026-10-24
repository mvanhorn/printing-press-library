// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Listing-snapshot helpers shared by impact (event resolution) and changes
// (the change feed): known versions with release dates, consecutive
// snapshot pairs and the listing fields that differ between two snapshots.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"
)

// s3bVersion is one known version of an app with its release time (zero
// when no release date is stored) and notes.
type s3bVersion struct {
	Version    string
	ReleasedAt time.Time
	Notes      string
	FirstSeen  time.Time
}

// s3bKnownVersions merges the stored version history (version_entries) with
// the versions listing snapshots carried (Lookup's currentVersionReleaseDate)
// and returns them newest release first (undated versions last), together
// with the app's snapshots (oldest first) for callers that need them.
func s3bKnownVersions(ctx context.Context, st *store.Store, appID string) ([]s3bVersion, []store.AppSnapshot, error) {
	entries, err := store.VersionEntries(ctx, st, appID)
	if err != nil {
		return nil, nil, err
	}
	snaps, err := store.AppSnapshots(ctx, st, appID, "", time.Time{})
	if err != nil {
		return nil, nil, err
	}
	byVersion := map[string]*s3bVersion{}
	order := make([]string, 0)
	add := func(v s3bVersion) *s3bVersion {
		if cur, ok := byVersion[v.Version]; ok {
			return cur
		}
		c := v
		byVersion[v.Version] = &c
		order = append(order, v.Version)
		return &c
	}
	for _, e := range entries {
		v := strings.TrimSpace(e.Version)
		if v == "" {
			continue
		}
		t, _ := s3bParseStoredTime(e.ReleasedAt)
		add(s3bVersion{Version: v, ReleasedAt: t, Notes: e.Notes, FirstSeen: e.FirstSeenAt})
	}
	for _, s := range snaps {
		v := strings.TrimSpace(s.Version)
		if v == "" {
			continue
		}
		t, _ := s3bParseStoredTime(s.Fields.VersionReleasedAt)
		cur := add(s3bVersion{Version: v, ReleasedAt: t, Notes: s.Fields.ReleaseNotes, FirstSeen: s.CapturedAt})
		if cur.ReleasedAt.IsZero() && !t.IsZero() {
			cur.ReleasedAt = t
		}
		if cur.Notes == "" {
			cur.Notes = s.Fields.ReleaseNotes
		}
		if cur.FirstSeen.IsZero() || (!s.CapturedAt.IsZero() && s.CapturedAt.Before(cur.FirstSeen)) {
			cur.FirstSeen = s.CapturedAt
		}
	}
	out := make([]s3bVersion, 0, len(order))
	for _, v := range order {
		out = append(out, *byVersion[v])
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].ReleasedAt, out[j].ReleasedAt
		switch {
		case a.IsZero() != b.IsZero():
			return !a.IsZero()
		case !a.Equal(b):
			return a.After(b)
		}
		return out[i].Version > out[j].Version
	})
	return out, snaps, nil
}

// s3bSnapshotPair is two consecutive snapshots of one app in one storefront.
type s3bSnapshotPair struct {
	Storefront string
	Source     string
	Prev, Cur  store.AppSnapshot
}

// s3bConsecutivePairs groups snapshots (oldest first) by storefront and, when
// bySource is set, by source too, and returns each group's consecutive pairs
// in time order (storefront, then capture time).
func s3bConsecutivePairs(snaps []store.AppSnapshot, scope s3bScope, bySource bool, keep func(store.AppSnapshot) bool) []s3bSnapshotPair {
	type groupKey struct{ sf, source string }
	groups := map[groupKey][]store.AppSnapshot{}
	keys := make([]groupKey, 0)
	for _, s := range snaps {
		if !scope.has(s.Storefront) || (keep != nil && !keep(s)) {
			continue
		}
		k := groupKey{sf: s.Storefront}
		if bySource {
			k.source = s.Source
		}
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], s)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].sf != keys[j].sf {
			return keys[i].sf < keys[j].sf
		}
		return keys[i].source < keys[j].source
	})
	out := make([]s3bSnapshotPair, 0)
	for _, k := range keys {
		g := groups[k]
		sort.SliceStable(g, func(i, j int) bool {
			if !g[i].CapturedAt.Equal(g[j].CapturedAt) {
				return g[i].CapturedAt.Before(g[j].CapturedAt)
			}
			return g[i].ID < g[j].ID
		})
		for i := 1; i < len(g); i++ {
			out = append(out, s3bSnapshotPair{Storefront: k.sf, Source: k.source, Prev: g[i-1], Cur: g[i]})
		}
	}
	return out
}

// s3bListingChange is a content-hash change between consecutive snapshots of
// the same source in one storefront.
type s3bListingChange struct {
	s3bSnapshotPair
	Fields []string
}

// s3bListingChanges returns every listing content change, oldest first.
// Snapshots are compared within one source, because a lookup-only snapshot
// carries no subtitle, privacy, IAP or event data.
func s3bListingChanges(snaps []store.AppSnapshot, scope s3bScope) []s3bListingChange {
	out := make([]s3bListingChange, 0)
	for _, p := range s3bConsecutivePairs(snaps, scope, true, nil) {
		if p.Prev.ContentHash == "" || p.Cur.ContentHash == "" || p.Prev.ContentHash == p.Cur.ContentHash {
			continue
		}
		out = append(out, s3bListingChange{s3bSnapshotPair: p, Fields: s3bChangedFields(p.Prev, p.Cur)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Cur.CapturedAt.Before(out[j].Cur.CapturedAt) })
	return out
}

// s3bChangedFields lists the listing fields (the ones the content hash
// covers) that differ between two snapshots.
func s3bChangedFields(a, b store.AppSnapshot) []string {
	fa, fb := a.Fields, b.Fields
	checks := []struct {
		name    string
		changed bool
	}{
		{"title", a.Title != b.Title},
		{"subtitle", a.Subtitle != b.Subtitle},
		{"version", a.Version != b.Version},
		{"release_notes", fa.ReleaseNotes != fb.ReleaseNotes},
		{"price", a.Price != b.Price || a.Currency != b.Currency},
		{"description", fa.DescriptionHash != fb.DescriptionHash},
		{"editors_choice", a.EditorsChoice != b.EditorsChoice},
		{"seller", fa.SellerName != fb.SellerName},
		{"genre", fa.PrimaryGenreID != fb.PrimaryGenreID},
		{"content_rating", fa.ContentRating != fb.ContentRating},
		{"minimum_os", fa.MinimumOSVersion != fb.MinimumOSVersion},
		{"screenshots", a.ScreenshotsHash != b.ScreenshotsHash},
		{"privacy", a.PrivacyHash != b.PrivacyHash},
		{"iap", a.IAPHash != b.IAPHash},
		{"events", a.EventsHash != b.EventsHash},
	}
	out := make([]string, 0)
	for _, c := range checks {
		if c.changed {
			out = append(out, c.name)
		}
	}
	return out
}

// s3bPriceLabel renders a snapshot's price for a change row.
func s3bPriceLabel(s store.AppSnapshot) string {
	if strings.TrimSpace(s.FormattedPrice) != "" {
		return s.FormattedPrice
	}
	if s.Price == 0 {
		return "Free"
	}
	return strings.TrimSpace(fmt.Sprintf("%.2f %s", s.Price, s.Currency))
}

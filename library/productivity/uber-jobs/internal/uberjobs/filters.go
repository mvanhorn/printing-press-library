// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Filters are the posting filters shared by postings, save, new, and screen.
// Server-side keys go to the site; the rest apply client-side over the full
// read, after HTML stripping.
type Filters struct {
	Query               string   `json:"query,omitempty"`
	Countries           []string `json:"countries,omitempty"` // ISO3
	Team                string   `json:"team,omitempty"`
	SubTeam             string   `json:"sub_team,omitempty"`
	ContractType        string   `json:"contract_type,omitempty"`
	WorkPattern         string   `json:"work_pattern,omitempty"`
	PostedWithin        string   `json:"posted_within,omitempty"`
	DescriptionContains []string `json:"description_contains,omitempty"`
	DescriptionExcludes []string `json:"description_not_contains,omitempty"`
	Sort                string   `json:"sort,omitempty"`
}

// ServerQuery returns the site query for the server-side part of f.
func (f Filters) ServerQuery() (Query, []string, error) {
	q := Query{Search: f.Query, Team: f.Team, SubTeam: f.SubTeam, ContractType: f.ContractType}
	var unmapped []string
	for _, c := range f.Countries {
		_, name, ok := ResolveCountry(c)
		if !ok {
			unmapped = append(unmapped, c)
			continue
		}
		q.Countries = append(q.Countries, name)
	}
	if len(unmapped) > 0 {
		return q, unmapped, fmt.Errorf("unknown country %s: use an ISO3 code such as GBR, USA, DEU", strings.Join(unmapped, ", "))
	}
	return q, nil, nil
}

// MatchClient applies the client-side filters (work pattern, posted-within,
// description phrases) to a normalized posting.
func (f Filters) MatchClient(p Posting, window time.Duration, now time.Time) bool {
	if f.WorkPattern != "" && (p.WorkPattern == nil || !strings.EqualFold(*p.WorkPattern, f.WorkPattern)) {
		return false
	}
	if window > 0 && !WithinWindow(p, window, now) {
		return false
	}
	desc := ""
	if p.Description != nil {
		desc = strings.ToLower(*p.Description)
	}
	// Blank phrases are skipped: they would match every description.
	for _, phrase := range f.DescriptionContains {
		if ph := strings.ToLower(strings.TrimSpace(phrase)); ph != "" && !strings.Contains(desc, ph) {
			return false
		}
	}
	for _, phrase := range f.DescriptionExcludes {
		if ph := strings.ToLower(strings.TrimSpace(phrase)); ph != "" && strings.Contains(desc, ph) {
			return false
		}
	}
	return true
}

// MatchLocal applies every filter to a stored posting without the site. The
// keyword check is a plain case-insensitive substring over title, team, and
// description, an approximation of the site's search used only offline.
func (f Filters) MatchLocal(p Posting, window time.Duration, now time.Time) bool {
	if q := strings.ToLower(strings.TrimSpace(f.Query)); q != "" {
		// Fields join on a line break so a phrase never straddles two fields.
		hay := strings.ToLower(p.Title)
		for _, v := range []*string{p.JobCategory, p.SubTeam, p.Description} {
			if v != nil {
				hay += "\n" + strings.ToLower(*v)
			}
		}
		if !strings.Contains(hay, q) {
			return false
		}
	}
	if len(f.Countries) > 0 {
		want := map[string]bool{}
		for _, c := range f.Countries {
			if iso, _, ok := ResolveCountry(c); ok {
				want[iso] = true
			}
		}
		hit := false
		for _, l := range p.Locations {
			if l.CountryCode != nil && want[*l.CountryCode] {
				hit = true
			}
		}
		if !hit {
			return false
		}
	}
	if f.Team != "" && (p.JobCategory == nil || !strings.EqualFold(*p.JobCategory, f.Team)) {
		return false
	}
	if f.SubTeam != "" && (p.SubTeam == nil || !strings.EqualFold(*p.SubTeam, f.SubTeam)) {
		return false
	}
	if f.ContractType != "" && (p.ContractType == nil || !strings.EqualFold(*p.ContractType, f.ContractType)) {
		return false
	}
	return f.MatchClient(p, window, now)
}

// SavedSearch is one row of uj_saved_searches. BaselineSize counts the
// postings currently in the search: members not marked removed.
type SavedSearch struct {
	Name           string  `json:"name"`
	Filters        Filters `json:"filters"`
	CreatedAt      string  `json:"created_at"`
	BaselineAt     *string `json:"baseline_at"`
	LastAdvancedAt *string `json:"last_advanced_at"`
	LastCheckedAt  *string `json:"last_checked_at"`
	BaselineSize   int     `json:"baseline_size"`
}

// SaveSearch creates or replaces a saved search. Replacing with different
// filters drops the old membership so new never diffs across changed filters.
func SaveSearch(ctx context.Context, db *sql.DB, name string, f Filters, now time.Time) (*SavedSearch, bool, error) {
	data, err := json.Marshal(f)
	if err != nil {
		return nil, false, err
	}
	prev, err := GetSearch(ctx, db, name)
	if err != nil {
		return nil, false, err
	}
	stamp := now.UTC().Format(time.RFC3339)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	reset := false
	if prev == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO uj_saved_searches (name, filters, created_at) VALUES (?, ?, ?)`, name, string(data), stamp)
	} else {
		old, _ := json.Marshal(prev.Filters)
		if string(old) != string(data) {
			reset = true
			if _, err = tx.ExecContext(ctx, `DELETE FROM uj_saved_search_members WHERE name = ?`, name); err == nil {
				_, err = tx.ExecContext(ctx, `UPDATE uj_saved_searches SET filters = ?, baseline_at = NULL, last_advanced_at = NULL WHERE name = ?`, string(data), name)
			}
		}
	}
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	s, err := GetSearch(ctx, db, name)
	return s, reset, err
}

// GetSearch returns one saved search, or nil when absent.
func GetSearch(ctx context.Context, db Querier, name string) (*SavedSearch, error) {
	var s SavedSearch
	var filters string
	var baseline, advanced, checked sql.NullString
	err := db.QueryRowContext(ctx, `SELECT name, filters, created_at, baseline_at, last_advanced_at, last_checked_at FROM uj_saved_searches WHERE name = ?`, name).
		Scan(&s.Name, &filters, &s.CreatedAt, &baseline, &advanced, &checked)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(filters), &s.Filters); err != nil {
		return nil, err
	}
	s.BaselineAt, s.LastAdvancedAt, s.LastCheckedAt = nullStr(baseline), nullStr(advanced), nullStr(checked)
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM uj_saved_search_members WHERE name = ? AND removed_on IS NULL`, name).Scan(&s.BaselineSize); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSearches returns every saved search, sorted by name.
func ListSearches(ctx context.Context, db *sql.DB) ([]SavedSearch, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM uj_saved_searches ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			_ = rows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	out := make([]SavedSearch, 0, len(names))
	for _, n := range names {
		s, err := GetSearch(ctx, db, n)
		if err != nil {
			return nil, err
		}
		if s != nil {
			out = append(out, *s)
		}
	}
	return out, nil
}

// DeleteSearch removes a saved search and its membership; false when absent.
func DeleteSearch(ctx context.Context, db *sql.DB, name string) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM uj_saved_search_members WHERE name = ?`, name); err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM uj_saved_searches WHERE name = ?`, name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, tx.Commit()
}

// Member is one posting a saved search has contained. Rows are append-only:
// a posting that leaves the search gets removed_on instead of a delete, and
// a posting that comes back clears it.
type Member struct {
	PostingID string  `json:"id"`
	Title     *string `json:"title"`
	FirstSeen string  `json:"first_seen"`
	LastSeen  string  `json:"last_seen"`
	RemovedOn *string `json:"removed_on"`
}

// Members returns every member of a saved search, present or removed.
func Members(ctx context.Context, db *sql.DB, name string) (map[string]Member, error) {
	rows, err := db.QueryContext(ctx, `SELECT posting_id, title, first_seen, last_seen, removed_on FROM uj_saved_search_members WHERE name = ?`, name)
	if err != nil {
		return nil, err
	}
	out := map[string]Member{}
	for rows.Next() {
		var m Member
		var title, removed sql.NullString
		if err := rows.Scan(&m.PostingID, &title, &m.FirstSeen, &m.LastSeen, &removed); err != nil {
			_ = rows.Close()
			return nil, err
		}
		m.Title, m.RemovedOn = nullStr(title), nullStr(removed)
		out[m.PostingID] = m
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// MembershipDiff compares one read of a saved search against its members.
type MembershipDiff struct {
	// Added are postings in the read that are not present members, in read order.
	Added []Posting
	// Reopened marks added ids that were members before and had been removed.
	Reopened map[string]bool
	// Removed are present members missing from the read, sorted by id.
	Removed []Member
}

// DiffMembership diffs a read against the members. Removed is meaningful only
// when the read was complete; callers must not report it otherwise.
func DiffMembership(members map[string]Member, current []Posting) MembershipDiff {
	d := MembershipDiff{Added: []Posting{}, Reopened: map[string]bool{}, Removed: []Member{}}
	seen := make(map[string]bool, len(current))
	for _, p := range current {
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		m, ok := members[p.ID]
		switch {
		case !ok:
			d.Added = append(d.Added, p)
		case m.RemovedOn != nil:
			d.Added = append(d.Added, p)
			d.Reopened[p.ID] = true
		}
	}
	for id, m := range members {
		if m.RemovedOn == nil && !seen[id] {
			d.Removed = append(d.Removed, m)
		}
	}
	sort.Slice(d.Removed, func(i, j int) bool { return d.Removed[i].PostingID < d.Removed[j].PostingID })
	return d
}

// AdvanceMembership records one complete, uncapped read in a single
// transaction: postings in the read become present members (first_seen kept,
// last_seen now, removed_on cleared), and present members missing from the
// read get removed_on now. first stamps baseline_at. Call it only after a read
// that faithfully applied every saved filter; a failed write fails the command.
func AdvanceMembership(ctx context.Context, db *sql.DB, name string, current []Posting, first bool, now time.Time) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	removed, err := AdvanceMembershipTx(ctx, tx, name, current, first, now)
	if err != nil {
		return 0, err
	}
	return removed, tx.Commit()
}

// AdvanceMembershipTx is AdvanceMembership inside the caller's transaction,
// so several searches can advance together or not at all.
func AdvanceMembershipTx(ctx context.Context, tx *sql.Tx, name string, current []Posting, first bool, now time.Time) (int, error) {
	stamp := now.UTC().Format(time.RFC3339)
	ids := make([]string, 0, len(current))
	for _, p := range current {
		ids = append(ids, p.ID)
		if _, err := tx.ExecContext(ctx, `
INSERT INTO uj_saved_search_members (name, posting_id, title, first_seen, last_seen, removed_on)
VALUES (?, ?, ?, ?, ?, NULL)
ON CONFLICT(name, posting_id) DO UPDATE SET title = excluded.title, last_seen = excluded.last_seen, removed_on = NULL`,
			name, p.ID, strPtr(p.Title), stamp, stamp); err != nil {
			return 0, fmt.Errorf("recording member %s: %w", p.ID, err)
		}
	}
	res, err := tx.ExecContext(ctx, `UPDATE uj_saved_search_members SET removed_on = ? WHERE name = ? AND removed_on IS NULL AND posting_id NOT IN (SELECT value FROM json_each(?))`, stamp, name, mustJSON(ids))
	if err != nil {
		return 0, fmt.Errorf("marking removed members: %w", err)
	}
	removed, _ := res.RowsAffected()
	if first {
		_, err = tx.ExecContext(ctx, `UPDATE uj_saved_searches SET baseline_at = ?, last_advanced_at = ?, last_checked_at = ? WHERE name = ?`, stamp, stamp, stamp, name)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE uj_saved_searches SET last_advanced_at = ?, last_checked_at = ? WHERE name = ?`, stamp, stamp, name)
	}
	if err != nil {
		return 0, err
	}
	return int(removed), nil
}

// Querier is a *sql.DB or a *sql.Tx.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Execer is a *sql.DB or a *sql.Tx.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// MarkChecked records a check that did not advance the membership.
func MarkChecked(ctx context.Context, db Execer, name string, now time.Time) error {
	_, err := db.ExecContext(ctx, `UPDATE uj_saved_searches SET last_checked_at = ? WHERE name = ?`, now.UTC().Format(time.RFC3339), name)
	return err
}

func nullStr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the amazon-jobs CLI's live-search and
// local-store commands. Not generated; preserved across `generate --force`.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/store"
)

// searchPath is the single backing endpoint for amazon.jobs. It returns the
// full job record inline (no separate detail endpoint exists).
const searchPath = "/en/search.json"

// dbName keys the local SQLite mirror path shared by every store-backed command.
const dbName = "amazon-jobs-pp-cli"

// Job is the typed view of one amazon.jobs record. Nullable pipeline flags are
// pointers so client-side filters can stay NULL-safe (the API frequently omits
// is_intern / is_manager / university_job).
type Job struct {
	IDIcims                 string `json:"id_icims"`
	ID                      string `json:"id"`
	Title                   string `json:"title"`
	JobPath                 string `json:"job_path"`
	Description             string `json:"description"`
	DescriptionShort        string `json:"description_short"`
	BasicQualifications     string `json:"basic_qualifications"`
	PreferredQualifications string `json:"preferred_qualifications"`
	JobCategory             string `json:"job_category"`
	BusinessCategory        string `json:"business_category"`
	JobFamily               string `json:"job_family"`
	Location                string `json:"location"`
	City                    string `json:"city"`
	State                   string `json:"state"`
	CountryCode             string `json:"country_code"`
	NormalizedLocation      string `json:"normalized_location"`
	CompanyName             string `json:"company_name"`
	JobScheduleType         string `json:"job_schedule_type"`
	IsIntern                *bool  `json:"is_intern"`
	IsManager               *bool  `json:"is_manager"`
	UniversityJob           *bool  `json:"university_job"`
	PostedDate              string `json:"posted_date"`
	UpdatedTime             string `json:"updated_time"`
	Team                    struct {
		Label string `json:"label"`
	} `json:"team"`

	// UpdatedDiverged is computed by this CLI, not returned by the API. It is
	// true when `updated_time` is dramatically fresher than `posted_date`,
	// meaning the row was re-indexed or edited rather than newly posted. It is
	// omitted when false so the common case leaves the payload unchanged.
	// See updatedDiverged in amazonjobs_dates.go for why this matters.
	UpdatedDiverged bool `json:"updated_diverged,omitempty"`
}

// applyURL is the canonical human-facing listing URL for a job.
func (j Job) applyURL() string {
	if j.JobPath == "" {
		return ""
	}
	return "https://www.amazon.jobs" + j.JobPath
}

// buildSearchValues assembles the confirmed server-side query. The location
// filters use the bracketed wire keys the API requires; result_limit is forced
// >= 1 because the API returns 0 hits when it is 0 combined with a filter.
func buildSearchValues(query, country, state, city, sort string, limit, offset int) url.Values {
	v := url.Values{}
	v.Set("base_query", query)
	if country != "" {
		v.Set("normalized_country_code[]", country)
	}
	if state != "" {
		v.Set("normalized_state_name[]", state)
	}
	if city != "" {
		v.Set("normalized_city_name[]", city)
	}
	if sort == "" {
		sort = "recent"
	}
	v.Set("sort", sort)
	if limit < 1 {
		limit = 20
	}
	v.Set("result_limit", strconv.Itoa(limit))
	if offset < 0 {
		offset = 0
	}
	v.Set("offset", strconv.Itoa(offset))
	return v
}

// searchPage fetches one page of results, returning the total hit count and the
// raw job objects (kept raw so callers can both typed-parse and upsert them).
func searchPage(ctx context.Context, c *client.Client, values url.Values) (int, []json.RawMessage, error) {
	data, err := c.GetWithHeadersValues(ctx, searchPath, values, nil)
	if err != nil {
		return 0, nil, err
	}
	var resp struct {
		Hits int               `json:"hits"`
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, nil, fmt.Errorf("parsing search response: %w", err)
	}
	return resp.Hits, resp.Jobs, nil
}

// parseJob decodes one raw job record into the typed view.
func parseJob(raw json.RawMessage) (Job, error) {
	var j Job
	if err := json.Unmarshal(raw, &j); err != nil {
		return Job{}, err
	}
	return j, nil
}

// boolFlag returns the *bool a client-side filter should use: nil when the flag
// was not set, otherwise the requested value.
func boolFlag(changed bool, val bool) *bool {
	if !changed {
		return nil
	}
	return &val
}

// effectiveBool treats a null API flag as false (Amazon leaves is_manager /
// is_intern null for most roles that are not managers / interns).
func effectiveBool(p *bool) bool { return p != nil && *p }

// clientFilters bundles the facets the .json endpoint ignores as server params
// and must therefore be applied locally while scanning pages.
//
// This is a struct rather than a positional parameter list because the set has
// grown past the point where eight positional arguments stay readable at the
// call site.
type clientFilters struct {
	category   string
	schedule   string
	intern     *bool
	manager    *bool
	university *bool

	// postedCutoff is the inclusive date floor from --posted-within. Zero when
	// the flag was not set.
	postedCutoff time.Time
	// postedWithinRaw is the user's verbatim flag value, kept only so
	// --dry-run can echo what they typed rather than a normalized duration.
	postedWithinRaw string

	// descContains and descExcludes are compiled from --description-contains
	// and --description-not-contains. Nil when the flag was not set.
	descContains *regexp.Regexp
	descExcludes *regexp.Regexp
}

// active reports whether any client-side filter is set.
func (f clientFilters) active() bool {
	return f.category != "" || f.schedule != "" ||
		f.intern != nil || f.manager != nil || f.university != nil ||
		!f.postedCutoff.IsZero() || f.descContains != nil || f.descExcludes != nil
}

// descriptionHaystack concatenates the free-text fields --description-contains
// searches, with HTML stripped.
//
// Stripping matters: the raw fields are HTML fragments, so a pattern spanning a
// <br/> or wrapped in <b> tags would miss against the raw text. find applies
// cleanJob only to rows that already survived filtering, so the strip has to
// happen here too rather than relying on the caller.
func descriptionHaystack(j Job) string {
	return strings.Join([]string{
		plainText(j.Description),
		plainText(j.BasicQualifications),
		plainText(j.PreferredQualifications),
	}, "\n")
}

// matches applies every active filter. All predicates are NULL-safe.
func (f clientFilters) matches(j Job) bool {
	if f.category != "" {
		cat := strings.ToLower(f.category)
		if !strings.Contains(strings.ToLower(j.JobCategory), cat) &&
			!strings.Contains(strings.ToLower(j.BusinessCategory), cat) {
			return false
		}
	}
	if f.schedule != "" && !strings.EqualFold(strings.TrimSpace(j.JobScheduleType), strings.TrimSpace(f.schedule)) {
		return false
	}
	if f.intern != nil && effectiveBool(j.IsIntern) != *f.intern {
		return false
	}
	if f.manager != nil && effectiveBool(j.IsManager) != *f.manager {
		return false
	}
	if f.university != nil && effectiveBool(j.UniversityJob) != *f.university {
		return false
	}
	if !f.postedCutoff.IsZero() {
		posted, ok := parsePostedDate(j.PostedDate)
		// A row whose posted_date is missing or unparseable cannot be shown to
		// be inside the window, so an explicit recency filter excludes it. The
		// alternative -- passing it through -- would quietly reintroduce the
		// stale reqs the flag exists to remove.
		if !ok || posted.Before(f.postedCutoff) {
			return false
		}
	}
	if f.descContains != nil || f.descExcludes != nil {
		hay := descriptionHaystack(j)
		if f.descContains != nil && !f.descContains.MatchString(hay) {
			return false
		}
		if f.descExcludes != nil && f.descExcludes.MatchString(hay) {
			return false
		}
	}
	return true
}

// describe renders the active client-side filters as a short human-readable
// list. Used by `find --dry-run` so the preview names the filters that run
// after the request, which the query string cannot show.
func (f clientFilters) describe() string {
	parts := []string{}
	if f.category != "" {
		parts = append(parts, fmt.Sprintf("category~%q", f.category))
	}
	if f.schedule != "" {
		parts = append(parts, fmt.Sprintf("schedule~%q", f.schedule))
	}
	for _, b := range []struct {
		name string
		val  *bool
	}{
		{"intern", f.intern},
		{"manager", f.manager},
		{"university", f.university},
	} {
		if b.val != nil {
			parts = append(parts, fmt.Sprintf("%s=%t", b.name, *b.val))
		}
	}
	if !f.postedCutoff.IsZero() {
		parts = append(parts, fmt.Sprintf("posted-within=%s (on or after %s)",
			f.postedWithinRaw, f.postedCutoff.Format(postedDateLayout)))
	}
	if f.descContains != nil {
		parts = append(parts, fmt.Sprintf("description~%s", f.descContains))
	}
	if f.descExcludes != nil {
		parts = append(parts, fmt.Sprintf("description!~%s", f.descExcludes))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

// compileDescriptionPattern compiles a --description-contains style pattern as
// a case-insensitive regular expression.
//
// Patterns that are not valid regex syntax fall back to a literal match instead
// of erroring. Real queries here are things like "C++", "Relocation assistance
// is NOT provided", or "N1" -- ordinary prose that happens to contain regex
// metacharacters. Failing those with a syntax error would make the common case
// the broken one, so an unparseable pattern is treated as the literal text the
// user typed.
func compileDescriptionPattern(pattern string) (*regexp.Regexp, error) {
	if strings.TrimSpace(pattern) == "" {
		return nil, nil
	}
	if re, err := regexp.Compile("(?i)" + pattern); err == nil {
		return re, nil
	}
	return regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
}

var (
	htmlBrRe   = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlLiRe   = regexp.MustCompile(`(?i)<li\s*/?>`)
	htmlTagRe  = regexp.MustCompile(`<[^>]+>`)
	blankLines = regexp.MustCompile(`\n{3,}`)
)

// plainText converts amazon.jobs HTML fragments into readable plain text:
// <br/> and <li> become line breaks, remaining tags are dropped, and HTML
// entities are decoded via cliutil.CleanText. Amazon descriptions use <br/>
// heavily, which cliutil.CleanText alone leaves intact.
func plainText(s string) string {
	if s == "" {
		return ""
	}
	s = htmlBrRe.ReplaceAllString(s, "\n")
	s = htmlLiRe.ReplaceAllString(s, "\n- ")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = cliutil.CleanText(s) // decode entities + trim
	s = blankLines.ReplaceAllString(s, "\n\n")
	return s
}

// cleanJob returns a copy of the job with HTML converted to plain text in the
// free-text fields, for --plain / human rendering.
func cleanJob(j Job) Job {
	j.Description = plainText(j.Description)
	j.DescriptionShort = plainText(j.DescriptionShort)
	j.BasicQualifications = plainText(j.BasicQualifications)
	j.PreferredQualifications = plainText(j.PreferredQualifications)
	j.Title = plainText(j.Title)
	return j
}

// resolveDBPath resolves the local mirror path, honoring an explicit --db flag.
func resolveDBPath(dbFlag string) string {
	if strings.TrimSpace(dbFlag) != "" {
		return dbFlag
	}
	return defaultDBPath(dbName)
}

// emitResult writes v as machine JSON when the caller asked for a machine
// format (or stdout is piped), otherwise runs humanFn for a terminal render.
// printJSONFiltered routes through the shared output pipeline so --select,
// --compact, --csv, and --quiet all work for free.
func emitResult(cmd *cobra.Command, flags *rootFlags, v any, humanFn func(w io.Writer)) error {
	out := cmd.OutOrStdout()
	machine := flags.asJSON || flags.agent ||
		(!isTerminal(out) && !flags.plain && !flags.csv && !flags.quiet)
	if machine {
		return printJSONFiltered(out, v, flags)
	}
	humanFn(out)
	return nil
}

// emitLiveResult is emitResult for commands that fetched from the live API
// (pp:data-source live). It tags the --agent envelope meta.source="live" so
// agents can distinguish fresh API data from local-store reads, matching the
// provenance convention the endpoint-mirror commands already use (see
// promoted_postings). Without this, live results fall through to the shared
// default of meta.source="local" and get mislabeled as cache reads.
func emitLiveResult(cmd *cobra.Command, flags *rootFlags, v any, humanFn func(w io.Writer)) error {
	out := cmd.OutOrStdout()
	machine := flags.asJSON || flags.agent ||
		(!isTerminal(out) && !flags.plain && !flags.csv && !flags.quiet)
	if machine {
		raw, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return printOutputWithFlagsMeta(out, json.RawMessage(raw), flags, map[string]any{"source": "live"})
	}
	humanFn(out)
	return nil
}

// SavedSearch is a persisted named query plus its diff cursor.
type SavedSearch struct {
	Name       string   `json:"name"`
	Query      string   `json:"query"`
	Country    string   `json:"country,omitempty"`
	State      string   `json:"state,omitempty"`
	City       string   `json:"city,omitempty"`
	Sort       string   `json:"sort,omitempty"`
	LastSeen   []string `json:"last_seen_ids,omitempty"`
	CreatedAt  string   `json:"created_at,omitempty"`
	UpdatedAt  string   `json:"updated_at,omitempty"`
	LastSynced string   `json:"last_synced,omitempty"`
}

// ensureSavedSearches lazily creates the saved-search table. Hand-authored so
// it survives regen; kept out of the generated store package on purpose.
func ensureSavedSearches(ctx context.Context, db *store.Store) error {
	_, err := db.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS saved_searches (
        name        TEXT PRIMARY KEY,
        query       TEXT NOT NULL DEFAULT '',
        country     TEXT NOT NULL DEFAULT '',
        state       TEXT NOT NULL DEFAULT '',
        city        TEXT NOT NULL DEFAULT '',
        sort        TEXT NOT NULL DEFAULT 'recent',
        last_seen   TEXT NOT NULL DEFAULT '[]',
        created_at  TEXT NOT NULL DEFAULT '',
        updated_at  TEXT NOT NULL DEFAULT '',
        last_synced TEXT NOT NULL DEFAULT ''
    )`)
	if err != nil {
		return fmt.Errorf("creating saved_searches table: %w", err)
	}
	return nil
}

// upsertSavedSearch inserts or updates a saved search, preserving last_seen and
// created_at when the row already exists.
func upsertSavedSearch(ctx context.Context, db *store.Store, s SavedSearch, nowISO string) error {
	if err := ensureSavedSearches(ctx, db); err != nil {
		return err
	}
	_, err := db.DB().ExecContext(ctx, `
        INSERT INTO saved_searches (name, query, country, state, city, sort, last_seen, created_at, updated_at, last_synced)
        VALUES (?, ?, ?, ?, ?, ?, '[]', ?, ?, '')
        ON CONFLICT(name) DO UPDATE SET
            query = excluded.query,
            country = excluded.country,
            state = excluded.state,
            city = excluded.city,
            sort = excluded.sort,
            updated_at = excluded.updated_at`,
		s.Name, s.Query, s.Country, s.State, s.City, s.Sort, nowISO, nowISO)
	if err != nil {
		return fmt.Errorf("saving search %q: %w", s.Name, err)
	}
	return nil
}

// getSavedSearch loads one saved search. Returns (nil, nil) when absent.
func getSavedSearch(ctx context.Context, db *store.Store, name string) (*SavedSearch, error) {
	if err := ensureSavedSearches(ctx, db); err != nil {
		return nil, err
	}
	var s SavedSearch
	var lastSeen sql.NullString
	row := db.DB().QueryRowContext(ctx, `
        SELECT name, query, country, state, city, sort, last_seen, created_at, updated_at, last_synced
        FROM saved_searches WHERE name = ?`, name)
	err := row.Scan(&s.Name, &s.Query, &s.Country, &s.State, &s.City, &s.Sort,
		&lastSeen, &s.CreatedAt, &s.UpdatedAt, &s.LastSynced)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading saved search %q: %w", name, err)
	}
	if lastSeen.Valid && lastSeen.String != "" {
		_ = json.Unmarshal([]byte(lastSeen.String), &s.LastSeen)
	}
	return &s, nil
}

// listSavedSearches returns all saved searches ordered by name.
func listSavedSearches(ctx context.Context, db *store.Store) ([]SavedSearch, error) {
	if err := ensureSavedSearches(ctx, db); err != nil {
		return nil, err
	}
	rows, err := db.DB().QueryContext(ctx, `
        SELECT name, query, country, state, city, sort, last_seen, created_at, updated_at, last_synced
        FROM saved_searches ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing saved searches: %w", err)
	}
	defer rows.Close()
	out := make([]SavedSearch, 0)
	for rows.Next() {
		var s SavedSearch
		var lastSeen sql.NullString
		if err := rows.Scan(&s.Name, &s.Query, &s.Country, &s.State, &s.City, &s.Sort,
			&lastSeen, &s.CreatedAt, &s.UpdatedAt, &s.LastSynced); err != nil {
			return nil, fmt.Errorf("scanning saved search: %w", err)
		}
		if lastSeen.Valid && lastSeen.String != "" {
			_ = json.Unmarshal([]byte(lastSeen.String), &s.LastSeen)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating saved searches: %w", err)
	}
	return out, nil
}

// deleteSavedSearch removes a saved search. Returns whether a row was removed.
func deleteSavedSearch(ctx context.Context, db *store.Store, name string) (bool, error) {
	if err := ensureSavedSearches(ctx, db); err != nil {
		return false, err
	}
	res, err := db.DB().ExecContext(ctx, `DELETE FROM saved_searches WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("deleting saved search %q: %w", name, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// updateSavedSeen persists the current known id set and last-synced time.
func updateSavedSeen(ctx context.Context, db *store.Store, name string, ids []string, nowISO string) error {
	blob, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = db.DB().ExecContext(ctx, `
        UPDATE saved_searches SET last_seen = ?, last_synced = ? WHERE name = ?`,
		string(blob), nowISO, name)
	if err != nil {
		return fmt.Errorf("updating saved search cursor %q: %w", name, err)
	}
	return nil
}

// nowISO returns an RFC3339 UTC timestamp. Wrapped so callers read cleanly.
func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }

// guardDataSource rejects a --data-source value a command cannot honor, per the
// per-command `// pp:data-source` contract.
func guardDataSource(flags *rootFlags, strategy string) error {
	ds := strings.ToLower(strings.TrimSpace(flags.dataSource))
	switch strategy {
	case "live":
		if ds == "local" {
			return usageErr(fmt.Errorf("this command has no local data source; --data-source local is not supported"))
		}
	case "local":
		if ds == "live" {
			return usageErr(fmt.Errorf("this command has no live equivalent; --data-source live is not supported"))
		}
	}
	return nil
}

// storeMissing reports whether the local mirror file is absent, printing a
// sync hint and an empty machine result. Store-reading commands call this after
// dryRunOK and before opening the store.
func storeMissing(cmd *cobra.Command, flags *rootFlags, dbPath string) bool {
	if _, err := os.Stat(dbPath); err == nil {
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: amazon-jobs-pp-cli sync --max-pages 5 --db %s\n", dbPath, dbPath)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return true
}

// Copyright 2026 muhammad-khan. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: shared helpers for the ZAZU-aware novel commands.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/buildalert/internal/client"

	_ "modernc.org/sqlite"
)

// buildAlertApp mirrors the subset of /dapi/leads/live-leads .data[i].application
// fields the ZAZU joins need.
type buildAlertApp struct {
	ID                      string  `json:"id"`
	CouncilIdentifier       string  `json:"councilIdentifier"`
	CountyIdentifier        string  `json:"countyIdentifier"`
	FullDescription         string  `json:"fullDescription"`
	URL                     string  `json:"url"`
	InternalUniqueReference string  `json:"internalUniqueReference"`
	Reference               string  `json:"reference"`
	Address                 string  `json:"address"`
	PostCode                string  `json:"postCode"`
	Status                  string  `json:"status"`
	Latitude                float64 `json:"latitude"`
	Longitude               float64 `json:"longitude"`
	DistanceAway            float64 `json:"distanceAway"`
	EstimationValueBand     string  `json:"estimationValueBand"`
	CanSendLetter           bool    `json:"canSendLetter"`
	LetterBeenSent          bool    `json:"letterBeenSent"`
}

type buildAlertLead struct {
	ID           string         `json:"id"`
	AIMatchScore *int           `json:"aiMatchScore"`
	Date         int64          `json:"date"`
	State        int            `json:"state"`
	Read         bool           `json:"read"`
	IsNew        bool           `json:"isNew"`
	Application  buildAlertApp  `json:"application"`
	Raw          map[string]any `json:"-"`
}

type liveLeadsPage struct {
	TotalItems   int               `json:"totalItems"`
	ItemsPerPage int               `json:"itemsPerPage"`
	PageCount    int               `json:"pageCount"`
	CurrentPage  int               `json:"currentPage"`
	Data         []json.RawMessage `json:"data"`
}

// fetchAllLeads pulls every page of /dapi/leads/live-leads via the BuildAlert
// HTTP client. Returns the parsed envelopes plus the raw rows so callers can
// pass the full payload to filterFields/printOutputWithFlags. Pagination is
// honored; the call respects the global --rate-limit via the Client.
func fetchAllLeads(ctx context.Context, c *client.Client, projectTypes, states string, minValue int) ([]buildAlertLead, []json.RawMessage, error) {
	out := []buildAlertLead{}
	rawOut := []json.RawMessage{}
	page := 1
	for {
		params := map[string]string{
			"page":         fmt.Sprintf("%d", page),
			"itemsPerPage": "50",
			"orderBy":      "createdDate",
			"force":        "",
		}
		if states == "" {
			params["states"] = "-1"
		} else {
			params["states"] = states
		}
		if projectTypes != "" {
			params["projectTypes"] = projectTypes
		}
		if minValue > 0 {
			params["minValue"] = fmt.Sprintf("%d", minValue)
		}
		raw, err := c.Get(ctx, "/dapi/leads/live-leads", params)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching leads page %d: %w", page, err)
		}
		var p liveLeadsPage
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, nil, fmt.Errorf("decoding leads page %d: %w", page, err)
		}
		for _, item := range p.Data {
			var lead buildAlertLead
			if err := json.Unmarshal(item, &lead); err != nil {
				continue
			}
			out = append(out, lead)
			rawOut = append(rawOut, item)
		}
		if page >= p.PageCount || len(p.Data) == 0 {
			break
		}
		page++
		if page > 100 {
			break
		}
	}
	return out, rawOut, nil
}

// openZazuDB opens the user's ZAZU bd-mirror.sqlite in read-only mode.
func openZazuDB(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("--zazu-db is required: path to ZAZU bd-mirror.sqlite (or comma-separated list)")
	}
	// Expand leading ~/ to the user's home directory.
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		}
	} else if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("zazu-db %q: %w", path, err)
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", strings.ReplaceAll(path, "\\", "/"))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening zazu-db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging zazu-db: %w", err)
	}
	return db, nil
}

// openZazuDBs opens one or more ZAZU SQLite mirrors. The path argument may be
// a single path or a comma-separated list. Common setup: harrow-mirror.sqlite
// (set by ZAZU_DB_PATH and used by the bot for Harrow leads) plus
// bd-mirror.sqlite (legacy B&D sheets — Residential/Commercial/Brent Commercial/
// Brent Residential). The buildalert ZAZU-aware commands treat the union as
// "ZAZU has this", so a lead present in either mirror is not flagged missing.
//
// Each returned DB must be Closed by the caller.
func openZazuDBs(pathSpec string) ([]*sql.DB, error) {
	if pathSpec == "" {
		return nil, errors.New("--zazu-db is required: path to ZAZU SQLite mirror (or comma-separated list of mirrors)")
	}
	paths := []string{}
	for _, p := range strings.Split(pathSpec, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) == 0 {
		return nil, errors.New("--zazu-db: no valid paths after parsing")
	}
	dbs := make([]*sql.DB, 0, len(paths))
	for _, p := range paths {
		db, err := openZazuDB(p)
		if err != nil {
			// Close any successfully opened DBs before returning the error.
			for _, opened := range dbs {
				opened.Close()
			}
			return nil, err
		}
		dbs = append(dbs, db)
	}
	return dbs, nil
}

// closeAll closes every DB in the slice; errors are swallowed (read-only
// connections can fail to close cleanly on Windows if Sleep handles are
// still draining).
func closeAll(dbs []*sql.DB) {
	for _, db := range dbs {
		_ = db.Close()
	}
}

// councilMatchesSheet returns true when the BuildAlert councilIdentifier
// matches a ZAZU `sheet` name. ZAZU's sheet naming is project-specific:
//   - harrow-mirror.sqlite uses `Harrow Residential` / `Harrow Commercial`
//   - bd-mirror.sqlite uses `Brent Residential` / `Brent Commercial` plus
//     bare `Residential` / `Commercial` (the latter are also Harrow-era
//     legacy sheets).
//
// The match rule: case-insensitive prefix on the council name, ignoring
// category suffix words (Residential, Commercial). Bare sheet names like
// `Residential` and `Commercial` are intentionally NOT matched against
// any specific council — they're legacy and would over-match.
func councilMatchesSheet(council, sheet string) bool {
	c := strings.ToLower(strings.TrimSpace(council))
	s := strings.ToLower(strings.TrimSpace(sheet))
	if c == "" || s == "" {
		return false
	}
	if c == s {
		return true
	}
	// Match "<council> residential" / "<council> commercial" / "<council> <anything>"
	if strings.HasPrefix(s, c+" ") {
		return true
	}
	return false
}

// zazuApplicationKeys returns the union of (council, reference) tuples
// across one or more ZAZU mirrors. Each mirror's `sheet` column is normalized
// against the BuildAlert councilIdentifier set via councilMatchesSheet — so a
// row with sheet `Harrow Residential` matches BuildAlert leads where
// councilIdentifier=`harrow`, and a row with sheet `Brent Commercial` matches
// councilIdentifier=`brent`. Sheets that don't match a known prefix (legacy
// bare `Residential`/`Commercial`) are stored under their literal sheet name
// for backward-compat with single-prefix joins.
//
// `councils` is the set of BuildAlert councilIdentifiers present in the
// current lead pull. Provide this so the join can map ZAZU sheets to
// council slugs without enumerating every possible council.
func zazuApplicationKeys(ctx context.Context, dbs []*sql.DB, councils []string) (map[string]struct{}, error) {
	out := make(map[string]struct{}, 1024)
	for _, db := range dbs {
		rows, err := db.QueryContext(ctx, `SELECT sheet, reference FROM applications`)
		if err != nil {
			return nil, fmt.Errorf("querying zazu applications: %w", err)
		}
		for rows.Next() {
			var sheet, ref sql.NullString
			if err := rows.Scan(&sheet, &ref); err != nil {
				continue
			}
			if !ref.Valid || ref.String == "" {
				continue
			}
			// Always add the literal-sheet key for backward-compat.
			out[zazuKey(sheet.String, ref.String)] = struct{}{}
			// Also add an entry under every matching council in the pull set,
			// so BuildAlert councilIdentifier=`harrow` can match a row whose
			// sheet is `Harrow Residential`.
			for _, c := range councils {
				if councilMatchesSheet(c, sheet.String) {
					out[zazuKey(c, ref.String)] = struct{}{}
				}
			}
		}
		rows.Close()
	}
	return out, nil
}

// zazuLetterReferences returns the union of references across ZAZU's
// letters_sent logs in each mirror. The letters_sent table is keyed by
// reference (no sheet column), so the join key is reference-only.
func zazuLetterReferences(ctx context.Context, dbs []*sql.DB) (map[string]struct{}, error) {
	out := make(map[string]struct{}, 256)
	for _, db := range dbs {
		rows, err := db.QueryContext(ctx, `SELECT DISTINCT reference FROM letters_sent`)
		if err != nil {
			// letters_sent may not exist in every mirror; that's OK — skip.
			continue
		}
		for rows.Next() {
			var ref sql.NullString
			if err := rows.Scan(&ref); err != nil {
				continue
			}
			if ref.Valid && ref.String != "" {
				out[ref.String] = struct{}{}
			}
		}
		rows.Close()
	}
	return out, nil
}

// zazuApplicationCountsByCouncil returns council_slug -> application count
// across one or more ZAZU mirrors. Sheets are folded onto councils via
// councilMatchesSheet for every council in the provided set. Sheets that
// match no council are emitted under their literal (lowercased) sheet name
// so the caller can still surface them in coverage output.
func zazuApplicationCountsByCouncil(ctx context.Context, dbs []*sql.DB, councils []string) (map[string]int, error) {
	out := make(map[string]int, 16)
	for _, db := range dbs {
		rows, err := db.QueryContext(ctx, `SELECT sheet, COUNT(*) FROM applications GROUP BY sheet`)
		if err != nil {
			return nil, fmt.Errorf("querying zazu council counts: %w", err)
		}
		for rows.Next() {
			var sheet sql.NullString
			var n int
			if err := rows.Scan(&sheet, &n); err != nil {
				continue
			}
			matched := false
			for _, c := range councils {
				if councilMatchesSheet(c, sheet.String) {
					out[strings.ToLower(c)] += n
					matched = true
				}
			}
			if !matched {
				out[strings.ToLower(strings.TrimSpace(sheet.String))] += n
			}
		}
		rows.Close()
	}
	return out, nil
}

// zazuKey normalizes (council, reference) into the join key used by all
// ZAZU-aware commands. BuildAlert's councilIdentifier (e.g. "hillingdon") maps
// 1:1 to ZAZU's `sheet` column.
func zazuKey(council, reference string) string {
	return strings.ToLower(strings.TrimSpace(council)) + "::" + strings.TrimSpace(reference)
}

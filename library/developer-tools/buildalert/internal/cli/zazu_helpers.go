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
		return nil, errors.New("--zazu-db is required: path to ZAZU bd-mirror.sqlite")
	}
	// Expand leading ~/ to the user's home directory. Go's os package does not
	// expand tildes (unlike most shells); without this, documented examples
	// like `--zazu-db ~/Downloads/Zazu/bd-mirror.sqlite` fail on Windows with
	// "The system cannot find the path specified." because the literal "~"
	// is treated as a directory name. Match POSIX shell semantics: only "~/"
	// and bare "~" are expanded — "~user" is not handled (rare on Windows).
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

// zazuApplicationKeys returns the set of (sheet, reference) tuples present in
// the ZAZU applications table.
func zazuApplicationKeys(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT sheet, reference FROM applications`)
	if err != nil {
		return nil, fmt.Errorf("querying zazu applications: %w", err)
	}
	defer rows.Close()
	out := make(map[string]struct{}, 1024)
	for rows.Next() {
		var sheet, ref sql.NullString
		if err := rows.Scan(&sheet, &ref); err != nil {
			continue
		}
		out[zazuKey(sheet.String, ref.String)] = struct{}{}
	}
	return out, rows.Err()
}

// zazuLetterReferences returns the set of references present in ZAZU's
// letters_sent log. ZAZU's letters_sent table is keyed by reference (no sheet
// column), so the join key is reference-only.
func zazuLetterReferences(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT DISTINCT reference FROM letters_sent`)
	if err != nil {
		return nil, fmt.Errorf("querying zazu letters_sent: %w", err)
	}
	defer rows.Close()
	out := make(map[string]struct{}, 256)
	for rows.Next() {
		var ref sql.NullString
		if err := rows.Scan(&ref); err != nil {
			continue
		}
		if ref.Valid {
			out[ref.String] = struct{}{}
		}
	}
	return out, rows.Err()
}

// zazuApplicationCountsByCouncil returns council_slug -> application count from ZAZU.
func zazuApplicationCountsByCouncil(ctx context.Context, db *sql.DB) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `SELECT sheet, COUNT(*) FROM applications GROUP BY sheet`)
	if err != nil {
		return nil, fmt.Errorf("querying zazu council counts: %w", err)
	}
	defer rows.Close()
	out := make(map[string]int, 16)
	for rows.Next() {
		var sheet sql.NullString
		var n int
		if err := rows.Scan(&sheet, &n); err != nil {
			continue
		}
		out[sheet.String] = n
	}
	return out, rows.Err()
}

// zazuKey normalizes (council, reference) into the join key used by all
// ZAZU-aware commands. BuildAlert's councilIdentifier (e.g. "hillingdon") maps
// 1:1 to ZAZU's `sheet` column.
func zazuKey(council, reference string) string {
	return strings.ToLower(strings.TrimSpace(council)) + "::" + strings.TrimSpace(reference)
}
